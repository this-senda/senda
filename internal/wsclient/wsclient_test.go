package wsclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"senda/internal/model"
	"senda/internal/vars"
)

// A persistent echo server (never closes) is exactly the case that used to hang
// Connect forever. The idle timeout must end the probe and return the exchange.
func TestConnectIdleTimeoutReturnsOnQuietServer(t *testing.T) {
	idleTimeout = 100 * time.Millisecond
	defer func() { idleTimeout = 5 * time.Second }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		// Echo one message, then stay open (go quiet) — never close.
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		_ = c.Write(r.Context(), websocket.MessageText, data)
		<-r.Context().Done()
	}))
	defer srv.Close()

	req := model.Request{
		URL:  "ws" + srv.URL[len("http"):], // http(s)://… → ws://…
		Body: model.Body{Raw: "ping"},
	}

	done := make(chan model.WSSession, 1)
	go func() { done <- Connect(context.Background(), req, vars.Build()) }()

	select {
	case s := <-done:
		if s.Error != "" {
			t.Fatalf("unexpected error: %s", s.Error)
		}
		if len(s.Messages) != 2 || s.Messages[0].Direction != "sent" || s.Messages[1].Direction != "received" {
			t.Fatalf("want sent+received, got %+v", s.Messages)
		}
		if s.Messages[1].Data != "ping" {
			t.Fatalf("echo mismatch: %q", s.Messages[1].Data)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Connect hung on a persistent server — idle timeout did not fire")
	}
}
