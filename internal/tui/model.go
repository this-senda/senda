package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"senda/internal/model"
	"senda/internal/pipeline"
	"senda/internal/wsclient"
)

// focus identifies which pane receives navigation/scroll keys.
type focus int

const (
	focusTree focus = iota
	focusReq
	focusResp
)

// layoutMode selects the pane arrangement (cycled with ctrl+\).
type layoutMode int

const (
	layoutStacked layoutMode = iota // tree | (request stacked over response)
	layout3Pane                     // tree | request | response, three columns
	layoutFocus                     // no tree; request over response, centered
	layoutModeCount
)

var layoutNames = []string{"stacked", "three-pane", "focus"}

// reqTab indexes the request-detail tabs (ATAC-style tab bar).
type reqTab int

const (
	tabParams reqTab = iota
	tabHeaders
	tabBody
	tabAuth
	tabScripts
	tabTests
	tabDocs
	reqTabCount
)

var reqTabNames = []string{"Params", "Headers", "Body", "Auth", "Scripts", "Tests", "Docs"}

// respTab indexes the response-detail tabs.
type respTab int

const (
	rtabBody respTab = iota
	rtabHeaders
	rtabCookies
	rtabTests
	rtabTiming
	rtabLogs
	respTabCount
)

var respTabNames = []string{"Body", "Headers", "Cookies", "Tests", "Timing", "Logs"}

// row is one visible line in the flattened collection tree.
type row struct {
	node  *model.TreeNode
	depth int
}

// openReq is one entry in the open-request tab bar.
type openReq struct {
	path   string
	method string
	name   string
}

// respMsg carries a completed send back into the update loop.
type respMsg struct {
	resp model.Response
	err  string
}

// reqLoadedMsg carries a request read from disk after the cursor moved onto it.
type reqLoadedMsg struct {
	req  model.Request
	path string
	err  string
}

// editDoneMsg fires after $EDITOR exits so we can reload the edited request.
type editDoneMsg struct {
	path string
	err  error
}

type tuiModel struct {
	coll     model.Collection
	collPath string
	session  *pipeline.Session

	envs   []model.Environment
	envIdx int // -1 = none

	rows     []row
	cursor   int
	treeOff  int  // first visible tree row (vertical scroll offset)
	pendingG bool // first 'g' of a 'gg' (go-to-top) chord was pressed
	expanded map[string]bool

	cur     model.Request // request under cursor (if a file)
	curPath string
	loaded  bool

	open []openReq // open-request tabs, in open order

	// In-memory edit buffers, keyed by tab key (curPath, or a scratch sentinel
	// for unsaved requests). Only dirty/scratch tabs are cached here so unsaved
	// edits survive tab switches; clean tabs still reload from disk.
	buf      map[string]model.Request
	dirty    map[string]bool
	scratchN int // counter for unique scratch tab keys

	// inline edit mode. When editing, the active widget owns keystrokes.
	editing  bool
	editMode editMode        // which field is being edited (URL or body)
	input    textinput.Model // single-line URL editor
	body     textarea.Model  // multi-line body editor
	ac       autocomplete    // {{var}}/faker completion popup

	// save-as prompt for scratch tabs (ctrl+s on an unsaved request)
	saveOpen  bool
	saveInput textinput.Model

	fakeCache []acItem // cached faker tokens for autocomplete (built lazily)

	resp    *model.Response
	respErr string

	reqTab  reqTab
	respTab respTab

	reqVp   viewport.Model
	vp      viewport.Model // response body/detail viewport
	spin    spinner.Model
	sending bool
	focus   focus
	layout  layoutMode

	// overlays
	showHelp   bool
	pickerOpen bool
	pickerIdx  int // 0 = none, 1..len(envs) = env i-1
	exportOpen bool
	exportIdx  int // index into codegen.Targets

	// command palette (ctrl+k)
	paletteOpen  bool
	paletteQuery string
	paletteIdx   int

	// environments manager (ctrl+e)
	envMgrOpen bool
	envMgrIdx  int

	// collection folder picker (O)
	browseOpen bool
	browseDir  string
	browseDirs []string
	browseIdx  int
	browseErr  string

	// websocket session (populated when a WS request is connected)
	ws       *wsState
	wsMgr    *wsclient.Manager     // lazily created on first connect
	wsEvents chan wsclient.WSEvent // live frames from the read pump
	wsID     string                // active connection id for Send/Close

	w, h   int
	status string
}

// wsFrame is one message in the websocket log: a timestamp, direction (sent or
// received), and the JSON/text payload.
type wsFrame struct {
	ts   string
	out  bool
	text string
}

// wsState holds the live websocket connection view for a WS request.
type wsState struct {
	connected bool
	url       string
	msgs      int
	uptime    string
	since     time.Time // connect time; uptime is derived from it while live
	err       string    // dial/close error, shown in the header
	frames    []wsFrame
	compose   string
	// last inbound frame inspector fields
	opcode string
	size   string
}

// wsView reports whether the loaded request is a websocket request (its panes
// replace request/response with the connection log and frame inspector).
func (m tuiModel) wsView() bool {
	return m.loaded && !m.exportOpen && !m.editing && m.isWSRequest()
}

// isWSRequest reports whether the active request is a websocket: method WS, or a
// ws://-/wss://-scheme URL. The scheme check is what makes the connection pane
// appear as soon as a wss:// URL is typed.
func (m tuiModel) isWSRequest() bool {
	if strings.EqualFold(m.cur.Method, "WS") {
		return true
	}
	u := strings.ToLower(strings.TrimSpace(m.cur.URL))
	return strings.HasPrefix(u, "ws://") || strings.HasPrefix(u, "wss://")
}

func newModel(coll model.Collection, collPath string, envs []model.Environment, initialEnv string) tuiModel {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := tuiModel{
		coll:     coll,
		collPath: collPath,
		session:  pipeline.NewSession(),
		envs:     envs,
		envIdx:   -1,
		expanded: map[string]bool{},
		buf:      map[string]model.Request{},
		dirty:    map[string]bool{},
		reqVp:    viewport.New(),
		vp:       viewport.New(),
		spin:     sp,
		focus:    focusTree,
	}
	for i, e := range envs {
		if e.Name == initialEnv {
			m.envIdx = i
		}
	}
	// Expand every folder by default so the whole collection is visible.
	m.expandAll(coll.Tree)
	m.rebuild()
	return m
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) envName() string {
	if m.envIdx < 0 || m.envIdx >= len(m.envs) {
		return ""
	}
	return m.envs[m.envIdx].Name
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.resize()
		m.refreshReqView()
		m.refreshRespView()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			return m.handleMouseClick(msg.X, msg.Y)
		}
		return m, nil

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)

	case reqLoadedMsg:
		if msg.path != m.curPath {
			return m, nil // cursor moved on; stale load
		}
		m.loaded = msg.err == ""
		m.cur = msg.req
		m.status = msg.err
		if m.loaded {
			m.buf[msg.path] = msg.req
			m.openTab(msg.path, msg.req.Method)
		}
		m.refreshReqView()
		return m, nil

	case respMsg:
		m.sending = false
		m.respErr = msg.err
		m.resp = &msg.resp
		m.respTab = rtabBody
		m.refreshRespView()
		m.vp.SetYOffset(0)
		m.focus = focusResp
		return m, nil

	case editDoneMsg:
		if msg.err != nil {
			m.status = "editor: " + msg.err.Error()
			return m, nil
		}
		return m, loadRequestCmd(msg.path)

	case wsConnectedMsg:
		return m, m.onWSConnected(msg)

	case wsEventMsg:
		return m, m.onWSEvent(msg)

	case wsTickMsg:
		if m.ws != nil && m.ws.connected {
			return m, wsTick() // keep the uptime clock ticking
		}
		return m, nil

	case spinner.TickMsg:
		if !m.sending {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	// Forward everything else to the focused scroller.
	switch m.focus {
	case focusResp:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	case focusReq:
		var cmd tea.Cmd
		m.reqVp, cmd = m.reqVp.Update(msg)
		return m, cmd
	}
	return m, nil
}

// testsView reports whether the dedicated tests results view (test list |
// summary+timing, no tree) should be shown.
func (m tuiModel) testsView() bool {
	return !m.exportOpen && m.respTab == rtabTests && m.resp != nil && len(m.resp.Asserts) > 0
}

func (m tuiModel) currentRow() (row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

func (m tuiModel) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
