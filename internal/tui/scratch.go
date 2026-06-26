package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"senda/internal/model"
	"senda/internal/store"
)

// scratchPrefix marks an unsaved, in-memory request tab. It is not a real path,
// so it never hits disk; isScratch tells the two apart.
const scratchPrefix = "\x00scratch:"

func isScratch(key string) bool { return strings.HasPrefix(key, scratchPrefix) }

// newScratch opens a fresh in-memory request tab (like the GUI's new tab). The
// request lives only in buf until ctrl+s saves it to a file.
func (m tuiModel) newScratch() (tea.Model, tea.Cmd) {
	m.flush()
	m.scratchN++
	key := fmt.Sprintf("%s%d", scratchPrefix, m.scratchN)
	req := model.Request{Method: "GET"}
	m.open = append(m.open, openReq{path: key, method: "GET", name: fmt.Sprintf("untitled-%d", m.scratchN)})
	m.buf[key] = req
	m.dirty[key] = true
	m.curPath = key
	m.cur = req
	m.loaded = true
	m.focus = focusReq
	m.editing, m.ac.open = false, false
	m.refreshReqView()
	return m, nil
}

// startSave persists the active request. A saved file is written in place; an
// unsaved scratch opens a name prompt first.
func (m tuiModel) startSave() (tea.Model, tea.Cmd) {
	if !m.loaded {
		return m, nil
	}
	if m.editing { // commit any in-progress field edit first
		m.commitURL()
	}
	if isScratch(m.curPath) {
		ti := textinput.New()
		ti.SetValue("untitled.yaml")
		ti.Focus()
		m.saveInput = ti
		m.saveOpen = true
		return m, nil
	}
	if err := store.SaveRequest(m.curPath, m.cur); err != nil {
		m.status = "save: " + err.Error()
		return m, nil
	}
	m.dirty[m.curPath] = false
	m.status = "saved " + filepath.Base(m.curPath)
	return m, nil
}

// handleSave drives the scratch save-as name prompt.
func (m tuiModel) handleSave(s string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch s {
	case "esc":
		m.saveOpen = false
		return m, nil
	case "enter":
		return m.commitSave()
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

// commitSave writes the scratch request under the typed name (relative to the
// collection root), rebinds the tab to the real path, and reloads the tree so
// the new file appears. ponytail: always saves under the collection root; add a
// folder picker if per-folder placement is ever needed.
func (m tuiModel) commitSave() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.saveInput.Value())
	if name == "" {
		m.saveOpen = false
		return m, nil
	}
	if !strings.HasSuffix(name, ".yaml") {
		name += ".yaml"
	}
	path := filepath.Join(m.collPath, name)
	if err := store.SaveRequest(path, m.cur); err != nil {
		m.status = "save: " + err.Error()
		m.saveOpen = false
		return m, nil
	}

	old := m.curPath
	delete(m.buf, old)
	delete(m.dirty, old)
	for i, o := range m.open {
		if o.path == old {
			m.open[i].path = path
			m.open[i].name = strings.TrimSuffix(filepath.Base(path), ".yaml")
		}
	}
	m.buf[path] = m.cur
	m.dirty[path] = false
	m.curPath = path
	m.saveOpen = false
	m.status = "saved " + name

	// Reload the collection so the tree shows the new file.
	if coll, err := store.OpenCollection(m.collPath); err == nil {
		m.coll = coll
		m.expandAll(coll.Tree)
		m.rebuild()
	}
	return m, nil
}
