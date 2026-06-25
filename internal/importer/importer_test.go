package importer

import (
	"os"
	"strings"
	"testing"

	"senda/internal/model"
)

func TestCurlBasicGet(t *testing.T) {
	req, err := Curl(`curl https://api.example.com/users`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.URL != "https://api.example.com/users" {
		t.Errorf("url = %q", req.URL)
	}
	if req.Name != "users" {
		t.Errorf("name = %q, want users", req.Name)
	}
}

func TestCurlPostJSONWithHeaders(t *testing.T) {
	cmd := `curl -X POST https://api.example.com/login \
	  -H "Content-Type: application/json" \
	  -H 'Accept: application/json' \
	  -d '{"user":"bob","pass":"secret"}'`
	req, err := Curl(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" {
		t.Errorf("method = %q", req.Method)
	}
	if len(req.Headers) != 2 {
		t.Fatalf("headers = %d, want 2: %+v", len(req.Headers), req.Headers)
	}
	if req.Body.Type != model.BodyJSON {
		t.Errorf("body type = %q, want json", req.Body.Type)
	}
	if req.Body.Raw != `{"user":"bob","pass":"secret"}` {
		t.Errorf("body = %q", req.Body.Raw)
	}
}

func TestCurlImpliesPostWhenData(t *testing.T) {
	req, err := Curl(`curl https://x.test/submit -d 'a=1&b=2'`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" {
		t.Errorf("method = %q, want POST", req.Method)
	}
	if req.Body.Type != model.BodyRaw {
		t.Errorf("body type = %q, want raw", req.Body.Type)
	}
}

func TestCurlBasicAuth(t *testing.T) {
	req, err := Curl(`curl -u admin:hunter2 https://x.test/secure`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Auth.Type != model.AuthBasic || req.Auth.Username != "admin" || req.Auth.Password != "hunter2" {
		t.Errorf("auth = %+v", req.Auth)
	}
}

func TestCurlGetWithData(t *testing.T) {
	req, err := Curl(`curl -G https://x.test/search -d q=hello -d page=2`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.URL != "https://x.test/search?q=hello&page=2" {
		t.Errorf("url = %q", req.URL)
	}
	if req.Body.Type != model.BodyNone {
		t.Errorf("body type = %q, want none", req.Body.Type)
	}
}

func TestCurlForm(t *testing.T) {
	req, err := Curl(`curl -F name=bob -F avatar=@pic.png https://x.test/upload`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Body.Type != model.BodyForm || len(req.Body.Form) != 2 {
		t.Fatalf("form body = %+v", req.Body)
	}
	if req.Body.Form[1].Value != "pic.png" {
		t.Errorf("@-prefixed file value = %q, want pic.png", req.Body.Form[1].Value)
	}
}

const postmanSample = `{
  "info": {"name": "Sample"},
  "item": [
    {
      "name": "Users",
      "item": [
        {
          "name": "List users",
          "request": {
            "method": "GET",
            "header": [{"key": "Accept", "value": "application/json"}],
            "url": {"raw": "https://api.test/users?page=1", "query": [{"key": "page", "value": "1"}]},
            "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{token}}"}]}
          }
        }
      ]
    },
    {
      "name": "Ping",
      "request": {"method": "GET", "url": "https://api.test/ping"}
    }
  ]
}`

func TestPostman(t *testing.T) {
	out, err := Postman([]byte(postmanSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("imported %d, want 2", len(out))
	}
	var listUsers *Imported
	for i := range out {
		if out[i].Request.Name == "List users" {
			listUsers = &out[i]
		}
	}
	if listUsers == nil {
		t.Fatal("List users not found")
	}
	if len(listUsers.Dir) != 1 || listUsers.Dir[0] != "Users" {
		t.Errorf("dir = %v, want [Users]", listUsers.Dir)
	}
	if listUsers.Request.URL != "https://api.test/users" {
		t.Errorf("url = %q", listUsers.Request.URL)
	}
	if len(listUsers.Request.Params) != 1 || listUsers.Request.Params[0].Key != "page" {
		t.Errorf("params = %+v", listUsers.Request.Params)
	}
	if listUsers.Request.Auth.Type != model.AuthBearer || listUsers.Request.Auth.Token != "{{token}}" {
		t.Errorf("auth = %+v", listUsers.Request.Auth)
	}
}

// Variants from postman-collection's examples/collection-v2.json (v2.0 era):
// string request shorthand, raw-HTTP-block header string, url object without
// "raw" (string host + string path), object-form auth params.
const postmanV2Sample = `{
  "info": {"name": "v2 quirks"},
  "item": [
    {
      "name": "status",
      "request": "http://echo.getpostman.com/status/200"
    },
    {
      "name": "string headers",
      "request": {
        "method": "POST",
        "header": "Content-Type: application/json\r\nAuthorization: Hawk id=\"x\", ts=\"123\"",
        "body": {"mode": "raw", "raw": "blahblah"},
        "url": {
          "protocol": "https",
          "host": "sub.example.com",
          "port": "8443",
          "path": "path/to/document"
        },
        "auth": {"type": "basic", "basic": {"username": "ada", "password": "hunter2"}}
      }
    },
    {
      "name": "array url parts",
      "request": {
        "method": "GET",
        "url": {"host": ["api", "test"], "path": ["v1", "users"]}
      }
    }
  ]
}`

func TestPostmanV2Quirks(t *testing.T) {
	out, err := Postman([]byte(postmanV2Sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("imported %d, want 3", len(out))
	}
	byName := map[string]model.Request{}
	for _, im := range out {
		byName[im.Request.Name] = im.Request
	}

	status := byName["status"]
	if status.Method != "GET" || status.URL != "http://echo.getpostman.com/status/200" {
		t.Errorf("string request = %+v", status)
	}

	sh := byName["string headers"]
	if sh.URL != "https://sub.example.com:8443/path/to/document" {
		t.Errorf("rebuilt url = %q", sh.URL)
	}
	if len(sh.Headers) != 2 || sh.Headers[0].Key != "Content-Type" || sh.Headers[0].Value != "application/json" {
		t.Errorf("string-block headers = %+v", sh.Headers)
	}
	if sh.Headers[1].Key != "Authorization" || sh.Headers[1].Value != `Hawk id="x", ts="123"` {
		t.Errorf("header with colon in value = %+v", sh.Headers[1])
	}
	if sh.Auth.Type != model.AuthBasic || sh.Auth.Username != "ada" || sh.Auth.Password != "hunter2" {
		t.Errorf("object-form basic auth = %+v", sh.Auth)
	}
	if sh.Body.Type != model.BodyRaw || sh.Body.Raw != "blahblah" {
		t.Errorf("body = %+v", sh.Body)
	}

	arr := byName["array url parts"]
	if arr.URL != "https://api.test/v1/users" {
		t.Errorf("array-parts url = %q", arr.URL)
	}
}

// The real example collection from postmanlabs/postman-collection (vendored
// via scripts/fetch-postman-example.mjs) — the file a user actually failed to
// import. Every request must come out with a usable method + URL.
func TestPostmanRealV2Example(t *testing.T) {
	data, err := os.ReadFile("testdata/collection-v2.json")
	if err != nil {
		t.Skipf("testdata missing: %v", err)
	}
	out, err := Postman(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("no requests imported")
	}
	for _, im := range out {
		if im.Request.Method == "" {
			t.Errorf("%s: empty method", im.Request.Name)
		}
		if im.Request.URL == "" {
			t.Errorf("%s: empty URL", im.Request.Name)
		}
	}
}

// Guard: the shipped example collection must always import cleanly.
func TestExampleCollectionImportable(t *testing.T) {
	data, err := os.ReadFile("../../example-collection/public-apis.postman_collection.json")
	if err != nil {
		t.Skipf("example collection missing: %v", err)
	}
	out, err := Postman(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 20 {
		t.Fatalf("example collection should have at least 20 requests, got %d", len(out))
	}
	byName := map[string]model.Request{}
	for _, im := range out {
		if im.Request.Method == "" || im.Request.URL == "" {
			t.Errorf("%s: missing method/url", im.Request.Name)
		}
		byName[im.Request.Name] = im.Request
	}
	if gql := byName["Country by code"]; gql.Body.Type != model.BodyGraphQL || gql.Body.Variables == "" {
		t.Errorf("graphql request mis-imported: %+v", gql.Body)
	}
	if ba := byName["Basic auth"]; ba.Auth.Type != model.AuthBasic || ba.Auth.Username != "postman" {
		t.Errorf("basic auth mis-imported: %+v", ba.Auth)
	}
	if uuid := byName["Random UUID"]; uuid.Method != "GET" || uuid.URL != "https://httpbin.org/uuid" {
		t.Errorf("string-shorthand request mis-imported: %+v", uuid)
	}
}

const openapiSample = `
openapi: 3.0.0
servers:
  - url: https://api.test/v1
paths:
  /users:
    get:
      operationId: listUsers
      summary: List all users
      description: Returns a paginated list of users.
      tags: [users]
      parameters:
        - name: page
          in: query
          description: Page number to fetch.
    post:
      operationId: createUser
      tags: [users]
      requestBody:
        content:
          application/json:
            example:
              name: bob
`

func TestOpenAPI(t *testing.T) {
	out, err := OpenAPI([]byte(openapiSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("imported %d, want 2", len(out))
	}
	byName := map[string]Imported{}
	for _, im := range out {
		byName[im.Request.Name] = im
	}
	lu, ok := byName["listUsers"]
	if !ok {
		t.Fatal("listUsers missing")
	}
	if lu.Request.URL != "{{baseUrl}}/users" {
		t.Errorf("url = %q", lu.Request.URL)
	}
	if len(lu.Dir) != 1 || lu.Dir[0] != "users" {
		t.Errorf("dir = %v", lu.Dir)
	}
	if !strings.Contains(lu.Request.Docs, "List all users") || !strings.Contains(lu.Request.Docs, "paginated list") {
		t.Errorf("docs = %q, want summary + description", lu.Request.Docs)
	}
	if len(lu.Request.Params) != 1 || lu.Request.Params[0].Desc != "Page number to fetch." {
		t.Errorf("param desc = %v, want page description", lu.Request.Params)
	}
	cu := byName["createUser"]
	if cu.Request.Body.Type != model.BodyJSON {
		t.Errorf("body type = %q, want json", cu.Request.Body.Type)
	}
	if cu.Request.Method != "POST" {
		t.Errorf("method = %q", cu.Request.Method)
	}
}

const openapiRichSample = `
openapi: 3.0.0
servers:
  - url: https://api.test/v1
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
paths:
  /widgets/{id}:
    get:
      operationId: getWidget
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
        - name: fields
          in: query
          required: true
          description: Comma-separated fields.
          example: name,color
      responses:
        '200':
          description: OK
        '404':
          description: Not found
    post:
      operationId: createWidget
      security: []
      requestBody:
        content:
          application/x-www-form-urlencoded:
            schema:
              type: object
              required: [name]
              properties:
                name: { type: string }
                color: { type: string }
      responses:
        '201':
          description: Created
`

func TestOpenAPIRichImport(t *testing.T) {
	out, err := OpenAPI([]byte(openapiRichSample))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Imported{}
	for _, im := range out {
		byName[im.Request.Name] = im
	}

	gw := byName["getWidget"].Request
	// Path param → {{id}} in URL; auth from document-level bearer scheme.
	if gw.URL != "{{baseUrl}}/widgets/{{id}}" {
		t.Errorf("url = %q, want {{baseUrl}} + {{id}} path var", gw.URL)
	}
	if gw.Auth.Type != model.AuthBearer {
		t.Errorf("auth = %q, want bearer (inherited document security)", gw.Auth.Type)
	}
	// Required query param: enabled + example prefilled + desc kept.
	if len(gw.Params) != 1 {
		t.Fatalf("params = %v, want 1 (fields)", gw.Params)
	}
	if p := gw.Params[0]; !p.Enabled || p.Value != "name,color" || p.Desc == "" {
		t.Errorf("fields param = %+v, want enabled + example value + desc", p)
	}
	// Lowest 2xx → status assert.
	if len(gw.Asserts) != 1 || gw.Asserts[0].Value != "200" {
		t.Errorf("asserts = %+v, want status==200", gw.Asserts)
	}

	cw := byName["createWidget"].Request
	// Operation-level empty security overrides document default → no auth.
	if cw.Auth.Type != model.AuthNone {
		t.Errorf("auth = %q, want none (security: [])", cw.Auth.Type)
	}
	// Form body with rows from schema; required field enabled.
	if cw.Body.Type != model.BodyForm {
		t.Fatalf("body type = %q, want form", cw.Body.Type)
	}
	form := map[string]bool{}
	for _, kv := range cw.Body.Form {
		form[kv.Key] = kv.Enabled
	}
	if !form["name"] || form["color"] {
		t.Errorf("form rows = %+v, want name enabled, color disabled", cw.Body.Form)
	}
	if len(cw.Asserts) != 1 || cw.Asserts[0].Value != "201" {
		t.Errorf("asserts = %+v, want status==201", cw.Asserts)
	}
}

func TestOpenAPIMeta(t *testing.T) {
	spec := `
openapi: 3.0.0
info:
  title: Train API
  description: Book train trips.
servers:
  - url: https://api.test/v1
    description: production
  - url: https://staging.api.test/v1
paths:
  /ping:
    get:
      operationId: ping
      responses:
        '200': { description: OK }
`
	desc, envs, err := OpenAPIMeta([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(desc, "Train API") || !strings.Contains(desc, "Book train trips") {
		t.Errorf("desc = %q", desc)
	}
	if len(envs) != 2 {
		t.Fatalf("envs = %v, want 2", envs)
	}
	// First server named from its description; baseUrl carries the URL.
	if envs[0].Name != "production" || envs[0].Vars[0].Value != "https://api.test/v1" {
		t.Errorf("env[0] = %+v", envs[0])
	}
	// Second server has no description → named from host.
	if envs[1].Name != "staging.api.test" {
		t.Errorf("env[1] name = %q, want host-derived", envs[1].Name)
	}
}

// TestOpenAPIRefsAndPathParams covers the constructs that broke the hand-rolled
// parser: OpenAPI 3.1, $ref-ed parameters, and path-level parameters shared
// across operations.
func TestOpenAPIRefsAndPathParams(t *testing.T) {
	out, err := OpenAPI([]byte(openapiRefSample))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Imported{}
	for _, im := range out {
		byName[im.Request.Name] = im
	}
	gs := byName["get-stations"]
	if gs.Request.Name == "" {
		t.Fatal("get-stations missing")
	}
	// page comes from a $ref parameter; both query params must resolve.
	var keys []string
	for _, p := range gs.Request.Params {
		keys = append(keys, p.Key)
	}
	if !contains(keys, "page") || !contains(keys, "search") {
		t.Errorf("query params = %v, want page + search", keys)
	}
	// bookingId is a path-level parameter (path) — not query/header, so it is
	// not added as a param, but the operation must still import without error.
	if _, ok := byName["get-booking"]; !ok {
		t.Error("get-booking missing (path-level params block)")
	}
}

// TestOpenAPITrainTravel imports the real Train Travel API spec (OpenAPI 3.1,
// heavy $ref use, path-level params, external file ref in an extension).
func TestOpenAPITrainTravel(t *testing.T) {
	data, err := os.ReadFile("testdata/train-travel-3.1.yaml")
	if err != nil {
		t.Skipf("testdata missing: %v", err)
	}
	out, err := OpenAPI(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	byName := map[string]Imported{}
	for _, im := range out {
		byName[im.Request.Name] = im
	}
	// Every operation across all paths must be present.
	want := []string{
		"get-stations", "get-trips", "get-bookings", "create-booking",
		"get-booking", "delete-booking", "create-booking-payment",
	}
	for _, n := range want {
		if _, ok := byName[n]; !ok {
			t.Errorf("operation %q missing", n)
		}
	}
	if len(byName) != len(want) {
		t.Errorf("imported %d ops, want %d: %v", len(byName), len(want), keysOf(byName))
	}
	// $ref-ed params (page, limit) plus inline params must all resolve.
	gs := byName["get-stations"]
	var q []string
	for _, p := range gs.Request.Params {
		q = append(q, p.Key)
	}
	for _, k := range []string{"page", "limit", "coordinates", "search", "country"} {
		if !contains(q, k) {
			t.Errorf("get-stations missing query param %q (got %v)", k, q)
		}
	}
	// Requests reference {{baseUrl}}; the server URL becomes an environment.
	if gs.Request.URL != "{{baseUrl}}/stations" {
		t.Errorf("get-stations URL = %q", gs.Request.URL)
	}
	// requestBody example pulled into JSON body.
	pay := byName["create-booking-payment"]
	if pay.Request.Body.Type != model.BodyJSON || pay.Request.Body.Raw == "" {
		t.Errorf("create-booking-payment body = %+v, want non-empty JSON", pay.Request.Body)
	}
	// Tag → folder.
	if len(pay.Request.Method) == 0 || pay.Request.Method != "POST" {
		t.Errorf("create-booking-payment method = %q", pay.Request.Method)
	}
}

func keysOf(m map[string]Imported) []string {
	var k []string
	for n := range m {
		k = append(k, n)
	}
	return k
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

const openapiRefSample = `
openapi: 3.1.0
servers:
  - url: https://api.example.com
x-topics:
  - title: Getting started
    content:
      $ref: ./docs/getting-started.md
paths:
  /stations:
    get:
      operationId: get-stations
      tags: [Stations]
      parameters:
        - $ref: '#/components/parameters/page'
        - name: search
          in: query
          schema:
            type: string
  /bookings/{bookingId}:
    parameters:
      - name: bookingId
        in: path
        required: true
        schema:
          type: string
    get:
      operationId: get-booking
      tags: [Bookings]
components:
  parameters:
    page:
      name: page
      in: query
      schema:
        type: integer
`
