# CLAUDE.md

Senda — fast, git-native API client. Collections = plain folders of YAML. Built Wails v3 (Go shell) + SolidJS (UI). Ships TUI + headless CLI too.

## Architecture

- **Frontend** (`frontend/`, SolidJS + TS + CodeMirror 6): pure view + local UI state. No HTTP, no disk. Call Go via Wails bindings.
- **Backend** (Go): all network/disk/CPU work. Rule: **touch network, disk, or CPU-heavy → Go.** Frontend only render.
- **Disk = source of truth.** App stateless editor over `*.yaml` collection files.
- Two binaries: desktop GUI (`main.go` + `app*.go`, Wails/CGO/webview → `senda-desktop`) and unified pure-Go `senda` (`cmd/senda` dispatch → TUI default + `run`/`mock`/`docs` subcommands + `gui` launcher execs `senda-desktop`). TUI in `internal/tui`. Pure-Go binary no frontend/webview, runs anywhere (servers/CI/containers).

### Go packages (`internal/`)

`httpclient` build/send requests, timing/size. `store` read/write/walk collection dirs, YAML, file watch. `model` core structs (Collection, Request, Environment, Response). `vars` resolve `{{var}}`. `auth`, `assert`, `script` (Goja JS sandbox), `runner`, `pipeline`, `history`, `importer`, `codegen`, `docgen`, `schemaval`, `security`, `mockserver`, `load`, `sseclient`, `wsclient`, `aigen`, `gitguard`, `buildinfo`. App-bound API surface on one `*App` struct (`internal/app/app*.go`).

### Collection layout (`.senda/`)

Collection root identified by `.senda/` dir (`store.isCollectionRoot`). Top level holds only request YAML + request folders; **everything else under `.senda/`** (`store.ConfigDirName`):

- `senda.meta.yaml` — collection/folder metadata (name, color, vars, auth). Sub-folders keep `senda.meta.yaml` inline, not in `.senda/`.
- `environments/<name>.yaml` — shareable env (`store.EnvironmentsDir`). Secret overlay sibling `<name>.secret.yaml`.
- `senda.secret.yaml` — collection-level secret overlay (vars only). **Secrets = any `*.secret.yaml`/`.yml`** (`store.isSecretFile`); excluded from request tree.
- `history.jsonl` — append-only run log, 500-cap (`internal/history`).
- `mocks/`, `security/` — mock defs, security templates (`security/templates/` = synced git checkout).

`store.Migrate` (runs on open) moves legacy root-level config into `.senda/`. Secret + history files = local-only, git must not see → `internal/gitguard` checks on open.

### Adding a frontend-callable backend method

Round-trip, one change: (1) exported method on `*App` in `app*.go` file; (2) `task generate:bindings` (regens `frontend/bindings/`, gitignored); (3) wrapper in `frontend/src/lib/api.ts`; (4) stub line in `frontend/src/test-stubs/app.ts` (vitest/dev-mock alias bindings to stubs — missing stub = `undefined` at runtime). `refreshCollection` (`lib/actions.ts`) = single choke point every collection-open path funnels through. Git = `go-git/v5` library only, never `git` binary.

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
- User-facing feature added/changed → update docs across ALL of `docs/` + README, not just `*.md`. Grep `docs/` for feature area. Specifically: `README.md` Features list, `docs/roadmap.md` (move shipped items out of "Next"), and **`docs/index.html`** — hand-maintained public landing page with curated feature-card grid (append to existing card, don't break grid). `CHANGELOG.md` = release-please auto-generated — never hand-edit.
- Two settings modals edit SAME `senda.meta.yaml`, don't mix up: `CollectionSettings.tsx` = collection ROOT (opened from collection context menu → "Collection settings"; reads `collection()` from store), `FolderSettings.tsx` = sub-folders (opened from folder's context menu; takes `props.path`). Collection-root-only config (proxy/TLS, default auth) lives in `CollectionSettings`. `FolderSettings` has `isRoot`-style path check available but NEVER opened for root — putting root-only UI behind check there makes it unreachable. New bound `Git*`/meta App method touched by either modal → mirror in `tests/visual/mock-backend.mjs` (see e2e note below).
- New field on `model.Collection` → add to BOTH hand-copy sites in `store.go` (`ReadMeta` and `SaveCollection` copy fields explicitly, not via struct assignment) or silently won't persist.

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
- `.scm-open` `.scm-row` `.scm-badge` `.scm-section-head` `.scm-field-label` `.scm-old` `.scm-new` `.scm-raw` `.scm-diff-empty` — source-control (git comparison) modal (`SourceControlPanel.tsx`). `source-control.spec` drives them off mocked `GitStatus`/`GitDiff` in `tests/visual/mock-backend.mjs` — add a method to mock when you add bound `Git*` App method, or panel renders empty.

TWO mocks, don't confuse: **e2e** webServer runs `bun run dev:mock` (`vite --mode test` → `installDevMock`), so playwright specs hit **`frontend/src/lib/devMock.ts`**. `tests/visual/mock-backend.mjs` = separate **visual screenshot** harness (`shoot.mjs`, `addInitScript`). Bound App method a spec calls (e.g. `PickFile`) must exist in **devMock.ts** or returns undefined and flow silently no-ops — missing method does NOT error. Add to mock-backend.mjs too only if visual harness renders that path.

Critical paths — change one side, update the other in SAME commit (each pairing below has bitten CI):

- Swap native `window.prompt`/`confirm`/`alert` → in-app `lib/dialog.ts` helper (or back): grep specs for `page.on("dialog")` and switch to `.dlg-input`/`.dlg-ok`. Native handler never fires on custom modal → tab/state never updates → silent assert timeout.
- Rename/retag any selector in list above: fix matching spec same change (specs match by class, no fallback).
- Add new `promptDialog`/`confirmDialog` call on tested flow: spec must drive `<Dialog/>`, never assume native popup.

Shell invariant: window must never scroll. `.panes` needs definite `grid-template-rows`; `html,body` stay `overflow:hidden`. height:100% child (CM6) of indefinite parent overflows shell into void. Guarded by `shell-no-scroll.spec`.

Pre-push UI: `cd frontend && bun run test:e2e` (needs WebKit deps; CI has them).