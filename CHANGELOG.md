# Changelog

All notable changes to Senda are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.1.2](https://github.com/this-senda/senda/compare/v0.1.1...v0.1.2) (2026-06-20)


### Features

* **cli:** add --json to senda -v for CI parsing ([#11](https://github.com/this-senda/senda/issues/11)) ([fdd149a](https://github.com/this-senda/senda/commit/fdd149aca55b8ec421692aecc4f09b503f32f1a5))

## [Unreleased]

### Added

- Documentation: a `senda-tui` terminal screenshot gallery and an animated
  walkthrough GIF (open → send → response → tests → command palette → switch
  environment → layouts), shown in the README. They are generated headlessly in
  pure Go — the new `internal/termimg` package renders the real `tuiModel`
  output to PNG and an animated GIF (no PTY, no ffmpeg). Regenerate with
  `task shots:tui`; the Screenshots CI workflow refreshes them alongside the
  desktop images.

### Changed

- **Consolidated the two pure-Go binaries (`senda-cli` + `senda-tui`) into a
  single `senda` binary** with subcommand dispatch: bare `senda` opens the
  terminal UI (falls back to help when there's no TTY, or opens `senda ./dir`
  code-style), `senda run` is the headless CI runner (formerly `senda-cli`),
  `senda mock` / `senda docs` expose the mock server and doc generator, and
  `senda gui` launches the desktop app (`senda-desktop`) detached, `code`-style —
  it execs the GUI binary found beside `senda` or on `$PATH`. The desktop GUI
  stays a separate `senda-desktop` artifact because it links GTK4/WebKit via CGO
  and can't run headless. Release archives now ship `senda` + `senda-desktop`;
  installers gained `SENDA_NO_DESKTOP=1` / `-NoDesktop` to skip the GUI on
  headless hosts. The TUI moved from `cmd/senda-tui` to `internal/tui`; the
  runner moved from `cmd/senda-cli` to `cmd/senda`.
- Collection layout: all non-request files — `senda.meta.yaml`, `senda.secret.yaml`,
  `environments/`, `mocks/` and security templates (formerly `.security/`) — now
  live under a single `.senda/` directory, leaving the collection root with only
  request YAML and folders. Existing collections are migrated into the new layout
  automatically (and idempotently) the first time they are opened. Per-folder
  `senda.meta.yaml` files stay inline next to their requests.

### Documentation

- Synced the README and docs with the shipped feature set — mock server,
  security scanning, WebSocket/SSE, JSON Schema response validation, GraphQL
  introspection, CLI doc generation (`--docs`) and data-driven runs (`--data`),
  and the terminal UI — and added a headless-CLI section.
- Corrected stale build instructions across README/CONTRIBUTING/docs: `wails3`
  (not `wails`), the default GTK4/webkitgtk-6.0 webview with `gtk3` as the legacy
  opt-out (the old `webkit2_41` tag is gone), output at `bin/senda-desktop`, Go
  1.25+, and the binary-size badge (the previous ~5.7 MB was a tag-less stub; a
  stripped build is ~24 MB).
- Added an honest "How this was built" note disclosing that Senda is largely
  AI-assisted.
- Added a `senda-cli` screenshot and animated walkthrough GIF to the README,
  generated reproducibly with `task shots:cli` — it runs the real send pipeline
  against a local in-process server and renders the actual CLI output (a still
  plus a frame-by-frame streaming GIF) via `internal/termimg` (pure Go, no
  network).

---

## [0.1.0] — 2026-06-13

Initial public release — feature-complete for everyday API work, from the
3-pane desktop shell to the headless CLI and terminal UI.

### Added

**Core**
- 3-pane shell: sidebar, request editor, response viewer
- HTTP methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS
- Collections: one folder per collection, one YAML file per request
- Multi-request tabs with state persisted across restarts
- Resizable panes (sidebar / request / response) with persisted sizes

**Request editor**
- URL, method, query params, headers with per-row enable/disable toggles
- Body types: JSON, raw, form-urlencoded, multipart/form-data, GraphQL
- JSON Format button (validates and pretty-prints)
- Auth: Bearer token, Basic, API key, OAuth 2 — per-request or collection-level
- Pre-request and post-request JS scripting (Goja sandbox, 5s runaway guard)
- `senda.setVar()` / `senda.getVar()` — session-scoped runtime variables

**Response viewer**
- Status badge, duration, size at a glance
- Headers and body tabs; body in CodeMirror 6 (virtualized viewport)
- 2 MiB inline cap with "Show anyway" escape hatch; full size still reported

**Environments & variables**
- Named environments stored as YAML alongside requests
- `{{var}}` interpolation in URL, headers, and body
- Precedence: runtime (script) → environment → collection base → request
- Secrets: `*.secret.yaml` files gitignored and merged at send-time only

**Testing & assertions**
- Assertions per request: target / operator / value rows
- Targets: status, duration, body size, JSON path, header values, raw body
- Operators: `eq`, `neq`, `contains`, `notcontains`, `matches` (regex), `gt/gte/lt/lte`, `exists`, `notexists`
- Assertions run on every send and in folder/load test runs

**Running collections**
- Folder runner: sequential execution with timing and assertion results per request; live-streaming results
- Load testing: concurrent VU mode with configurable duration, target RPS, max VUs; p50/p95/p99 latency streamed live

**Import & code generation**
- Import from: curl commands, Postman v2.0+ collections, OpenAPI 3.0 specs
- Generate code: curl, fetch, httpie, Python `requests`, Go `net/http`

**Developer experience**
- Command palette (`Ctrl+K` / `Ctrl+P`): fuzzy-search requests, switch environments, trigger actions
- Theming: 13 built-in themes (Catppuccin Latte/Frappé/Macchiato/Mocha, Nord, VS Code Light/Dark, monochrome, pastel variants) with independent light/dark picks and system-follow mode
- Keyboard shortcuts: `Ctrl+T` new tab, `Ctrl+N` new request, `Ctrl+W` close tab, `Ctrl+S` save, `Ctrl+Enter` send, `Ctrl+Tab` cycle tabs
- Request history: every sent request logged to `.senda/history.jsonl`; browsable in the history panel
- File watch: auto-refreshes when a YAML file is edited externally
- Drag-and-drop: reorder requests and folders in the tree
- Cookie jar: persistent session cookies across sends and runs
- Environment editor: create and edit environments from the UI
- Docs tab: per-request markdown notes stored in YAML

**CLI runner**
- `senda-cli` binary: same send pipeline as the desktop app (scripts, vars, secrets, asserts, cookies); scriptable in CI; exit 1 on any failure

**Tech stack**
- Wails v3 (Go) desktop shell + SolidJS + TypeScript frontend
- ~24 MB binary, ~100 MB RAM (vs ~150 MB binary / ~400 MB RAM for Electron)
- CodeMirror 6 for all editing (virtualized — handles 20+ MB without freeze)
- Goja (Go-native JS engine) for scripting — no Node dependency
- Plain YAML storage — `git diff` on your requests just works

[0.1.0]: https://github.com/this-senda/senda/releases/tag/v0.1.0
