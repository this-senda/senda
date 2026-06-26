// Package pipeline is the full request-send pipeline shared by the desktop
// app and the CLI runner: pre-script -> variable scope -> send -> asserts ->
// post-script, plus the session-scoped runtime variables scripts write to.
package pipeline

import (
	"context"
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

	mu      sync.Mutex
	runtime map[string]string
}

// NewSession builds a fresh session with its own client, jar and vars.
func NewSession() *Session {
	return &Session{HTTP: httpclient.New(), runtime: map[string]string{}}
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
	return vars.Build(layers...)
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

	// Scope built after the pre-script so freshly set runtime vars interpolate.
	scope := s.Scope(collPath, reqPath, envName)
	if err := s.HTTP.Configure(collectionNetConfig(collPath, scope)); err != nil {
		return model.Response{Error: err.Error(), ScriptLogs: scriptLogs}, req.URL
	}
	req.Auth = effectiveAuth(collPath, reqPath, req.Auth)
	resp := s.HTTP.Send(ctx, req, scope)
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
