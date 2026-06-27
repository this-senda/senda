package importer

import (
	"bytes"
	"fmt"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
)

// SpecField is one editable top-level property of a request body. Type uses the
// docs convention (e.g. "string", "integer", "string[]" for arrays of strings).
type SpecField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Desc     string `json:"desc"`
}

// SpecOpDetail is the form-editable view of one operation. Method and Path are
// read-only context (editing them restructures the document); the editor writes
// back Summary, Description and the request body's top-level fields.
type SpecOpDetail struct {
	OperationID string      `json:"operationId"`
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	HasBody     bool        `json:"hasBody"` // a JSON requestBody exists to edit
	BodyFields  []SpecField `json:"bodyFields"`
}

// SpecOperationDetail builds the form view of one operation from a parsed spec.
func SpecOperationDetail(data []byte, opID string) (SpecOpDetail, error) {
	m, err := buildV3(data)
	if err != nil {
		return SpecOpDetail{}, err
	}
	for p, item := range m.Model.Paths.PathItems.FromOldest() {
		if item == nil {
			continue
		}
		for method, op := range item.GetOperations().FromOldest() {
			if op == nil || opKey(method, p, op) != opID {
				continue
			}
			d := SpecOpDetail{
				OperationID: opID,
				Method:      strings.ToUpper(method),
				Path:        p,
				Summary:     op.Summary,
				Description: op.Description,
			}
			if op.RequestBody != nil && op.RequestBody.Content != nil {
				if mt := jsonMediaType(op.RequestBody.Content); mt != nil && mt.Schema != nil {
					if sc := mt.Schema.Schema(); sc != nil {
						d.HasBody = true
						req := map[string]bool{}
						for _, r := range sc.Required {
							req[r] = true
						}
						if sc.Properties != nil {
							for name, prop := range sc.Properties.FromOldest() {
								f := SpecField{Name: name, Required: req[name]}
								if prop != nil {
									if ps := prop.Schema(); ps != nil {
										f.Type = schemaTypeName(ps)
										f.Desc = ps.Description
									}
								}
								d.BodyFields = append(d.BodyFields, f)
							}
						}
					}
				}
			}
			return d, nil
		}
	}
	return SpecOpDetail{}, fmt.Errorf("openapi: operation %q not found", opID)
}

// UpdateSpecOperation applies a form edit to one operation by mutating the YAML
// node tree in place and re-encoding. Everything the form doesn't touch — other
// operations, components, comments, key order, extra schema keywords on retained
// fields — is preserved. Only Summary, Description and the request body's
// top-level fields are written.
func UpdateSpecOperation(data []byte, opID string, d SpecOpDetail) ([]byte, error) {
	var root yamlv3.Node
	if err := yamlv3.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("openapi: parse: %w", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yamlv3.MappingNode {
		return nil, fmt.Errorf("openapi: not a mapping document")
	}
	doc := root.Content[0]

	paths := mapVal(doc, "paths")
	if paths == nil {
		return nil, fmt.Errorf("openapi: no paths")
	}
	opNode := findOpNode(paths, opID)
	if opNode == nil {
		return nil, fmt.Errorf("openapi: operation %q not found", opID)
	}

	setOrDeleteScalar(opNode, "summary", d.Summary)
	setOrDeleteScalar(opNode, "description", d.Description)
	if d.HasBody {
		if err := applyBodyFields(opNode, d.BodyFields); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	enc := yamlv3.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return nil, fmt.Errorf("openapi: encode: %w", err)
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

// --- yaml.Node helpers (mapping = alternating key/value Content) ---

// mapVal returns the value node for key in a mapping node, or nil.
func mapVal(m *yamlv3.Node, key string) *yamlv3.Node {
	if m == nil || m.Kind != yamlv3.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalar(value string) *yamlv3.Node {
	return &yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: value}
}

// setScalar sets (or inserts) a string-valued key in a mapping.
func setScalar(m *yamlv3.Node, key, value string) {
	if v := mapVal(m, key); v != nil {
		v.Kind, v.Tag, v.Value, v.Content = yamlv3.ScalarNode, "!!str", value, nil
		return
	}
	m.Content = append(m.Content, scalar(key), scalar(value))
}

// delKey removes a key (and its value) from a mapping if present.
func delKey(m *yamlv3.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

// setOrDeleteScalar writes value, or removes the key entirely when value is empty
// (so clearing a summary doesn't leave `summary: ""` behind).
func setOrDeleteScalar(m *yamlv3.Node, key, value string) {
	if strings.TrimSpace(value) == "" {
		delKey(m, key)
		return
	}
	setScalar(m, key, value)
}

// findOpNode locates an operation's value node under the paths mapping, matching
// by the same opKey used everywhere else (operationId, else "METHOD /path").
func findOpNode(paths *yamlv3.Node, opID string) *yamlv3.Node {
	methods := map[string]bool{"get": true, "post": true, "put": true, "delete": true, "patch": true, "head": true, "options": true, "trace": true}
	for i := 0; i+1 < len(paths.Content); i += 2 {
		path := paths.Content[i].Value
		item := paths.Content[i+1]
		if item.Kind != yamlv3.MappingNode {
			continue
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			method := strings.ToLower(item.Content[j].Value)
			if !methods[method] {
				continue
			}
			op := item.Content[j+1]
			key := strings.ToUpper(method) + " " + path
			if oid := mapVal(op, "operationId"); oid != nil && oid.Value != "" {
				key = oid.Value
			}
			if key == opID {
				return op
			}
		}
	}
	return nil
}

// applyBodyFields rewrites the request body's top-level properties + required
// list to match fields, editing existing property nodes in place (so unmodelled
// keywords like format/enum survive) and adding/removing as needed.
func applyBodyFields(op *yamlv3.Node, fields []SpecField) error {
	schema := schemaNode(op)
	if schema == nil {
		return fmt.Errorf("openapi: operation has no application/json request body schema")
	}
	// A $ref body points at components/schemas; writing properties alongside the
	// ref would corrupt it. Refuse — the user edits the referenced schema directly.
	if mapVal(schema, "$ref") != nil {
		return fmt.Errorf("openapi: request body uses a $ref schema — edit the referenced component directly")
	}
	setScalar(schema, "type", "object")

	props := mapVal(schema, "properties")
	if props == nil || props.Kind != yamlv3.MappingNode {
		props = &yamlv3.Node{Kind: yamlv3.MappingNode}
		schema.Content = append(schema.Content, scalar("properties"), props)
	}

	want := map[string]bool{}
	var required []string
	for _, f := range fields {
		if f.Name == "" {
			continue
		}
		want[f.Name] = true
		if f.Required {
			required = append(required, f.Name)
		}
		node := mapVal(props, f.Name)
		if node == nil || node.Kind != yamlv3.MappingNode {
			node = &yamlv3.Node{Kind: yamlv3.MappingNode}
			props.Content = append(props.Content, scalar(f.Name), node)
		}
		setFieldType(node, f.Type)
		setOrDeleteScalar(node, "description", f.Desc)
	}

	// Drop properties the form removed.
	kept := props.Content[:0]
	for i := 0; i+1 < len(props.Content); i += 2 {
		if want[props.Content[i].Value] {
			kept = append(kept, props.Content[i], props.Content[i+1])
		}
	}
	props.Content = kept

	setRequired(schema, required)
	return nil
}

// schemaNode walks op → requestBody → content → application/json → schema.
func schemaNode(op *yamlv3.Node) *yamlv3.Node {
	rb := mapVal(op, "requestBody")
	if rb == nil {
		return nil
	}
	content := mapVal(rb, "content")
	if content == nil {
		return nil
	}
	mt := mapVal(content, "application/json")
	if mt == nil {
		return nil
	}
	return mapVal(mt, "schema")
}

// setFieldType writes a field's type, expanding "x[]" to an array of x and
// otherwise clearing any leftover array `items`.
func setFieldType(node *yamlv3.Node, typ string) {
	if typ == "" {
		return
	}
	if base, ok := strings.CutSuffix(typ, "[]"); ok {
		setScalar(node, "type", "array")
		items := mapVal(node, "items")
		if items == nil || items.Kind != yamlv3.MappingNode {
			items = &yamlv3.Node{Kind: yamlv3.MappingNode}
			node.Content = append(node.Content, scalar("items"), items)
		}
		setScalar(items, "type", base)
		return
	}
	setScalar(node, "type", typ)
	delKey(node, "items")
}

// setRequired replaces (or removes) the schema's required sequence.
func setRequired(schema *yamlv3.Node, required []string) {
	if len(required) == 0 {
		delKey(schema, "required")
		return
	}
	seq := &yamlv3.Node{Kind: yamlv3.SequenceNode}
	for _, r := range required {
		seq.Content = append(seq.Content, scalar(r))
	}
	if v := mapVal(schema, "required"); v != nil {
		*v = *seq
		return
	}
	schema.Content = append(schema.Content, scalar("required"), seq)
}
