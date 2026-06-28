package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"senda/internal/model"
	"senda/internal/pipeline"
	"senda/internal/runner"
	"senda/internal/store"
	"senda/internal/termimg"
)

// TestCLIShot regenerates the senda run documentation screenshot
// (docs/screenshots/cli/01-run.png). It runs the *real* send pipeline against a
// local in-process server — no network — and renders senda run's actual stdout
// (via the shared formatResult) to a PNG with internal/termimg, the same
// headless renderer behind the TUI screenshots. Gated by SENDA_CLI_SHOTS so it
// never runs in a normal `go test ./...`; drive it with `task shots:cli`.
//
// Output dir: $SENDA_CLI_SHOT_DIR (default cmd/senda/tmp/shots). The
// Taskfile copies the result into docs/screenshots/cli/.
func TestCLIShot(t *testing.T) {
	if os.Getenv("SENDA_CLI_SHOTS") == "" {
		t.Skip("set SENDA_CLI_SHOTS=1 to regenerate the senda run screenshot")
	}

	// A tiny stand-in API. Each handler sleeps a few ms so the rendered
	// durations read like a real run instead of "0ms".
	var n int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(7+atomic.AddInt64(&n, 1)*3) * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Location", "/users/3")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":3,"name":"Grace Hopper","email":"grace@example.com"}`)
		case http.MethodPut, http.MethodPatch:
			fmt.Fprint(w, `{"id":1,"name":"Ada Lovelace","role":"admin"}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default: // GET
			if strings.HasPrefix(r.URL.Path, "/users/") {
				fmt.Fprint(w, `{"id":1,"name":"Ada Lovelace","email":"ada@example.com"}`)
				return
			}
			fmt.Fprint(w, `{"users":[{"id":1,"name":"Ada Lovelace"},{"id":2,"name":"Linus Torvalds"}],"meta":{"count":2}}`)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	kv := func(k, v string) model.KV { return model.KV{Key: k, Value: v, Enabled: true} }
	as := func(target, op, value string) model.Assert {
		return model.Assert{Target: target, Op: op, Value: value, Enabled: true}
	}
	jsonHdr := []model.KV{kv("Content-Type", "application/json")}

	// One folder of a believable CRUD workflow, each request with assertions so
	// the run shows the test tally. Numeric filename prefixes fix the order.
	reqs := []struct {
		file string
		req  model.Request
	}{
		{"1-list-users", model.Request{Name: "List users", Method: "GET", URL: "{{baseUrl}}/users",
			Asserts: []model.Assert{as("status", "eq", "200"), as("body", "contains", "users")}}},
		{"2-get-user", model.Request{Name: "Get user", Method: "GET", URL: "{{baseUrl}}/users/1",
			Asserts: []model.Assert{as("status", "eq", "200"), as("body", "contains", "Ada")}}},
		{"3-create-user", model.Request{Name: "Create user", Method: "POST", URL: "{{baseUrl}}/users", Headers: jsonHdr,
			Body:    model.Body{Type: "json", Raw: `{"name":"Grace Hopper","email":"grace@example.com"}`},
			Asserts: []model.Assert{as("status", "eq", "201"), as("body", "contains", "id")}}},
		{"4-update-user", model.Request{Name: "Update user", Method: "PUT", URL: "{{baseUrl}}/users/1", Headers: jsonHdr,
			Body:    model.Body{Type: "json", Raw: `{"role":"admin"}`},
			Asserts: []model.Assert{as("status", "eq", "200"), as("json.role", "eq", "admin")}}},
		{"5-delete-user", model.Request{Name: "Delete user", Method: "DELETE", URL: "{{baseUrl}}/users/1",
			Asserts: []model.Assert{as("status", "eq", "204")}}},
		{"6-search-users", model.Request{Name: "Search users", Method: "GET", URL: "{{baseUrl}}/users?q=ada",
			Asserts: []model.Assert{as("status", "eq", "200"), as("duration", "lt", "2000")}}},
	}
	for _, r := range reqs {
		if err := store.SaveRequest(filepath.Join(dir, r.file+".yaml"), r.req); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveEnvironment(dir, model.Environment{
		Name: "dev",
		Vars: []model.KV{kv("baseUrl", srv.URL)},
	}); err != nil {
		t.Fatal(err)
	}

	// Run exactly as the CLI does: read each request from disk, send it through a
	// pipeline session, format the line with the shared helper.
	paths, err := store.ListRequests(dir)
	if err != nil {
		t.Fatal(err)
	}
	session := pipeline.NewSession()
	send := func(ctx context.Context, path string) (model.Request, model.Response, error) {
		req, err := store.ReadRequest(path)
		if err != nil {
			return req, model.Response{}, err
		}
		resp, _ := session.Send(ctx, req, dir, path, "dev")
		return req, resp, nil
	}

	var lines []string
	results := runner.RunFolder(context.Background(), paths, send, func(r model.RunResult) {
		lines = append(lines, formatResult(r))
	})
	passed := 0
	for _, r := range results {
		if r.OK {
			passed++
		}
	}
	summary := fmt.Sprintf("%d/%d passed", passed, len(results))

	const cmd = "senda run -collection ./my-api -env dev"
	const prompt = "\x1b[32;1m$\x1b[0m " // bold green shell prompt
	still := prompt + cmd + "\n\n" + strings.Join(lines, "\n") + "\n\n" + summary + "\n"

	if passed != len(results) {
		t.Fatalf("screenshot run had failures (%d/%d) — fix the stub before regenerating:\n%s",
			passed, len(results), still)
	}

	outDir := os.Getenv("SENDA_CLI_SHOT_DIR")
	if outDir == "" {
		outDir = filepath.Join("tmp", "shots")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Still PNG — the final state of the run, rendered at 2× then downscaled
	// (SSAA) with softer hinting for smooth glyphs at normal size.
	shotOpts := termimg.Defaults()
	shotOpts.Supersample = 2
	shotOpts.SoftHinting = true
	rend, err := termimg.New(shotOpts)
	if err != nil {
		t.Fatalf("load fonts (need fonts-dejavu-core + fonts-freefont-ttf, or set SENDA_TUI_FONT*): %v", err)
	}
	f, err := os.Create(filepath.Join(outDir, "01-run.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := rend.PNG(f, still); err != nil {
		f.Close()
		t.Fatalf("render: %v", err)
	}
	f.Close()
	t.Logf("wrote 01-run.png:\n%s", still)

	// walkthrough.gif — type the command, then watch the results stream in line
	// by line (senda run prints each as its request lands), then the summary.
	// termimg.GIF pads shorter frames to the tallest, so the output grows
	// downward from a fixed top-left origin. Skip with SENDA_CLI_GIF=0.
	if os.Getenv("SENDA_CLI_GIF") == "0" {
		return
	}
	rGif, err := termimg.New(shotOpts)
	if err != nil {
		t.Fatal(err)
	}

	// assemble renders one frame: the prompt with `typed` so far, the first
	// nLines result lines, and optionally the summary.
	assemble := func(typed string, nLines int, withSummary bool) string {
		s := prompt + typed
		if nLines > 0 || withSummary {
			s += "\n\n"
			for i := 0; i < nLines && i < len(lines); i++ {
				s += lines[i] + "\n"
			}
			if withSummary {
				s += "\n" + summary
			}
		}
		return s
	}

	var frames []termimg.Frame
	add := func(ansi string, delayCs int) { frames = append(frames, termimg.Frame{ANSI: ansi, Delay: delayCs}) }

	add(assemble("", 0, false), 45) // empty prompt
	typed := ""
	for _, tok := range []string{"senda", " run", " -collection", " ./my-api", " -env", " dev"} {
		typed += tok
		add(assemble(typed, 0, false), 16) // typing the command
	}
	add(assemble(cmd, 0, false), 60) // a beat before the first result
	for i := 1; i <= len(lines); i++ {
		add(assemble(cmd, i, false), 48) // each result streams in
	}
	add(assemble(cmd, len(lines), false), 35)
	add(assemble(cmd, len(lines), true), 280) // hold the summary before looping

	gf, err := os.Create(filepath.Join(outDir, "walkthrough.gif"))
	if err != nil {
		t.Fatal(err)
	}
	defer gf.Close()
	if err := rGif.GIF(gf, frames); err != nil {
		t.Fatalf("gif: %v", err)
	}
	if st, _ := gf.Stat(); st != nil {
		t.Logf("wrote walkthrough.gif (%d frames, %d bytes)", len(frames), st.Size())
	}
}
