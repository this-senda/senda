# Contributing to Senda

Thanks for your interest. This guide covers how to set up a dev environment,
run the test suite, and submit changes.

## Before you start

Open an issue before starting **large** changes (new features, architectural
refactors). Small fixes (bugs, typos, docs) can go straight to a PR.

By participating, you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md).

## Local setup

```bash
git clone https://github.com/this-senda/senda.git
cd senda
cd frontend && bun install && cd ..
```

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Backend (`go.mod` targets 1.25.7) |
| Bun | 1.x | Frontend (never npm/node) |
| Wails CLI v3 | alpha2.104+ | Dev server + build |
| webkitgtk | 6.0 (4.1 legacy) | Native webview (Linux only) |

Install Wails:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

The default build targets the GTK4 / webkitgtk-6.0 webview; the GTK3 /
webkit2gtk-4.1 stack is a legacy opt-out (`gtk3` build tag).

On **Arch Linux**:

```bash
sudo pacman -S webkitgtk-6.0    # GTK4 webview (default)
# or, for the legacy stack:
sudo pacman -S webkit2gtk-4.1   # GTK3 webview
```

On **Ubuntu / Debian**:

```bash
sudo apt install libwebkitgtk-6.0-dev   # default (legacy: libwebkit2gtk-4.1-dev)
```

### Run in development

```bash
wails3 dev   # live reload, GTK4 / webkitgtk-6.0
```

The legacy GTK3 / webkit2gtk-4.1 webview is selected by a build tag (see the
release-build section below); `wails3 dev` always runs the default GTK4 stack.

## Running tests

All of these must pass before opening a PR:

```bash
# Go backend
go test ./...
gofmt -l .       # must output nothing

# Frontend unit tests (no Wails toolchain needed)
cd frontend && bun run test
```

### Documentation screenshots (optional)

A Playwright script drives the UI headlessly (against `vite --mode test` with a
mocked backend) and rewrites every image under `docs/screenshots/`, including
the walkthrough GIF:

```bash
cd frontend
bunx playwright install chromium   # one-time
bun run shots                       # starts the dev server, captures, encodes the GIF
cp tests/visual/__screenshots__/*.png tests/visual/__screenshots__/*.gif ../docs/screenshots/
```

`task shots` (repo root) does the capture and copy in one step. Where
Playwright's browser CDN is blocked, set `SENDA_CHROME=/path/to/chrome`. See
[`docs/screenshots/README.md`](docs/screenshots/README.md).

## Build a release binary

```bash
wails3 task linux:build         # stripped release (~24 MB) → bin/senda-desktop
wails3 task linux:build:debug   # unstripped, keeps symbols/DWARF (~32 MB)
```

`wails3 build` runs the OS-appropriate `:build` task; swap `linux` for
`darwin` or `windows` to target another platform. Release binaries are
stripped (`-s -w`) by default — reach for `build:debug` when you need symbols
to profile or to debug a release-only crash. To use the legacy GTK3 webview,
override the build tags: `wails3 task linux:build PROD_TAGS="production gtk3"`.

> The `production` build tag is applied by the Taskfile. If you call
> `go build` directly, you must pass `-tags production` or the binary will exit
> with an error at startup.

## Project structure

```
senda/
├── frontend/src/
│   ├── App.tsx                   # 3-pane shell, keyboard shortcuts
│   ├── components/               # UI components
│   └── lib/                      # store, api bindings, actions, helpers
│
├── internal/
│   ├── model/                    # Core structs
│   ├── httpclient/               # HTTP send pipeline
│   ├── pipeline/                 # Unified send pipeline
│   ├── store/                    # YAML serialization, filesystem tree, file watch
│   ├── vars/                     # {{var}} interpolation
│   ├── script/                   # Goja JS sandbox
│   ├── assert/ · schemaval/      # Assertion + JSON Schema evaluation
│   ├── auth/                     # Auth scheme helpers
│   ├── runner/ · load/           # Folder runner + load test engine
│   ├── codegen/ · docgen/        # Code + API-doc generation
│   ├── importer/                 # Import from Postman / OpenAPI / curl
│   ├── mockserver/ · security/   # Mock server + security scanner
│   ├── wsclient/ · sseclient/    # WebSocket + SSE clients
│   ├── history/                  # JSONL-based request history
│   ├── aigen/                    # Optional LLM-assisted assertions
│   ├── tui/                       # Terminal UI (Bubble Tea, no webview)
│   └── termimg/ · buildinfo/     # TUI screenshot renderer + version string
│
├── cmd/senda/                     # Unified pure-Go binary: TUI default + run/mock/docs + gui launcher
├── app.go                        # Wails-bound API surface (IPC)
├── app_features.go               # Import, codegen, runner, mock, WS/SSE
├── app_security.go               # Security-scan bindings
├── app_watch.go                  # File watcher integration
└── docs/                         # architecture, roadmap, design decisions (ADRs)
```

## Code conventions

- **Go**: standard `gofmt` formatting; no linter errors (`go vet ./...`).
- **TypeScript**: `tsc --noEmit` must pass.
- **Frontend package manager**: Bun only — never npm or yarn.
- **Comments**: only when the _why_ is non-obvious.
- **Tests**: new behaviour should include a test. Bugfixes should include a
  regression test.

## Wails bindings

If you add or change a method signature in `app.go` or `app_features.go`, 
regenerate the TypeScript bindings:

```bash
task generate:bindings    # via Taskfile
```

## Architecture docs

- [`docs/architecture.md`](docs/architecture.md) — system design, IPC contract, data model
- [`docs/decisions/`](docs/decisions/) — Architecture Decision Records (ADRs)

## Submitting a PR

1. Branch from `main`.
2. Keep commits focused; use conventional commit messages where possible.
3. All tests pass (`go test ./...` + `cd frontend && bun run test`).
4. Update `docs/` if you changed public behaviour.
5. Open the PR — the template will prompt for details.
