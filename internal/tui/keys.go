package tui

import tea "charm.land/bubbletea/v2"

func (m tuiModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	// Overlays consume keys first.
	if m.showHelp {
		switch s {
		case "?", "esc", "q", "enter", " ":
			m.showHelp = false
		}
		return m, nil
	}
	if m.pickerOpen {
		return m.handlePicker(s)
	}
	if m.exportOpen {
		return m.handleExport(s)
	}
	if m.paletteOpen {
		return m.handlePalette(s)
	}
	if m.envMgrOpen {
		return m.handleEnvMgr(s)
	}
	if m.browseOpen {
		return m.handleBrowse(s)
	}

	// Global keys.
	switch s {
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+k":
		m.paletteOpen = true
		m.paletteQuery = ""
		m.paletteIdx = 0
		return m, nil
	case "q":
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "E":
		m.pickerOpen = true
		m.pickerIdx = m.envIdx + 1 // map -1->0
		return m, nil
	case "O":
		return m.openBrowser(), nil
	case "tab":
		m.focus = m.nextFocus(1)
		return m, nil
	case "shift+tab":
		m.focus = m.nextFocus(-1)
		return m, nil
	case "ctrl+\\":
		m.layout = (m.layout + 1) % layoutModeCount
		if m.layout == layoutFocus && m.focus == focusTree {
			m.focus = focusReq
		}
		m.resize()
		m.refreshReqView()
		m.refreshRespView()
		return m, nil
	case "[":
		if len(m.envs) > 0 {
			m.envIdx--
			if m.envIdx < -1 {
				m.envIdx = len(m.envs) - 1
			}
		}
		return m, nil
	case "]":
		if len(m.envs) > 0 {
			m.envIdx++
			if m.envIdx >= len(m.envs) {
				m.envIdx = -1
			}
		}
		return m, nil
	case "s":
		if m.loaded && !m.sending {
			m.sending = true
			m.respErr = ""
			return m, tea.Batch(m.spin.Tick, m.sendCmd())
		}
		return m, nil
	case "e":
		if m.curPath != "" {
			return m, m.editCmd(m.curPath)
		}
		return m, nil
	case "x":
		if m.loaded {
			m.exportOpen = true
			m.resize()
			m.refreshReqView()
		}
		return m, nil
	case "ctrl+w":
		m.closeTab()
		return m, nil
	case "ctrl+e":
		if len(m.envs) > 0 {
			m.envMgrOpen = true
			m.envMgrIdx = m.envIdx
			if m.envMgrIdx < 0 {
				m.envMgrIdx = 0
			}
		}
		return m, nil
	}

	switch m.focus {
	case focusReq:
		return m.handleReqKey(s, msg)
	case focusResp:
		return m.handleRespKey(s, msg)
	default:
		return m.handleTreeKey(s)
	}
}

func (m tuiModel) handleTreeKey(s string) (tea.Model, tea.Cmd) {
	// Any key clears the pending 'g'; the 'g' case re-arms it for a 'gg' chord.
	wasG := m.pendingG
	m.pendingG = false
	switch s {
	case "up", "k":
		m.moveCursor(-1)
		return m, m.onCursorMove()
	case "down", "j":
		m.moveCursor(1)
		return m, m.onCursorMove()
	case "ctrl+u":
		m.moveCursor(-m.treePage() / 2)
		return m, m.onCursorMove()
	case "ctrl+d":
		m.moveCursor(m.treePage() / 2)
		return m, m.onCursorMove()
	case "ctrl+b", "pgup":
		m.moveCursor(-m.treePage())
		return m, m.onCursorMove()
	case "ctrl+f", "pgdown":
		m.moveCursor(m.treePage())
		return m, m.onCursorMove()
	case "g":
		if wasG {
			m.cursor = 0 // gg → top
			return m, m.onCursorMove()
		}
		m.pendingG = true // first g; wait for the second
		return m, nil
	case "home":
		m.cursor = 0
		return m, m.onCursorMove()
	case "G", "end":
		m.cursor = len(m.rows) - 1
		return m, m.onCursorMove()
	case "enter", "right", "l":
		return m.activate()
	case "left", "h":
		// Collapse current folder, or jump to parent.
		if r, ok := m.currentRow(); ok && r.node.IsDir && m.expanded[r.node.Path] {
			m.expanded[r.node.Path] = false
			m.rebuild()
			m.clampTreeScroll()
		}
		return m, nil
	}
	return m, nil
}

func (m tuiModel) handleReqKey(s string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch s {
	case "left", "h":
		m.reqTab = (m.reqTab + reqTabCount - 1) % reqTabCount
		m.refreshReqView()
		m.reqVp.SetYOffset(0)
		return m, nil
	case "right", "l":
		m.reqTab = (m.reqTab + 1) % reqTabCount
		m.refreshReqView()
		m.reqVp.SetYOffset(0)
		return m, nil
	}
	if len(s) == 1 && s[0] >= '1' && s[0] <= '7' {
		m.reqTab = reqTab(s[0] - '1')
		m.refreshReqView()
		m.reqVp.SetYOffset(0)
		return m, nil
	}
	var cmd tea.Cmd
	m.reqVp, cmd = m.reqVp.Update(msg)
	return m, cmd
}

func (m tuiModel) handleRespKey(s string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch s {
	case "left", "h":
		m.respTab = (m.respTab + respTabCount - 1) % respTabCount
		m.refreshRespView()
		m.vp.SetYOffset(0)
		return m, nil
	case "right", "l":
		m.respTab = (m.respTab + 1) % respTabCount
		m.refreshRespView()
		m.vp.SetYOffset(0)
		return m, nil
	}
	if len(s) == 1 && s[0] >= '1' && s[0] <= '6' {
		m.respTab = respTab(s[0] - '1')
		m.refreshRespView()
		m.vp.SetYOffset(0)
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m tuiModel) handlePicker(s string) (tea.Model, tea.Cmd) {
	n := len(m.envs) + 1 // +1 for "none"
	switch s {
	case "up", "k":
		m.pickerIdx = (m.pickerIdx + n - 1) % n
	case "down", "j":
		m.pickerIdx = (m.pickerIdx + 1) % n
	case "enter", " ":
		m.envIdx = m.pickerIdx - 1
		m.pickerOpen = false
	case "esc", "q", "E":
		m.pickerOpen = false
	}
	return m, nil
}

func (m tuiModel) handleExport(s string) (tea.Model, tea.Cmd) {
	n := len(codegenTabs)
	switch s {
	case "left", "h", "shift+tab":
		m.exportIdx = (m.exportIdx + n - 1) % n
	case "right", "l", "tab":
		m.exportIdx = (m.exportIdx + 1) % n
	case "esc", "q", "x":
		m.exportOpen = false
		m.resize()
		m.refreshReqView()
	default:
		// number keys jump directly to a language tab
		if len(s) == 1 && s[0] >= '1' && s[0] <= byte('0'+n) {
			m.exportIdx = int(s[0] - '1')
		}
	}
	return m, nil
}

func (m tuiModel) handleEnvMgr(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "esc", "q", "ctrl+e":
		m.envMgrOpen = false
	case "up", "k":
		if m.envMgrIdx > 0 {
			m.envMgrIdx--
		}
	case "down", "j":
		if m.envMgrIdx < len(m.envs)-1 {
			m.envMgrIdx++
		}
	case "enter", " ":
		m.envIdx = m.envMgrIdx // set active
	}
	return m, nil
}
