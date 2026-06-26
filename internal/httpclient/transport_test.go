package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"senda/internal/model"
	"senda/internal/vars"
)

// TestConfigureInsecureTLS proves the TLS config actually flows through Send:
// httptest.NewTLSServer presents a self-signed cert, so a default client
// rejects it and only Insecure:true lets the handshake through.
func TestConfigureInsecureTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := New()
	req := model.Request{Method: "GET", URL: srv.URL}
	scope := vars.Build()

	// Default transport: self-signed cert is untrusted -> error.
	if resp := c.Send(context.Background(), req, scope); resp.Error == "" {
		t.Fatalf("expected TLS verification error without Insecure, got status %d", resp.Status)
	}

	// Insecure skips verification -> request succeeds.
	if err := c.Configure(NetConfig{Insecure: true}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if resp := c.Send(context.Background(), req, scope); resp.Error != "" {
		t.Fatalf("expected success with Insecure, got error %q", resp.Error)
	}
}

func TestConfigureNoRebuildWhenUnchanged(t *testing.T) {
	c := New()
	if err := c.Configure(NetConfig{Insecure: true}); err != nil {
		t.Fatal(err)
	}
	tr := c.hc.Transport
	if tr == nil {
		t.Fatal("expected a custom transport after Configure")
	}
	if err := c.Configure(NetConfig{Insecure: true}); err != nil {
		t.Fatal(err)
	}
	if c.hc.Transport != tr {
		t.Fatal("transport rebuilt for unchanged config; connection pool would be dropped")
	}
}

func TestConfigureZeroResetsToDefault(t *testing.T) {
	c := New()
	_ = c.Configure(NetConfig{Proxy: "http://127.0.0.1:9"})
	if c.hc.Transport == nil {
		t.Fatal("expected custom transport for proxy config")
	}
	if err := c.Configure(NetConfig{}); err != nil {
		t.Fatal(err)
	}
	if c.hc.Transport != nil {
		t.Fatal("zero config should reset Transport to nil (http.DefaultTransport)")
	}
}

func TestConfigureBadProxy(t *testing.T) {
	c := New()
	if err := c.Configure(NetConfig{Proxy: "://nope"}); err == nil {
		t.Fatal("expected error for malformed proxy URL")
	}
}
