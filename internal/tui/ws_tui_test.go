package tui

import (
	"context"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"senda/internal/model"
	"senda/internal/vars"
	"senda/internal/wsclient"
)

// TestWSLive does a real round-trip against an echo server. Gated by SENDA_WS_LIVE
// (needs network) so it never runs in normal test passes.
func TestWSLive(t *testing.T) {
	if os.Getenv("SENDA_WS_LIVE") == "" {
		t.Skip("set SENDA_WS_LIVE=1 for the live echo round-trip")
	}
	mgr := wsclient.NewManager()
	req := model.Request{Method: "WS", URL: "wss://echo.websocket.org", Body: model.Body{Raw: "ping-senda"}}
	got := make(chan string, 4)
	id, err := mgr.Open(context.Background(), req, vars.Build(), func(ev wsclient.WSEvent) {
		if !ev.Closed {
			got <- ev.Message.Data
		}
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer mgr.Close(id)
	for {
		select {
		case msg := <-got:
			if msg == "ping-senda" {
				t.Logf("echo received: %q", msg)
				return
			}
		case <-time.After(8 * time.Second):
			t.Fatal("no echo within 8s")
		}
	}
}

// keyPress builds a minimal key message; handleWSKey only inspects the string
// form, so a zero message is sufficient for these tests.
func keyPress(string) tea.KeyPressMsg { return tea.KeyPressMsg{} }

func TestIsWSRequest(t *testing.T) {
	cases := []struct {
		method, url string
		want        bool
	}{
		{"GET", "https://x.com", false},
		{"GET", "wss://echo.websocket.org", true},
		{"GET", "ws://localhost:9000", true},
		{"WS", "https://x.com", true},
		{"POST", "  WSS://X ", true},
	}
	for _, c := range cases {
		m := tuiModel{loaded: true, cur: model.Request{Method: c.method, URL: c.url}}
		if got := m.isWSRequest(); got != c.want {
			t.Errorf("isWSRequest(%s %s) = %v, want %v", c.method, c.url, got, c.want)
		}
	}
}

func TestCommitPromotesWS(t *testing.T) {
	m := tuiModel{
		buf: map[string]model.Request{}, dirty: map[string]bool{},
		curPath: "k", cur: model.Request{Method: "GET"},
		open:     []openReq{{path: "k", method: "GET"}},
		editing:  true,
		editMode: editURL,
	}
	m.input = newTestInput("wss://echo.websocket.org")
	m.commitEdit()
	if m.cur.Method != "WS" {
		t.Fatalf("method not promoted: %q", m.cur.Method)
	}
	if m.open[0].method != "WS" {
		t.Fatalf("tab method not updated: %q", m.open[0].method)
	}
}

// TestWSStreaming exercises the connect→event plumbing without a real socket: a
// connected message arms the listener, an event folds into the frame log, and a
// close event flips the state to disconnected.
func TestWSStreaming(t *testing.T) {
	m := tuiModel{ws: &wsState{}}
	ch := make(chan wsclient.WSEvent, 4)

	if cmd := m.onWSConnected(wsConnectedMsg{id: "ws-1", url: "wss://x", ch: ch}); cmd == nil {
		t.Fatal("connected should return a listen+tick command")
	}
	if !m.ws.connected || m.wsID != "ws-1" {
		t.Fatalf("not wired: connected=%v id=%q", m.ws.connected, m.wsID)
	}

	m.wsEvents = ch
	m.onWSEvent(wsEventMsg{Message: model.WSMessage{Direction: "received", Data: "hi"}})
	if len(m.ws.frames) != 1 || m.ws.frames[0].text != "hi" || m.ws.frames[0].out {
		t.Fatalf("frame not recorded: %+v", m.ws.frames)
	}
	if m.ws.msgs != 1 {
		t.Fatalf("msgs = %d, want 1", m.ws.msgs)
	}

	m.onWSEvent(wsEventMsg{Closed: true, Error: "boom"})
	if m.ws.connected || m.ws.err != "boom" {
		t.Fatalf("close not handled: connected=%v err=%q", m.ws.connected, m.ws.err)
	}
}

func TestWSComposeTyping(t *testing.T) {
	m := tuiModel{loaded: true, ws: &wsState{connected: true}, cur: model.Request{Method: "WS", URL: "wss://x"}}
	for _, r := range "hi" {
		mm, _, handled := m.handleWSKey(string(r), keyPress(string(r)))
		if !handled {
			t.Fatalf("char %q not handled", r)
		}
		m = mm.(tuiModel)
	}
	if m.ws.compose != "hi" {
		t.Fatalf("compose = %q, want hi", m.ws.compose)
	}
	// Space arrives as the key string "space" — it must append a literal space.
	mm, _, handled := m.handleWSKey("space", keyPress("space"))
	if !handled {
		t.Fatal("space not handled")
	}
	if m = mm.(tuiModel); m.ws.compose != "hi " {
		t.Fatalf("compose after space = %q, want %q", m.ws.compose, "hi ")
	}
	// ctrl+c must fall through (not handled) so the app can still quit.
	if _, _, handled := m.handleWSKey("ctrl+c", keyPress("ctrl+c")); handled {
		t.Fatal("ctrl+c should fall through to global keys")
	}
}
