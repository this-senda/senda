package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"senda/internal/model"
)

// TestDoSendScriptChain exercises the full pipeline: a login request's
// post-script extracts a token into a runtime var, and the next request's
// {{token}} placeholder plus pre-script header both resolve against it.
func TestDoSendScriptChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok-99"})
		case "/data":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"auth":  r.Header.Get("Authorization"),
				"trace": r.Header.Get("X-Trace"),
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	app := NewApp()
	ctx := context.Background()

	login := model.Request{
		Method:     "GET",
		URL:        srv.URL + "/login",
		PostScript: `senda.setVar("token", res.json.token);`,
	}
	resp, _ := app.session.Send(ctx, login, "", "", "")
	if resp.Error != "" || resp.Status != 200 {
		t.Fatalf("login: %+v", resp)
	}
	if got := app.session.RuntimeKVs(); len(got) != 1 || got[0].Key != "token" || got[0].Value != "tok-99" {
		t.Fatalf("runtime vars = %+v", got)
	}

	data := model.Request{
		Method: "GET",
		URL:    srv.URL + "/data",
		Headers: []model.KV{
			{Key: "Authorization", Value: "Bearer {{token}}", Enabled: true},
		},
		PreScript: `req.setHeader("X-Trace", senda.getVar("token") + "-trace");`,
	}
	resp, _ = app.session.Send(ctx, data, "", "", "")
	if resp.Error != "" {
		t.Fatalf("data: %+v", resp)
	}
	if !strings.Contains(resp.Body, `"auth":"Bearer tok-99"`) {
		t.Errorf("interpolated runtime var missing: %s", resp.Body)
	}
	if !strings.Contains(resp.Body, `"trace":"tok-99-trace"`) {
		t.Errorf("pre-script header missing: %s", resp.Body)
	}
}

// TestDoSendPreScriptError aborts the send and surfaces the script error.
func TestDoSendPreScriptError(t *testing.T) {
	app := NewApp()
	req := model.Request{URL: "http://unused.invalid", PreScript: `throw new Error("nope")`}
	resp, _ := app.session.Send(context.Background(), req, "", "", "")
	if resp.Status != 0 || !strings.Contains(resp.Error, "nope") {
		t.Fatalf("want pre-script error, got %+v", resp)
	}
}

// TestResolveScopeFolderChain verifies the UI-facing scope resolver surfaces
// folder-level variables (and applies the same precedence as a send): a folder
// var overrides the collection var of the same name, reported with source
// "folder". This is what keeps {{HOST}} from showing "unresolved" in the URL
// field when HOST is defined on the enclosing folder rather than the collection.
func TestResolveScopeFolderChain(t *testing.T) {
	root := t.TempDir()
	writeMeta := func(dir, body string) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "senda.meta.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Collection root defines HOST + BASE; the Trips folder overrides HOST.
	writeMeta(root, "vars:\n  - {key: HOST, value: https://prod.example.com, enabled: true}\n  - {key: BASE, value: /api, enabled: true}\n")
	tripsDir := filepath.Join(root, "Trips")
	writeMeta(tripsDir, "vars:\n  - {key: HOST, value: http://localhost:8787, enabled: true}\n")
	reqPath := filepath.Join(tripsDir, "get-trips.yaml")
	if err := os.WriteFile(reqPath, []byte("method: GET\nurl: \"{{HOST}}{{BASE}}/trips\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scope := NewApp().ResolveScope(root, reqPath, "")
	byKey := map[string]ScopeVar{}
	for _, v := range scope {
		byKey[v.Key] = v
	}
	if got := byKey["HOST"]; got.Value != "http://localhost:8787" || got.Source != "folder" {
		t.Errorf("HOST = %+v, want folder override http://localhost:8787", got)
	}
	if got := byKey["BASE"]; got.Value != "/api" || got.Source != "collection" {
		t.Errorf("BASE = %+v, want collection /api", got)
	}
}
