package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"senda/internal/fake"
	"senda/internal/model"
)

// editMode selects which field the inline editor is bound to.
type editMode int

const (
	editNone editMode = iota
	editURL           // single-line textinput over cur.URL
	editBody          // multi-line textarea over cur.Body.Raw
)

// acItem is one autocomplete candidate: insert is the text written into the
// field (a var name or "$faker.token"), label/detail are shown in the popup.
type acItem struct {
	insert string
	label  string
	detail string
}

// autocomplete is the {{…}} completion popup state for the inline editor.
type autocomplete struct {
	open  bool
	items []acItem // candidates filtered by the partial token
	idx   int
	start int // offset in the field value where the partial token begins (after "{{")
}

// bodyRawTypes are the body kinds whose payload lives in Body.Raw and is editable
// as free text. form/multipart use key-value tables (a later phase); none has no
// payload.
var bodyRawTypes = map[model.BodyType]bool{
	model.BodyJSON: true, model.BodyRaw: true, model.BodyGraphQL: true,
}

// bodyTypeCycle is the order the Body tab's type rotates through on `t`. It lists
// only the HTTP body encodings; websocket/sse are transport-coupled (driven by
// the WS method + connect pane), not pickable body types here.
var bodyTypeCycle = []model.BodyType{
	model.BodyNone, model.BodyJSON, model.BodyRaw, model.BodyForm,
	model.BodyMultipart, model.BodyGraphQL,
}

// cycleBodyType advances the active request's body type and marks the tab dirty.
func (m tuiModel) cycleBodyType() (tea.Model, tea.Cmd) {
	if !m.loaded {
		return m, nil
	}
	cur := m.cur.Body.Type
	if cur == "" {
		cur = model.BodyNone
	}
	next := model.BodyJSON
	for i, t := range bodyTypeCycle {
		if t == cur {
			next = bodyTypeCycle[(i+1)%len(bodyTypeCycle)]
			break
		}
	}
	m.cur.Body.Type = next
	m.buf[m.curPath] = m.cur
	m.dirty[m.curPath] = true
	m.refreshReqView()
	return m, nil
}

// startEditURL enters insert mode on the URL field.
func (m tuiModel) startEditURL() (tea.Model, tea.Cmd) {
	if !m.loaded {
		return m, nil
	}
	ti := textinput.New()
	ti.SetValue(m.cur.URL)
	if w := m.reqVp.Width() - 14; w > 8 {
		ti.SetWidth(w)
	}
	cmd := ti.Focus()
	ti.CursorEnd()
	m.input = ti
	m.editing = true
	m.editMode = editURL
	m.ac = autocomplete{}
	return m, cmd
}

// startEditBody enters insert mode on the body payload. It only applies to raw
// body types; for none/form/multipart it is a no-op (cycle the type with `t`).
func (m tuiModel) startEditBody() (tea.Model, tea.Cmd) {
	if !m.loaded || !bodyRawTypes[m.cur.Body.Type] {
		return m, nil
	}
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.SetValue(m.cur.Body.Raw)
	if w := m.reqVp.Width(); w > 8 {
		ta.SetWidth(w)
	}
	if h := m.reqVp.Height() - 2; h > 2 {
		ta.SetHeight(h)
	}
	cmd := ta.Focus()
	m.body = ta
	m.editing = true
	m.editMode = editBody
	m.ac = autocomplete{}
	m.refreshReqView()
	return m, cmd
}

// commitEdit writes the edited field back into the active request buffer, marks
// the tab dirty, and leaves insert mode.
func (m *tuiModel) commitEdit() {
	switch m.editMode {
	case editURL:
		m.cur.URL = m.input.Value()
		// A ws://-/wss://-scheme URL makes this a websocket request; promote the
		// method so the connection pane takes over (the transport, not the body).
		u := strings.ToLower(strings.TrimSpace(m.cur.URL))
		if (strings.HasPrefix(u, "ws://") || strings.HasPrefix(u, "wss://")) && !strings.EqualFold(m.cur.Method, "WS") {
			m.cur.Method = "WS"
			for i := range m.open {
				if m.open[i].path == m.curPath {
					m.open[i].method = "WS"
				}
			}
		}
	case editBody:
		m.cur.Body.Raw = m.body.Value()
	}
	m.buf[m.curPath] = m.cur
	m.dirty[m.curPath] = true
	m.editing = false
	m.editMode = editNone
	m.ac.open = false
	m.refreshReqView()
}

// commitURL is retained for the save flow (commits a URL edit in progress).
func (m *tuiModel) commitURL() { m.commitEdit() }

// handleEdit routes keys while an inline editor is active. The popup, when open,
// consumes navigation/accept keys; everything else feeds the active widget and
// re-evaluates the {{token}} under the cursor.
func (m tuiModel) handleEdit(s string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.ac.open {
		switch s {
		case "up", "ctrl+p":
			if n := len(m.ac.items); n > 0 {
				m.ac.idx = (m.ac.idx + n - 1) % n
			}
			return m, nil
		case "down", "ctrl+n":
			if n := len(m.ac.items); n > 0 {
				m.ac.idx = (m.ac.idx + 1) % n
			}
			return m, nil
		case "tab", "enter":
			m.acceptCompletion()
			return m, nil
		case "esc":
			m.ac.open = false
			return m, nil
		}
	} else {
		// esc always commits. enter commits the single-line URL but inserts a
		// newline in the multi-line body.
		if s == "esc" || (s == "enter" && m.editMode == editURL) {
			m.commitEdit()
			return m, nil
		}
	}
	var cmd tea.Cmd
	if m.editMode == editBody {
		m.body, cmd = m.body.Update(msg)
		m.refreshReqView() // textarea lives in the scroll viewport; re-render it
	} else {
		m.input, cmd = m.input.Update(msg)
	}
	m.refreshCompletion()
	return m, cmd
}

// editText returns the active widget's value and the absolute cursor offset.
func (m tuiModel) editText() (string, int) {
	if m.editMode == editBody {
		v := m.body.Value()
		return v, lineColToOffset(v, m.body.Line(), m.body.Column())
	}
	return m.input.Value(), m.input.Position()
}

// editSet replaces the active widget's value and positions the cursor at offset.
func (m *tuiModel) editSet(val string, off int) {
	if m.editMode == editBody {
		m.body.SetValue(val)
		ln, col := offsetToLineCol(val, off)
		m.body.MoveToBegin()
		for i := 0; i < ln; i++ {
			m.body.CursorDown()
		}
		m.body.SetCursorColumn(col)
		return
	}
	m.input.SetValue(val)
	m.input.SetCursor(off)
}

// refreshCompletion inspects the text left of the cursor for an open "{{" token
// and opens/filters the popup accordingly.
func (m *tuiModel) refreshCompletion() {
	val, pos := m.editText()
	if pos > len(val) {
		pos = len(val)
	}
	left := val[:pos]
	open := strings.LastIndex(left, "{{")
	// Bail if there is no open "{{", or a "}}" already closed it.
	if open < 0 || strings.Contains(left[open:], "}}") {
		m.ac.open = false
		return
	}
	token := strings.TrimSpace(left[open+2:])
	m.ac.start = open + 2
	m.ac.items = filterCandidates(m.candidates(), token)
	m.ac.idx = 0
	m.ac.open = len(m.ac.items) > 0
}

// acceptCompletion replaces the partial token after "{{" with the selected
// candidate and appends "}}" if the field doesn't already close it.
func (m *tuiModel) acceptCompletion() {
	if m.ac.idx >= len(m.ac.items) {
		m.ac.open = false
		return
	}
	val, pos := m.editText()
	if pos > len(val) {
		pos = len(val)
	}
	chosen := m.ac.items[m.ac.idx].insert
	prefix := val[:m.ac.start]
	suffix := val[pos:]
	close := "}}"
	if strings.HasPrefix(suffix, "}}") {
		close = ""
	}
	m.editSet(prefix+chosen+close+suffix, len(prefix+chosen+close))
	m.ac.open = false
}

// candidates returns all completion entries: active-environment variables first,
// then every parameterless faker token. The faker list is generated once and
// cached (fake.Tokens generates a sample per token, so it isn't free).
func (m *tuiModel) candidates() []acItem {
	var out []acItem
	if m.envIdx >= 0 && m.envIdx < len(m.envs) {
		for _, v := range m.envs[m.envIdx].Vars {
			if v.Key == "" {
				continue
			}
			out = append(out, acItem{insert: v.Key, label: v.Key, detail: "env var"})
		}
	}
	if m.fakeCache == nil {
		for _, t := range fake.Tokens() {
			insert := "$" + t.Name
			if t.Category != "" {
				insert = "$" + t.Category + "." + t.Name
			}
			m.fakeCache = append(m.fakeCache, acItem{insert: insert, label: "$" + t.Name, detail: t.Example})
		}
	}
	return append(out, m.fakeCache...)
}

// filterCandidates keeps entries whose label/insert contains the partial token
// (case-insensitive). An empty token returns everything.
func filterCandidates(all []acItem, token string) []acItem {
	token = strings.ToLower(strings.TrimPrefix(token, "$"))
	if token == "" {
		return all
	}
	var out []acItem
	for _, it := range all {
		if strings.Contains(strings.ToLower(it.label), token) || strings.Contains(strings.ToLower(it.insert), token) {
			out = append(out, it)
		}
	}
	return out
}

// lineColToOffset converts a (line, column) cursor in val to a byte offset.
func lineColToOffset(val string, line, col int) int {
	off := 0
	lines := strings.Split(val, "\n")
	for i := 0; i < line && i < len(lines); i++ {
		off += len(lines[i]) + 1 // +1 for the newline
	}
	return off + col
}

// offsetToLineCol is the inverse of lineColToOffset.
func offsetToLineCol(val string, off int) (line, col int) {
	if off > len(val) {
		off = len(val)
	}
	nl := strings.Count(val[:off], "\n")
	col = off - (strings.LastIndex(val[:off], "\n") + 1)
	return nl, col
}
