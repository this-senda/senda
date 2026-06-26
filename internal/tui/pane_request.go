package tui

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"

	"senda/internal/model"
	"senda/internal/vars"
)

// reqBadges builds the request tab badges (counts + the auth-type pill).
func (m tuiModel) reqBadges() []string {
	authBadge := ""
	switch m.cur.Auth.Type {
	case model.AuthBearer:
		authBadge = "Bearer"
	case model.AuthBasic:
		authBadge = "Basic"
	case model.AuthAPIKey:
		authBadge = "API key"
	case model.AuthOAuth2:
		authBadge = "OAuth2"
	}
	scripts := 0
	if strings.TrimSpace(m.cur.PreScript) != "" {
		scripts++
	}
	if strings.TrimSpace(m.cur.PostScript) != "" {
		scripts++
	}
	// order: Params, Headers, Body, Auth, Scripts, Tests, Docs
	return []string{
		countBadge(len(m.cur.Params)), countBadge(len(m.cur.Headers)), "",
		authBadge, countBadge(scripts), countBadge(len(m.cur.Asserts)), "",
	}
}

func (m tuiModel) paneRequest(w, h int) string {
	focused := m.focus == focusReq
	if !m.loaded {
		lines := []string{m.paneLabel("REQUEST", focused), "", styleDim.Render("  select a request (enter)")}
		return paneBlock(lines, w, h)
	}
	tabs := tabStrip(reqTabNames, m.reqBadges(), int(m.reqTab))

	lines := []string{m.paneLabel("REQUEST", focused), m.urlRow(w)}
	if pv := m.urlPreview(w); pv != "" {
		lines = append(lines, pv)
	}
	lines = append(lines, tabs)
	lines = append(lines, strings.Split(m.reqVp.View(), "\n")...)
	return paneBlock(lines, w, h)
}

// urlRow renders the method pill · URL field · Send button row.
func (m tuiModel) urlRow(w int) string {
	method := strings.ToUpper(m.cur.Method)
	if method == "" {
		method = "GET"
	}
	pill := base.Background(bgInput).Foreground(methodColor(method)).Bold(true).Render(" " + method + " ▾ ")
	send := base.Background(colSendBg).Foreground(bgApp).Bold(true).Render(" Send ↵ ")
	urlW := w - lipgloss.Width(pill) - lipgloss.Width(send) - 4
	if urlW < 8 {
		urlW = 8
	}
	if m.editing && m.editMode == editURL {
		field := base.Background(bgInput).Render(" ") + base.Background(bgInput).Render(m.input.View())
		return pill + base.Render(" ") + field + base.Render(" ") + send
	}
	raw := truncate(m.cur.URL, urlW-1)
	content := colorizeVars(raw, m.urlScope(), bgInput)
	if pad := urlW - 1 - lipgloss.Width(raw); pad > 0 {
		content += base.Background(bgInput).Render(strings.Repeat(" ", pad))
	}
	url := base.Background(bgInput).Render(" ") + content
	return pill + base.Render(" ") + url + base.Render(" ") + send
}

// resolveNonSecret substitutes {{var}} for the active environment's non-secret
// variables, leaving secrets (e.g. {{token}}) as placeholders.
func (m tuiModel) resolveNonSecret(s string) string {
	if m.envIdx < 0 || m.envIdx >= len(m.envs) {
		return s
	}
	var kvs []model.KV
	for _, v := range m.envs[m.envIdx].Vars {
		if !isSecret(v.Key) {
			kvs = append(kvs, v)
		}
	}
	return vars.Build(kvs).Apply(s)
}

// urlVarRe matches {{name}} placeholders (same grammar as internal/vars).
var urlVarRe = regexp.MustCompile(`\{\{\s*([\w.-]+)\s*\}\}`)

// urlScope builds the full layered scope (collection + env + secrets + runtime)
// for the active environment, used to tell resolvable vars from missing ones.
func (m tuiModel) urlScope() *vars.Scope {
	if m.session != nil {
		return m.session.Scope(m.collPath, m.curPath, m.envName())
	}
	return vars.Build()
}

// colorizeVars renders s with {{var}} tokens tinted by resolvability: accent
// when the name resolves in sc, red when it doesn't. Plain text stays colFg.
// Every segment carries the bg background so it tiles the URL field.
func colorizeVars(s string, sc *vars.Scope, bg color.Color) string {
	st := base.Background(bg)
	var b strings.Builder
	last := 0
	for _, loc := range urlVarRe.FindAllStringSubmatchIndex(s, -1) {
		if loc[0] > last {
			b.WriteString(st.Foreground(colFg).Render(s[last:loc[0]]))
		}
		name := s[loc[2]:loc[3]]
		col := colBad
		if sc != nil {
			if _, ok := sc.Get(name); ok {
				col = colAccent
			}
		}
		b.WriteString(st.Foreground(col).Render(s[loc[0]:loc[1]]))
		last = loc[1]
	}
	if last < len(s) {
		b.WriteString(st.Foreground(colFg).Render(s[last:]))
	}
	return b.String()
}

// urlPreview renders a dim resolved-URL line shown under the URL field, without
// mutating the editable text. Resolved values are green, secrets masked, and
// still-missing tokens stay red. Returns "" when the URL has no placeholders.
func (m tuiModel) urlPreview(w int) string {
	s := m.cur.URL
	if !urlVarRe.MatchString(s) {
		return ""
	}
	sc := m.urlScope()
	var b strings.Builder
	last := 0
	for _, loc := range urlVarRe.FindAllStringSubmatchIndex(s, -1) {
		if loc[0] > last {
			b.WriteString(base.Foreground(colFg).Render(s[last:loc[0]]))
		}
		name := s[loc[2]:loc[3]]
		switch {
		case isSecret(name):
			b.WriteString(styleDim.Render("••••"))
		default:
			if v, ok := sc.Get(name); ok {
				b.WriteString(base.Foreground(colGood).Render(v))
			} else {
				b.WriteString(base.Foreground(colBad).Render(s[loc[0]:loc[1]]))
			}
		}
		last = loc[1]
	}
	if last < len(s) {
		b.WriteString(base.Foreground(colFg).Render(s[last:]))
	}
	line := styleDim.Render("  ↳ ") + truncate(b.String(), w-8)
	if env := m.envName(); env != "" {
		line += base.Render("  ") + base.Foreground(colAccent).Render("◆ "+env)
	}
	return line
}

// numberedShell renders shell/code with a dim line-number gutter, coloring
// single-quoted strings green.
func numberedShell(s string) string {
	lines := strings.Split(s, "\n")
	gw := len(fmt.Sprintf("%d", len(lines)))
	if gw < 2 {
		gw = 2
	}
	var b strings.Builder
	for i, ln := range lines {
		gutter := styleDim.Render(fmt.Sprintf("%*d ", gw, i+1))
		b.WriteString(gutter + colorShell(ln) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// colorShell colors single-quoted string literals green; the rest stays fg.
func colorShell(line string) string {
	var b strings.Builder
	i := 0
	for i < len(line) {
		if line[i] == '\'' {
			j := i + 1
			for j < len(line) && line[j] != '\'' {
				j++
			}
			if j < len(line) {
				j++
			}
			b.WriteString(base.Foreground(colGood).Render(line[i:j]))
			i = j
		} else {
			j := i
			for j < len(line) && line[j] != '\'' {
				j++
			}
			b.WriteString(base.Foreground(colFg).Render(line[i:j]))
			i = j
		}
	}
	return b.String()
}

// renderReqTab builds the scrollable content for the active request tab.
func (m tuiModel) renderReqTab(w int) string {
	switch m.reqTab {
	case tabParams:
		return kvTable(m.cur.Params, w)
	case tabHeaders:
		return kvTable(m.cur.Headers, w)
	case tabBody:
		return m.renderBodyTab(w)
	case tabAuth:
		return m.renderAuthTab()
	case tabTests:
		return m.renderAssertsTab()
	case tabScripts:
		return m.renderScriptsTab()
	case tabDocs:
		if strings.TrimSpace(m.cur.Docs) == "" {
			return styleDim.Render("(no docs)")
		}
		return m.cur.Docs
	}
	return ""
}

// kvTable renders key/values as an aligned KEY / VALUE / DESCRIPTION table,
// matching the senda request mockups. Disabled rows are dimmed.
func kvTable(kvs []model.KV, w int) string {
	const boxW = 2 // checkbox column + gap
	keyW := w * 28 / 100
	valW := w * 36 / 100
	if keyW < 8 {
		keyW = 8
	}
	if valW < 8 {
		valW = 8
	}
	descW := w - boxW - keyW - valW - 2
	if descW < 4 {
		descW = 4
	}
	row := func(box, k, v, d string, st lipgloss.Style) string {
		return st.Foreground(colDim).Render(box+" ") +
			st.Render(padRight(truncate(k, keyW), keyW)) + base.Render(" ") +
			st.Render(padRight(truncate(v, valW), valW)) + base.Render(" ") +
			st.Foreground(colDim).Render(truncate(d, descW))
	}
	var b strings.Builder
	b.WriteString(styleDim.Render(strings.Repeat(" ", boxW)+padRight("KEY", keyW)+" "+padRight("VALUE", valW)+" DESCRIPTION") + "\n")
	for _, kv := range kvs {
		st := base.Foreground(colFg)
		box := "□"
		if !kv.Enabled {
			st = styleDim
			box = "▣"
		}
		b.WriteString(row(box, kv.Key, kv.Value, kv.Desc, st) + "\n")
	}
	// Dim placeholder row inviting a new entry, matching the mockups.
	b.WriteString(row("□", "key", "value", "", styleDim))
	return strings.TrimRight(b.String(), "\n")
}

func (m tuiModel) renderBodyTab(w int) string {
	bd := m.cur.Body
	t := bd.Type
	if t == "" {
		t = model.BodyNone
	}
	// While editing the body, the textarea owns the pane.
	if m.editing && m.editMode == editBody {
		return styleDim.Render("type: ") + string(t) + "\n" + m.body.View()
	}
	hint := ""
	if m.focus == focusReq {
		hint = styleDim.Render("   (t change type")
		if bodyRawTypes[t] {
			hint += styleDim.Render(" · i edit")
		}
		hint += styleDim.Render(")")
	}
	header := styleDim.Render("type: ") + string(t) + hint
	switch t {
	case model.BodyNone:
		return header
	case model.BodyForm, model.BodyMultipart:
		return header + "\n\n" + kvTable(bd.Form, w)
	case model.BodyGraphQL:
		out := header + "\n\n" + bd.Raw
		if strings.TrimSpace(bd.Variables) != "" {
			out += "\n\n" + styleDim.Render("variables:") + "\n" + bd.Variables
		}
		return out
	default: // json, raw, websocket, sse
		if strings.TrimSpace(bd.Raw) == "" {
			return header + "\n\n" + styleDim.Render("(empty — press i to edit)")
		}
		return header + "\n\n" + numberedCode(prettyJSON(bd.Raw))
	}
}

func (m tuiModel) renderAuthTab() string {
	a := m.cur.Auth
	t := a.Type
	if t == "" {
		t = model.AuthInherit
	}
	lines := []string{styleDim.Render("type: ") + string(t)}
	add := func(k, v string) {
		if v != "" {
			lines = append(lines, styleDim.Render(k+": ")+v)
		}
	}
	switch t {
	case model.AuthBearer:
		add("token", mask(a.Token))
	case model.AuthBasic:
		add("username", a.Username)
		add("password", mask(a.Password))
	case model.AuthAPIKey:
		add("key", a.Key)
		add("value", mask(a.KeyValue))
		add("placement", string(a.Placement))
	case model.AuthOAuth2:
		add("grant", string(a.Grant))
		add("tokenUrl", a.TokenURL)
		add("clientId", a.ClientID)
		add("clientSecret", mask(a.ClientSecret))
		add("scope", a.Scope)
	}
	return strings.Join(lines, "\n")
}

func (m tuiModel) renderAssertsTab() string {
	if len(m.cur.Asserts) == 0 {
		return styleDim.Render("(no asserts)")
	}
	var b strings.Builder
	for _, a := range m.cur.Asserts {
		mark := "•"
		if !a.Enabled {
			mark = styleDim.Render("✗")
		}
		b.WriteString(fmt.Sprintf("%s %s %s %s\n", mark, a.Target, a.Op, a.Value))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m tuiModel) renderScriptsTab() string {
	var parts []string
	if strings.TrimSpace(m.cur.PreScript) != "" {
		parts = append(parts, styleTitle.Render("pre-request")+"\n"+m.cur.PreScript)
	}
	if strings.TrimSpace(m.cur.PostScript) != "" {
		parts = append(parts, styleTitle.Render("post-response")+"\n"+m.cur.PostScript)
	}
	if len(parts) == 0 {
		return styleDim.Render("(no scripts)")
	}
	return strings.Join(parts, "\n\n")
}
