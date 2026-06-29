# Senda — Architecture

> Status: Draft · Last updated: 2026-06-09

## 1. High-level

```
┌─────────────────────────────────────────────┐
│                 Senda (Wails app)             │
│                                              │
│  ┌────────────────┐      ┌────────────────┐  │
│  │  Frontend       │ IPC  │  Backend (Go)  │  │
│  │  SolidJS + TS   │<────>│                │  │
│  │  CodeMirror 6   │ Wails│  HTTP engine   │  │
│  │  (OS webview)   │ bind │  File store    │  │
│  │                 │      │  Var resolver  │  │
│  └────────────────┘      └────────────────┘  │
│                                  │            │
└──────────────────────────────────┼───────────┘
                                    ▼
                          ~/collections/*.yaml
                           (plain files, git)
```

- **Frontend** = pure view + local UI state. No HTTP, no disk. Calls Go via Wails bindings.
- **Backend (Go)** = all heavy/privileged work: send HTTP, read/write files, resolve variables, format large bodies.
- **Disk** = source of truth. App is a stateless editor over files.

Rule: **if it touches network, disk, or is CPU-heavy → Go.** Frontend only renders.

## 2. Backend (Go) — modules

| Package | Responsibility |
|---------|---------------|
| `httpclient` | Build + send `*http.Request`, capture timing/size, stream response. |
| `store` | Read/write/walk collection dirs. YAML (de)serialize. File watch. |
| `model` | Core structs: `Collection`, `Request`, `Environment`, `Response`. |
| `vars` | Resolve `{{var}}` against active env + collection vars. |
| `app` | Wails-bound API surface (the methods frontend calls). |

### 2.1 Bound API (frontend ↔ Go contract)

Initial surface (grows as features land):

```go
// app/api.go — methods exposed to frontend via Wails
func (a *App) ListCollections() ([]CollectionMeta, error)
func (a *App) OpenCollection(path string) (Collection, error)
func (a *App) ReadRequest(path string) (Request, error)
func (a *App) SaveRequest(path string, req Request) error
func (a *App) DeleteRequest(path string) error
func (a *App) SendRequest(req Request, envName string) (Response, error)
func (a *App) ListEnvironments(collPath string) ([]Environment, error)
func (a *App) SaveEnvironment(collPath string, env Environment) error
```

## 3. Data model

```go
type Request struct {
    Name    string            `yaml:"name"`
    Method  string            `yaml:"method"`
    URL     string            `yaml:"url"`
    Params  []KV              `yaml:"params,omitempty"`   // query params
    Headers []KV              `yaml:"headers,omitempty"`
    Body    Body              `yaml:"body,omitempty"`
}

type Body struct {
    Type string `yaml:"type"` // none | json | form | raw | multipart
    Raw  string `yaml:"raw,omitempty"`
    Form []KV   `yaml:"form,omitempty"`
}

type KV struct {
    Key      string `yaml:"key"`
    Value    string `yaml:"value"`
    Enabled  bool   `yaml:"enabled"`
    Desc     string `yaml:"desc,omitempty"`
}

type Environment struct {
    Name string `yaml:"name"`
    Vars []KV   `yaml:"vars"`
}

type Response struct {
    Status     int               // 200
    StatusText string            // "OK"
    DurationMs int64
    SizeBytes  int64
    Headers    map[string][]string
    Body       []byte            // streamed; large bodies handled separately
    Truncated  bool              // true if body exceeded inline limit
}
```

## 4. On-disk layout

```
my-api/                      # a collection = a directory
├── senda.meta.yaml                # collection meta (name, base vars)
├── environments/
│   ├── dev.yaml
│   └── prod.yaml            # secrets gitignored, see §6
├── auth/
│   ├── login.yaml           # one request per file
│   └── refresh.yaml
└── users/
    ├── list-users.yaml
    └── create-user.yaml
```

- **One request = one YAML file.** Cleanest git diffs.
- **Folder tree = collection tree.** Filesystem is the structure; no index file to drift.
- Example request file:

```yaml
name: Create user
method: POST
url: "{{baseUrl}}/users"
headers:
  - { key: Content-Type, value: application/json, enabled: true }
  - { key: Authorization, value: "Bearer {{token}}", enabled: true }
body:
  type: json
  raw: |
    { "name": "Ada", "email": "ada@example.com" }
```

## 5. Large-response strategy (the key perf design)

The one thing that kills naive API clients. Designed for upfront:

1. Go streams response, records full size.
2. If body ≤ **inline limit** (e.g. 2MB) → send to frontend, pretty-print, CodeMirror render.
3. If body > limit → send **head slice + metadata**, set `Truncated=true`. Frontend shows "Response too large (N MB) — [View raw] [Save to file]".
4. Pretty-printing/formatting of big JSON done **in Go**, never on UI thread.
5. CodeMirror 6 only renders visible lines (virtualized) — even 2MB stays smooth.

## 6. Variable resolution

- Precedence: **request-local → environment → collection vars**.
- Resolution happens in **Go** (`vars` pkg) at send time, not in UI.
- `{{var}}` syntax. Unresolved var → surfaced as warning, not silent blank.
- Secrets: values in `environments/*.secret.yaml` (and collection-level `senda.secret.yaml`), gitignored by default; merged at runtime, never written to the committed env file. Optionally encrypted at rest (`internal/secretcrypt`, AES-256-GCM) into a YAML envelope; the 32-byte key is resolved per-collection from `SENDA_SECRET_KEY`, the OS keychain, or `~/.config/senda/keys/<keyID>.key` — never stored beside the ciphertext. The `secretsEncrypted`/`secretsKeyID` meta flags drive it; reads transparently decrypt, so the headless CLI works with the key supplied via env.

## 7. Frontend (SolidJS)

- **State**: Solid stores/signals. One store per open request (the editor model), one for the collection tree, one for active env.
- **No global heavy state** — request bodies/responses held transiently, not in a giant reactive blob.
- **Components**: `Sidebar` (tree), `RequestEditor` (tabs: params/headers/body), `ResponseViewer` (tabs: body/headers/timing), `EnvSwitcher`.
- **Editor**: CodeMirror 6 instances for body + response. Lazy-init, dispose on tab close.
- **IPC**: thin `api.ts` wrapper over generated Wails bindings; all calls async, show pending state.
- **Theming**: `lib/theme.ts` registry of CSS-variable token maps applied inline on `<html>`; light/dark/system mode, persisted per side. See `docs/theming.md`.

## 8. Threading / responsiveness rules

- UI thread never does: HTTP, file IO, JSON pretty-print of large data, syntax highlight of full big payload.
- All of above → Go (Wails runs Go bindings off the webview thread).
- Frontend shows optimistic/pending states; awaits Go result.

## 9. Build / dev

- `wails dev` — hot reload frontend + Go.
- `wails build` — single binary per platform.
- Frontend: Vite + SolidJS + TS. Lint + typecheck in CI.
- Go: standard `go test ./...`. `httpclient` + `vars` + `store` unit-tested with table tests.

## 10. Risks

| Risk | Mitigation |
|------|-----------|
| Large-response freeze | §5 strategy: 2 MiB inline cap + viewport rendering. |
| CodeMirror 6 + Solid wiring (fewer wrappers) | thin custom integration, isolate in one component. |
| YAML edge cases (multiline body, unicode) | golden-file round-trip tests in `store`. |
| Scope creep into scripting/runner | deferred to post-MVP — see ADR-0004. |
| Webview inconsistency across OS (WebView2/WebKit) | test matrix; avoid bleeding-edge CSS. |
