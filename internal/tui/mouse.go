package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// handleMouseClick routes a left-click at screen (x,y) to the element under it:
// open-request tabs, tree rows, or the request/response sub-tab strips.
func (m tuiModel) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	if m.showHelp || m.pickerOpen || m.paletteOpen || m.envMgrOpen || m.browseOpen {
		return m, nil
	}
	if y == 1 { // open-request tab bar
		return m.clickOpenTab(x)
	}
	if y < 2 {
		return m, nil // title bar
	}

	d := m.dims()
	switch {
	case m.exportOpen:
		if y == 5 && x >= 1 && x < 1+d.reqW {
			return m.setReqTab(tabStripHit(reqTabNames, m.reqBadges(), x-1))
		}
		cgX0 := d.reqW + 3
		if y == 4 && x >= cgX0 {
			return m.clickCodegenTab(x - cgX0)
		}
		return m, nil
	case m.testsView():
		return m, nil
	case m.wsView():
		return m.clickTree(x, y, d.treeW)
	}

	switch m.layout {
	case layoutFocus:
		return m, nil
	case layout3Pane:
		if x < d.treeW+2 {
			return m.clickTree(x, y, d.treeW)
		}
		reqX0 := d.treeW + 3
		respX0 := d.treeW + d.reqW + 5
		if y == 5 && x >= reqX0 && x < reqX0+d.reqW {
			return m.setReqTab(tabStripHit(reqTabNames, m.reqBadges(), x-reqX0))
		}
		if y == 5 && x >= respX0 && x < respX0+d.respW {
			return m.setRespTab(tabStripHit(respTabNames, m.respBadges(), x-respX0))
		}
		return m, nil
	default: // stacked
		if x < d.treeW+2 {
			return m.clickTree(x, y, d.treeW)
		}
		reqX0 := d.treeW + 3
		if y == 5 && x >= reqX0 {
			return m.setReqTab(tabStripHit(reqTabNames, m.reqBadges(), x-reqX0))
		}
		if y == d.reqH+7 && x >= reqX0 { // response tab row sits below the request card
			return m.setRespTab(tabStripHit(respTabNames, m.respBadges(), x-reqX0))
		}
		return m, nil
	}
}

// clickTree selects (and, for folders, toggles) the tree row under a click.
func (m tuiModel) clickTree(x, y, w int) (tea.Model, tea.Cmd) {
	if x < 1 || x > w {
		return m, nil
	}
	if y < 5 { // COLLECTIONS header + blank row occupy y3,y4
		return m, nil
	}
	idx := m.treeOff + (y - 5)
	if idx < 0 || idx >= len(m.rows) {
		return m, nil
	}
	m.cursor = idx
	m.focus = focusTree
	if r := m.rows[idx]; r.node.IsDir {
		m.expanded[r.node.Path] = !m.expanded[r.node.Path]
		m.rebuild()
		m.clampTreeScroll()
	}
	return m, m.onCursorMove()
}

func (m tuiModel) setReqTab(i int) (tea.Model, tea.Cmd) {
	if i < 0 {
		return m, nil
	}
	m.focus = focusReq
	m.reqTab = reqTab(i)
	m.refreshReqView()
	m.reqVp.SetYOffset(0)
	return m, nil
}

func (m tuiModel) setRespTab(i int) (tea.Model, tea.Cmd) {
	if i < 0 {
		return m, nil
	}
	m.focus = focusResp
	m.respTab = respTab(i)
	m.refreshRespView()
	m.vp.SetYOffset(0)
	return m, nil
}

// clickOpenTab activates the open-request tab whose segment contains x.
func (m tuiModel) clickOpenTab(x int) (tea.Model, tea.Cmd) {
	xx := outerMargin + 1
	for _, o := range m.open {
		method := strings.ToUpper(o.method)
		if method == "" {
			method = "—"
		}
		segW := 1 + lipgloss.Width(method) + lipgloss.Width(o.name) + 2 + 2
		if x >= xx && x < xx+segW {
			m.curPath = o.path
			m.loaded = false
			m.focus = focusReq
			for i, r := range m.rows { // sync the tree cursor to the opened request
				if r.node.Path == o.path {
					m.cursor = i
					m.clampTreeScroll()
					break
				}
			}
			return m, loadRequestCmd(o.path)
		}
		xx += segW
	}
	return m, nil
}

// clickCodegenTab switches the codegen language tab under x (relative to the
// codegen pane). Tabs are rendered as " label " segments.
func (m tuiModel) clickCodegenTab(localX int) (tea.Model, tea.Cmd) {
	x := 0
	for i, t := range codegenTabs {
		segW := lipgloss.Width(t.label) + 2
		if localX >= x && localX < x+segW {
			m.exportIdx = i
			return m, nil
		}
		x += segW
	}
	return m, nil
}

// handleMouseWheel scrolls the tree when the pointer is over it, otherwise the
// focused viewport.
func (m tuiModel) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	if m.browseOpen {
		if n := len(m.browseDirs); n > 0 {
			if msg.Button == tea.MouseWheelUp {
				m.browseIdx = (m.browseIdx + n - 1) % n
			} else {
				m.browseIdx = (m.browseIdx + 1) % n
			}
		}
		return m, nil
	}
	if m.showHelp || m.pickerOpen || m.paletteOpen || m.envMgrOpen {
		return m, nil
	}
	d := m.dims()
	overTree := d.treeW > 0 && msg.X < d.treeW+2 && !m.exportOpen && m.layout != layoutFocus
	if overTree {
		if msg.Button == tea.MouseWheelUp {
			m.moveCursor(-3)
		} else {
			m.moveCursor(3)
		}
		return m, m.onCursorMove()
	}
	var cmd tea.Cmd
	if m.focus == focusReq {
		m.reqVp, cmd = m.reqVp.Update(msg)
	} else {
		m.vp, cmd = m.vp.Update(msg)
	}
	return m, cmd
}
