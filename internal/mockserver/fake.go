package mockserver

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"senda/internal/fake"
)

// Schema-driven fake bodies. The faker generators themselves live in package
// fake (shared with request-time {{$token}} resolution).

// fakeFromSchema generates a minimal fake JSON object from a JSON Schema.
// Supports type:object with string/integer/number/boolean/array properties.
func fakeFromSchema(schema string) string {
	var s map[string]any
	if err := json.Unmarshal([]byte(schema), &s); err != nil {
		return "{}"
	}
	out, err := json.MarshalIndent(buildFake(s), "", "  ")
	if err != nil {
		return "{}"
	}
	return string(out)
}

func buildFake(schema map[string]any) any {
	// Honour a concrete value the schema carries before synthesising one.
	if ex, ok := schema["example"]; ok {
		return ex
	}
	if ex, ok := schema["examples"].([]any); ok && len(ex) > 0 {
		return ex[0]
	}
	if d, ok := schema["default"]; ok {
		return d
	}
	switch t := schemaType(schema); t {
	case "object":
		props, _ := schema["properties"].(map[string]any)
		obj := map[string]any{}
		for k, v := range props {
			if vs, ok := v.(map[string]any); ok {
				obj[k] = buildFakeProp(k, vs)
			}
		}
		return obj
	case "array":
		if items, ok := schema["items"].(map[string]any); ok {
			return []any{buildFake(items)}
		}
		return []any{}
	case "integer", "number":
		if ex, ok := schema["examples"].([]any); ok && len(ex) > 0 {
			return ex[0]
		}
		return rand.Intn(1000)
	case "boolean":
		return rand.Intn(2) == 1
	default: // string
		return fakeStringFor("", schema)
	}
}

// schemaType returns the JSON Schema type, tolerating OpenAPI 3.1's union form
// where "type" is a list (e.g. ["string","null"]) — the first non-null entry
// wins. An absent type yields "" (treated as string downstream).
func schemaType(schema map[string]any) string {
	switch t := schema["type"].(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if s, ok := v.(string); ok && s != "null" {
				return s
			}
		}
	}
	return ""
}

// buildFakeProp generates a value for a named property, using the property name
// as a hint (e.g. "email", "name") when the schema gives no format/examples.
func buildFakeProp(name string, schema map[string]any) any {
	if _, ok := schema["example"]; ok {
		return buildFake(schema)
	}
	switch schemaType(schema) {
	case "string", "":
		return fakeStringFor(name, schema)
	default:
		return buildFake(schema)
	}
}

func fakeStringFor(name string, schema map[string]any) string {
	if ex, ok := schema["examples"].([]any); ok && len(ex) > 0 {
		return fmt.Sprintf("%v", ex[0])
	}
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		return fmt.Sprintf("%v", enum[0])
	}
	if f, _ := schema["format"].(string); f != "" {
		switch f {
		case "email":
			return fake.Email()
		case "uuid":
			return fake.UUID()
		case "date":
			return time.Now().UTC().Format("2006-01-02")
		case "date-time":
			return time.Now().UTC().Format(time.RFC3339)
		case "uri", "url":
			return "https://example.com"
		}
	}
	// Property-name heuristics.
	switch n := strings.ToLower(name); {
	case strings.Contains(n, "email"):
		return fake.Email()
	case strings.Contains(n, "name"):
		return fake.Name()
	case strings.Contains(n, "phone"):
		return fake.Phone()
	case strings.Contains(n, "city"):
		return fake.City()
	case strings.Contains(n, "country"):
		return fake.Country()
	case strings.Contains(n, "id"):
		return fake.UUID()
	case n != "":
		return fake.Word()
	}
	return "example"
}
