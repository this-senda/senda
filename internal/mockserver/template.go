package mockserver

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"senda/internal/fake"
)

// tmplCtx is the data available to {{...}} expressions in a response.
type tmplCtx struct {
	params  map[string]string
	query   url.Values
	headers http.Header
	body    any // decoded JSON request body (map/slice) or nil
}

var tmplRe = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// renderString replaces every {{expr}} token in s with its evaluated value.
func renderString(s string, ctx tmplCtx) string {
	return tmplRe.ReplaceAllStringFunc(s, func(tok string) string {
		expr := strings.TrimSpace(tmplRe.FindStringSubmatch(tok)[1])
		if v, ok := evalExpr(expr, ctx); ok {
			return v
		}
		return tok // leave unknown tokens untouched
	})
}

// renderAny walks v (decoded YAML/JSON) and renders every string leaf, so
// native-object bodies get the same templating as string bodies.
func renderAny(v any, ctx tmplCtx) any {
	switch t := v.(type) {
	case string:
		return renderString(t, ctx)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = renderAny(val, ctx)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = renderAny(val, ctx)
		}
		return out
	default:
		return v
	}
}

// evalExpr evaluates a single template expression. Two forms:
//   - dotted lookup: params.id, query.x, headers.x-api-key, body.user.name
//   - function call: faker.name, uuid, randomInt 1 100, now "2006-01-02"
func evalExpr(expr string, ctx tmplCtx) (string, bool) {
	fields := splitArgs(expr)
	if len(fields) == 0 {
		return "", false
	}
	head := fields[0]
	args := fields[1:]

	// Namespaced lookups.
	if dot := strings.IndexByte(head, '.'); dot > 0 {
		ns, rest := head[:dot], head[dot+1:]
		switch ns {
		case "params":
			v, ok := ctx.params[rest]
			return v, ok
		case "query":
			if ctx.query == nil {
				return "", false
			}
			return ctx.query.Get(rest), ctx.query.Has(rest)
		case "headers", "header":
			if ctx.headers == nil {
				return "", false
			}
			v := ctx.headers.Get(rest)
			return v, v != ""
		case "body":
			return lookupBody(ctx.body, rest)
		case "faker":
			return fake.Func(rest)
		}
	}

	// Bare functions.
	switch strings.ToLower(head) {
	case "uuid":
		return fake.UUID(), true
	case "now":
		layout := time.RFC3339
		if len(args) > 0 {
			layout = args[0]
		}
		return time.Now().UTC().Format(layout), true
	case "randomint":
		lo, hi := 0, 100
		if len(args) >= 2 {
			if n, err := strconv.Atoi(args[0]); err == nil {
				lo = n
			}
			if n, err := strconv.Atoi(args[1]); err == nil {
				hi = n
			}
		}
		if hi <= lo {
			return strconv.Itoa(lo), true
		}
		return strconv.Itoa(lo + rand.Intn(hi-lo+1)), true
	}
	return "", false
}

// lookupBody walks a dotted path through a decoded JSON body.
func lookupBody(body any, path string) (string, bool) {
	cur := body
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = m[seg]
		if !ok {
			return "", false
		}
	}
	if cur == nil {
		return "", false
	}
	return fmt.Sprintf("%v", cur), true
}

// splitArgs splits an expression into space-separated fields, honouring
// double-quoted segments (so now "2006-01-02" keeps its layout intact).
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}
