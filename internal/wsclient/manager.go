package wsclient

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"senda/internal/model"
	"senda/internal/vars"
)

// WSEvent is pushed to the frontend for each live message and on close.
// ID routes the event to the right open connection (one per request tab).
type WSEvent struct {
	ID      string          `json:"id"`
	Message model.WSMessage `json:"message,omitempty"`
	Closed  bool            `json:"closed,omitempty"`
	Code    int             `json:"code,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type liveConn struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
}

// Manager holds open WebSocket connections so the UI can send messages
// interactively across separate Wails calls. Connect() (in wsclient.go) stays
// the one-shot probe for CLI/headless use.
type Manager struct {
	mu    sync.Mutex
	conns map[string]*liveConn
	seq   int
}

func NewManager() *Manager { return &Manager{conns: map[string]*liveConn{}} }

// Open dials the endpoint, sends the body's initial message if any, starts a
// read pump that emits each received message (and a final close event), and
// returns an id used by Send/Close. The read pump outlives this call, so it
// runs on a background context cancelled only by Close.
func (m *Manager) Open(parent context.Context, req model.Request, scope *vars.Scope, emit func(WSEvent)) (string, error) {
	rawURL := scope.Apply(req.URL)

	hdrs := http.Header{}
	for _, kv := range req.Headers {
		if kv.Enabled {
			hdrs.Set(scope.Apply(kv.Key), scope.Apply(kv.Value))
		}
	}

	dialCtx, dialCancel := context.WithTimeout(parent, 30*time.Second)
	defer dialCancel()
	conn, _, err := websocket.Dial(dialCtx, rawURL, &websocket.DialOptions{HTTPHeader: hdrs})
	if err != nil {
		return "", fmt.Errorf("dial: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.seq++
	id := fmt.Sprintf("ws-%d", m.seq)
	m.conns[id] = &liveConn{conn: conn, cancel: cancel}
	m.mu.Unlock()

	// Initial message from body Raw. Frontend renders the "sent" row itself.
	if req.Body.Raw != "" {
		_ = conn.Write(ctx, websocket.MessageText, []byte(scope.Apply(req.Body.Raw)))
	}

	go m.readPump(ctx, id, conn, emit)
	return id, nil
}

func (m *Manager) readPump(ctx context.Context, id string, conn *websocket.Conn, emit func(WSEvent)) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			ev := WSEvent{ID: id, Closed: true}
			if cs := websocket.CloseStatus(err); cs != -1 {
				ev.Code = int(cs)
			} else if ctx.Err() != nil {
				ev.Code = int(websocket.StatusNormalClosure)
			} else {
				ev.Error = err.Error()
			}
			emit(ev)
			m.drop(id)
			return
		}
		emit(WSEvent{ID: id, Message: model.WSMessage{
			Direction: "received",
			Data:      string(data),
			At:        time.Now().UnixMilli(),
		}})
	}
}

// Send writes a text message over an open connection.
func (m *Manager) Send(id, message string) error {
	m.mu.Lock()
	lc := m.conns[id]
	m.mu.Unlock()
	if lc == nil {
		return fmt.Errorf("no such connection: %s", id)
	}
	return lc.conn.Write(context.Background(), websocket.MessageText, []byte(message))
}

// Close tears down a connection. The read pump's ctx cancels, it emits a close
// event, and drops itself. Closing an unknown/already-closed id is a no-op.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	lc := m.conns[id]
	m.mu.Unlock()
	if lc == nil {
		return nil
	}
	lc.cancel()
	return lc.conn.Close(websocket.StatusNormalClosure, "")
}

func (m *Manager) drop(id string) {
	m.mu.Lock()
	delete(m.conns, id)
	m.mu.Unlock()
}
