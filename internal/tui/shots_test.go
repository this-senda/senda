package tui

import (
	"os"
	"path/filepath"
	"testing"

	"senda/internal/model"
	"senda/internal/termimg"
)

// TestTUIShots regenerates the senda terminal-UI documentation: a set of PNG stills plus
// an animated walkthrough.gif, rendered headlessly from real tuiModel.render()
// output via internal/termimg (no PTY, no ffmpeg). Gated by SENDA_TUI_SHOTS so it
// never runs in normal `go test ./...`; drive it with `task shots:tui`.
//
// Output dir: $SENDA_TUI_SHOT_DIR (default internal/tui/tmp/shots). The
// Taskfile copies the results into docs/screenshots/tui/.
func TestTUIShots(t *testing.T) {
	if os.Getenv("SENDA_TUI_SHOTS") == "" {
		t.Skip("set SENDA_TUI_SHOTS=1 to regenerate TUI screenshots")
	}
	dir := os.Getenv("SENDA_TUI_SHOT_DIR")
	if dir == "" {
		dir = filepath.Join("tmp", "shots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Stills + GIF render at 2× then downscale (SSAA) with softer hinting, so
	// glyphs get smooth grayscale-AA edges at the normal output size — sharp, not
	// chunky — without shipping oversized images.
	shotOpts := termimg.Defaults()
	shotOpts.Supersample = 2
	shotOpts.SoftHinting = true
	r, err := termimg.New(shotOpts)
	if err != nil {
		t.Fatalf("load fonts (need fonts-dejavu-core + fonts-freefont-ttf, or set SENDA_TUI_FONT*): %v", err)
	}
	rGif, err := termimg.New(shotOpts)
	if err != nil {
		t.Fatal(err)
	}

	save := func(name string, m tuiModel) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := r.PNG(f, m.render()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Logf("wrote %s.png", name)
	}

	const W, H = 120, 34

	// 01 — three-pane: collection tree | request | response.
	{
		m := snapModel(W, H)
		m.layout = layout3Pane
		m.resize()
		loadUsersRequest(&m)
		m.expanded["/senda-api/Auth"] = false
		m.expanded["/senda-api/Orders"] = false
		m.rebuild()
		m.cursor = 2
		m.focus = focusTree
		save("01-three-pane", m)
	}

	// 02 — tests + timing: assertion results with pass/fail and a timing waterfall.
	{
		m := snapModel(W, H)
		m.layout = layoutStacked
		m.resize()
		loadUsersRequest(&m)
		m.cur.Asserts = make([]model.Assert, 6)
		m.resp = testsResponse()
		m.respTab = rtabTests
		m.refreshRespView()
		save("02-tests-timing", m)
	}

	// 03 — command palette: fuzzy jump to requests and commands (ctrl+k).
	{
		m := snapModel(W, H)
		m.layout = layout3Pane
		m.resize()
		loadUsersRequest(&m)
		m.paletteOpen = true
		m.paletteQuery = "user"
		save("03-command-palette", m)
	}

	// 04 — environments manager: per-scope variables, secrets, resolved preview.
	{
		m := snapModel(W, H)
		m.envIdx = 2 // prod
		m.openTab("/senda-api/Users/get-users.yaml", "GET")
		m.open[len(m.open)-1].name = "/users"
		m.envMgrOpen = true
		m.envMgrIdx = 2
		save("04-environments", m)
	}

	// 05 — code export: generate curl / fetch / httpie / python / go.
	{
		m := snapModel(W, H)
		m.layout = layout3Pane
		m.resize()
		loadUsersRequest(&m)
		m.reqTab = tabHeaders
		m.refreshReqView()
		m.exportOpen = true
		m.exportIdx = 0
		save("05-code-export", m)
	}

	// 06 — websocket: live connection log and frame inspector.
	{
		m := snapModel(W, H)
		m.layout = layout3Pane
		m.resize()
		m.cursor = 0
		m.curPath = "/senda-api/Realtime/events.yaml"
		m.loaded = true
		m.cur = model.Request{Method: "WS", URL: "wss://{{ws_url}}/events/stream"}
		m.expanded["/senda-api/Auth"] = false
		m.expanded["/senda-api/Orders"] = false
		m.rebuild()
		for i, row := range m.rows {
			if row.node.Path == "/senda-api/Realtime/events.yaml" {
				m.cursor = i
			}
		}
		m.openTab("/senda-api/Realtime/events.yaml", "WS")
		m.open[len(m.open)-1].name = "/events/stream"
		m.openTab("/senda-api/Orders/get-orders.yaml", "GET")
		m.open[len(m.open)-1].name = "/orders"
		m.ws = sampleWS()
		m.focus = focusReq
		save("06-websocket", m)
	}

	// 07 — stacked layout: tree | request stacked over response.
	{
		m := snapModel(W, H)
		m.layout = layoutStacked
		m.resize()
		loadUsersRequest(&m)
		m.envIdx = 1 // staging
		save("07-stacked-layout", m)
	}

	// 08 — focus layout: distraction-free request over response, no tree.
	{
		m := snapModel(W, H)
		m.layout = layoutFocus
		m.resize()
		loadUsersRequest(&m)
		m.focus = focusResp
		save("08-focus-mode", m)
	}

	// walkthrough.gif — an animated tour.
	if os.Getenv("SENDA_TUI_GIF") != "0" {
		frames := walkthroughFrames(W, H)
		gf, err := os.Create(filepath.Join(dir, "walkthrough.gif"))
		if err != nil {
			t.Fatal(err)
		}
		defer gf.Close()
		if err := rGif.GIF(gf, frames); err != nil {
			t.Fatalf("gif: %v", err)
		}
		if st, _ := gf.Stat(); st != nil {
			t.Logf("wrote walkthrough.gif (%d frames, %d bytes)", len(frames), st.Size())
		}
	}
}

// testsResponse is the 200 response with a timing waterfall and six assertions
// (one failing) used by the tests-timing still and walkthrough.
func testsResponse() *model.Response {
	return &model.Response{
		Status: 200, StatusText: "OK", DurationMs: 142, SizeBytes: 4300,
		Timing: &model.ResponseTiming{DNSMs: 4, ConnectMs: 11, TLSMs: 38, FirstByteMs: 124, DownloadMs: 18},
		Asserts: []model.AssertResult{
			{Target: "status code is 200", Pass: true},
			{Target: "response time below 300 ms", Pass: true},
			{Target: "content-type is application/json", Pass: true},
			{Target: "body.meta.limit is a number", Pass: true},
			{Target: "every user has a valid email", Pass: true},
			{Target: "role in [admin, member, guest]", Pass: false, Actual: "owner", Error: `expected "member" — received "owner" at data[1].role`},
		},
	}
}

// sampleWS is the connected websocket session shown in the WS still.
func sampleWS() *wsState {
	return &wsState{
		connected: true,
		url:       "wss://rt.senda.dev/events/stream",
		msgs:      42,
		uptime:    "3m 12s",
		opcode:    "0x1 text",
		size:      "86 bytes",
		compose:   `{ "action": "subscribe", "channel": "…" }`,
		frames: []wsFrame{
			{ts: "12:04:01", out: true, text: `{ "action": "subscribe", "channel": "orders" }`},
			{ts: "12:04:01", out: false, text: `{ "event": "subscribed", "channel": "orders" }`},
			{ts: "12:04:09", out: false, text: `{ "event": "order.created", "id": "ord_5521", "total": 4200 }`},
			{ts: "12:04:14", out: false, text: `{ "event": "order.updated", "id": "ord_5521", "status": "paid" }`},
			{ts: "12:04:22", out: true, text: `{ "action": "ping" }`},
			{ts: "12:04:22", out: false, text: `{ "event": "pong", "ts": 1760440 }`},
		},
	}
}

// walkthroughFrames storyboards the animated tour: open a request, send it, watch
// the response and tests land, drive the command palette, cycle environments, and
// flip through layouts. Delays are in centiseconds.
func walkthroughFrames(w, h int) []termimg.Frame {
	m := snapModel(w, h)
	m.layout = layout3Pane
	m.resize()
	loadUsersRequest(&m)
	m.expanded["/senda-api/Auth"] = false
	m.expanded["/senda-api/Orders"] = false
	m.rebuild()
	m.cursor = 2
	savedResp := m.resp // the 200 GET /users response, restored after detours

	var frames []termimg.Frame
	add := func(delay int) { frames = append(frames, termimg.Frame{ANSI: m.render(), Delay: delay}) }

	// 1. Just opened — request loaded, awaiting a response.
	m.resp = nil
	m.respTab = rtabBody
	m.refreshRespView()
	m.focus = focusTree
	add(150)

	// 2. Send (s) — the response pane shows "sending…".
	m.sending = true
	m.focus = focusResp
	m.refreshRespView()
	add(45)
	add(40)

	// 3. Response lands on the Body tab.
	m.sending = false
	m.resp = savedResp
	m.respTab = rtabBody
	m.refreshRespView()
	m.vp.SetYOffset(0)
	add(165)

	// 4. Inspect the Tests tab — assertions + timing.
	m.cur.Asserts = make([]model.Assert, 6)
	m.resp = testsResponse()
	m.respTab = rtabTests
	m.refreshRespView()
	add(175)

	// 5. Command palette (ctrl+k) — type a query, move the selection.
	m.cur.Asserts = nil
	m.resp = savedResp
	m.respTab = rtabBody
	m.refreshRespView()
	m.focus = focusTree
	m.paletteOpen = true
	m.paletteQuery = ""
	m.paletteIdx = 0
	add(45)
	for _, q := range []string{"u", "us", "use", "user"} {
		m.paletteQuery = q
		add(12)
	}
	add(45)
	m.paletteIdx = 1
	add(45)
	m.paletteIdx = 2
	add(60)
	m.paletteOpen = false
	m.paletteQuery = ""
	m.paletteIdx = 0

	// 6. Cycle environments ([ / ]) — prod → staging → local → prod.
	add(55)
	m.envIdx = 1 // staging
	add(70)
	m.envIdx = 0 // local
	add(70)
	m.envIdx = 2 // prod
	add(80)

	// 7. Cycle layout (ctrl+\) into distraction-free focus mode.
	m.layout = layoutFocus
	m.focus = focusResp
	m.resize()
	m.refreshReqView()
	m.refreshRespView()
	add(165)

	// 8. Back to the three-pane home view.
	m.layout = layout3Pane
	m.focus = focusTree
	m.resize()
	m.refreshReqView()
	m.refreshRespView()
	add(210)

	return frames
}
