package importer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// SpecOp is one operation in an OpenAPI document, for the relink dropdown.
type SpecOp struct {
	OperationID string `json:"operationId"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Summary     string `json:"summary"`
}

// SpecError is a single spec parse/validation problem for the editor.
type SpecError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// opKey is the stable identifier linking a request to its source operation: the
// spec's operationId when present, else a synthesized "METHOD /path". The same
// key is stamped on import (convertOpenAPI) and matched here, so a relinked or
// imported request always resolves to the right operation.
func opKey(method, path string, op *v3.Operation) string {
	if op != nil && op.OperationId != "" {
		return op.OperationId
	}
	return strings.ToUpper(method) + " " + path
}

// buildV3 parses an OpenAPI 3 document the same way OpenAPI() does (internal
// refs resolved, external refs skipped). Shared by the spec-editor helpers.
// libopenapi panics on some malformed input (e.g. valid YAML that isn't an
// OpenAPI doc), so recover and surface it as an error — ValidateSpec is fed
// whatever the user types.
func buildV3(data []byte) (m *libopenapi.DocumentModel[v3.Document], err error) {
	defer func() {
		if r := recover(); r != nil {
			m, err = nil, fmt.Errorf("openapi: %v", r)
		}
	}()
	doc, err := libopenapi.NewDocumentWithConfiguration(data, &datamodel.DocumentConfiguration{
		SkipExternalRefResolution: true,
	})
	if err != nil {
		return nil, err
	}
	m, err = doc.BuildV3Model()
	if err != nil {
		return nil, err
	}
	if m == nil || m.Model.Paths == nil || m.Model.Paths.PathItems == nil {
		return nil, fmt.Errorf("openapi: no paths found")
	}
	return m, nil
}

// ValidateSpec parses a spec and returns any parse/build problems. An empty
// slice means the document is structurally valid OpenAPI 3. Line stays 0 —
// libopenapi's build errors don't carry source positions.
func ValidateSpec(data []byte) []SpecError {
	if _, err := buildV3(data); err != nil {
		var out []SpecError
		for _, line := range strings.Split(err.Error(), "\n") {
			if s := strings.TrimSpace(line); s != "" {
				out = append(out, SpecError{Message: s})
			}
		}
		return out
	}
	return nil
}

// SpecOperations lists every operation in a spec (sorted by path then method)
// for the request's relink dropdown.
func SpecOperations(data []byte) ([]SpecOp, error) {
	m, err := buildV3(data)
	if err != nil {
		return nil, err
	}
	items := m.Model.Paths.PathItems
	paths := make([]string, 0, items.Len())
	for p := range items.KeysFromOldest() {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var out []SpecOp
	for _, p := range paths {
		item := items.GetOrZero(p)
		if item == nil {
			continue
		}
		for method, op := range item.GetOperations().FromOldest() {
			if op == nil {
				continue
			}
			out = append(out, SpecOp{
				OperationID: opKey(method, p, op),
				Method:      strings.ToUpper(method),
				Path:        p,
				Summary:     op.Summary,
			})
		}
	}
	return out, nil
}

// RequestBodySchema returns the JSON Schema (refs inlined) of an operation's
// application/json requestBody, for body validation + autocomplete. Returns an
// empty string when the operation has no JSON request body.
func RequestBodySchema(data []byte, operationID string) (string, error) {
	m, err := buildV3(data)
	if err != nil {
		return "", err
	}
	for p, item := range m.Model.Paths.PathItems.FromOldest() {
		if item == nil {
			continue
		}
		for method, op := range item.GetOperations().FromOldest() {
			if op == nil || opKey(method, p, op) != operationID {
				continue
			}
			if op.RequestBody == nil || op.RequestBody.Content == nil {
				return "", nil
			}
			mt := jsonMediaType(op.RequestBody.Content)
			if mt == nil || mt.Schema == nil {
				return "", nil
			}
			sc := mt.Schema.Schema()
			if sc == nil {
				return "", nil
			}
			return renderSchemaJSON(sc)
		}
	}
	return "", fmt.Errorf("openapi: operation %q not found", operationID)
}

// renderSchemaJSON inlines a schema's refs to a JSON Schema string. Guarded by
// recover because MarshalJSONInline can panic/overflow on recursive ($ref to
// self) schemas, which real-world specs do use.
func renderSchemaJSON(sc *base.Schema) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = "", fmt.Errorf("openapi: render schema: %v", r)
		}
	}()
	raw, err := sc.MarshalJSONInline()
	if err != nil {
		return "", fmt.Errorf("openapi: render schema: %w", err)
	}
	return string(raw), nil
}

// SpecTitle returns a filesystem-friendly slug from the document's info.title
// (e.g. "Petstore API" -> "petstore-api"), or "" when there's no usable title.
// Used to name the spec file saved on import.
func SpecTitle(data []byte) string {
	m, err := buildV3(data)
	if err != nil || m.Model.Info == nil {
		return ""
	}
	return slugify(m.Model.Info.Title)
}

// slugify lowercases and hyphenates a title into a safe file basename.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
