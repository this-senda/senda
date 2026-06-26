package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"senda/internal/wsclient"
)

// wsConnectedMsg is the result of dialing: either an id + event channel, or an
// error. The dial runs in a command so the 30s timeout never blocks the UI.
type wsConnectedMsg struct {
	id  string
	url string
	err string
	ch  chan wsclient.WSEvent
}

// wsEventMsg carries one live frame (or the close event) from the read pump.
type wsEventMsg wsclient.WSEvent

// wsTickMsg refreshes the uptime clock once a second while connected.
type wsTickMsg struct{}

func wsTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return wsTickMsg{} })
}

// listenWSCmd blocks on the event channel and re-arms itself after each frame so
// the read pump streams continuously into the update loop.
func listenWSCmd(ch chan wsclient.WSEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return wsEventMsg(ev)
	}
}

// connectWS dials the active request's websocket endpoint. The read pump emits
// into a buffered channel which listenWSCmd drains back as messages.
func (m *tuiModel) connectWS() tea.Cmd {
	m.disconnectWS() // drop any existing session first
	if m.wsMgr == nil {
		m.wsMgr = wsclient.NewManager()
	}
	mgr := m.wsMgr
	req := m.cur
	scope := m.urlScope()
	ch := make(chan wsclient.WSEvent, 64)
	m.wsEvents = ch
	url := scope.Apply(req.URL)
	m.ws = &wsState{url: url, since: time.Now()}
	if strings.TrimSpace(req.Body.Raw) != "" {
		// The manager sends the body as the initial message; record it locally.
		m.ws.frames = append(m.ws.frames, wsFrame{ts: nowHM(), out: true, text: scope.Apply(req.Body.Raw)})
		m.ws.msgs++
	}
	return func() tea.Msg {
		id, err := mgr.Open(context.Background(), req, scope, func(ev wsclient.WSEvent) { ch <- ev })
		if err != nil {
			return wsConnectedMsg{err: err.Error(), ch: ch}
		}
		return wsConnectedMsg{id: id, url: url, ch: ch}
	}
}

// disconnectWS closes the active connection (if any) and clears session state.
func (m *tuiModel) disconnectWS() {
	if m.wsMgr != nil && m.wsID != "" {
		_ = m.wsMgr.Close(m.wsID)
	}
	m.wsID = ""
	m.wsEvents = nil
}

// onWSConnected wires up a freshly dialed connection (or records a dial error).
func (m *tuiModel) onWSConnected(msg wsConnectedMsg) tea.Cmd {
	if m.ws == nil {
		return nil
	}
	if msg.err != "" {
		m.ws.connected = false
		m.ws.err = msg.err
		return nil
	}
	m.wsID = msg.id
	m.ws.connected = true
	m.ws.err = ""
	return tea.Batch(listenWSCmd(msg.ch), wsTick())
}

// onWSEvent folds one live frame (or the close event) into the session view.
func (m *tuiModel) onWSEvent(ev wsEventMsg) tea.Cmd {
	if m.ws == nil {
		return nil
	}
	if ev.Closed {
		m.ws.connected = false
		if ev.Error != "" {
			m.ws.err = ev.Error
		}
		m.wsID = ""
		return nil // stop listening; the pump has exited
	}
	text := ev.Message.Data
	m.ws.frames = append(m.ws.frames, wsFrame{ts: nowHM(), out: false, text: text})
	m.ws.msgs++
	m.ws.opcode = "0x1 text"
	m.ws.size = fmt.Sprintf("%d bytes", len(text))
	return listenWSCmd(m.wsEvents) // keep draining
}

// sendWSCompose writes the composed message and records the sent frame.
func (m *tuiModel) sendWSCompose() {
	msg := strings.TrimSpace(m.ws.compose)
	if msg == "" || m.wsMgr == nil || m.wsID == "" {
		return
	}
	if err := m.wsMgr.Send(m.wsID, m.ws.compose); err != nil {
		m.ws.err = err.Error()
		return
	}
	m.ws.frames = append(m.ws.frames, wsFrame{ts: nowHM(), out: true, text: m.ws.compose})
	m.ws.msgs++
	m.ws.compose = ""
}

// handleWSKey drives the websocket connection pane. It returns handled=false for
// keys it doesn't own so global keys (quit, focus cycle, layout) still work.
func (m tuiModel) handleWSKey(s string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	connected := m.ws != nil && m.ws.connected
	switch s {
	case "ctrl+r": // (re)connect
		return m, m.connectWS(), true
	case "ctrl+d": // disconnect
		if connected {
			m.disconnectWS()
			if m.ws != nil {
				m.ws.connected = false
			}
		}
		return m, nil, true
	case "ctrl+l": // clear log
		if m.ws != nil {
			m.ws.frames = nil
		}
		return m, nil, true
	}
	if connected {
		switch s {
		case "enter":
			m.sendWSCompose()
			return m, nil, true
		case "backspace":
			if n := len(m.ws.compose); n > 0 {
				m.ws.compose = m.ws.compose[:n-1]
			}
			return m, nil, true
		case "esc":
			m.ws.compose = ""
			return m, nil, true
		case "space": // bubbletea reports space as "space", not " "
			m.ws.compose += " "
			return m, nil, true
		}
		// Printable single rune → append to the compose buffer.
		if len(s) == 1 && s[0] >= ' ' {
			m.ws.compose += s
			return m, nil, true
		}
		return m, nil, false // tab/ctrl+c/etc. fall through to global keys
	}
	// Disconnected: connect on s/enter, edit the URL with i.
	switch s {
	case "s", "enter":
		return m, m.connectWS(), true
	case "i":
		mm, cmd := m.startEditURL()
		return mm, cmd, true
	}
	return m, nil, false
}

// nowHM is the wall-clock HH:MM:SS stamp used on frames.
func nowHM() string { return time.Now().Format("15:04:05") }
