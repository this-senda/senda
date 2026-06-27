// Package httpclient builds and sends HTTP requests, capturing timing, size
// and applying the large-response truncation policy.
package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"senda/internal/auth"
	"senda/internal/buildinfo"
	"senda/internal/model"
	"senda/internal/vars"
)

// MaxInlineBytes is the largest response body streamed in full to the
// frontend. Beyond this the body is truncated and flagged; the UI offers a
// "view raw / save" escape hatch. Keeps the webview from choking on huge
// payloads (architecture §5).
const MaxInlineBytes = 2 << 20 // 2 MiB

// DefaultUserAgent identifies Senda to servers, mirroring how Postman sends
// "PostmanRuntime/<version>". Without this, Go's stdlib leaks
// "Go-http-client/2.0". Applied only when the request has no User-Agent header.
// buildinfo.Version is "dev" unless injected via -ldflags at release time.
var DefaultUserAgent = "SendaRuntime/" + buildinfo.Version

// Client sends requests. Wraps a configured *http.Client with a session
// cookie jar (cookies persist across sends until ClearCookies).
type Client struct {
	hc  *http.Client
	jar *cookiejar.Jar
	mu  sync.Mutex // guards the transport swap in Configure
	net NetConfig  // last-applied transport config; see Configure
}

// NetConfig is the resolved (post-{{var}}) network configuration for a
// collection: an optional proxy URL and TLS client-cert / CA / insecure
// settings. The zero value means "Go defaults" (env proxy, system roots).
// All fields are comparable so Configure can skip rebuilding an unchanged
// transport — preserving the connection pool across sends.
type NetConfig struct {
	Proxy    string
	CertFile string
	KeyFile  string
	CAFile   string
	Insecure bool
}

// New returns a Client with a sane default timeout and a fresh cookie jar.
func New() *Client {
	jar, _ := cookiejar.New(nil) // only errors on bad options; nil is fine
	return &Client{hc: &http.Client{Timeout: 60 * time.Second, Jar: jar}, jar: jar}
}

// Configure applies a collection's proxy/TLS settings to the transport. It is
// a no-op when cfg matches the last-applied config (so the pooled connections
// survive repeated sends). The cookie jar lives on the *http.Client and is
// untouched by transport swaps.
func (c *Client) Configure(cfg NetConfig) error {
	// Lock so concurrent senders sharing one session (a flow's parallel nodes)
	// can't race on the transport swap. cfg is constant per collection, so after
	// the first call this is the cheap cfg==c.net no-op, and the mutex orders
	// that single swap happens-before every subsequent c.hc.Do.
	c.mu.Lock()
	defer c.mu.Unlock()
	if cfg == c.net {
		return nil
	}
	tr, err := buildTransport(cfg)
	if err != nil {
		return err
	}
	if tr == nil {
		c.hc.Transport = nil // untyped nil => http.DefaultTransport (avoid typed-nil interface)
	} else {
		c.hc.Transport = tr
	}
	c.net = cfg
	return nil
}

// buildTransport returns a transport for cfg, or nil for the zero config so
// the client falls back to http.DefaultTransport.
func buildTransport(cfg NetConfig) (*http.Transport, error) {
	if cfg == (NetConfig{}) {
		return nil, nil
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.Proxy != "" {
		u, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", cfg.Proxy, err)
		}
		tr.Proxy = http.ProxyURL(u)
	}
	tlsCfg := &tls.Config{InsecureSkipVerify: cfg.Insecure} // gated behind explicit insecure:true
	if cfg.CertFile != "" || cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("CA file %q: no certificates found", cfg.CAFile)
		}
		tlsCfg.RootCAs = pool
	}
	tr.TLSClientConfig = tlsCfg
	return tr, nil
}

// Cookies returns the jar's cookies that would be sent to rawURL.
func (c *Client) Cookies(rawURL string) ([]model.Cookie, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	var out []model.Cookie
	for _, ck := range c.jar.Cookies(u) {
		out = append(out, model.Cookie{Name: ck.Name, Value: ck.Value})
	}
	return out, nil
}

// ClearCookies discards every cookie by swapping in a fresh jar.
func (c *Client) ClearCookies() {
	jar, _ := cookiejar.New(nil)
	c.jar = jar
	c.hc.Jar = jar
}

// Send resolves variables, builds the request, sends it and returns a
// model.Response. Transport-level failures are returned in Response.Error
// rather than as a Go error, so the UI can render them uniformly.
func (c *Client) Send(ctx context.Context, req model.Request, scope *vars.Scope) model.Response {
	built, err := build(ctx, req, scope)
	if err != nil {
		return model.Response{Error: err.Error()}
	}
	if err := auth.Apply(ctx, c.hc, built, req.Auth, scope); err != nil {
		return model.Response{Error: err.Error()}
	}

	tl := &tracer{}
	built = built.WithContext(httptrace.WithClientTrace(built.Context(), tl.trace()))

	start := time.Now()
	resp, err := c.hc.Do(built)
	if err != nil {
		return model.Response{
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	body, truncated, size := readBody(resp.Body)
	dur := time.Since(start).Milliseconds()

	return model.Response{
		Status:     resp.StatusCode,
		StatusText: http.StatusText(resp.StatusCode),
		DurationMs: dur,
		SizeBytes:  size,
		Headers:    resp.Header,
		Body:       prettyIfJSON(resp.Header.Get("Content-Type"), body),
		Truncated:  truncated,
		Timing:     tl.timing(start, time.Now()),
	}
}

// tracer collects httptrace phase timestamps for one send. Redirected
// requests overwrite earlier phases, so the timing reflects the final hop.
//
// httptrace callbacks may fire on background dial goroutines that outlive Do
// (e.g. a speculative dial when the request actually got a pooled connection),
// so the writes here can race with timing()'s reads once sends run concurrently
// on a shared client (a flow's parallel node). mu guards every field.
type tracer struct {
	mu                        sync.Mutex
	dnsStart, dnsDone         time.Time
	connectStart, connectDone time.Time
	tlsStart, tlsDone         time.Time
	firstByte                 time.Time
	reused                    bool
}

func (t *tracer) trace() *httptrace.ClientTrace {
	set := func(f func()) { t.mu.Lock(); f(); t.mu.Unlock() }
	return &httptrace.ClientTrace{
		DNSStart:          func(httptrace.DNSStartInfo) { set(func() { t.dnsStart = time.Now() }) },
		DNSDone:           func(httptrace.DNSDoneInfo) { set(func() { t.dnsDone = time.Now() }) },
		ConnectStart:      func(string, string) { set(func() { t.connectStart = time.Now() }) },
		ConnectDone:       func(string, string, error) { set(func() { t.connectDone = time.Now() }) },
		TLSHandshakeStart: func() { set(func() { t.tlsStart = time.Now() }) },
		TLSHandshakeDone:  func(tls.ConnectionState, error) { set(func() { t.tlsDone = time.Now() }) },
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				set(func() { t.reused = true })
			}
		},
		GotFirstResponseByte: func() { set(func() { t.firstByte = time.Now() }) },
	}
}

// timing converts the collected timestamps into per-phase durations.
func (t *tracer) timing(start, end time.Time) *model.ResponseTiming {
	t.mu.Lock()
	defer t.mu.Unlock()
	span := func(a, b time.Time) int64 {
		if a.IsZero() || b.IsZero() || b.Before(a) {
			return 0
		}
		return b.Sub(a).Milliseconds()
	}
	tm := &model.ResponseTiming{
		DNSMs:     span(t.dnsStart, t.dnsDone),
		ConnectMs: span(t.connectStart, t.connectDone),
		TLSMs:     span(t.tlsStart, t.tlsDone),
		Reused:    t.reused,
	}
	if !t.firstByte.IsZero() {
		tm.FirstByteMs = span(start, t.firstByte)
		tm.DownloadMs = span(t.firstByte, end)
	}
	return tm
}

// build constructs the *http.Request with interpolated URL, query params,
// headers and body.
func build(ctx context.Context, req model.Request, scope *vars.Scope) (*http.Request, error) {
	rawURL := scope.Apply(req.URL)

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for _, p := range scope.ApplyKVs(req.Params) {
		q.Add(p.Key, p.Value)
	}
	u.RawQuery = q.Encode()

	bodyReader, contentType, err := buildBody(req.Body, scope)
	if err != nil {
		return nil, err
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	r, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	for _, h := range scope.ApplyKVs(req.Headers) {
		r.Header.Set(h.Key, h.Value)
	}
	if contentType != "" && r.Header.Get("Content-Type") == "" {
		r.Header.Set("Content-Type", contentType)
	}
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", DefaultUserAgent)
	}
	return r, nil
}

// buildBody returns a reader for the request body and a default Content-Type.
func buildBody(b model.Body, scope *vars.Scope) (io.Reader, string, error) {
	switch b.Type {
	case model.BodyJSON:
		return strings.NewReader(scope.Apply(b.Raw)), "application/json", nil
	case model.BodyRaw:
		return strings.NewReader(scope.Apply(b.Raw)), "", nil
	case model.BodyForm:
		form := url.Values{}
		for _, kv := range scope.ApplyKVs(b.Form) {
			form.Add(kv.Key, kv.Value)
		}
		return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil
	case model.BodyMultipart:
		return buildMultipart(b, scope)
	case model.BodyGraphQL:
		return buildGraphQL(b, scope)
	default:
		return nil, "", nil
	}
}

// buildMultipart encodes form rows as multipart/form-data; rows flagged File
// stream the file at Value (path) under its base name.
func buildMultipart(b model.Body, scope *vars.Scope) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, kv := range scope.ApplyKVs(b.Form) {
		if kv.Key == "" {
			continue
		}
		if kv.File {
			f, err := os.Open(kv.Value)
			if err != nil {
				return nil, "", fmt.Errorf("multipart field %q: %w", kv.Key, err)
			}
			part, err := w.CreateFormFile(kv.Key, filepath.Base(kv.Value))
			if err == nil {
				_, err = io.Copy(part, f)
			}
			f.Close()
			if err != nil {
				return nil, "", fmt.Errorf("multipart field %q: %w", kv.Key, err)
			}
		} else if err := w.WriteField(kv.Key, kv.Value); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}

// buildGraphQL wraps the query + variables into the standard GraphQL JSON
// envelope. Variables must be empty or a JSON object.
func buildGraphQL(b model.Body, scope *vars.Scope) (io.Reader, string, error) {
	payload := map[string]json.RawMessage{}
	q, _ := json.Marshal(scope.Apply(b.Raw))
	payload["query"] = q
	if v := strings.TrimSpace(scope.Apply(b.Variables)); v != "" {
		if !json.Valid([]byte(v)) {
			return nil, "", fmt.Errorf("graphql variables is not valid JSON")
		}
		payload["variables"] = json.RawMessage(v)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	return bytes.NewReader(data), "application/json", nil
}

// readBody reads up to MaxInlineBytes+1 to detect truncation, while still
// counting the full size by draining the rest.
func readBody(r io.Reader) (body []byte, truncated bool, size int64) {
	limited := io.LimitReader(r, MaxInlineBytes)
	buf, _ := io.ReadAll(limited)
	size = int64(len(buf))
	// Drain remainder to learn the true size.
	n, _ := io.Copy(io.Discard, r)
	if n > 0 {
		truncated = true
		size += n
	}
	return buf, truncated, size
}

// prettyIfJSON indents JSON bodies for readable display; other types pass
// through unchanged. Done here in Go to keep formatting off the UI thread.
func prettyIfJSON(contentType string, body []byte) string {
	if !strings.Contains(strings.ToLower(contentType), "json") {
		return string(body)
	}
	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		return string(body)
	}
	return out.String()
}
