package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"senda/internal/model"
	"senda/internal/vars"
)

// TestConcurrentConfigureAndSend is the regression for a flow's parallel nodes
// sharing one session: concurrent Configure (non-zero cfg forces a transport
// swap) + Send must be race-free. Run with -race to catch the bug it guards.
func TestConcurrentConfigureAndSend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New()
	cfg := NetConfig{Insecure: true} // non-zero → forces buildTransport on first call
	scope := vars.Build()
	req := model.Request{Method: "GET", URL: srv.URL}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Configure(cfg); err != nil {
				t.Errorf("configure: %v", err)
				return
			}
			resp := c.Send(context.Background(), req, scope)
			if resp.Error != "" {
				t.Errorf("send: %s", resp.Error)
			}
		}()
	}
	wg.Wait()
}

// TestSendConcurrentNoRace sends many requests concurrently on a single Client,
// the way a flow's parallel node does. The per-send httptrace callbacks may run
// on background dial goroutines that outlive Do, so without synchronisation the
// timing capture races. Run with -race; this guards that path.
func TestSendConcurrentNoRace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New()
	scope := vars.Build()

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := c.Send(context.Background(), model.Request{Method: "GET", URL: srv.URL}, scope)
			if resp.Status != http.StatusOK {
				t.Errorf("status = %d, want 200 (err %q)", resp.Status, resp.Error)
			}
		}()
	}
	wg.Wait()
}
