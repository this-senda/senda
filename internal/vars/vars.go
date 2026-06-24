// Package vars resolves {{variable}} placeholders against a layered scope.
// Precedence (highest first): request-local -> environment -> collection.
package vars

import (
	"regexp"
	"strings"

	"senda/internal/fake"
	"senda/internal/model"
)

// placeholder matches {{name}} and dynamic faker tokens. A leading "$" marks a
// faker token (e.g. {{$email}}, {{$person.firstname}}) resolved at apply time
// rather than from scope; faker tokens may carry params: {{$number.int(min=1,
// max=9)}}. Plain vars never have parens.
var placeholder = regexp.MustCompile(`\{\{\s*(\$?[\w.\-]+(?:\([^)]*\))?)\s*\}\}`)

// Scope is a flattened, precedence-resolved variable map plus a record of
// which placeholders could not be resolved during the last Apply call.
type Scope struct {
	values     map[string]string
	Unresolved []string
}

// Build flattens the layers into a single lookup map. Later arguments win,
// so pass them lowest-precedence first: collection, environment, request.
func Build(layers ...[]model.KV) *Scope {
	values := map[string]string{}
	for _, layer := range layers {
		for _, kv := range layer {
			if kv.Enabled && kv.Key != "" {
				values[kv.Key] = kv.Value
			}
		}
	}
	return &Scope{values: values}
}

// Get looks up one variable in the flattened scope.
func (sc *Scope) Get(name string) (string, bool) {
	v, ok := sc.values[name]
	return v, ok
}

// Apply substitutes every {{name}} in s. Unknown names are left verbatim and
// recorded in Unresolved (de-duplicated) for surfacing as warnings.
func (sc *Scope) Apply(s string) string {
	seen := map[string]bool{}
	return placeholder.ReplaceAllStringFunc(s, func(match string) string {
		name := strings.TrimSpace(placeholder.FindStringSubmatch(match)[1])
		if strings.HasPrefix(name, "$") {
			if v, ok := fake.Func(name[1:]); ok {
				return v
			}
			// Unknown faker token: leave verbatim, record once.
		} else if v, ok := sc.values[name]; ok {
			return v
		}
		if !seen[name] {
			seen[name] = true
			sc.Unresolved = append(sc.Unresolved, name)
		}
		return match
	})
}

// ApplyKVs returns a copy of kvs with keys and values interpolated, dropping
// disabled rows.
func (sc *Scope) ApplyKVs(kvs []model.KV) []model.KV {
	out := make([]model.KV, 0, len(kvs))
	for _, kv := range kvs {
		if !kv.Enabled {
			continue
		}
		out = append(out, model.KV{
			Key:     sc.Apply(kv.Key),
			Value:   sc.Apply(kv.Value),
			Enabled: true,
			File:    kv.File,
		})
	}
	return out
}
