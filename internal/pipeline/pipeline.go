// Package pipeline is the full request-send pipeline shared by the desktop
// app and the CLI runner: pre-script -> variable scope -> send -> asserts ->
// post-script, plus the session-scoped runtime variables scripts write to.
package pipeline

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"senda/internal/assert"
	"senda/internal/httpclient"
	"senda/internal/model"
	"senda/internal/schemaval"
	"senda/internal/script"
	"senda/internal/store"
	"senda/internal/vars"
)

// Session owns an HTTP client (with its cookie jar) and the runtime variables
// accumulated by scripts. One Session spans many sends.
type Session struct {
	HTTP *httpclient.Client

	mu        sync.Mutex
	runtime   map[string]string
	responses map[string]model.Response // by request slug, for {{res.<slug>...}}
}

// NewSession builds a fresh session with its own client, jar and vars.
func NewSession() *Session {
	return &Session{HTTP: httpclient.New(), runtime: map[string]string{}, responses: map[string]model.Response{}}
}

// SetVar stores a runtime variable (script senda.setVar).
func (s *Session) SetVar(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtime[name] = value
}

// RuntimeKVs returns the runtime variables as a sorted KV list.
func (s *Session) RuntimeKVs() []model.KV {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.KV, 0, len(s.runtime))
	for k, v := range s.runtime {
		out = append(out, model.KV{Key: k, Value: v, Enabled: true})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// ClearRuntime wipes all runtime variables.
func (s *Session) ClearRuntime() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtime = map[string]string{}
}

// resolveRes resolves a {{res.<slug>.<target>}} reference against a previously
// sent request's response. <target> is the same grammar as assert targets
// (status, body, header.<Name>, json.<path>). Array indexing ([n]) isn't
// available here — the {{...}} grammar excludes brackets — use a post-script for
// that. Returns ok=false (leaving the placeholder unresolved) for any miss.
func (s *Session) resolveRes(name string) (string, bool) {
	rest, ok := strings.CutPrefix(name, "res.")
	if !ok {
		return "", false
	}
	slug, target, ok := strings.Cut(rest, ".")
	if !ok || slug == "" || target == "" {
		return "", false
	}
	s.mu.Lock()
	resp, ok := s.responses[slug]
	s.mu.Unlock()
	if !ok {
		return "", false
	}
	val, found, err := assert.Extract(target, resp)
	if err != nil || !found {
		return "", false
	}
	return val, true
}

// storeResponse records resp under the request file's slug so later requests in
// the same session (folder run or flow) can reference it via {{res.<slug>...}}.
func (s *Session) storeResponse(reqPath string, resp model.Response) {
	slug := slugOf(reqPath)
	if slug == "" {
		return
	}
	s.mu.Lock()
	s.responses[slug] = resp
	s.mu.Unlock()
}

// slugOf is the request file's basename without its YAML extension — the key
// used for {{res.<slug>...}} references. Empty for ad-hoc sends (no path).
func slugOf(reqPath string) string {
	base := filepath.Base(reqPath)
	base = strings.TrimSuffix(base, ".yaml")
	base = strings.TrimSuffix(base, ".yml")
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

// Scope builds the interpolation scope. Precedence, lowest first: collection
// vars/secrets, then each ancestor folder's vars (root-first so deeper folders
// override shallower ones), then env vars/secrets, then runtime vars (highest).
// reqPath is the request file's path; when empty only collection-level vars
// apply (no folder chain).
func (s *Session) Scope(collPath, reqPath, envName string) *vars.Scope {
	var layers [][]model.KV
	if collPath != "" {
		layers = append(layers, store.ReadMeta(collPath).Vars, store.CollectionSecrets(collPath))
		for _, dir := range store.FolderChain(collPath, reqPath) {
			layers = append(layers, store.ReadMeta(dir).Vars)
		}
		if envName != "" {
			if envs, err := store.ListEnvironments(collPath); err == nil {
				for _, e := range envs {
					if e.Name == envName {
						layers = append(layers, e.Vars)
						break
					}
				}
			}
			layers = append(layers, store.EnvironmentSecrets(collPath, envName))
		}
	}
	layers = append(layers, s.RuntimeKVs())
	sc := vars.Build(layers...)
	sc.Dynamic = s.resolveRes // enable {{res.<slug>...}} references
	return sc
}

// Resolve interpolates a standalone string against the session scope (env,
// collection/folder vars, runtime vars and {{res...}} references). Used by the
// flow engine for branch conditions and setvar extraction, which aren't tied to
// a single request. Pass reqPath "" for collection-level resolution.
func (s *Session) Resolve(collPath, reqPath, envName, str string) string {
	return s.Scope(collPath, reqPath, envName).Apply(str)
}

// collectionNetConfig reads the collection root's proxy/TLS settings and
// resolves {{var}} references through scope. Returns the zero NetConfig for
// ad-hoc sends (no collPath), which resets the client to Go's defaults.
func collectionNetConfig(collPath string, scope *vars.Scope) httpclient.NetConfig {
	if collPath == "" {
		return httpclient.NetConfig{}
	}
	m := store.ReadMeta(collPath)
	return httpclient.NetConfig{
		Proxy:    scope.Apply(m.Proxy),
		CertFile: scope.Apply(m.TLS.CertFile),
		KeyFile:  scope.Apply(m.TLS.KeyFile),
		CAFile:   scope.Apply(m.TLS.CAFile),
		Insecure: m.TLS.Insecure,
	}
}

// effectiveAuth resolves request auth for empty/inherit types by walking the
// folder chain from the deepest folder up to the collection root: the first
// folder declaring a concrete auth wins, then the collection root's auth, and
// finally AuthNone.
func effectiveAuth(collPath, reqPath string, reqAuth model.Auth) model.Auth {
	if reqAuth.Type != "" && reqAuth.Type != model.AuthInherit {
		return reqAuth
	}
	if collPath != "" {
		chain := store.FolderChain(collPath, reqPath)
		for i := len(chain) - 1; i >= 0; i-- { // deepest first
			if a := store.ReadMeta(chain[i]).Auth; a.Type != "" && a.Type != model.AuthInherit {
				return a
			}
		}
		if a := store.ReadMeta(collPath).Auth; a.Type != "" && a.Type != model.AuthInherit {
			return a
		}
	}
	return model.Auth{Type: model.AuthNone}
}

// Send runs the whole pipeline for one request. It returns the response and
// the interpolated URL (for history display). reqPath is the request file's
// path, used to resolve folder-level vars and auth; pass "" for ad-hoc sends.
func (s *Session) Send(ctx context.Context, req model.Request, collPath, reqPath, envName string) (model.Response, string) {
	return s.SendWithExtra(ctx, req, collPath, reqPath, envName, nil)
}

// SendWithExtra is like Send but injects extra variables at the top of the
// scope stack (highest priority after runtime vars). Used for data-driven runs.
func (s *Session) SendWithExtra(ctx context.Context, req model.Request, collPath, reqPath, envName string, extra map[string]string) (model.Response, string) {
	getVar := func(name string) string {
		if v, ok := extra[name]; ok {
			return v
		}
		s.mu.Lock()
		if v, ok := s.runtime[name]; ok {
			s.mu.Unlock()
			return v
		}
		s.mu.Unlock()
		if v, ok := s.Scope(collPath, reqPath, envName).Get(name); ok {
			return v
		}
		return ""
	}

	var scriptLogs []string
	if req.PreScript != "" {
		mutated, logs, err := script.RunPre(req.PreScript, req, getVar, s.SetVar)
		scriptLogs = append(scriptLogs, logs...)
		if err != nil {
			return model.Response{Error: err.Error(), ScriptLogs: scriptLogs}, req.URL
		}
		req = mutated
	}

	// Scope built after the pre-script so freshly set runtime vars interpolate
	// ({{res...}} references are enabled inside Scope). Extra vars (data-driven
	// rows, loop iterations) overlay at the top so they interpolate into the URL,
	// headers and body — not just the script getVar.
	scope := s.Scope(collPath, reqPath, envName)
	for k, v := range extra {
		scope.Set(k, v)
	}
	if err := s.HTTP.Configure(collectionNetConfig(collPath, scope)); err != nil {
		return model.Response{Error: err.Error(), ScriptLogs: scriptLogs}, req.URL
	}
	req.Auth = effectiveAuth(collPath, reqPath, req.Auth)
	resp := s.HTTP.Send(ctx, req, scope)
	s.storeResponse(reqPath, resp)
	if resp.Error == "" {
		resp.Asserts = assert.Eval(req.Asserts, resp)
		if req.ResponseSchema != "" {
			resp.Asserts = append(resp.Asserts, schemaval.Validate(req.ResponseSchema, resp.Body)...)
		}
	}
	if len(scope.Unresolved) > 0 && resp.Error == "" {
		resp.Error = "unresolved variables: " + joinUnique(scope.Unresolved)
	}

	if req.PostScript != "" && resp.Error == "" {
		pmResults, logs, err := script.RunPost(req.PostScript, req, resp, getVar, s.SetVar)
		scriptLogs = append(scriptLogs, logs...)
		if err != nil {
			resp.Error = err.Error()
		}
		resp.Asserts = append(resp.Asserts, pmResults...)
	}
	if len(scriptLogs) > 0 {
		resp.ScriptLogs = scriptLogs
	}
	return resp, scope.Apply(req.URL)
}

func joinUnique(ss []string) string {
	seen := map[string]bool{}
	var b strings.Builder
	for _, s := range ss {
		if seen[s] {
			continue
		}
		seen[s] = true
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteString(s)
	}
	return b.String()
}
