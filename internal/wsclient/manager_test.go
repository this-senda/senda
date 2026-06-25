package wsclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"senda/internal/model"
	"senda/internal/vars"
)

func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			typ, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if err := c.Write(r.Context(), typ, data); err != nil {
				return
			}
		}
	}))
}

// Open keeps the connection alive; Send delivers an interactive message and the
// echo comes back as a live "received" event through the emit callback.
func TestManagerOpenSendReceiveClose(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	var mu sync.Mutex
	var got []WSEvent
	emit := func(e WSEvent) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	}

	m := NewManager()
	req := model.Request{
		URL:  "ws" + srv.URL[len("http"):],
		Body: model.Body{Raw: "hello"}, // initial message, echoed back
	}
	id, err := m.Open(context.Background(), req, vars.Build(), emit)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := m.Send(id, "world"); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Wait for both echoes (hello, world) to arrive as received events.
	waitFor(t, &mu, &got, func() bool {
		n := 0
		for _, e := range got {
			if e.Message.Direction == "received" {
				n++
			}
		}
		return n >= 2
	})

	if err := m.Close(id); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Sending after close must fail (connection dropped from the registry).
	if err := m.Send(id, "late"); err == nil {
		t.Fatal("send after close should error")
	}

	// A close event must have been emitted.
	waitFor(t, &mu, &got, func() bool {
		for _, e := range got {
			if e.Closed {
				return true
			}
		}
		return false
	})
}

func waitFor(t *testing.T, mu *sync.Mutex, got *[]WSEvent, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		ok := cond()
		mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition not met within timeout; events=%+v", *got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
