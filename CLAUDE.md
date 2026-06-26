# CLAUDE.md

Senda — fast, git-native API client. Collections = plain folders of YAML. Built Wails v3 (Go shell) + SolidJS (UI). Ships TUI + headless CLI too.

## Architecture

- **Frontend** (`frontend/`, SolidJS + TS + CodeMirror 6): pure view + local UI state. No HTTP, no disk. Call Go via Wails bindings.
- **Backend** (Go): all network/disk/CPU work. Rule: **touch network, disk, or CPU-heavy → Go.** Frontend only render.
- **Disk = source of truth.** App stateless editor over `*.yaml` collection files.
- Two binaries: desktop GUI (`main.go` + `app*.go`, Wails/CGO/webview → `senda-desktop`) and unified pure-Go `senda` (`cmd/senda` dispatch → TUI default + `run`/`mock`/`docs` subcommands + `gui` launcher that execs `senda-desktop`). TUI in `internal/tui`. Pure-Go binary no frontend/webview, runs anywhere (servers/CI/containers).

### Go packages (`internal/`)

`httpclient` build/send requests, timing/size. `store` read/write/walk collection dirs, YAML, file watch. `model` core structs (Collection, Request, Environment, Response). `vars` resolve `{{var}}`. `auth`, `assert`, `script` (Goja JS sandbox), `runner`, `pipeline`, `history`, `importer`, `codegen`, `docgen`, `schemaval`, `security`, `mockserver`, `load`, `sseclient`, `wsclient`, `aigen`, `gitguard`, `buildinfo`. App-bound API surface lives on one `*App` struct (`internal/app/app*.go`).

### Collection layout (`.senda/`)

A collection root is identified by a `.senda/` dir (`store.isCollectionRoot`). Top level holds only request YAML + request folders; **everything else lives under `.senda/`** (`store.ConfigDirName`):

- `senda.meta.yaml` — collection/folder metadata (name, color, vars, auth). Sub-folders keep their `senda.meta.yaml` inline, not in `.senda/`.
- `environments/<name>.yaml` — shareable env (`store.EnvironmentsDir`). Secret overlay sibling `<name>.secret.yaml`.
- `senda.secret.yaml` — collection-level secret overlay (vars only). **Secrets = any `*.secret.yaml`/`.yml`** (`store.isSecretFile`); excluded from the request tree.
- `history.jsonl` — append-only run log, 500-cap (`internal/history`).
- `mocks/`, `security/` — mock defs, security templates (`security/templates/` is a synced git checkout).

`store.Migrate` (runs on open) moves legacy root-level config into `.senda/`. Secret + history files are the local-only ones git must not see → `internal/gitguard` checks this on open.

### Adding a frontend-callable backend method

Round-trip, all in one change: (1) exported method on `*App` in an `app*.go` file; (2) `task generate:bindings` (regens `frontend/bindings/`, gitignored); (3) wrapper in `frontend/src/lib/api.ts`; (4) stub line in `frontend/src/test-stubs/app.ts` (vitest/dev-mock alias bindings to stubs — missing stub = `undefined` at runtime). `refreshCollection` (`lib/actions.ts`) is the single choke point every collection-open path funnels through. Git = `go-git/v5` library only, never the `git` binary.

## Build & run

Wails v3 via Taskfile (`wails3 build` / `wails3 dev` dispatch to OS-namespaced tasks).

```
wails3 dev              # desktop, live reload
wails3 build            # prod desktop binary (stripped, ~24 MB) into bin/
task build:senda        # bin/senda (pure Go: TUI + run/mock/docs + gui launcher)
task tui -- <path>      # build + run the TUI against a collection
task generate:bindings  # regen TS bindings from Go services
```

`DEBUG=1` keep symbols (unstripped). `VERSION=x.y.z` inject version via ldflags.

## Test & check

```
go test ./...                          # Go tests
cd frontend && bun run test            # vitest
cd frontend && bun run typecheck       # tsc --noEmit
```

## Conventions

- Use `bunx`, not `npx`. Frontend deps via `bun install`.
- Frontend never do I/O — add backend work as bound Go method, regen bindings.
- Go `go 1.25.7`. Module name `senda`.
- Commit messages: no AI attribution trailers — never add `Co-Authored-By: Claude` or `Claude-Session:` lines.

## WebKit e2e (`frontend/tests/e2e/`) — load-bearing selectors

Specs drive real WebKit. Rename/retag any selector below → CI breaks. Touch markup → fix matching spec same change.

- `.tree-leaf` `.tree-folder` `.drop-hover` — Sidebar rows + drag target.
- `.tab-new` `.tab.active .tab-title` `.tab.active .tab-dot.on` — TabBar.
- `button.method-inline` + `.method-menu .method-opt` — verb picker = custom dropdown, NOT `<select>`. Open button, click `.method-opt`.
- `.url-icon-btn.dirty` — dirty/Save reveal.
- `.dlg-input` `.dlg-ok` (Cancel = first `.dlg-btn`) — in-app modal (`components/Dialog.tsx` driven by `lib/dialog.ts`). Replaced native `window.prompt`/`confirm`/`alert` (commit `0bc185f`). Specs must drive these, NOT `page.on("dialog")` (native only, never fires).
- `.tabs button`(Docs) `.docs-toolbar button`(Edit/Preview) `.docs-hint` `iframe.docs-preview`(`sandbox=""`+`srcdoc`).
- `.code-editor` — CM6 host (Body/Docs). `shell-no-scroll.spec` asserts clicking these tabs never scrolls window.
- WebKit can't pierce `sandbox=""` srcdoc iframe → assert `srcdoc` attr, never `frameLocator().locator()` inside.

Critical paths — change one side, update the other in the SAME commit (each pairing below has bitten CI):

- Swap a native `window.prompt`/`confirm`/`alert` → in-app `lib/dialog.ts` helper (or back): grep specs for `page.on("dialog")` and switch them to `.dlg-input`/`.dlg-ok`. Native handler never fires on the custom modal → tab/state never updates → silent assert timeout.
- Rename/retag any selector in the list above: fix the matching spec in the same change (specs match by class, no fallback).
- Add a new `promptDialog`/`confirmDialog` call on a tested flow: the spec must drive `<Dialog/>`, never assume a native popup.

Shell invariant: window must never scroll. `.panes` needs definite `grid-template-rows`; `html,body` stay `overflow:hidden`. height:100% child (CM6) of indefinite parent overflows shell into void. Guarded by `shell-no-scroll.spec`.

Pre-push UI: `cd frontend && bun run test:e2e` (needs WebKit deps; CI has them).