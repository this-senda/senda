package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sync"

	"gopkg.in/yaml.v3"

	"senda/internal/aigen"
	"senda/internal/codegen"
	"senda/internal/docgen"
	"senda/internal/history"
	"senda/internal/importer"
	"senda/internal/load"
	"senda/internal/mockserver"
	"senda/internal/model"
	"senda/internal/pipeline"
	"senda/internal/runner"
	"senda/internal/sseclient"
	"senda/internal/store"
	"senda/internal/wsclient"
)

// PickDirectory opens the native folder chooser for a collection directory and
// returns the selected absolute path, or "" if the user cancelled. This is
// folder-only: the Linux/GTK native dialog has mutually exclusive folder and
// file modes (GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER vs _OPEN), so a single
// dialog cannot offer both. Use PickZipCollection for packed .zip archives.
func (a *App) PickDirectory(title string) (string, error) {
	if err := a.requireWails(); err != nil {
		return "", err
	}
	return a.wails.Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		PromptForSingleSelection()
}

// PickZipCollection opens the native file chooser filtered to packed .zip
// collections and returns the selected absolute path, or "" if cancelled. It is
// the file-mode counterpart to PickDirectory; see that method for why folders
// and files need separate dialogs on Linux.
func (a *App) PickZipCollection(title string) (string, error) {
	if err := a.requireWails(); err != nil {
		return "", err
	}
	return a.wails.Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(true).
		AddFilter("Senda collection (.zip)", "*.zip").
		PromptForSingleSelection()
}

// PickFile opens the native file chooser and returns the selected absolute
// path, or "" if the user cancelled.
func (a *App) PickFile(title string) (string, error) {
	if err := a.requireWails(); err != nil {
		return "", err
	}
	return a.wails.Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(true).
		PromptForSingleSelection()
}

// ExportFile prompts for a save location (suggesting filename) and writes
// content there. Returns the chosen path, or "" if the user cancelled.
func (a *App) ExportFile(filename, content string) (string, error) {
	if err := a.requireWails(); err != nil {
		return "", err
	}
	path, err := a.wails.Dialog.SaveFile().
		SetFilename(filename).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(content), 0o644)
}

// ListCookies returns the session jar's cookies that apply to rawURL.
func (a *App) ListCookies(rawURL string) ([]model.Cookie, error) {
	return a.session.HTTP.Cookies(rawURL)
}

// ClearCookies wipes the session cookie jar.
func (a *App) ClearCookies() {
	a.session.HTTP.ClearCookies()
}

// ImportCurl parses a curl command line into a Request (not persisted; the UI
// opens it as an unsaved tab).
func (a *App) ImportCurl(cmd string) (model.Request, error) {
	return importer.Curl(cmd)
}

// ImportCollection parses external collection data (format: "postman" or
// "openapi") and writes each request as a YAML file under collPath/destSubdir,
// mirroring the source's folder structure. Returns the number written.
func (a *App) ImportCollection(collPath, format, data, destSubdir string) (int, error) {
	if collPath == "" {
		return 0, fmt.Errorf("no collection open")
	}
	var items []importer.Imported
	var err error
	switch format {
	case "postman":
		items, err = importer.Postman([]byte(data))
	case "openapi":
		items, err = importer.OpenAPI([]byte(data))
	default:
		return 0, fmt.Errorf("unknown import format %q", format)
	}
	if err != nil {
		return 0, err
	}

	base := collPath
	if destSubdir != "" {
		base = filepath.Join(collPath, destSubdir)
	}
	count := 0
	for _, im := range items {
		dir := base
		for _, seg := range im.Dir {
			dir = filepath.Join(dir, seg)
		}
		path := uniquePath(dir, im.Request.Name)
		if err := store.SaveRequest(path, im.Request); err != nil {
			return count, err
		}
		count++
	}

	// OpenAPI carries collection-level info: a description for the destination
	// folder and one environment per server (supplying {{baseUrl}}).
	if format == "openapi" {
		if desc, envs, err := importer.OpenAPIMeta([]byte(data)); err == nil {
			if desc != "" {
				meta := store.ReadMeta(base)
				meta.Description = desc
				_ = store.SaveCollection(meta)
			}
			for _, env := range envs {
				_ = store.SaveEnvironment(collPath, env)
			}
		}
	}
	return count, nil
}

// RenderMarkdown converts request Docs markdown into an HTML body fragment for
// the editor's Docs preview. Reuses the docgen renderer so the in-app preview
// matches the exported documentation.
func (a *App) RenderMarkdown(md string) string {
	return docgen.RenderFragment(md)
}

// GenerateMocksFromOpenAPI parses an OpenAPI 3 document and writes one mock
// definition file per operation into collPath/.senda/mocks/, so the mock server can
// serve the API straight from its documented responses/examples. Returns the
// number of mock files written.
func (a *App) GenerateMocksFromOpenAPI(collPath, data string) (int, error) {
	if collPath == "" {
		return 0, fmt.Errorf("no collection open")
	}
	defs, err := importer.OpenAPIMocks([]byte(data))
	if err != nil {
		return 0, err
	}
	mocksDir := store.MocksDir(collPath)
	if err := os.MkdirAll(mocksDir, 0o755); err != nil {
		return 0, err
	}
	count := 0
	for _, def := range defs {
		out, err := yaml.Marshal(def)
		if err != nil {
			return count, err
		}
		// Stable filename per operation so regenerating the same spec overwrites
		// rather than piling up get-stations-2.yaml duplicates (uniquePath would).
		dest := filepath.Join(mocksDir, slugify(def.Name)+".yaml")
		if err := os.WriteFile(dest, out, 0o644); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// MockPresets returns the names of the bundled mock presets that
// ScaffoldMockPreset can write (e.g. "oauth").
func (a *App) MockPresets() []string {
	return mockserver.Presets()
}

// ScaffoldMockPreset writes the YAML files of a bundled preset into
// collPath/mocks/, leaving any existing files untouched. Returns the names of
// the files actually written.
func (a *App) ScaffoldMockPreset(collPath, preset string) ([]string, error) {
	if collPath == "" {
		return nil, fmt.Errorf("no collection open")
	}
	mocksDir := store.MocksDir(collPath)
	written, _, err := mockserver.WritePreset(preset, mocksDir)
	return written, err
}

// PreviewMockRoutes loads collPath/mocks/ without starting a server and returns
// the routes it would serve, so the panel can show what exists before Start.
func (a *App) PreviewMockRoutes(collPath string) []mockserver.RouteInfo {
	if collPath == "" {
		return nil
	}
	mocksDir := store.MocksDir(collPath)
	srv, err := mockserver.New(mocksDir, nil, nil)
	if err != nil {
		return nil
	}
	return srv.Routes()
}

// uniquePath returns dir/name.yaml, suffixing -2, -3, … if taken.
func uniquePath(dir, name string) string {
	if name == "" {
		name = "imported"
	}
	candidate := filepath.Join(dir, name+".yaml")
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d.yaml", name, i))
	}
}

// GenerateCode renders a request as a snippet for the given target (curl,
// fetch, httpie, python, go).
func (a *App) GenerateCode(req model.Request, target string) (string, error) {
	return codegen.Generate(req, target)
}

// CodegenTargets lists the supported code-generation targets.
func (a *App) CodegenTargets() []string {
	return codegen.Targets
}

// RunFolder sends every request under folderPath in sequence and returns a
// per-request result. Variables resolve against the named environment.
// Progress streams to the frontend as "run:start" / "run:result" events so the
// UI fills in rows while slow requests are still in flight.
func (a *App) RunFolder(ctx context.Context, folderPath, collPath, envName string) ([]model.RunResult, error) {
	paths, err := store.ListRequests(folderPath)
	if err != nil {
		return nil, err
	}
	a.emit("run:start", map[string]any{"total": len(paths)})
	send := func(ctx context.Context, path string) (model.Request, model.Response, error) {
		req, err := store.ReadRequest(path)
		if err != nil {
			return req, model.Response{}, err
		}
		resp, appliedURL := a.session.Send(ctx, req, collPath, path, envName)
		// Record each request so the sidebar recency pills update after a folder
		// run, the same as a single send does.
		recordSend(collPath, req, resp, appliedURL)
		return req, resp, nil
	}
	return runner.RunFolder(ctx, paths, send, func(res model.RunResult) {
		a.emit("run:result", res)
	}), nil
}

// RunLoad runs a concurrent load test against every request under folderPath.
// Progress streams to the frontend as "load:tick" events (once per second).
// Each VU gets its own pipeline session so cookies and runtime vars are isolated.
func (a *App) RunLoad(ctx context.Context, folderPath, collPath, envName string, opts model.LoadOptions) (model.LoadSummary, error) {
	paths, err := store.ListRequests(folderPath)
	if err != nil {
		return model.LoadSummary{}, err
	}
	// Pre-read all request files once; they are static during the test.
	reqCache := make(map[string]model.Request, len(paths))
	for _, p := range paths {
		req, err := store.ReadRequest(p)
		if err != nil {
			return model.LoadSummary{}, err
		}
		reqCache[p] = req
	}
	factory := func() load.Send {
		sess := pipeline.NewSession()
		return func(ctx context.Context, path string) (model.Request, model.Response, error) {
			req := reqCache[path]
			resp, _ := sess.Send(ctx, req, collPath, path, envName)
			return req, resp, nil
		}
	}
	sum := load.Run(ctx, paths, opts, factory, func(t model.LoadTick) {
		a.emit("load:tick", t)
	})
	return sum, nil
}

// RenameNode renames a request file or folder, returning its new path.
func (a *App) RenameNode(path, newName string) (string, error) {
	return store.RenameNode(path, newName)
}

// MoveNode moves a request file or folder into destDir, returning its new path.
func (a *App) MoveNode(srcPath, destDir string) (string, error) {
	return store.MoveNode(srcPath, destDir)
}

// ListHistory returns recent sent-request entries for a collection (newest
// first, capped at limit; pass 0 for the default cap).
func (a *App) ListHistory(collPath string, limit int) ([]model.HistoryEntry, error) {
	return history.List(collPath, limit)
}

// ClearHistory deletes a collection's history log.
func (a *App) ClearHistory(collPath string) error {
	return history.Clear(collPath)
}

// CollectionActivity returns the last-run summary per request path for a
// collection, derived from its history log. The sidebar uses it to draw
// recency pills on requests and (rolled up) folders.
func (a *App) CollectionActivity(collPath string) (map[string]store.Activity, error) {
	return store.CollectionActivity(collPath)
}

// GenerateAssertions uses an LLM to suggest test assertions for the given
// response. Requires SENDA_AI_API_KEY (or ANTHROPIC_API_KEY) to be set.
func (a *App) GenerateAssertions(ctx context.Context, resp model.Response) ([]model.Assert, error) {
	cfg := aigen.ResolveConfig()
	return aigen.GenerateAssertions(ctx, cfg, resp)
}

// AIConfigured returns true when an AI API key is available.
func (a *App) AIConfigured() bool {
	cfg := aigen.ResolveConfig()
	return cfg.APIKey != ""
}

// ConnectWebSocket opens a WebSocket connection and returns the full session
// log (all messages sent and received until the connection closes or ctx is
// cancelled).
func (a *App) ConnectWebSocket(ctx context.Context, req model.Request, collPath, envName string) model.WSSession {
	scope := a.session.Scope(collPath, "", envName)
	return wsclient.Connect(ctx, req, scope)
}

// OpenWebSocket opens an interactive WebSocket connection that stays alive
// across calls. Received messages and the close event are emitted to the
// frontend as "ws:event" so the connection survives switching request tabs.
// Returns a connection id for SendWebSocketMessage / CloseWebSocket.
func (a *App) OpenWebSocket(ctx context.Context, req model.Request, collPath, envName string) (string, error) {
	scope := a.session.Scope(collPath, "", envName)
	return a.ws.Open(ctx, req, scope, func(e wsclient.WSEvent) {
		a.emit("ws:event", e)
	})
}

// SendWebSocketMessage sends a text message over an open connection.
func (a *App) SendWebSocketMessage(id, message string) error {
	return a.ws.Send(id, message)
}

// CloseWebSocket closes an open connection. Unknown id is a no-op.
func (a *App) CloseWebSocket(id string) error {
	return a.ws.Close(id)
}

// ConnectSSE connects to an SSE endpoint and returns all events received
// until the connection closes or ctx is cancelled. Events are also streamed
// to the frontend as "sse:event" events for real-time display.
func (a *App) ConnectSSE(ctx context.Context, req model.Request, collPath, envName string) model.SSESession {
	scope := a.session.Scope(collPath, "", envName)
	return sseclient.Connect(ctx, req, scope, func(e model.SSEEvent) {
		if a.wails != nil {
			a.wails.Event.Emit("sse:event", e)
		}
	})
}

// --- Mock server ---

var mockMu sync.Mutex

// StartMockServer starts a local mock server reading definitions from
// collPath/mocks/. Returns the bound address.
func (a *App) StartMockServer(collPath, addr string) (string, error) {
	mockMu.Lock()
	defer mockMu.Unlock()
	if a.mockServer != nil {
		_ = a.mockServer.Stop()
	}
	mocksDir := store.MocksDir(collPath)
	srv, err := mockserver.New(mocksDir, func(entry mockserver.LogEntry) {
		a.emit("mock:log", entry)
	}, func() {
		a.emit("mock:routes", nil)
	})
	if err != nil {
		return "", err
	}
	if addr == "" {
		addr = ":8787"
	}
	bound, err := srv.Start(addr)
	if err != nil {
		return "", err
	}
	a.mockServer = srv
	return bound, nil
}

// StopMockServer stops the running mock server.
func (a *App) StopMockServer() error {
	mockMu.Lock()
	defer mockMu.Unlock()
	if a.mockServer == nil {
		return nil
	}
	err := a.mockServer.Stop()
	a.mockServer = nil
	return err
}

// MockServerRoutes returns the loaded routes of the running mock server.
func (a *App) MockServerRoutes() []mockserver.RouteInfo {
	if a.mockServer != nil {
		return a.mockServer.Routes()
	}
	return nil
}

// MockServerLog returns the request log of the running mock server.
func (a *App) MockServerLog() []mockserver.LogEntry {
	if a.mockServer != nil {
		return a.mockServer.Log()
	}
	return nil
}

// MockServerInfo returns the running server's config (address, active scenario,
// proxy, CORS, declared scenarios) for the panel.
func (a *App) MockServerInfo() mockserver.Info {
	if a.mockServer != nil {
		return a.mockServer.Info()
	}
	return mockserver.Info{}
}

// SetMockScenario switches the active scenario on the running server.
func (a *App) SetMockScenario(name string) {
	if a.mockServer != nil {
		a.mockServer.SetScenario(name)
	}
}

// ResetMockState restores the running server's resource records to their seeds.
func (a *App) ResetMockState() {
	if a.mockServer != nil {
		a.mockServer.ResetState()
	}
}

// SetMockRouteResponse forces the rule route at method+path to return the
// response variant with the given status code (0 clears the override). This
// lets a tester live-switch what an endpoint returns without editing files.
func (a *App) SetMockRouteResponse(method, path string, status int) {
	if a.mockServer != nil {
		a.mockServer.SetRouteResponse(method, path, status)
	}
}

// SaveResponseAsMock writes a mock definition file under collPath/mocks/ from a
// captured response, so a real round-trip can be turned into a fixture.
func (a *App) SaveResponseAsMock(collPath, name, method, path string, status int, headers map[string]string, body string) (string, error) {
	if collPath == "" {
		return "", fmt.Errorf("no collection")
	}
	mocksDir := store.MocksDir(collPath)
	if err := os.MkdirAll(mocksDir, 0o755); err != nil {
		return "", err
	}
	def := mockserver.MockDef{
		Name:    name,
		Method:  method,
		Path:    path,
		Status:  status,
		Headers: headers,
		Body:    body,
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(mocksDir, slugify(name)+".yaml")
	if err := os.WriteFile(dest, out, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

// slugify turns a mock name into a safe lowercase filename stem.
func slugify(name string) string {
	if name == "" {
		return "mock"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "mock"
	}
	return s
}
