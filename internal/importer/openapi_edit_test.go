package importer

import (
	"strings"
	"testing"
)

const openapiEditSample = `openapi: 3.0.0
info:
  title: Edit Test
paths:
  /a:
    post:
      operationId: opA
      summary: old summary
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
                  format: email
                age:
                  type: integer
  /b:
    get:
      operationId: opB
      summary: keep me
components:
  schemas:
    Foo:
      type: object
`

func TestUpdateSpecOperation(t *testing.T) {
	// Edit opA: new summary; keep name (toggle required off-then-on), drop age,
	// add city. name's extra `format` keyword must survive.
	detail := SpecOpDetail{
		OperationID: "opA",
		HasBody:     true,
		Summary:     "new summary",
		BodyFields: []SpecField{
			{Name: "name", Type: "string", Required: true},
			{Name: "city", Type: "string"},
		},
	}
	out, err := UpdateSpecOperation([]byte(openapiEditSample), "opA", detail)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(out)

	// opA changes applied.
	da, err := SpecOperationDetail(out, "opA")
	if err != nil {
		t.Fatal(err)
	}
	if da.Summary != "new summary" {
		t.Errorf("summary = %q", da.Summary)
	}
	names := map[string]SpecField{}
	for _, f := range da.BodyFields {
		names[f.Name] = f
	}
	if _, ok := names["age"]; ok {
		t.Error("age should have been removed")
	}
	if _, ok := names["city"]; !ok {
		t.Error("city should have been added")
	}
	if !names["name"].Required {
		t.Error("name should be required")
	}

	// Unmodelled keyword on a retained field survives (in-place edit, not rebuild).
	if !strings.Contains(raw, "format: email") {
		t.Errorf("format: email dropped:\n%s", raw)
	}

	// Other operation and components untouched.
	db, err := SpecOperationDetail(out, "opB")
	if err != nil {
		t.Fatal(err)
	}
	if db.Summary != "keep me" {
		t.Errorf("opB summary changed to %q", db.Summary)
	}
	if !strings.Contains(raw, "Foo") {
		t.Error("components/schemas/Foo dropped")
	}
}

const openapiRefBodySample = `openapi: 3.0.0
info:
  title: Ref Body
paths:
  /a:
    post:
      operationId: opR
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/User'
components:
  schemas:
    User:
      type: object
      properties:
        name: { type: string }
`

// Editing a $ref-bodied operation is refused rather than corrupting the ref.
func TestUpdateSpecOperationRefBodyRefused(t *testing.T) {
	detail := SpecOpDetail{
		OperationID: "opR",
		HasBody:     true,
		BodyFields:  []SpecField{{Name: "name", Type: "string", Required: true}},
	}
	_, err := UpdateSpecOperation([]byte(openapiRefBodySample), "opR", detail)
	if err == nil || !strings.Contains(err.Error(), "$ref") {
		t.Fatalf("err = %v, want a $ref refusal", err)
	}
}

func TestUpdateSpecOperationArrayField(t *testing.T) {
	detail := SpecOpDetail{
		OperationID: "opA",
		HasBody:     true,
		BodyFields:  []SpecField{{Name: "tags", Type: "string[]", Required: false}},
	}
	out, err := UpdateSpecOperation([]byte(openapiEditSample), "opA", detail)
	if err != nil {
		t.Fatal(err)
	}
	d, err := SpecOperationDetail(out, "opA")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.BodyFields) != 1 || d.BodyFields[0].Type != "string[]" {
		t.Errorf("array field round-trip = %+v, want one string[] field", d.BodyFields)
	}
}
