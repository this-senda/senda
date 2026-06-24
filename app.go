package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"senda/internal/fake"
	"senda/internal/mockserver"
	"senda/internal/model"
	"senda/internal/pipeline"
	"senda/internal/store"
)

// App is the Wails-bound facade. Every exported method is callable from the
// SolidJS frontend. It holds no UI state beyond the pipeline session (cookie
// jar + script-set runtime vars) — disk is the source of truth.
type App struct {
	session    *pipeline.Session
	wails      *application.App   // set in main after application.New; nil in tests
	mockServer *mockserver.Server // the running mock server, or nil when stopped
}

// NewApp constructs the application backend.
func NewApp() *App {
	return &App{session: pipeline.NewSession()}
}

// Ping is a trivial liveness check used by the frontend to confirm bindings.
func (a *App) Ping() string { return "senda-ok" }

// SendRequest runs the full pipeline (scripts, vars, asserts) and records the
// send in the collection history.
func (a *App) SendRequest(ctx context.Context, req model.Request, collPath, reqPath, envName string) model.Response {
	resp, appliedURL := a.session.Send(ctx, req, collPath, reqPath, envName)
	recordSend(collPath, req, resp, appliedURL)
	return resp
}

// FakerTokens lists the namespaced {{$token}} faker generators (from gofakeit's
// catalog) for editor autocomplete, so the frontend needn't hardcode the set.
func (a *App) FakerTokens() []fake.Token {
	return fake.Tokens()
}

// ListRuntimeVars exposes the session's script-set variables to the UI.
func (a *App) ListRuntimeVars() []model.KV {
	return a.session.RuntimeKVs()
}

// ScopeVar is one resolved variable surfaced to the UI for {{var}} highlighting
// and autocomplete. Source is the layer the value resolves from. Secret values
// are masked (Value is "") — they live server-side and never reach the client.
type ScopeVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"` // collection | folder | env | secret | runtime
}

// ResolveScope returns the variable scope for one request, mirroring the
// pipeline's resolution precedence (collection -> folder chain root-first -> env
// -> runtime, later layers win) so the frontend can highlight {{var}} tokens
// against the same names the send will actually resolve. reqPath may be empty
// for ad-hoc sends (only collection/env/runtime apply, no folder chain).
func (a *App) ResolveScope(collPath, reqPath, envName string) []ScopeVar {
	order := []string{}
	byKey := map[string]ScopeVar{}
	set := func(kvs []model.KV, source string, secret bool) {
		for _, kv := range kvs {
			if kv.Key == "" {
				continue
			}
			if !secret && !kv.Enabled {
				continue
			}
			if _, seen := byKey[kv.Key]; !seen {
				order = append(order, kv.Key)
			}
			val := kv.Value
			if secret {
				val = ""
			}
			byKey[kv.Key] = ScopeVar{Key: kv.Key, Value: val, Source: source}
		}
	}
	if collPath != "" {
		set(store.ReadMeta(collPath).Vars, "collection", false)
		set(store.CollectionSecrets(collPath), "secret", true)
		for _, dir := range store.FolderChain(collPath, reqPath) {
			set(store.ReadMeta(dir).Vars, "folder", false)
		}
		if envName != "" {
			if envs, err := store.ListEnvironments(collPath); err == nil {
				for _, e := range envs {
					if e.Name == envName {
						set(e.Vars, "env", false)
						break
					}
				}
			}
			set(store.EnvironmentSecrets(collPath, envName), "secret", true)
		}
	}
	set(a.session.RuntimeKVs(), "runtime", false)

	out := make([]ScopeVar, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

// ClearRuntimeVars wipes all script-set variables.
func (a *App) ClearRuntimeVars() {
	a.session.ClearRuntime()
}

// OpenCollection loads a collection by directory path and (re)starts the
// file watcher on it.
func (a *App) OpenCollection(path string) (model.Collection, error) {
	c, err := store.OpenCollection(path)
	if err == nil {
		a.watchCollection(path)
	}
	return c, err
}

// SaveCollection persists collection metadata (name, vars, auth) to senda.meta.yaml.
func (a *App) SaveCollection(c model.Collection) error {
	return store.SaveCollection(c)
}

// PackCollection folds edits back into the source .zip for an archive-backed
// collection (no-op for plain directory collections). path is the live path
// returned by OpenCollection. Call on an explicit save action.
func (a *App) PackCollection(path string) error {
	return store.PackArchive(path)
}

// ReadFolderMeta loads a folder's (or collection root's) senda.meta.yaml metadata
// — color, tags, description, vars, auth — without building its tree. Used by
// the folder settings dialog. A folder with no senda.meta.yaml yields zero metadata
// with Name defaulted to the directory name.
func (a *App) ReadFolderMeta(path string) model.Collection {
	return store.ReadMeta(path)
}

// ReadRequest loads a single request file.
func (a *App) ReadRequest(path string) (model.Request, error) {
	return store.ReadRequest(path)
}

// SaveRequest persists a request to disk.
func (a *App) SaveRequest(path string, req model.Request) error {
	return store.SaveRequest(path, req)
}

// DeleteRequest removes a request file.
func (a *App) DeleteRequest(path string) error {
	return store.DeleteRequest(path)
}

// DeleteNode removes a request file or a folder (recursively).
func (a *App) DeleteNode(path string) error {
	return store.DeleteNode(path)
}

// CreateFolder makes a new folder in the collection tree.
func (a *App) CreateFolder(path string) error {
	return store.CreateFolder(path)
}

// ListEnvironments returns all environments for a collection.
func (a *App) ListEnvironments(collPath string) ([]model.Environment, error) {
	return store.ListEnvironments(collPath)
}

// SaveEnvironment writes one environment file.
func (a *App) SaveEnvironment(collPath string, env model.Environment) error {
	return store.SaveEnvironment(collPath, env)
}
