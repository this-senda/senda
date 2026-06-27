# Screenshots

Screenshots and the walkthrough GIF referenced in the root README live here.
They are generated automatically — don't edit them by hand.

## Regenerating

The capture is fully automated: a Playwright script drives `vite --mode test`
(the app running in a plain browser against hand-written Wails binding stubs and
a capture-time mock backend), takes every screenshot, and encodes the
`walkthrough.gif`. No Go toolchain or generated bindings required.

```bash
cd frontend
bun install
bunx playwright install chromium   # one-time: fetch the headless browser
bun run shots                       # starts vite --mode test, captures, encodes the GIF
```

Output lands in `frontend/tests/visual/__screenshots__/`. Copy it across:

```bash
cp frontend/tests/visual/__screenshots__/*.png docs/screenshots/
cp frontend/tests/visual/__screenshots__/*.gif docs/screenshots/
```

Or do both in one step with `task shots` from the repo root.

In CI, run the **Screenshots** workflow (`.github/workflows/screenshots.yml`,
`workflow_dispatch`) — it regenerates everything and commits it back.

### Custom browser

`bun run shots` uses Playwright's bundled Chromium. Where Playwright's browser
CDN is blocked, point it at any Chrome/Chromium build instead:

```bash
SENDA_CHROME=/path/to/chrome bun run shots
```

A [Chrome for Testing](https://googlechromelabs.github.io/chrome-for-testing/)
build works well. Other env knobs: `SENDA_URL` (target a server you started
yourself) and `SENDA_GIF=0` (skip the GIF pass).

## Files

Stills are captured at a 1280×820 viewport; the GIF at 1000×640.
Numbered to match the order they appear in the root README.

| File | Feature shown |
|------|--------------|
| `01-empty-shell.png` | Fresh launch — empty 3-pane layout |
| `02-collection-open.png` | Collection loaded, tree visible, env switcher |
| `03-request-open.png` | Request open in the editor — URL, method, tab bar |
| `04-request-response.png` | Request sent — status/time/size + response body |
| `05-body-json.png` | JSON body editor (CodeMirror) |
| `06-headers.png` | Request headers with enable/disable toggles |
| `07-assertions.png` | Assertion rows (Tests tab) with pass/fail |
| `08-scripting.png` | Pre/post-request script tab (JS sandbox) |
| `09-command-palette.png` | Command palette (Ctrl+K) |
| `10-appearance-modal.png` | Appearance dialog — mode toggle + theme pickers |
| `11-theme-catppuccin-mocha.png` | Full app in the Catppuccin Mocha theme |
| `12-mock-server.png` | Mock server — live routes, scenarios, request log |
| `13-environments.png` | Environment editor — per-environment variables |
| `14-auth.png` | Auth tab — Bearer / Basic / API key / OAuth 2.0 |
| `15-code-generation.png` | Code generation — curl / fetch / httpie / python / go |
| `16-import.png` | Import — curl, Postman, OpenAPI, and HAR |
| `17-history.png` | Request history panel |
| `18-workspace-rail.png` | Workspace rail — multi-collection switcher + icon picker |
| `19-faker-autocomplete.png` | `{{$faker}}` token autocomplete in the JSON body |
| `walkthrough.gif` | Animated tour: open → workspace icon → send → tests → faker → palette → theme |
