package importer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"senda/internal/buildinfo"
	"senda/internal/mockserver"
	"senda/internal/model"
)

// HAR (HTTP Archive 1.2) is the format browsers and proxies export — Chrome
// DevTools' "Copy all as HAR" yields one JSON document holding every captured
// request and its response. We parse the log into requests (HAR), into
// mock-server definitions (HARMocks), and the reverse: marshal a single
// senda request+response back into a HAR (MarshalHAR).

type harFile struct {
	Log harLog `json:"log"`
}

type harLog struct {
	Version string     `json:"version"`
	Creator harCreator `json:"creator"`
	Entries []harEntry `json:"entries"`
}

type harCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type harEntry struct {
	StartedDateTime string      `json:"startedDateTime,omitempty"`
	Time            float64     `json:"time"`
	Request         harRequest  `json:"request"`
	Response        harResponse `json:"response"`
	Timings         harTimings  `json:"timings"`
	Cache           struct{}    `json:"cache"`
}

type harRequest struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	HTTPVersion string       `json:"httpVersion"`
	Cookies     []harCookie  `json:"cookies"`
	Headers     []harNV      `json:"headers"`
	QueryString []harNV      `json:"queryString"`
	PostData    *harPostData `json:"postData,omitempty"`
	HeadersSize int          `json:"headersSize"`
	BodySize    int          `json:"bodySize"`
}

type harResponse struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Cookies     []harCookie `json:"cookies"`
	Headers     []harNV     `json:"headers"`
	Content     harContent  `json:"content"`
	RedirectURL string      `json:"redirectURL"`
	HeadersSize int         `json:"headersSize"`
	BodySize    int         `json:"bodySize"`
}

type harNV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harPostData struct {
	MimeType string     `json:"mimeType"`
	Text     string     `json:"text,omitempty"`
	Params   []harParam `json:"params,omitempty"`
}

type harParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

type harTimings struct {
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
}

// secretHeaders are request headers stripped on import — they carry live
// credentials that must never land in a git-tracked YAML file.
// ponytail: strip, don't extract to senda.secret.yaml; upgrade to extraction if
// users ask to keep the values as {{vars}}.
var secretHeaders = map[string]bool{
	"authorization": true,
	"cookie":        true,
}

// staticExts and analyticsHosts drive the noise filter: a captured HAR from a
// browsing session is mostly assets and tracking, not API calls.
var staticExts = map[string]bool{
	".js": true, ".mjs": true, ".css": true, ".map": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".ico": true, ".webp": true, ".avif": true, ".bmp": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".mp4": true, ".webm": true, ".mp3": true, ".wav": true,
}

var analyticsHosts = []string{
	"google-analytics", "googletagmanager", "doubleclick", "googlesyndication",
	"facebook.net", "connect.facebook", "segment.io", "segment.com",
	"sentry.io", "mixpanel", "hotjar", "intercom", "amplitude", "fullstory",
	"datadoghq", "newrelic", "nr-data.net", "clarity.ms", "bugsnag", "optimizely",
}

// HAR parses a HAR document into importable requests, one per kept entry.
// Static assets and analytics beacons are filtered out, secret headers stripped,
// duplicate request lines collapsed (first wins), and entries grouped into a
// folder per host. Each request keeps its captured response body as
// ResponseExample. Returns the requests and the number of entries skipped.
func HAR(data []byte) ([]Imported, int, error) {
	var doc harFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, 0, fmt.Errorf("har: %w", err)
	}
	if len(doc.Log.Entries) == 0 {
		return nil, 0, fmt.Errorf("har: no entries found")
	}

	var out []Imported
	skipped := 0
	seen := map[string]bool{}
	for _, e := range doc.Log.Entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil || u.Host == "" {
			skipped++
			continue
		}
		if isNoise(u, e.Response.Content.MimeType) {
			skipped++
			continue
		}
		key := e.Request.Method + " " + u.Scheme + "://" + u.Host + u.Path + "?" + u.RawQuery
		if seen[key] {
			skipped++
			continue
		}
		seen[key] = true
		out = append(out, Imported{
			Dir:     []string{sanitize(u.Host)},
			Request: harToRequest(u, e),
		})
	}
	if len(out) == 0 {
		return nil, skipped, fmt.Errorf("har: no API requests found (all %d entries filtered as static/analytics)", skipped)
	}
	return out, skipped, nil
}

func harToRequest(u *url.URL, e harEntry) model.Request {
	req := model.Request{
		Name:   harName(u),
		Method: strings.ToUpper(e.Request.Method),
		URL:    u.Scheme + "://" + u.Host + u.Path,
		Auth:   model.Auth{Type: model.AuthInherit},
		Body:   model.Body{Type: model.BodyNone},
	}
	if req.Method == "" {
		req.Method = "GET"
	}

	// Sorted keys: map iteration order is random, and senda is git-native — a
	// stable param order keeps re-imports from churning the YAML diff.
	q := u.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range q[k] {
			req.Params = append(req.Params, model.KV{Key: k, Value: v, Enabled: true})
		}
	}

	for _, h := range e.Request.Headers {
		if strings.HasPrefix(h.Name, ":") { // HTTP/2 pseudo-headers (:method, :path…)
			continue
		}
		if secretHeaders[strings.ToLower(h.Name)] {
			continue
		}
		req.Headers = append(req.Headers, model.KV{Key: h.Name, Value: h.Value, Enabled: true})
	}

	if pd := e.Request.PostData; pd != nil {
		switch {
		case len(pd.Params) > 0:
			form := make([]model.KV, 0, len(pd.Params))
			for _, p := range pd.Params {
				form = append(form, model.KV{Key: p.Name, Value: p.Value, Enabled: true})
			}
			req.Body = model.Body{Type: model.BodyForm, Form: form}
		case pd.Text != "":
			if strings.Contains(pd.MimeType, "json") || looksJSON(pd.Text) {
				req.Body = model.Body{Type: model.BodyJSON, Raw: pd.Text}
			} else {
				req.Body = model.Body{Type: model.BodyRaw, Raw: pd.Text}
			}
		}
	}

	req.ResponseExample = decodeContent(e.Response.Content)
	return req
}

// HARMocks turns a HAR document into mock-server definitions — one route per
// kept request, serving the captured status, headers, and body. Duplicate
// method+path entries collapse to the first seen.
func HARMocks(data []byte) ([]mockserver.MockDef, error) {
	var doc harFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("har: %w", err)
	}
	if len(doc.Log.Entries) == 0 {
		return nil, fmt.Errorf("har: no entries found")
	}

	var out []mockserver.MockDef
	seen := map[string]bool{}
	for _, e := range doc.Log.Entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil || u.Host == "" {
			continue
		}
		if isNoise(u, e.Response.Content.MimeType) {
			continue
		}
		// Skip entries with no captured response (status 0 = pending/failed) —
		// a mock can't serve a zero status.
		if e.Response.Status == 0 {
			continue
		}
		method := strings.ToUpper(e.Request.Method)
		if method == "" {
			method = "GET"
		}
		p := u.Path
		if p == "" {
			p = "/"
		}
		key := method + " " + p
		if seen[key] {
			continue
		}
		seen[key] = true

		headers := map[string]string{}
		for _, h := range e.Response.Headers {
			name := strings.ToLower(h.Name)
			if strings.HasPrefix(h.Name, ":") || name == "set-cookie" {
				continue
			}
			// Drop transfer encodings — the mock writes a fixed body itself.
			if name == "content-encoding" || name == "content-length" || name == "transfer-encoding" {
				continue
			}
			headers[h.Name] = h.Value
		}

		out = append(out, mockserver.MockDef{
			// Name must be unique per method+path: writeMocks derives the
			// filename from it, so a bare path segment would collide (GET /users
			// vs POST /users both -> users.yaml) and silently drop a route.
			Name:   sanitize(strings.ToLower(method) + "-" + strings.Trim(p, "/")),
			Method: method,
			Path:   p,
			Responses: []mockserver.ResponseDef{{
				Status:  e.Response.Status,
				Headers: headers,
				Body:    decodeContent(e.Response.Content),
			}},
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("har: no API requests found")
	}
	return out, nil
}

// MarshalHAR renders one senda request and its response as a HAR 1.2 document,
// so a captured send can be shared with any HAR-aware tooling.
func MarshalHAR(req model.Request, resp model.Response) ([]byte, error) {
	hreq := harRequest{
		Method:      strings.ToUpper(req.Method),
		URL:         req.URL,
		HTTPVersion: "HTTP/1.1",
		Cookies:     []harCookie{},
		Headers:     []harNV{},
		QueryString: []harNV{},
		HeadersSize: -1,
		BodySize:    -1,
	}
	for _, h := range req.Headers {
		if h.Enabled {
			hreq.Headers = append(hreq.Headers, harNV{Name: h.Key, Value: h.Value})
		}
	}
	for _, q := range req.Params {
		if q.Enabled {
			hreq.QueryString = append(hreq.QueryString, harNV{Name: q.Key, Value: q.Value})
		}
	}
	if req.Body.Raw != "" {
		mime := "text/plain"
		if req.Body.Type == model.BodyJSON {
			mime = "application/json"
		}
		hreq.PostData = &harPostData{MimeType: mime, Text: req.Body.Raw}
		hreq.BodySize = len(req.Body.Raw)
	}

	hresp := harResponse{
		Status:      resp.Status,
		StatusText:  resp.StatusText,
		HTTPVersion: "HTTP/1.1",
		Cookies:     []harCookie{},
		Headers:     []harNV{},
		Content: harContent{
			Size:     len(resp.Body),
			MimeType: firstHeader(resp.Headers, "Content-Type"),
			Text:     resp.Body,
		},
		RedirectURL: "",
		HeadersSize: -1,
		BodySize:    int(resp.SizeBytes),
	}
	for name, vals := range resp.Headers {
		for _, v := range vals {
			hresp.Headers = append(hresp.Headers, harNV{Name: name, Value: v})
		}
	}

	doc := harFile{Log: harLog{
		Version: "1.2",
		Creator: harCreator{Name: "senda", Version: buildinfo.Version},
		Entries: []harEntry{{
			Time:     float64(resp.DurationMs),
			Request:  hreq,
			Response: hresp,
			Timings:  harTimings{Send: 0, Wait: float64(resp.DurationMs), Receive: 0},
		}},
	}}
	return json.MarshalIndent(doc, "", "  ")
}

// decodeContent returns the response body text, base64-decoding it when the HAR
// flags the content as base64 (binary or some proxies' default).
func decodeContent(c harContent) string {
	if c.Text == "" {
		return ""
	}
	if strings.EqualFold(c.Encoding, "base64") {
		if dec, err := base64.StdEncoding.DecodeString(c.Text); err == nil {
			return string(dec)
		}
	}
	return c.Text
}

// isNoise reports whether a captured entry is a static asset or analytics
// beacon rather than an API call worth importing.
func isNoise(u *url.URL, mimeType string) bool {
	if ext := strings.ToLower(path.Ext(u.Path)); ext != "" && staticExts[ext] {
		return true
	}
	mt := strings.ToLower(mimeType)
	if strings.HasPrefix(mt, "image/") || strings.HasPrefix(mt, "font/") ||
		strings.HasPrefix(mt, "video/") || strings.HasPrefix(mt, "audio/") ||
		strings.Contains(mt, "text/css") || strings.Contains(mt, "javascript") {
		return true
	}
	host := strings.ToLower(u.Host)
	for _, a := range analyticsHosts {
		if strings.Contains(host, a) {
			return true
		}
	}
	return false
}

// harName derives a request name from the URL's last path segment, falling back
// to the host.
func harName(u *url.URL) string {
	seg := path.Base(strings.TrimRight(u.Path, "/"))
	if seg == "" || seg == "." || seg == "/" {
		seg = u.Host
	}
	return sanitize(seg)
}

func firstHeader(headers map[string][]string, key string) string {
	for k, vals := range headers {
		if strings.EqualFold(k, key) && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}
