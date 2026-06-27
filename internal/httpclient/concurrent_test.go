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
