package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// paletteItem is one selectable entry in the command palette: either a request
// to jump to (rowIdx ≥ 0) or a named command to run (cmd != "").
type paletteItem struct {
	label  string
	desc   string
	rowIdx int
	cmd    string
}

// paletteItems returns all requests in the tree followed by global commands,
// filtered by the current query (case-insensitive substring match).
func (m tuiModel) paletteItems() []paletteItem {
	var all []paletteItem
	for i, r := range m.rows {
		if r.node.IsDir {
			continue
		}
		method := strings.ToUpper(r.node.Method)
		if method == "" {
			method = "—"
		}
		name := strings.TrimSuffix(r.node.Name, ".yaml")
		all = append(all, paletteItem{label: method + " " + name, desc: m.parentFolder(i), rowIdx: i})
	}
	env := m.envName()
	if env == "" {
		env = "none"
	}
	all = append(all,
		paletteItem{label: "Send request", desc: "command", rowIdx: -1, cmd: "send"},
		paletteItem{label: "Generate code → cURL", desc: "command", rowIdx: -1, cmd: "export"},
		paletteItem{label: "Switch environment → " + env, desc: "command", rowIdx: -1, cmd: "env"},
		paletteItem{label: "Cycle layout", desc: "command", rowIdx: -1, cmd: "layout"},
	)
	q := strings.ToLower(strings.TrimSpace(m.paletteQuery))
	if q == "" {
		return all
	}
	var out []paletteItem
	for _, it := range all {
		// Commands always remain available; requests filter by the query.
		if it.cmd != "" || strings.Contains(strings.ToLower(it.label), q) {
			out = append(out, it)
		}
	}
	return out
}

func (m tuiModel) handlePalette(s string) (tea.Model, tea.Cmd) {
	items := m.paletteItems()
	switch s {
	case "esc", "ctrl+k":
		m.paletteOpen = false
		return m, nil
	case "backspace":
		if n := len(m.paletteQuery); n > 0 {
			m.paletteQuery = m.paletteQuery[:n-1]
			m.paletteIdx = 0
		}
		return m, nil
	case "up", "ctrl+p":
		if len(items) > 0 {
			m.paletteIdx = (m.paletteIdx + len(items) - 1) % len(items)
		}
		return m, nil
	case "down", "ctrl+n":
		if len(items) > 0 {
			m.paletteIdx = (m.paletteIdx + 1) % len(items)
		}
		return m, nil
	case "enter":
		if m.paletteIdx >= len(items) {
			m.paletteOpen = false
			return m, nil
		}
		return m.runPalette(items[m.paletteIdx])
	}
	// Printable single char extends the query (space arrives as "space").
	if s == "space" {
		s = " "
	}
	if len(s) == 1 && s[0] >= ' ' {
		m.paletteQuery += s
		m.paletteIdx = 0
	}
	return m, nil
}

// runPalette closes the palette and performs the selected item's action.
func (m tuiModel) runPalette(it paletteItem) (tea.Model, tea.Cmd) {
	m.paletteOpen = false
	if it.rowIdx >= 0 {
		m.cursor = it.rowIdx
		m.focus = focusReq
		return m, m.onCursorMove()
	}
	switch it.cmd {
	case "send":
		if m.loaded && !m.sending {
			m.sending = true
			m.respErr = ""
			return m, tea.Batch(m.spin.Tick, m.sendCmd())
		}
	case "export":
		if m.loaded {
			m.exportOpen = true
			m.resize()
			m.refreshReqView()
		}
	case "env":
		m.pickerOpen = true
		m.pickerIdx = m.envIdx + 1
	case "layout":
		m.layout = (m.layout + 1) % layoutModeCount
		if m.layout == layoutFocus && m.focus == focusTree {
			m.focus = focusReq
		}
		m.resize()
		m.refreshReqView()
		m.refreshRespView()
	}
	return m, nil
}
