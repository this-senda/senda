// Package wsclient implements a WebSocket session for Senda.
// It connects to a ws:// or wss:// endpoint, optionally sends an initial
// message from the request body, and records the full exchange.
package wsclient

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"senda/internal/model"
	"senda/internal/vars"
)

// idleTimeout bounds how long Connect waits for the next server message before
// treating the probe as done. var (not const) so tests can shorten it.
var idleTimeout = 5 * time.Second

// Connect opens a WebSocket connection, sends the initial message (if any),
// listens until ctx is cancelled or the server closes the connection, and
// returns the full session log.
func Connect(ctx context.Context, req model.Request, scope *vars.Scope) model.WSSession {
	rawURL := scope.Apply(req.URL)

	// Build extra headers.
	hdrs := http.Header{}
	for _, kv := range req.Headers {
		if kv.Enabled {
			hdrs.Set(scope.Apply(kv.Key), scope.Apply(kv.Value))
		}
	}

	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, rawURL, &websocket.DialOptions{
		HTTPHeader: hdrs,
	})
	if err != nil {
		return model.WSSession{Error: "dial: " + err.Error()}
	}
	defer conn.CloseNow()

	var session model.WSSession

	// Send initial message if body is set.
	if req.Body.Raw != "" {
		msg := scope.Apply(req.Body.Raw)
		if werr := conn.Write(ctx, websocket.MessageText, []byte(msg)); werr != nil {
			return model.WSSession{Error: "write: " + werr.Error()}
		}
		session.Messages = append(session.Messages, model.WSMessage{
			Direction: "sent",
			Data:      msg,
			At:        time.Now().UnixMilli(),
		})
	}

	// Read messages until the server closes, the user cancels, or the server
	// goes quiet. The UI is a one-shot probe (send initial message, collect
	// replies) with no live message input, so a persistent server like an echo
	// endpoint would otherwise block here forever and the call never returns.
	// ponytail: idle timeout = completion signal for the probe. Replace with
	// Wails event streaming + a connection handle if interactive WS is wanted.
	for {
		readCtx, rcancel := context.WithTimeout(ctx, idleTimeout)
		msgType, data, err := conn.Read(readCtx)
		rcancel()
		if err != nil {
			// Server quiet for `idle` with the outer ctx still live → done probing.
			if readCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
				session.CloseCode = int(websocket.StatusNormalClosure)
			} else if websocket.CloseStatus(err) != -1 {
				// Normal close — extract code.
				session.CloseCode = int(websocket.CloseStatus(err))
			} else if ctx.Err() != nil {
				// Context cancelled by user (stop button).
				session.CloseCode = int(websocket.StatusNormalClosure)
			} else {
				session.Error = err.Error()
			}
			break
		}
		_ = msgType
		session.Messages = append(session.Messages, model.WSMessage{
			Direction: "received",
			Data:      string(data),
			At:        time.Now().UnixMilli(),
		})
	}
	conn.Close(websocket.StatusNormalClosure, "")
	return session
}

// SendMessage sends a single message over an existing connection.
// This is used for the UI "send message" action; the connection must be kept
// alive in a goroutine via Connect.
// For the simple desktop use case we open a fresh connection per send.
func SendMessage(ctx context.Context, req model.Request, message string, scope *vars.Scope) model.WSSession {
	req.Body.Raw = message
	return Connect(ctx, req, scope)
}
