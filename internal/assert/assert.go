// Package assert evaluates a request's declarative post-response checks.
// Each Assert names a target (status, duration, size, body, header.<Name>,
// json.<path>), an operator and an expected value; Eval returns one result
// per enabled assert. Pure — no network, no disk.
package assert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"senda/internal/model"
)

// Ops lists the supported comparison operators, in UI display order.
var Ops = []string{
	"eq", "neq", "contains", "notcontains",
	"gt", "gte", "lt", "lte",
	"exists", "notexists", "matches",
}

// Eval runs every enabled assert against the response. Disabled rows are
// skipped entirely (no result emitted).
func Eval(asserts []model.Assert, resp model.Response) []model.AssertResult {
	var out []model.AssertResult
	for _, a := range asserts {
		if !a.Enabled {
			continue
		}
		out = append(out, evalOne(a, resp))
	}
	return out
}

func evalOne(a model.Assert, resp model.Response) model.AssertResult {
	res := model.AssertResult{Target: a.Target, Op: a.Op, Value: a.Value}
	actual, found, err := Extract(strings.TrimSpace(a.Target), resp)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if found {
		res.Actual = actual
	}
	switch a.Op {
	case "exists":
		res.Pass = found
	case "notexists":
		res.Pass = !found
	default:
		if !found {
			res.Error = "target not found"
			return res
		}
		pass, err := Compare(a.Op, actual, a.Value)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.Pass = pass
	}
	return res
}

// Extract resolves a target expression (status, duration, size, body,
// header.<Name>, json.<path>) to its string form. found=false means the target
// legitimately does not exist (missing header / JSON path). Shared with vars so
// {{res.<slug>.<target>}} references reuse the same grammar as assertions.
func Extract(target string, resp model.Response) (actual string, found bool, err error) {
	switch {
	case target == "status":
		return strconv.Itoa(resp.Status), true, nil
	case target == "duration":
		return strconv.FormatInt(resp.DurationMs, 10), true, nil
	case target == "size":
		return strconv.FormatInt(resp.SizeBytes, 10), true, nil
	case target == "body":
		return resp.Body, true, nil
	case strings.HasPrefix(target, "header."):
		name := http.CanonicalHeaderKey(strings.TrimPrefix(target, "header."))
		vals, ok := resp.Headers[name]
		if !ok || len(vals) == 0 {
			return "", false, nil
		}
		return strings.Join(vals, ", "), true, nil
	case target == "json" || strings.HasPrefix(target, "json.") || strings.HasPrefix(target, "json["):
		return extractJSON(strings.TrimPrefix(target, "json"), resp.Body)
	case target == "":
		return "", false, fmt.Errorf("empty assert target")
	default:
		return "", false, fmt.Errorf("unknown assert target %q", target)
	}
}

// extractJSON decodes the body and walks path (e.g. ".users[0].name").
func extractJSON(path, body string) (string, bool, error) {
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false, fmt.Errorf("response body is not JSON: %v", err)
	}
	v, found, err := walk(v, path)
	if err != nil || !found {
		return "", found, err
	}
	return stringify(v), true, nil
}

// walk follows a dot/index path ("a.b[2].c") into decoded JSON. A missing key,
// out-of-range index or type mismatch is found=false, not an error — only a
// malformed path errors.
func walk(v any, path string) (any, bool, error) {
	rest := path
	for rest != "" {
		if rest[0] == '.' {
			rest = rest[1:]
			continue
		}
		if rest[0] == '[' {
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				return nil, false, fmt.Errorf("unclosed [ in json path")
			}
			idx, err := strconv.Atoi(rest[1:end])
			if err != nil {
				return nil, false, fmt.Errorf("bad array index %q in json path", rest[1:end])
			}
			arr, ok := v.([]any)
			if !ok || idx < 0 || idx >= len(arr) {
				return nil, false, nil
			}
			v = arr[idx]
			rest = rest[end+1:]
			continue
		}
		end := strings.IndexAny(rest, ".[")
		key := rest
		if end >= 0 {
			key, rest = rest[:end], rest[end:]
		} else {
			rest = ""
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		v, ok = m[key]
		if !ok {
			return nil, false, nil
		}
	}
	return v, true, nil
}

// stringify renders a decoded JSON value for comparison: scalars bare,
// objects/arrays as compact JSON.
func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case json.Number:
		return t.String()
	default:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return fmt.Sprintf("%v", v)
		}
		return strings.TrimSuffix(buf.String(), "\n")
	}
}

// Compare evaluates one operator (eq, neq, contains, notcontains, gt, gte, lt,
// lte, matches) against two string operands. exists/notexists are handled by
// the caller (they depend on presence, not value). Shared with the flow engine's
// branch nodes.
func Compare(op, actual, expected string) (bool, error) {
	switch op {
	case "eq":
		return actual == expected || numericEqual(actual, expected), nil
	case "neq":
		return actual != expected && !numericEqual(actual, expected), nil
	case "contains":
		return strings.Contains(actual, expected), nil
	case "notcontains":
		return !strings.Contains(actual, expected), nil
	case "gt", "gte", "lt", "lte":
		a, err1 := strconv.ParseFloat(actual, 64)
		e, err2 := strconv.ParseFloat(expected, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("%s needs numeric operands (actual %q, expected %q)", op, actual, expected)
		}
		switch op {
		case "gt":
			return a > e, nil
		case "gte":
			return a >= e, nil
		case "lt":
			return a < e, nil
		default:
			return a <= e, nil
		}
	case "matches":
		re, err := regexp.Compile(expected)
		if err != nil {
			return false, fmt.Errorf("bad regex %q: %v", expected, err)
		}
		return re.MatchString(actual), nil
	case "":
		return false, fmt.Errorf("empty assert operator")
	default:
		return false, fmt.Errorf("unknown assert operator %q", op)
	}
}

// numericEqual treats "1.0" and "1" as equal when both sides parse as numbers.
func numericEqual(a, b string) bool {
	fa, err1 := strconv.ParseFloat(a, 64)
	fb, err2 := strconv.ParseFloat(b, 64)
	return err1 == nil && err2 == nil && fa == fb
}
