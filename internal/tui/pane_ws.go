package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// fmtUptime renders a connection duration as "Nm SSs".
func fmtUptime(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%dm %02ds", s/60, s%60)
}

// paneWsConnection renders the websocket CONNECTION pane: status header, the
// message log (↑ sent / ↓ received), and a compose input pinned to the bottom.
func (m tuiModel) paneWsConnection(w, h int) string {
	focused := m.focus == focusReq
	ws := m.ws
	if ws == nil {
		lines := []string{
			m.paneLabel("CONNECTION", focused), base.Render(""),
			styleDim.Render("● disconnected") + styleDim.Render("   press ^R to connect"),
		}
		return paneBlock(lines, w, h)
	}

	dot := base.Foreground(colGood).Render("●")
	if !ws.connected {
		dot = styleDim.Render("●")
	}
	state := "connected"
	if !ws.connected {
		state = "disconnected"
	}
	uptime := ws.uptime
	if ws.connected && !ws.since.IsZero() {
		uptime = fmtUptime(time.Since(ws.since))
	}
	header := dot + base.Foreground(colGood).Render(" "+state) +
		styleDim.Render("   url ") + base.Foreground(colFg).Render(ws.url) +
		styleDim.Render("   msgs ") + base.Foreground(colFg).Render(fmt.Sprintf("%d", ws.msgs)) +
		styleDim.Render("   up ") + base.Foreground(colFg).Render(uptime)

	statusRow := styleDim.Render("— handshake complete · 101 switching protocols —")
	if ws.err != "" {
		statusRow = base.Foreground(colBad).Render("● error: " + ws.err)
	} else if !ws.connected {
		statusRow = styleDim.Render("● disconnected · ^R to connect")
	}
	rows := []string{m.paneLabel("CONNECTION", focused), header, base.Render(""), statusRow}
	for _, f := range ws.frames {
		arrow := base.Foreground(colGood).Render("↓")
		if f.out {
			arrow = base.Foreground(colAccent).Render("↑")
		}
		line := styleDim.Render(f.ts) + base.Render("  ") + arrow + base.Render(" ") + colorJSON(f.text)
		rows = append(rows, line)
	}
	// Pin the compose input to the bottom row.
	for len(rows) < h-1 {
		rows = append(rows, base.Render(""))
	}
	in := base.Background(bgInput)
	compose := in.Foreground(colAccent).Render(" ↑ ") + in.Foreground(colFg).Render(ws.compose)
	hint := in.Foreground(colDim).Render("⏎ send  ^L clear ")
	pad := w - lipgloss.Width(stripStyle(compose)) - lipgloss.Width(stripStyle(hint))
	if pad < 1 {
		pad = 1
	}
	rows = append(rows, compose+in.Render(strings.Repeat(" ", pad))+hint)
	return paneBlock(rows, w, h)
}

// paneWsFrame renders the FRAME inspector pane: the last inbound frame as
// numbered JSON, plus opcode/size/health metadata.
func (m tuiModel) paneWsFrame(w, h int) string {
	focused := m.focus == focusResp
	ws := m.ws
	if ws == nil {
		return paneBlock([]string{m.paneLabel("FRAME", focused), base.Render(""), styleDim.Render("(no frame)")}, w, h)
	}
	// Inspect the last inbound data frame, skipping ping/pong control frames.
	var last *wsFrame
	var lastTs string
	for i := range ws.frames {
		f := &ws.frames[i]
		if f.out || strings.Contains(f.text, "pong") || strings.Contains(f.text, "ping") {
			continue
		}
		last = f
		lastTs = f.ts
	}
	lines := []string{m.paneLabel("FRAME", focused),
		styleDim.Render("last inbound · ") + base.Foreground(colFg).Render(lastTs), base.Render("")}
	if last != nil {
		lines = append(lines, strings.Split(numberedCode(prettyJSON(last.text)), "\n")...)
	}
	lines = append(lines, base.Render(""))
	meta := func(k, v string) string {
		pad := w - lipgloss.Width(k) - lipgloss.Width(v)
		if pad < 1 {
			pad = 1
		}
		return styleDim.Render(k) + base.Render(strings.Repeat(" ", pad)) + base.Foreground(colFg).Render(v)
	}
	lines = append(lines,
		meta("opcode", ws.opcode),
		meta("size", ws.size),
		meta("ping/pong", "healthy"),
	)
	return paneBlock(lines, w, h)
}
