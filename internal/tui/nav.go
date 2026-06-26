package tui

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"senda/internal/model"
)

func (m *tuiModel) expandAll(n *model.TreeNode) {
	if n == nil {
		return
	}
	if n.IsDir {
		m.expanded[n.Path] = true
	}
	for _, c := range n.Children {
		m.expandAll(c)
	}
}

// rebuild flattens the visible tree into m.rows, honoring collapsed folders.
func (m *tuiModel) rebuild() {
	m.rows = m.rows[:0]
	if m.coll.Tree == nil {
		return
	}
	var walk func(nodes []*model.TreeNode, depth int)
	walk = func(nodes []*model.TreeNode, depth int) {
		for _, n := range nodes {
			m.rows = append(m.rows, row{node: n, depth: depth})
			if n.IsDir && m.expanded[n.Path] {
				walk(n.Children, depth+1)
			}
		}
	}
	// The collection root is shown as the top tree row (▾ senda-api), matching
	// the mockups; its children hang one level beneath it.
	m.rows = append(m.rows, row{node: m.coll.Tree, depth: 0})
	if m.expanded[m.coll.Tree.Path] {
		walk(m.coll.Tree.Children, 1)
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// openTab records path in the open-request tab bar if not already present.
func (m *tuiModel) openTab(path, method string) {
	for _, o := range m.open {
		if o.path == path {
			return
		}
	}
	name := strings.TrimSuffix(filepath.Base(path), ".yaml")
	m.open = append(m.open, openReq{path: path, method: method, name: name})
}

// closeTab removes the active request's tab and, if it was current, clears the
// loaded request.
func (m *tuiModel) closeTab() {
	for i, o := range m.open {
		if o.path == m.curPath {
			m.open = append(m.open[:i], m.open[i+1:]...)
			m.disconnectWS()
			m.ws = nil
			delete(m.buf, m.curPath)
			delete(m.dirty, m.curPath)
			m.loaded = false
			m.curPath = ""
			m.cur = model.Request{}
			m.editing, m.ac.open = false, false
			m.refreshReqView()
			return
		}
	}
}

// treePage returns the number of visible tree rows (used by page motions).
func (m tuiModel) treePage() int {
	p := m.dims().treeH - 2
	if p < 1 {
		p = 1
	}
	return p
}

// moveCursor shifts the tree cursor by delta, clamped to the row range.
func (m *tuiModel) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(m.rows)-1 {
		m.cursor = len(m.rows) - 1
	}
}

// clampTreeScroll adjusts the tree scroll offset so the cursor stays within the
// visible window (treeH minus the header + blank rows).
func (m *tuiModel) clampTreeScroll() {
	cap := m.dims().treeH - 2
	if cap < 1 {
		cap = 1
	}
	if m.cursor < m.treeOff {
		m.treeOff = m.cursor
	}
	if m.cursor >= m.treeOff+cap {
		m.treeOff = m.cursor - cap + 1
	}
	if maxOff := len(m.rows) - cap; m.treeOff > maxOff {
		m.treeOff = maxOff
	}
	if m.treeOff < 0 {
		m.treeOff = 0
	}
}

// onCursorMove loads the request under the cursor (if it is a file).
func (m *tuiModel) onCursorMove() tea.Cmd {
	m.clampTreeScroll()
	r, ok := m.currentRow()
	if !ok || r.node.IsDir {
		m.flush()
		m.curPath = ""
		m.loaded = false
		m.refreshReqView()
		return nil
	}
	return m.openKey(r.node.Path)
}

// flush stashes the active tab's in-memory request into buf when it has unsaved
// edits (dirty) or is an unsaved scratch, so switching away never loses them.
func (m *tuiModel) flush() {
	if m.loaded && m.curPath != "" && (m.dirty[m.curPath] || isScratch(m.curPath)) {
		m.buf[m.curPath] = m.cur
	}
}

// openKey switches the active tab to key: it reuses the in-memory buffer for
// dirty/scratch tabs (preserving unsaved edits) and otherwise reloads from disk.
func (m *tuiModel) openKey(key string) tea.Cmd {
	if key == m.curPath {
		return nil
	}
	m.flush()
	m.disconnectWS() // tear down any live websocket on the tab we're leaving
	m.ws = nil
	m.curPath = key
	m.editing, m.ac.open = false, false
	if req, ok := m.buf[key]; ok && (m.dirty[key] || isScratch(key)) {
		m.cur = req
		m.loaded = true
		m.refreshReqView()
		return nil
	}
	m.loaded = false
	return loadRequestCmd(key)
}

// activate toggles a folder or loads+marks a request for sending.
func (m tuiModel) activate() (tea.Model, tea.Cmd) {
	r, ok := m.currentRow()
	if !ok {
		return m, nil
	}
	if r.node.IsDir {
		m.expanded[r.node.Path] = !m.expanded[r.node.Path]
		m.rebuild()
		m.clampTreeScroll()
		return m, nil
	}
	return m, m.openKey(r.node.Path)
}

// nextFocus advances pane focus by dir (±1), skipping the tree pane while in
// focus-mode layout (where the tree is hidden).
func (m tuiModel) nextFocus(dir int) focus {
	f := (int(m.focus) + dir + 3) % 3
	if m.layout == layoutFocus && focus(f) == focusTree {
		f = (f + dir + 3) % 3
	}
	return focus(f)
}
