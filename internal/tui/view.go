package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m tuiModel) render() string {
	if m.w == 0 || m.h == 0 {
		return "loading…"
	}
	switch {
	case m.envMgrOpen:
		return m.envMgrView()
	}

	d := m.dims()

	var body string
	switch {
	case m.exportOpen:
		// Codegen view: request | codegen, no tree.
		req := card(m.paneRequest(d.reqW, d.reqH), m.focus == focusReq)
		cg := card(m.paneCodegen(d.respW, d.respH), m.focus == focusResp)
		body = m.cardsRow(d.bodyH, req, cg)
	case m.testsView():
		// Tests results view: test list | summary+timing, no tree.
		left := card(m.paneTestList(d.reqW, d.reqH), m.focus == focusReq)
		right := card(m.paneTestSummary(d.respW, d.respH), m.focus == focusResp)
		body = m.cardsRow(d.bodyH, left, right)
	case m.wsView():
		// WebSocket view: tree | connection log | frame inspector.
		tree := card(m.paneTree(d.treeW, d.treeH), m.focus == focusTree)
		conn := card(m.paneWsConnection(d.reqW, d.reqH), m.focus == focusReq)
		frame := card(m.paneWsFrame(d.respW, d.respH), m.focus == focusResp)
		body = m.cardsRow(d.bodyH, tree, conn, frame)
	default:
		switch m.layout {
		case layout3Pane:
			tree := card(m.paneTree(d.treeW, d.treeH), m.focus == focusTree)
			req := card(m.paneRequest(d.reqW, d.reqH), m.focus == focusReq)
			resp := card(m.paneResponse(d.respW, d.respH), m.focus == focusResp)
			body = m.cardsRow(d.bodyH, tree, req, resp)
		case layoutFocus:
			block := card(m.paneFocus(d.reqW, d.reqH), true)
			body = lipgloss.Place(m.w, d.bodyH, lipgloss.Center, lipgloss.Top, block,
				lipgloss.WithWhitespaceStyle(appbg))
		default: // layoutStacked
			tree := card(m.paneTree(d.treeW, d.treeH), m.focus == focusTree)
			req := card(m.paneRequest(d.reqW, d.reqH), m.focus == focusReq)
			resp := card(m.paneResponse(d.respW, d.respH), m.focus == focusResp)
			rightParts := []string{req}
			if paneGap > 0 {
				rightParts = append(rightParts, vstrip(d.reqW+2, paneGap))
			}
			rightParts = append(rightParts, resp)
			right := lipgloss.JoinVertical(lipgloss.Left, rightParts...)
			body = m.cardsRow(d.bodyH, tree, right)
		}
	}

	screen := lipgloss.JoinVertical(lipgloss.Left, m.titleBar(), m.reqTabsBar(), body, m.statusBar())

	switch {
	case m.showHelp:
		return m.composite(screen, m.helpBox(), false)
	case m.pickerOpen:
		return m.composite(screen, m.pickerBox(), false)
	case m.browseOpen:
		return m.composite(screen, m.browseBox(), false)
	case m.paletteOpen:
		return m.composite(screen, m.paletteBox(), true)
	case m.saveOpen:
		return m.composite(screen, m.saveBox(), true)
	case m.ac.open:
		return m.composite(screen, m.acBox(), true)
	}
	return screen
}

// composite layers a popup box over the live screen so the app shows behind the
// modal (dimmed), matching the mockups. When top is set the box anchors near the
// top-center (command palette); otherwise it is centered.
func (m tuiModel) composite(screen, box string, top bool) string {
	boxW, boxH := lipgloss.Size(box)
	x := (m.w - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := (m.h - boxH) / 2
	if top {
		y = 3
	}
	if y < 0 {
		y = 0
	}
	screen = dimANSI(screen, 0.55) // recede the live UI behind the modal
	comp := lipgloss.NewCompositor(
		lipgloss.NewLayer(screen),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	)
	return comp.Render()
}

// titleBar renders the top chrome: window dots, a centered app/path/env line,
// and a right-aligned active-environment chip on the deepest background.
func (m tuiModel) titleBar() string {
	env := m.envName()
	if env == "" {
		env = "no env"
	}
	sep := appbg.Foreground(colSubtle).Render("  │  ")
	center := appbg.Foreground(colAccent).Bold(true).Render("▲ senda") +
		sep + appbg.Foreground(colDim).Render(m.coll.Name) +
		sep + appbg.Foreground(colDim).Render(env)
	chip := lipgloss.NewStyle().Background(bgSel).Foreground(colAccent).Bold(true).Render(" ◆ " + env + " ▾ ")

	leadGap := (m.w - lipgloss.Width(center)) / 2
	if leadGap < 1 {
		leadGap = 1
	}
	tailGap := m.w - leadGap - lipgloss.Width(center) - lipgloss.Width(chip)
	if tailGap < 1 {
		tailGap = 1
	}
	bar := appbg.Render(strings.Repeat(" ", leadGap)) + center +
		appbg.Render(strings.Repeat(" ", tailGap)) + chip
	return appbg.Width(m.w).Render(truncate(bar, m.w))
}

// reqTabsBar renders the open-request tabs (active one highlighted) plus a "+".
func (m tuiModel) reqTabsBar() string {
	indent := appbg.Render(strings.Repeat(" ", outerMargin+1)) // align with card content
	if len(m.open) == 0 {
		return appbg.Foreground(colDim).Width(m.w).Render(indent + appbg.Foreground(colDim).Render("no open requests — enter on a request to open"))
	}
	parts := []string{indent}
	for _, o := range m.open {
		method := strings.ToUpper(o.method)
		if method == "" {
			method = "—"
		}
		active := o.path == m.curPath
		bg := bgApp
		nameFg := colDim
		if active {
			bg = bgPanel
			nameFg = colFg
		}
		st := lipgloss.NewStyle().Background(bg)
		close := st.Foreground(colDim).Render("× ")
		if m.dirty[o.path] {
			close = st.Foreground(colWarn).Render("● ")
		}
		seg := st.Foreground(methodColor(method)).Bold(true).Render(" "+method) +
			st.Foreground(nameFg).Render(" "+o.name+" ") +
			close
		parts = append(parts, seg)
	}
	parts = append(parts, appbg.Foreground(colDim).Render(" + "))
	row := strings.Join(parts, "")
	return appbg.Width(m.w).Render(truncate(row, m.w))
}

// paneLabel renders an uppercase pane header; accent when the pane is focused.
func (m tuiModel) paneLabel(text string, focused bool) string {
	c := colDim
	if focused {
		c = colAccent
	}
	return base.Foreground(c).Bold(true).Render(text)
}

// statusLine lays left and right across the full bar width on the app
// background: left flush-left, right flush-right, truncated to fit.
func (m tuiModel) statusLine(left, right string) string {
	return appbg.Width(m.w).Render(truncate(padBetween(left, right, m.w, appbg), m.w))
}

// modeChip renders a coloured vim-style mode badge (e.g. " NORMAL ").
func modeChip(label string, bg color.Color) string {
	return lipgloss.NewStyle().Background(bg).Foreground(bgApp).Bold(true).Render(" " + label + " ")
}

func (m tuiModel) statusBar() string {
	if m.editing {
		field := "url"
		commit := "↵/esc"
		if m.editMode == editBody {
			field, commit = "body", "esc"
		}
		hint := keyHint("{{", "vars/faker") + appbg.Render("   ") + keyHint(commit, "commit") + appbg.Render(" ")
		return m.statusLine(modeChip("INSERT", colWarn)+appbg.Foreground(colDim).Render("  "+field), hint)
	}
	if m.status != "" {
		return appbg.Foreground(colBad).Width(m.w).Render(" " + truncate(m.status, m.w-2))
	}
	// Codegen mode shows a VISUAL chip and language-specific hints.
	if m.exportOpen {
		crumb := appbg.Foreground(colDim).Render("  code · ") +
			appbg.Foreground(colFg).Render(codegenTabs[m.exportIdx].label) + appbg.Render(" ")
		hints := strings.Join([]string{
			keyHint("h/l", "language"), keyHint("yy", "copy"),
			keyHint("^S", "save"), keyHint("esc", "back"),
		}, appbg.Render("   ")) + appbg.Render(" ")
		return m.statusLine(modeChip("VISUAL", colAccent)+crumb, hints)
	}
	// WebSocket view shows a live indicator and connection controls.
	if m.wsView() {
		live := appbg.Foreground(colGood).Render("  ● ") + appbg.Foreground(colDim).Render("live ")
		hints := strings.Join([]string{
			keyHint("i", "compose"), keyHint("^R", "reconnect"),
			keyHint("^L", "clear"), keyHint("^D", "disconnect"),
		}, appbg.Render("   ")) + appbg.Render(" ")
		return m.statusLine(modeChip("NORMAL", colGood)+live, hints)
	}
	// Vim-style mode chip (left), breadcrumb, then right-aligned context hints.
	crumb := appbg.Foreground(colDim).Render("  " + m.breadcrumb() + " ")
	help := m.contextHints() + appbg.Render(" ")
	return m.statusLine(modeChip("NORMAL", colGood)+crumb, help)
}

// breadcrumb shows the current request's folder › METHOD path location.
func (m tuiModel) breadcrumb() string {
	r, ok := m.currentRow()
	if !ok {
		return ""
	}
	if r.node.IsDir {
		return r.node.Name
	}
	method := strings.ToUpper(r.node.Method)
	name := strings.TrimSuffix(r.node.Name, ".yaml")
	crumb := method + " " + name
	if parent := m.parentFolder(m.cursor); parent != "" {
		crumb = parent + " › " + crumb
	}
	return crumb
}

// parentFolder returns the name of the nearest enclosing folder for row idx.
func (m tuiModel) parentFolder(idx int) string {
	if idx < 0 || idx >= len(m.rows) {
		return ""
	}
	depth := m.rows[idx].depth
	for i := idx - 1; i >= 0; i-- {
		if m.rows[i].node.IsDir && m.rows[i].depth < depth {
			if m.rows[i].depth == 0 {
				return "" // the collection root — omit from the crumb
			}
			return m.rows[i].node.Name
		}
	}
	return ""
}

// keyHint renders a "key label" pair for the status bar: the key glyph in a
// brighter grey, the label dim.
func keyHint(key, label string) string {
	return appbg.Foreground(colFg).Render(key) + appbg.Foreground(colDim).Render(" "+label)
}

// contextHints returns the most relevant keys for the focused pane, styled like
// the mockup status bars (key + label pairs, spaced).
func (m tuiModel) contextHints() string {
	var pairs [][2]string
	switch m.focus {
	case focusReq:
		pairs = [][2]string{{"i", "edit url"}, {"s", "send"}, {"^S", "save"}, {"n", "new"}, {"?", "help"}}
	case focusResp:
		pairs = [][2]string{{"^K", "jump"}, {"↵", "send"}, {"^\\", "panes"}, {"gd", "docs"}, {"?", "help"}}
	default:
		pairs = [][2]string{{"^K", "palette"}, {"j/k", "move"}, {"l", "focus"}, {"⇧O", "open"}, {"↵", "send"}, {"?", "help"}}
	}
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = keyHint(p[0], p[1])
	}
	sep := appbg.Render("   ")
	return strings.Join(parts, sep)
}
