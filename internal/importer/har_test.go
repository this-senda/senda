package importer

import (
	"encoding/base64"
	"strings"
	"testing"

	"senda/internal/model"
)

// sampleHAR exercises the import paths: a JSON POST with secret headers, a plain
// GET, a static asset, an analytics beacon, a duplicate, and a base64 body.
const sampleHAR = `{
  "log": {
    "version": "1.2",
    "creator": {"name": "WebInspector", "version": "537.36"},
    "entries": [
      {
        "request": {
          "method": "POST",
          "url": "https://api.example.com/login?debug=1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"},
            {"name": "Authorization", "value": "Bearer secret-token"},
            {"name": "Cookie", "value": "sid=abc123"},
            {"name": ":authority", "value": "api.example.com"}
          ],
          "postData": {"mimeType": "application/json", "text": "{\"user\":\"bob\"}"}
        },
        "response": {
          "status": 200, "statusText": "OK",
          "headers": [{"name": "Content-Type", "value": "application/json"}, {"name": "Set-Cookie", "value": "sid=xyz"}],
          "content": {"size": 16, "mimeType": "application/json", "text": "{\"ok\":true}"}
        }
      },
      {
        "request": {"method": "GET", "url": "https://api.example.com/users", "headers": []},
        "response": {"status": 200, "statusText": "OK", "headers": [], "content": {"size": 2, "mimeType": "application/json", "text": "[]"}}
      },
      {
        "request": {"method": "GET", "url": "https://cdn.example.com/app.js", "headers": []},
        "response": {"status": 200, "statusText": "OK", "headers": [], "content": {"size": 0, "mimeType": "application/javascript", "text": ""}}
      },
      {
        "request": {"method": "GET", "url": "https://www.google-analytics.com/collect?v=1", "headers": []},
        "response": {"status": 200, "statusText": "OK", "headers": [], "content": {"size": 0, "mimeType": "text/plain", "text": ""}}
      },
      {
        "request": {"method": "GET", "url": "https://api.example.com/users", "headers": []},
        "response": {"status": 200, "statusText": "OK", "headers": [], "content": {"size": 2, "mimeType": "application/json", "text": "[]"}}
      },
      {
        "request": {"method": "GET", "url": "https://api.example.com/avatar", "headers": []},
        "response": {"status": 200, "statusText": "OK", "headers": [], "content": {"size": 5, "mimeType": "text/plain", "encoding": "base64", "text": "aGVsbG8="}}
      }
    ]
  }
}`

func TestHARImport(t *testing.T) {
	items, skipped, err := HAR([]byte(sampleHAR))
	if err != nil {
		t.Fatal(err)
	}
	// 6 entries: login, users, app.js (static), analytics, users dup, avatar.
	// Kept: login, users, avatar = 3. Skipped: 3.
	if len(items) != 3 {
		t.Fatalf("imported %d requests, want 3: %+v", len(items), items)
	}
	if skipped != 3 {
		t.Errorf("skipped = %d, want 3 (static + analytics + dup)", skipped)
	}

	login := items[0].Request
	if login.Method != "POST" || login.URL != "https://api.example.com/login" {
		t.Errorf("login = %s %s", login.Method, login.URL)
	}
	if len(items[0].Dir) != 1 || items[0].Dir[0] != "api.example.com" {
		t.Errorf("dir = %v, want [api.example.com]", items[0].Dir)
	}
	// Query split out of the URL into Params.
	if len(login.Params) != 1 || login.Params[0].Key != "debug" || login.Params[0].Value != "1" {
		t.Errorf("params = %+v, want debug=1", login.Params)
	}
	// Secret headers and HTTP/2 pseudo-headers stripped; only Content-Type kept.
	for _, h := range login.Headers {
		if strings.EqualFold(h.Key, "authorization") || strings.EqualFold(h.Key, "cookie") || strings.HasPrefix(h.Key, ":") {
			t.Errorf("header %q should have been stripped", h.Key)
		}
	}
	if len(login.Headers) != 1 {
		t.Errorf("kept %d headers, want 1 (Content-Type): %+v", len(login.Headers), login.Headers)
	}
	if login.Body.Type != model.BodyJSON || login.Body.Raw != `{"user":"bob"}` {
		t.Errorf("body = %+v, want json {\"user\":\"bob\"}", login.Body)
	}
	if login.ResponseExample != `{"ok":true}` {
		t.Errorf("responseExample = %q", login.ResponseExample)
	}

	// avatar body was base64 "aGVsbG8=" => "hello".
	avatar := items[2].Request
	if avatar.ResponseExample != "hello" {
		t.Errorf("avatar example = %q, want decoded 'hello'", avatar.ResponseExample)
	}
}

func TestHARMocks(t *testing.T) {
	defs, err := HARMocks([]byte(sampleHAR))
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 3 {
		t.Fatalf("mocks = %d, want 3: %+v", len(defs), defs)
	}
	login := defs[0]
	if login.Method != "POST" || login.Path != "/login" {
		t.Errorf("mock route = %s %s, want POST /login", login.Method, login.Path)
	}
	if len(login.Responses) != 1 || login.Responses[0].Status != 200 {
		t.Fatalf("responses = %+v", login.Responses)
	}
	if login.Responses[0].Body != `{"ok":true}` {
		t.Errorf("mock body = %v", login.Responses[0].Body)
	}
	// Set-Cookie stripped from mock response headers.
	if _, ok := login.Responses[0].Headers["Set-Cookie"]; ok {
		t.Error("Set-Cookie should be stripped from mock response")
	}
}

// Two routes sharing a path's last segment (GET vs POST /users) must get
// distinct mock Names, else writeMocks' filename collides and drops one.
func TestHARMocksUniqueNames(t *testing.T) {
	har := `{"log":{"entries":[
	  {"request":{"method":"GET","url":"https://api.x.com/users","headers":[]},"response":{"status":200,"statusText":"OK","headers":[],"content":{"size":2,"mimeType":"application/json","text":"[]"}}},
	  {"request":{"method":"POST","url":"https://api.x.com/users","headers":[]},"response":{"status":201,"statusText":"Created","headers":[],"content":{"size":2,"mimeType":"application/json","text":"{}"}}}
	]}}`
	defs, err := HARMocks([]byte(har))
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 2 {
		t.Fatalf("mocks = %d, want 2 (GET + POST /users)", len(defs))
	}
	if defs[0].Name == defs[1].Name {
		t.Errorf("mock names collide: both %q", defs[0].Name)
	}
}

// Query params come out in a stable (sorted) order regardless of map iteration.
func TestHARParamOrderStable(t *testing.T) {
	har := `{"log":{"entries":[
	  {"request":{"method":"GET","url":"https://api.x.com/s?z=1&a=2&m=3","headers":[]},"response":{"status":200,"statusText":"OK","headers":[],"content":{"size":0,"mimeType":"application/json","text":""}}}
	]}}`
	items, _, err := HAR([]byte(har))
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, p := range items[0].Request.Params {
		got = append(got, p.Key)
	}
	want := []string{"a", "m", "z"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("param order = %v, want %v", got, want)
	}
}

func TestHAREmpty(t *testing.T) {
	if _, _, err := HAR([]byte(`{"log":{"entries":[]}}`)); err == nil {
		t.Error("empty HAR should error")
	}
}

func TestMarshalHARRoundTrip(t *testing.T) {
	req := model.Request{
		Name:    "create-user",
		Method:  "POST",
		URL:     "https://api.example.com/users",
		Params:  []model.KV{{Key: "ref", Value: "x", Enabled: true}},
		Headers: []model.KV{{Key: "Content-Type", Value: "application/json", Enabled: true}},
		Body:    model.Body{Type: model.BodyJSON, Raw: `{"name":"a"}`},
	}
	resp := model.Response{
		Status:     201,
		StatusText: "Created",
		DurationMs: 42,
		SizeBytes:  9,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       `{"id":1}`,
	}

	out, err := MarshalHAR(req, resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"version": "1.2"`) {
		t.Error("missing HAR version")
	}

	// Re-import: request fields survive the round trip.
	items, _, err := HAR(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("round-trip imported %d, want 1", len(items))
	}
	got := items[0].Request
	if got.Method != "POST" || got.URL != "https://api.example.com/users" {
		t.Errorf("round-trip req = %s %s", got.Method, got.URL)
	}
	if got.Body.Raw != `{"name":"a"}` {
		t.Errorf("round-trip body = %q", got.Body.Raw)
	}
	if got.ResponseExample != `{"id":1}` {
		t.Errorf("round-trip responseExample = %q", got.ResponseExample)
	}
}

// guard: base64 helper used above actually decodes as the test claims.
func TestDecodeBase64Sanity(t *testing.T) {
	if got := decodeContent(harContent{Text: base64.StdEncoding.EncodeToString([]byte("hi")), Encoding: "base64"}); got != "hi" {
		t.Errorf("decodeContent = %q", got)
	}
}
