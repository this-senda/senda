// Package model holds the core data types shared across the Senda backend.
// These structs are the on-disk YAML schema and the Wails binding contract.
package model

// KV is an enable-able key/value pair used for params, headers, form fields
// and environment variables. File marks a multipart row whose Value is a
// filesystem path to upload rather than a literal value.
type KV struct {
	Key     string `yaml:"key" json:"key"`
	Value   string `yaml:"value" json:"value"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Desc    string `yaml:"desc,omitempty" json:"desc,omitempty"`
	File    bool   `yaml:"file,omitempty" json:"file,omitempty"`
	// Type is the documented schema type (string, integer, object…) for params
	// and body fields. Doc-only: surfaced in generated API docs, ignored when
	// sending. Empty for plain rows (headers, env vars).
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
}

// BodyType enumerates the supported request body encodings.
type BodyType string

const (
	BodyNone      BodyType = "none"
	BodyJSON      BodyType = "json"
	BodyRaw       BodyType = "raw"
	BodyForm      BodyType = "form"
	BodyMultipart BodyType = "multipart"
	BodyGraphQL   BodyType = "graphql"
	BodyWebSocket BodyType = "websocket"
	BodySSE       BodyType = "sse"
)

// Body is a request payload. Only the fields matching Type are meaningful:
// json/raw use Raw; form and multipart use Form (multipart rows may set
// KV.File); graphql uses Raw as the query and Variables as a JSON object.
type Body struct {
	Type      BodyType `yaml:"type" json:"type"`
	Raw       string   `yaml:"raw,omitempty" json:"raw,omitempty"`
	Form      []KV     `yaml:"form,omitempty" json:"form,omitempty"`
	Variables string   `yaml:"variables,omitempty" json:"variables,omitempty"`
	// Fields documents a json body's top-level properties (KV.Key=name,
	// Type=schema type, Enabled=required, Desc=description). Doc-only: the sent
	// payload is Raw; this just feeds the generated API docs' body-param table.
	Fields []KV `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// AuthType enumerates the supported authentication schemes. Empty is treated
// the same as AuthInherit.
type AuthType string

const (
	AuthInherit AuthType = "inherit" // use the collection-level auth
	AuthNone    AuthType = "none"    // send no auth
	AuthBearer  AuthType = "bearer"  // Authorization: Bearer <token>
	AuthBasic   AuthType = "basic"   // HTTP Basic
	AuthAPIKey  AuthType = "apikey"  // key/value in a header or query param
	AuthOAuth2  AuthType = "oauth2"  // fetch a token, then send as Bearer
)

// APIKeyPlacement says where an API key is sent.
type APIKeyPlacement string

const (
	APIKeyHeader APIKeyPlacement = "header"
	APIKeyQuery  APIKeyPlacement = "query"
)

// OAuth2Grant enumerates the supported OAuth2 token-fetch flows. Only the
// non-interactive grants (no browser redirect) are supported.
type OAuth2Grant string

const (
	OAuth2ClientCredentials OAuth2Grant = "client_credentials"
	OAuth2Password          OAuth2Grant = "password"
)

// Auth is a request's (or collection's) authentication config. The struct is
// flat so it round-trips cleanly through Wails bindings and YAML; only the
// fields relevant to Type carry meaning. Values support {{var}} interpolation.
type Auth struct {
	Type AuthType `yaml:"type" json:"type"`

	// bearer
	Token string `yaml:"token,omitempty" json:"token,omitempty"`

	// basic
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// apikey
	Key       string          `yaml:"key,omitempty" json:"key,omitempty"`
	KeyValue  string          `yaml:"keyValue,omitempty" json:"keyValue,omitempty"`
	Placement APIKeyPlacement `yaml:"placement,omitempty" json:"placement,omitempty"`

	// oauth2 (client_credentials / password grants)
	Grant         OAuth2Grant `yaml:"grant,omitempty" json:"grant,omitempty"`
	TokenURL      string      `yaml:"tokenUrl,omitempty" json:"tokenUrl,omitempty"`
	ClientID      string      `yaml:"clientId,omitempty" json:"clientId,omitempty"`
	ClientSecret  string      `yaml:"clientSecret,omitempty" json:"clientSecret,omitempty"`
	Scope         string      `yaml:"scope,omitempty" json:"scope,omitempty"`
	OAuthUsername string      `yaml:"oauthUsername,omitempty" json:"oauthUsername,omitempty"`
	OAuthPassword string      `yaml:"oauthPassword,omitempty" json:"oauthPassword,omitempty"`
}

// Assert is one declarative post-response check on a request. Target selects
// what to inspect ("status", "duration", "size", "body", "header.<Name>" or
// "json.<path>"), Op how to compare, Value what to compare against.
type Assert struct {
	Target  string `yaml:"target" json:"target"`
	Op      string `yaml:"op" json:"op"`
	Value   string `yaml:"value,omitempty" json:"value,omitempty"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
}

// AssertResult is the outcome of evaluating one Assert against a response.
type AssertResult struct {
	Target string `json:"target"`
	Op     string `json:"op"`
	Value  string `json:"value,omitempty"`
	Actual string `json:"actual,omitempty"`
	Pass   bool   `json:"pass"`
	Error  string `json:"error,omitempty"`
}

// Request is a single HTTP request, persisted as one YAML file on disk.
// PreScript runs before send (may mutate the request and set runtime vars);
// PostScript runs after the response arrives (typically extracts values into
// runtime vars for later requests).
type Request struct {
	Name            string    `yaml:"name" json:"name"`
	Method          string    `yaml:"method" json:"method"`
	URL             string    `yaml:"url" json:"url"`
	Params          []KV      `yaml:"params,omitempty" json:"params"`
	PathParams      []KV      `yaml:"pathParams,omitempty" json:"pathParams,omitempty"` // doc-only: {path} params (KV.Type/Desc), not sent — the URL carries them as {{vars}}
	Headers         []KV      `yaml:"headers,omitempty" json:"headers"`
	Body            Body      `yaml:"body,omitempty" json:"body"`
	Auth            Auth      `yaml:"auth,omitempty" json:"auth"`
	Asserts         []Assert  `yaml:"asserts,omitempty" json:"asserts"`
	PreScript       string    `yaml:"preScript,omitempty" json:"preScript"`
	PostScript      string    `yaml:"postScript,omitempty" json:"postScript"`
	Docs            string    `yaml:"docs,omitempty" json:"docs"`
	ResponseSchema  string    `yaml:"responseSchema,omitempty" json:"responseSchema,omitempty"`   // inline JSON Schema (validation + schema reference)
	ResponseExample string    `yaml:"responseExample,omitempty" json:"responseExample,omitempty"` // doc-only: example success response body
	OnFail          string    `yaml:"onFail,omitempty" json:"onFail,omitempty"`                   // stop | continue | jump:<folder>
	Spec            *SpecLink `yaml:"spec,omitempty" json:"spec,omitempty"`                       // link to an OpenAPI operation for body schema hints
}

// SpecLink ties a request to one operation in a stored OpenAPI spec, so the body
// editor can pull that operation's requestBody schema for validation +
// autocomplete. File is relative to .senda/openapi; OperationID is the spec's
// operationId, else a synthesized "METHOD /path" key.
type SpecLink struct {
	File        string `yaml:"file" json:"file"`
	OperationID string `yaml:"operationId" json:"operationId"`
}

// Environment is a named set of variables (e.g. dev, prod).
type Environment struct {
	Name  string `yaml:"name" json:"name"`
	Color string `yaml:"color,omitempty" json:"color,omitempty"` // dot color in the env switcher pill
	Vars  []KV   `yaml:"vars" json:"vars"`
}

// ResponseTiming breaks a request's duration into phases, all in
// milliseconds. Sends that reuse a pooled connection report zero
// DNS/connect/TLS and set Reused.
type ResponseTiming struct {
	DNSMs       int64 `json:"dnsMs"`
	ConnectMs   int64 `json:"connectMs"`
	TLSMs       int64 `json:"tlsMs"`
	FirstByteMs int64 `json:"firstByteMs"` // start of send to first response byte
	DownloadMs  int64 `json:"downloadMs"`
	Reused      bool  `json:"reused"`
}

// Response is the result of sending a Request.
type Response struct {
	Status     int                 `json:"status"`
	StatusText string              `json:"statusText"`
	DurationMs int64               `json:"durationMs"`
	SizeBytes  int64               `json:"sizeBytes"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
	Truncated  bool                `json:"truncated"`
	Error      string              `json:"error,omitempty"`
	Asserts    []AssertResult      `json:"asserts,omitempty"`
	Timing     *ResponseTiming     `json:"timing,omitempty"`
	ScriptLogs []string            `json:"scriptLogs,omitempty"`
}

// TreeNode is a node in the collection sidebar tree. Folders have Children;
// requests have a Path pointing at their YAML file and the request's Method
// for the sidebar badge. Color/Tags/Description mirror a folder's senda.meta.yaml
// metadata so the sidebar can render them without re-reading each folder.
type TreeNode struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	IsDir       bool        `json:"isDir"`
	Method      string      `json:"method,omitempty"`
	Color       string      `json:"color,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Description string      `json:"description,omitempty"`
	Children    []*TreeNode `json:"children,omitempty"`
}

// Collection is an opened collection (or a folder within one): its path,
// metadata and tree. The same struct is the on-disk senda.meta.yaml schema for
// both the collection root and any sub-folder. Color/Tags/Description are
// purely organisational; Vars/Auth participate in the resolution chain
// (request -> folder(s) -> collection root). Proxy/TLS apply only at the
// collection root (pipeline reads them off the opened collection, not folders);
// both support {{var}} so machine-specific URLs and cert paths stay out of git.
type Collection struct {
	Name        string    `yaml:"name,omitempty" json:"name"`
	Path        string    `yaml:"-" json:"path"`
	Color       string    `yaml:"color,omitempty" json:"color,omitempty"`
	Tags        []string  `yaml:"tags,omitempty" json:"tags,omitempty"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	Vars        []KV      `yaml:"vars,omitempty" json:"vars"`
	Auth        Auth      `yaml:"auth,omitempty" json:"auth"`
	Proxy       string    `yaml:"proxy,omitempty" json:"proxy"`
	TLS         TLSConfig `yaml:"tls,omitempty" json:"tls"`
	Tree        *TreeNode `yaml:"-" json:"tree"`
}

// TLSConfig configures the transport for a collection's sends: a client
// certificate (mTLS), a custom CA bundle, and an opt-out of verification.
// All path fields support {{var}} so they resolve from env/OS vars at send
// time. The zero value means "use Go's defaults".
type TLSConfig struct {
	CertFile string `yaml:"certFile,omitempty" json:"certFile"`
	KeyFile  string `yaml:"keyFile,omitempty" json:"keyFile"`
	CAFile   string `yaml:"caFile,omitempty" json:"caFile"`
	Insecure bool   `yaml:"insecure,omitempty" json:"insecure"` // skip server cert verification
}

// RunResult is the outcome of sending one request during a folder run.
// Response carries the full response (body capped at the inline limit) so
// the runner UI can show per-request detail.
type RunResult struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Method     string    `json:"method"`
	URL        string    `json:"url"`
	Status     int       `json:"status"`
	DurationMs int64     `json:"durationMs"`
	SizeBytes  int64     `json:"sizeBytes"`
	OK         bool      `json:"ok"`
	AssertPass int       `json:"assertPass"`
	AssertFail int       `json:"assertFail"`
	Error      string    `json:"error,omitempty"`
	Response   *Response `json:"response,omitempty"`
}

// Flow is a declarative graph orchestrating requests with branching, loops and
// variable extraction beyond the sequential folder runner. Persisted as one
// *.flow.yaml under .senda/flows/. Execution starts at Start and follows each
// node's outgoing edge (Next / OnTrue / OnFalse) until a node has none.
type Flow struct {
	Name  string              `yaml:"name" json:"name"`
	Path  string              `yaml:"-" json:"path"`
	Start string              `yaml:"start" json:"start"`
	Nodes map[string]FlowNode `yaml:"nodes" json:"nodes"`
}

// FlowNode is one step in a Flow. Type selects which fields apply:
//
//	request  — runs Request (a collection-relative path); edge Next
//	branch   — evaluates Cond; edge OnTrue or OnFalse
//	setvar   — sets runtime var Var to the interpolated From; edge Next
//	delay    — sleeps Ms milliseconds; edge Next
//	loop     — runs the Body node list once per row of data file Data; edge Next
//	parallel — runs each list in Branches concurrently; edge Next
//
// Nodes listed in a loop Body or a parallel branch are owned by that container
// and run linearly (their own edges are ignored) — don't also target them with
// a Next/OnTrue/OnFalse from the main graph.
type FlowNode struct {
	Type     string     `yaml:"type" json:"type"`
	Request  string     `yaml:"request,omitempty" json:"request,omitempty"`
	Cond     *FlowCond  `yaml:"cond,omitempty" json:"cond,omitempty"`
	Var      string     `yaml:"var,omitempty" json:"var,omitempty"`
	From     string     `yaml:"from,omitempty" json:"from,omitempty"`
	Ms       int        `yaml:"ms,omitempty" json:"ms,omitempty"`
	Data     string     `yaml:"data,omitempty" json:"data,omitempty"`
	Body     []string   `yaml:"body,omitempty" json:"body,omitempty"`
	Branches [][]string `yaml:"branches,omitempty" json:"branches,omitempty"`
	Next     string     `yaml:"next,omitempty" json:"next,omitempty"`
	OnTrue   string     `yaml:"onTrue,omitempty" json:"onTrue,omitempty"`
	OnFalse  string     `yaml:"onFalse,omitempty" json:"onFalse,omitempty"`
}

// FlowCond is a branch comparison. Left and Right are interpolated (so they can
// reference {{res.<slug>...}} or {{var}}) then compared with Op, which uses the
// same operators as assertions (eq, neq, contains, gt, …).
type FlowCond struct {
	Left  string `yaml:"left" json:"left"`
	Op    string `yaml:"op" json:"op"`
	Right string `yaml:"right,omitempty" json:"right,omitempty"`
}

// FlowInfo is a flow's identity for listing in the sidebar (no node detail).
type FlowInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Cookie is one cookie from the session jar, surfaced to the UI.
type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HistoryEntry records one sent request for the recent-history list.
type HistoryEntry struct {
	At         string `json:"at"` // RFC3339 timestamp
	Method     string `json:"method"`
	URL        string `json:"url"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"durationMs"`
	SizeBytes  int64  `json:"sizeBytes"`
	Error      string `json:"error,omitempty"`
}

// LoadOptions configures a load test run.
type LoadOptions struct {
	VUs        int `json:"vus"`        // concurrent virtual users
	Duration   int `json:"duration"`   // seconds; 0 means use Iterations
	Iterations int `json:"iterations"` // per-VU loop count; 0 means use Duration
	RampUp     int `json:"rampUp"`     // seconds to ramp from 0 to full VU count; 0 = no ramp
}

// LoadTick is a streaming progress snapshot emitted once per second during a
// load test. All latency values are in milliseconds.
type LoadTick struct {
	Elapsed    float64     `json:"elapsed"` // seconds since start
	Total      int         `json:"total"`   // requests completed so far
	Errors     int         `json:"errors"`
	RPS        float64     `json:"rps"`
	P50        float64     `json:"p50"`
	P95        float64     `json:"p95"`
	P99        float64     `json:"p99"`
	StatusDist map[int]int `json:"statusDist"`
}

// SecurityOptions configures a security scan run. Severity and Tags filter
// which nuclei templates execute; empty values fall back to a sensible
// default profile (see internal/security).
type SecurityOptions struct {
	Severity  string   `json:"severity"`  // comma list: info,low,medium,high,critical; "" = all
	Tags      []string `json:"tags"`      // template tags filter (e.g. owasp, cve, misconfig)
	Builtin   bool     `json:"builtin"`   // true = only the embedded check pack (ignore synced .security templates)
	RateLimit int      `json:"rateLimit"` // max requests/second; 0 = default (50)
	Timeout   int      `json:"timeout"`   // per-request timeout seconds; 0 = default (10)
}

// ScanPlan previews what a security scan would execute for the current options
// without sending any probe traffic, so the UI can show the size of the run
// before the user starts it. Checks == Templates × Targets.
type ScanPlan struct {
	Targets   int `json:"targets"`   // unique resolved URLs under the folder
	Templates int `json:"templates"` // templates matching the severity/tags/source filter
	Checks    int `json:"checks"`    // template×target pairs that will run
}

// SecurityCheck is the outcome of one security template executed against one
// target, streamed to the UI as a "security:check" event while the scan is
// running. Matched means the check found the issue it probes for; Error means
// the probe itself failed (e.g. target unreachable) so the check is
// inconclusive; otherwise the check passed.
type SecurityCheck struct {
	TemplateID  string   `json:"templateId"`
	Name        string   `json:"name"`
	Severity    string   `json:"severity"`
	Target      string   `json:"target"`
	Matched     bool     `json:"matched"`
	MatchedAt   string   `json:"matchedAt,omitempty"` // URL that matched
	Error       string   `json:"error,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Reference   []string `json:"reference,omitempty"`
	Remediation string   `json:"remediation,omitempty"`
	OWASP       string   `json:"owasp,omitempty"` // e.g. "API8:2023 Security Misconfiguration"
	CWE         []string `json:"cwe,omitempty"`   // e.g. ["CWE-200"]
}

// SecuritySummary is the final aggregate result of a security scan.
type SecuritySummary struct {
	Targets    int            `json:"targets"`
	Checks     int            `json:"checks"`   // template×target pairs executed
	Findings   int            `json:"findings"` // checks that matched
	Passed     int            `json:"passed"`   // checks that ran clean
	Errors     int            `json:"errors"`   // probes that failed to run
	BySeverity map[string]int `json:"bySeverity"`
	Duration   float64        `json:"duration"` // elapsed seconds
}

// WSMessage is one WebSocket message in a session log.
type WSMessage struct {
	Direction string `json:"direction"` // "sent" | "received"
	Data      string `json:"data"`
	At        int64  `json:"at"` // unix ms
}

// WSSession holds the result of a WebSocket connection.
type WSSession struct {
	Messages  []WSMessage `json:"messages"`
	CloseCode int         `json:"closeCode"`
	Error     string      `json:"error,omitempty"`
}

// SSEEvent is one server-sent event.
type SSEEvent struct {
	ID    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"`
	Data  string `json:"data"`
	At    int64  `json:"at"` // unix ms
}

// SSESession holds the result of an SSE connection.
type SSESession struct {
	Events  []SSEEvent `json:"events"`
	Count   int        `json:"count"`
	FirstMs int64      `json:"firstMs,omitempty"` // latency to first event
	Error   string     `json:"error,omitempty"`
}

// LoadSummary is the final aggregate result of a load test run.
type LoadSummary struct {
	Total      int         `json:"total"`
	Errors     int         `json:"errors"`
	Duration   float64     `json:"duration"` // actual elapsed seconds
	RPS        float64     `json:"rps"`
	P50        float64     `json:"p50"`
	P95        float64     `json:"p95"`
	P99        float64     `json:"p99"`
	StatusDist map[int]int `json:"statusDist"`
}
