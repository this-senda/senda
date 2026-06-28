package importer

import (
	"strings"
	"testing"

	"senda/internal/schemaval"
)

const openapiSpecSample = `
openapi: 3.0.0
info:
  title: Widget API
servers:
  - url: https://api.test/v1
paths:
  /widgets:
    post:
      operationId: createWidget
      summary: Create a widget
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name: { type: string }
                size: { type: integer }
      responses:
        '201': { description: Created }
  /ping:
    get:
      responses:
        '200': { description: ok }
`

// Import stamps each request with its source operation, and an operation with
// no operationId falls back to the synthesized "METHOD /path" key.
func TestOpenAPIStampsSpecLink(t *testing.T) {
	out, err := OpenAPI([]byte(openapiSpecSample))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*string{}
	for _, im := range out {
		if im.Request.Spec == nil {
			t.Fatalf("%s: Spec not stamped", im.Request.Name)
		}
		op := im.Request.Spec.OperationID
		byName[im.Request.Method] = &op
	}
	if got := byName["POST"]; got == nil || *got != "createWidget" {
		t.Errorf("POST opid = %v, want createWidget", got)
	}
	if got := byName["GET"]; got == nil || *got != "GET /ping" {
		t.Errorf("GET opid = %v, want synthesized 'GET /ping'", got)
	}
}

func TestSpecOperations(t *testing.T) {
	ops, err := SpecOperations([]byte(openapiSpecSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("ops = %d, want 2", len(ops))
	}
	var found bool
	for _, o := range ops {
		if o.OperationID == "createWidget" && o.Method == "POST" && o.Path == "/widgets" {
			found = true
		}
	}
	if !found {
		t.Errorf("createWidget POST /widgets not in %+v", ops)
	}
}

// The extracted body schema validates conforming bodies and flags ones missing
// a required property — the full hint path (extract → validate).
func TestRequestBodySchemaValidates(t *testing.T) {
	raw, err := RequestBodySchema([]byte(openapiSpecSample), "createWidget")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "properties") || !strings.Contains(raw, "name") {
		t.Fatalf("schema = %q, want object schema with properties", raw)
	}

	good := schemaval.Validate(raw, `{"name":"gear","size":3}`)
	if len(good) != 1 || !good[0].Pass {
		t.Errorf("valid body got %+v, want one pass", good)
	}
	bad := schemaval.Validate(raw, `{"size":3}`)
	if len(bad) == 0 || bad[0].Pass {
		t.Errorf("body missing required name should fail, got %+v", bad)
	}
}

func TestRequestBodySchemaNoBody(t *testing.T) {
	raw, err := RequestBodySchema([]byte(openapiSpecSample), "GET /ping")
	if err != nil {
		t.Fatal(err)
	}
	if raw != "" {
		t.Errorf("no-body op schema = %q, want empty", raw)
	}
}

func TestValidateSpec(t *testing.T) {
	if errs := ValidateSpec([]byte(openapiSpecSample)); len(errs) != 0 {
		t.Errorf("valid spec reported %+v", errs)
	}
	if errs := ValidateSpec([]byte("not: openapi\n")); len(errs) == 0 {
		t.Error("garbage spec reported no errors")
	}
}
