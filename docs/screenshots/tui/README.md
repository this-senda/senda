# Terminal UI screenshots

Screenshots and the walkthrough GIF of the `senda` terminal UI referenced in the
root README live here. They are generated automatically — don't edit them by hand.

## Regenerating

Unlike the desktop screenshots (Playwright in a browser), the TUI images are
rendered **headlessly in pure Go**: the generator drives the real `tuiModel`,
captures each screen's ANSI output from `render()`, and paints it to PNG with a
monospace font via [`internal/termimg`](../../../internal/termimg). The
walkthrough GIF is built the same way — a storyboard of model states encoded as
delta frames with the standard-library `image/gif`. No PTY, no `ffmpeg`, no
external capture tools.

```bash
task shots:tui          # from the repo root → writes into docs/screenshots/tui/
```

or directly:

```bash
SENDA_TUI_SHOTS=1 SENDA_TUI_SHOT_DIR="$PWD/docs/screenshots/tui" \
  go test ./internal/tui -run TestTUIShots -count=1
```

### Fonts

Rendering needs two monospace fonts on the system font path:

- **DejaVu Sans Mono** — primary face (box-drawing, arrows, geometric glyphs).
- **FreeMono** — fallback for the few glyphs DejaVu lacks (e.g. the 🔒 secret
  marker).

On Debian/Ubuntu: `apt-get install -y fonts-dejavu-core fonts-freefont-ttf`.
Override the lookup with `SENDA_TUI_FONT`, `SENDA_TUI_FONT_BOLD`, and
`SENDA_TUI_FONT_FALLBACK` if your fonts live elsewhere. `SENDA_TUI_GIF=0` skips
the (slower) GIF pass.

These are regenerated **locally**, not in CI (the Screenshots workflow only
covers the desktop GUI).

> **Alternative — record the real binary:** `task shots:tui:vhs` drives the
> actual `senda` TUI with [vhs](https://github.com/charmbracelet/vhs) against
> `docs/recordings/senda-api` (backed by the built-in mock server) for a real
> terminal recording. Needs `vhs` + `ttyd` + `ffmpeg`. See
> [`docs/recordings/`](../../recordings/).
>
> **Privacy:** because this records a real shell/binary, `record.sh` guards
> against leaking local machine info — clean `PS1` (no `user@host`/path),
> `-trimpath` build, redirected mock output, and localhost-only requests (your
> IP is never exposed). Guards are listed at the top of `record.sh`; still,
> eyeball the generated PNGs/GIF before committing.
>
> **Max privacy:** `task shots:vhs:container` runs the whole thing inside a
> throwaway `charmbracelet/vhs` container (generic hostname, root user, `/vhs`
> workdir, chrome bundled). Nothing about your real machine can appear even if a
> guard failed. Needs `podman` or `docker`.

## Files

Stills and the GIF are captured from a 120×34 terminal at a 9×19-pixel cell.

| File | Screen shown |
|------|--------------|
| `01-three-pane.png` | Default three-pane layout — collection tree · request · response |
| `02-tests-timing.png` | Tests tab — assertion pass/fail and the timing waterfall |
| `03-command-palette.png` | Command palette (Ctrl+K) — fuzzy jump to requests and actions |
| `04-environments.png` | Environments manager — per-scope variables, secrets, resolved preview |
| `05-code-export.png` | Code export — curl / fetch / httpie / python / go |
| `06-websocket.png` | WebSocket view — live connection log and frame inspector |
| `07-stacked-layout.png` | Stacked layout — tree · request stacked over response |
| `08-focus-mode.png` | Focus layout — distraction-free request over response, no tree |
| `walkthrough.gif` | Animated tour: open → send → response → tests → palette → switch env → layouts |
