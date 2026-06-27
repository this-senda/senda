package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"senda/internal/model"
)

// recordingServer captures the paths it was asked for, so tests can assert which
// URL the pipeline actually built after interpolation.
func recordingServer(t *testing.T, body func(path string) string) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(body(r.URL.Path)))
	}))
	t.Cleanup(srv.Close)
	return srv, &paths
}

// TestExtraVarsInterpolateIntoURL is the regression test for the data-driven /
// loop bug: extra vars must reach {{}} interpolation, not only the script getVar.
func TestExtraVarsInterpolateIntoURL(t *testing.T) {
	srv, paths := recordingServer(t, func(string) string { return `{}` })
	s := NewSession()
	req := model.Request{Method: "GET", URL: srv.URL + "/posts/{{pid}}"}
	resp, _ := s.SendWithExtra(context.Background(), req, "", "", "", map[string]string{"pid": "42"})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if len(*paths) != 1 || (*paths)[0] != "/posts/42" {
		t.Errorf("server saw %v, want [/posts/42]", *paths)
	}
}

// TestResRefInterpolatesAcrossSends proves a second request resolves a value
// from the first request's response through a full send (not just resolveRes):
// the server checks the Authorization header it actually received.
func TestResRefInterpolatesAcrossSends(t *testing.T) {
	var mu sync.Mutex
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/me" {
			mu.Lock()
			gotAuth = r.Header.Get("Authorization")
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"sekret"}`))
	}))
	t.Cleanup(srv.Close)

	s := NewSession()
	// First send is "login.yaml" so its response is stored under slug "login".
	login := model.Request{Method: "GET", URL: srv.URL + "/login"}
	if resp, _ := s.SendWithExtra(context.Background(), login, "", "login.yaml", "", nil); resp.Error != "" {
		t.Fatalf("login error: %s", resp.Error)
	}
	// Second send references the first response's token in a header.
	next := model.Request{
		Method: "GET", URL: srv.URL + "/me",
		Headers: []model.KV{{Key: "Authorization", Value: "Bearer {{res.login.json.token}}", Enabled: true}},
	}
	if resp, _ := s.SendWithExtra(context.Background(), next, "", "me.yaml", "", nil); resp.Error != "" {
		t.Fatalf("me error (ref unresolved?): %s", resp.Error)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotAuth != "Bearer sekret" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sekret")
	}
}
