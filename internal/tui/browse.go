package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"senda/internal/store"
)

// openBrowser opens the collection picker, listing the directories beside the
// current collection so sibling collections show immediately.
func (m tuiModel) openBrowser() tuiModel {
	m.browseOpen = true
	m.browseDir = filepath.Dir(m.collPath)
	m.browseErr = ""
	m.loadBrowseDir()
	// Land on the current collection's folder, not the top of the list.
	cur := filepath.Base(m.collPath)
	for i, name := range m.browseDirs {
		if name == cur {
			m.browseIdx = i
			break
		}
	}
	return m
}

// loadBrowseDir reads the subdirectories of browseDir (hidden ones skipped).
func (m *tuiModel) loadBrowseDir() {
	m.browseDirs = nil
	m.browseIdx = 0
	ents, err := os.ReadDir(m.browseDir)
	if err != nil {
		m.browseErr = err.Error()
		return
	}
	for _, e := range ents {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			m.browseDirs = append(m.browseDirs, e.Name())
		}
	}
}

func (m tuiModel) handleBrowse(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "esc", "q", "O":
		m.browseOpen = false
	case "up", "k":
		if n := len(m.browseDirs); n > 0 {
			m.browseIdx = (m.browseIdx + n - 1) % n
		}
	case "down", "j":
		if n := len(m.browseDirs); n > 0 {
			m.browseIdx = (m.browseIdx + 1) % n
		}
	case "left", "h":
		m.browseDir = filepath.Dir(m.browseDir)
		m.loadBrowseDir()
	case "right", "l":
		if m.browseIdx < len(m.browseDirs) {
			m.browseDir = filepath.Join(m.browseDir, m.browseDirs[m.browseIdx])
			m.loadBrowseDir()
		}
	case "o":
		return m.openCollection(m.browseDir)
	case "enter", " ":
		target := m.browseDir
		if m.browseIdx < len(m.browseDirs) {
			target = filepath.Join(m.browseDir, m.browseDirs[m.browseIdx])
		}
		return m.openCollection(target)
	}
	return m, nil
}

// openCollection loads root as the active collection, replacing the model. On
// failure it leaves the browser open and shows the error.
func (m tuiModel) openCollection(root string) (tea.Model, tea.Cmd) {
	coll, err := store.OpenCollection(root)
	if err != nil {
		m.browseErr = err.Error()
		return m, nil
	}
	envs, err := store.ListEnvironments(root)
	if err != nil {
		m.browseErr = err.Error()
		return m, nil
	}
	nm := newModel(coll, root, envs, "")
	nm.w, nm.h = m.w, m.h
	nm.resize()
	nm.refreshReqView()
	nm.refreshRespView()
	return nm, nil
}
