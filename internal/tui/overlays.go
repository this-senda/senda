package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// paletteBox renders the fuzzy command palette: a query line, a REQUESTS section
// of matching requests, then a COMMANDS section — matching the mockup.
func (m tuiModel) paletteBox() string {
	items := m.paletteItems()
	w := m.w * 55 / 100
	if w < 48 {
		w = 48
	}
	// .Width(w) is the content box incl. padding; with Padding(0,1) the text
	// area is two cells narrower, so build lines to lineW.
	lineW := w - 4
	var b strings.Builder

	count := styleDim.Render(fmt.Sprintf("%d results", len(items)))
	query := lipgloss.NewStyle().Foreground(colFg).Render(m.paletteQuery) +
		lipgloss.NewStyle().Foreground(colAccent).Render("▏")
	head := lipgloss.NewStyle().Foreground(colAccent).Render("❯ ") + query
	pad := lineW - lipgloss.Width(stripStyle(head)) - lipgloss.Width(stripStyle(count))
	if pad < 1 {
		pad = 1
	}
	b.WriteString(head + strings.Repeat(" ", pad) + count + "\n\n")

	const max = 9
	reqHeader, cmdHeader := false, false
	for i, it := range items {
		if i >= max {
			b.WriteString(styleDim.Render(fmt.Sprintf("  … %d more", len(items)-max)) + "\n")
			break
		}
		if it.cmd == "" && !reqHeader {
			b.WriteString(styleDim.Render("REQUESTS") + "\n")
			reqHeader = true
		}
		if it.cmd != "" && !cmdHeader {
			if reqHeader {
				b.WriteString("\n")
			}
			b.WriteString(styleDim.Render("COMMANDS") + "\n")
			cmdHeader = true
		}
		b.WriteString(paletteRow(it, i == m.paletteIdx, lineW) + "\n")
	}

	foot := strings.Join([]string{
		keyHintBg("↕", "navigate"), keyHintBg("↵", "select"),
		keyHintBg("⌥", "filter: requests"), keyHintBg("esc", "close"),
	}, "   ")
	b.WriteString("\n" + foot)
	return styleBorder.Padding(0, 1).Width(w).Render(b.String())
}

// paletteRow renders one palette entry padded to lineW, highlighted when active.
func paletteRow(it paletteItem, sel bool, lineW int) string {
	bg := bgPanel
	if sel {
		bg = bgSel
	}
	st := base.Background(bg)
	var icon, mid, right string
	if it.cmd == "" { // request
		method, path := it.label, ""
		if fields := strings.SplitN(it.label, " ", 2); len(fields) == 2 {
			method, path = fields[0], fields[1]
		}
		icon = st.Foreground(methodColor(method)).Render("●")
		mid = st.Foreground(colFg).Bold(true).Render(method) + st.Render(" ") + st.Foreground(colFg).Render(path)
		if sel {
			right = kbdPill("↵ open", bg)
		} else if it.desc != "" {
			right = st.Foreground(colDim).Render(it.desc)
		}
	} else { // command
		icon = st.Foreground(colWarn).Render("✦")
		mid = st.Foreground(colFg).Render(it.label)
		if sel {
			right = kbdPill("↵ run", bg)
		} else if it.cmd == "send" {
			right = kbdPill("⌘↵", bg)
		}
	}
	left := st.Render("  ") + icon + st.Render("  ") + mid
	gap := lineW - lipgloss.Width(stripStyle(left)) - lipgloss.Width(stripStyle(right))
	if gap < 1 {
		gap = 1
	}
	return left + st.Render(strings.Repeat(" ", gap)) + right
}

// kbdPill renders a small keycap on the input background over row bg, used for
// the palette's right-aligned action hints (↵ open / ⌘↵).
func kbdPill(text string, rowBg color.Color) string {
	return base.Background(rowBg).Render(" ") +
		base.Background(bgInput).Foreground(colAccent).Render(" "+text+" ")
}

// keyHintBg is keyHint on the panel background (for popups/footers).
func keyHintBg(key, label string) string {
	return base.Foreground(colFg).Render(key) + styleDim.Render(" "+label)
}

// helpBox renders the keybinding help overlay.
func (m tuiModel) helpBox() string {
	rows := [][2]string{
		{"tab / shift+tab", "cycle pane focus"},
		{"ctrl+\\", "cycle layout (stacked/3-pane/focus)"},
		{"ctrl+k", "command palette"},
		{"j / k  ↓ / ↑", "move · scroll"},
		{"ctrl+d / ctrl+u", "tree: half page down / up"},
		{"ctrl+f / ctrl+b", "tree: page down / up"},
		{"gg / G", "tree: jump to top / bottom"},
		{"h / l  ← / →", "tree: collapse/expand · pane: switch tab"},
		{"1–7", "jump to tab"},
		{"enter", "expand folder / load request"},
		{"s", "send request"},
		{"e", "edit request in $EDITOR"},
		{"x", "export request as code"},
		{"ctrl+w", "close current request tab"},
		{"[ / ]", "cycle environment"},
		{"⇧O", "open collection folder"},
		{"E", "environment picker"},
		{"ctrl+e", "environments manager"},
		{"?", "toggle this help"},
		{"q / ctrl+c", "quit"},
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("senda — keys") + "\n\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("%s  %s\n", lipgloss.NewStyle().Foreground(colAccent).Render(fmt.Sprintf("%-16s", r[0])), r[1]))
	}
	b.WriteString("\n" + styleDim.Render("press ? or esc to close"))
	return styleBorderFoc.Padding(1, 2).Render(b.String())
}

// browseBox renders the collection folder picker overlay.
func (m tuiModel) browseBox() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("open collection") + "\n")
	b.WriteString(styleDim.Render(truncate(m.browseDir, 60)) + "\n\n")
	if len(m.browseDirs) == 0 {
		b.WriteString(styleDim.Render("  (no subfolders)") + "\n")
	}
	const max = 12
	// Scroll a window of `max` entries so the selection stays visible.
	start := 0
	if m.browseIdx >= max {
		start = m.browseIdx - max + 1
	}
	if start > 0 {
		b.WriteString(styleDim.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}
	end := start + max
	if end > len(m.browseDirs) {
		end = len(m.browseDirs)
	}
	for i := start; i < end; i++ {
		name := truncate(m.browseDirs[i], 40) + "/"
		line := "  " + name
		if i == m.browseIdx {
			line = styleSel.Render(padRight("▸ "+name, 28))
		}
		b.WriteString(line + "\n")
	}
	if end < len(m.browseDirs) {
		b.WriteString(styleDim.Render(fmt.Sprintf("  ↓ %d more", len(m.browseDirs)-end)) + "\n")
	}
	if m.browseErr != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(colWarn).Render(m.browseErr) + "\n")
	}
	b.WriteString("\n" + styleDim.Render("↑/↓ select · → enter folder · ← up · ↵ open · o open current · esc cancel"))
	return styleBorderFoc.Padding(1, 2).Render(b.String())
}

// pickerBox renders the environment selection overlay.
func (m tuiModel) pickerBox() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("environment") + "\n\n")
	opts := append([]string{"(none)"}, envNames(m.envs)...)
	for i, name := range opts {
		line := "  " + name
		if i == m.pickerIdx {
			line = styleSel.Render(padRight("▸ "+name, 24))
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + styleDim.Render("↑/↓ select · enter apply · esc cancel"))
	return styleBorderFoc.Padding(1, 2).Render(b.String())
}
