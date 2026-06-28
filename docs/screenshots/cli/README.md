# CLI screenshots

The still and walkthrough GIF of the `senda run` headless runner referenced in
the root README live here. They are generated automatically — don't edit them by
hand.

## Regenerating

Like the TUI images, these are rendered **headlessly in pure Go** — but the
content is a *real run*, not a storyboard. The generator stands up a local
in-process HTTP server, writes a small temporary collection, and sends every
request through the actual pipeline (the same path `senda run` uses). It then
paints `senda run`'s real stdout to PNG via
[`internal/termimg`](../../../internal/termimg), and builds the GIF by revealing
the run frame by frame as each result streams in. No network, no PTY, no
`ffmpeg`.

```bash
task shots:cli          # from the repo root → writes into docs/screenshots/cli/
```

or directly:

```bash
SENDA_CLI_SHOTS=1 SENDA_CLI_SHOT_DIR="$PWD/docs/screenshots/cli" \
  go test ./cmd/senda -run TestCLIShot -count=1
```

### Fonts

Same requirement as the TUI shots: **DejaVu Sans Mono** + **FreeMono** on the
system font path (`apt-get install -y fonts-dejavu-core fonts-freefont-ttf`, or
point `SENDA_TUI_FONT*` at your own). `SENDA_CLI_GIF=0` skips the GIF.

> **Alternative — record the real binary:** `task shots:cli:vhs` records the
> actual `senda run` with [vhs](https://github.com/charmbracelet/vhs) against
> `docs/recordings/senda-api` (Users folder, mock-backed). Needs `vhs` + `ttyd`
> + `ffmpeg`.

## Files

| File | Shown |
|------|-------|
| `01-run.png` | A folder run against an environment — per-request status, timing, assertion tally, summary |
| `walkthrough.gif` | The same run, animated: type the command, watch each result stream in, then the summary |
