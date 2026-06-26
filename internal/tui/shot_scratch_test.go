package tui

import (
	"os"
	"path/filepath"
	"testing"

	"senda/internal/model"
	"senda/internal/termimg"
)

// TestScratchShots renders the new scratch-request + inline-editor states to PNG
// (and plain-text) for visual verification. Gated so it never runs in CI.
//
//	SENDA_SCRATCH_SHOTS=1 go test ./internal/tui/ -run TestScratchShots -v
func TestScratchShots(t *testing.T) {
	if os.Getenv("SENDA_SCRATCH_SHOTS") == "" {
		t.Skip("set SENDA_SCRATCH_SHOTS=1 to render scratch screenshots")
	}
	dir := "tmp/shots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r, rerr := termimg.New(termimg.Defaults())

	save := func(name string, m tuiModel) {
		// Always write plain text so the result is inspectable without fonts.
		plain := ansiRe.ReplaceAllString(m.render(), "")
		os.WriteFile(filepath.Join(dir, name+".txt"), []byte(plain), 0o644)
		if rerr != nil {
			t.Logf("termimg unavailable (%v); wrote %s.txt only", rerr, name)
			return
		}
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

	// scratch base: a freshly created in-memory request (untitled-1, dirty dot).
	base := func() tuiModel {
		m := snapModel(W, H)
		m.layout = layout3Pane
		m.resize()
		key := scratchPrefix + "1"
		m.scratchN = 1
		m.open = append(m.open,
			openReq{path: "/senda-api/Users/get-users.yaml", method: "GET", name: "/users"},
			openReq{path: key, method: "POST", name: "untitled-1"},
		)
		m.buf[key] = model.Request{Method: "POST", URL: "https://"}
		m.dirty[key] = true
		m.curPath = key
		m.cur = m.buf[key]
		m.loaded = true
		m.focus = focusReq
		m.refreshReqView()
		return m
	}

	// 01 — scratch tab, not yet editing (shows untitled-1 with dirty ● marker).
	save("scratch-01-new", base())

	// 02 — inline URL editor in INSERT mode with the {{faker}} popup open.
	{
		m := base()
		m.editing = true
		m.input = newTestInput("https://{{em")
		m.refreshCompletion()
		save("scratch-02-faker-popup", m)
	}

	// 03 — ctrl+s save-as name prompt for the scratch request.
	{
		m := base()
		m.saveOpen = true
		m.saveInput = newTestInput("create-user.yaml")
		save("scratch-03-save-prompt", m)
	}

	// 04 — Body tab after picking JSON (t cycles type), showing the edit hint.
	{
		m := base()
		m.cur.Body = model.Body{Type: model.BodyJSON, Raw: `{"name": "ada"}`}
		m.buf[m.curPath] = m.cur
		m.reqTab = tabBody
		m.refreshReqView()
		save("scratch-04-body-json", m)
	}

	// 05 — editing the JSON body with the {{faker}} popup open.
	{
		m := base()
		m.cur.Body = model.Body{Type: model.BodyJSON}
		m.buf[m.curPath] = m.cur
		m.reqTab = tabBody
		mm, _ := m.startEditBody()
		m = mm.(tuiModel)
		m.body.SetValue("{\n  \"email\": \"{{em")
		m.body.CursorEnd()
		m.refreshCompletion()
		m.refreshReqView()
		save("scratch-05-body-edit", m)
	}

	// 06 — body editing, no popup: URL row must stay clean (static, no input).
	{
		m := base()
		m.cur.Body = model.Body{Type: model.BodyJSON}
		m.cur.URL = "https://api.example.com/users"
		m.buf[m.curPath] = m.cur
		m.reqTab = tabBody
		mm, _ := m.startEditBody()
		m = mm.(tuiModel)
		m.body.SetValue("{\n  \"name\": \"ada\",\n  \"role\": \"admin\"\n}")
		m.body.CursorEnd()
		m.refreshReqView()
		save("scratch-06-body-noPopup", m)
	}

	// 07 — live websocket: scratch request promoted to WS, connected, streaming.
	{
		m := base()
		m.layout = layout3Pane
		m.cur.Method = "WS"
		m.cur.URL = "wss://echo.websocket.org"
		m.open[len(m.open)-1].method = "WS"
		m.focus = focusReq
		m.ws = &wsState{
			connected: true, url: "wss://echo.websocket.org", msgs: 3, uptime: "0m 12s",
			compose: `{"action":"ping"}`, opcode: "0x1 text", size: "18 bytes",
			frames: []wsFrame{
				{ts: "12:04:01", out: true, text: `hello`},
				{ts: "12:04:01", out: false, text: `hello`},
				{ts: "12:04:09", out: false, text: `{"event":"tick","n":1}`},
			},
		}
		m.resize()
		save("scratch-07-ws-live", m)
	}
}
