// Package fake wraps gofakeit's data catalog. It backs the mock server's
// schema-driven bodies and request-time {{$token}} resolution in package vars.
// Tokens are namespaced by gofakeit category (e.g. {{$person.firstname}}); the
// resolver looks a token up by its last dotted segment, so the namespace is for
// grouping/display only.
package fake

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
)

// faker is a single crypto-seeded generator shared across the process. gofakeit
// generators are independent draws, so one instance is fine.
// ponytail: package-global, no per-request seeding; add a seeded Faker param if
// reproducible fake bodies are ever needed.
var faker = gofakeit.New(cryptoSeed())

func cryptoSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 1
	}
	return binary.LittleEndian.Uint64(b[:])
}

// Func resolves a faker token to a freshly generated value. The token may be
// namespaced ("person.firstname") or bare ("firstname"), and may carry params
// ("number.number(min=1,max=9)"); only the last dotted segment is the gofakeit
// lookup key. Returns false for unknown tokens or generators still missing a
// required param, so callers leave them verbatim.
func Func(token string) (string, bool) {
	name, params := parseToken(token)
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	info, ok := gofakeit.FuncLookups[strings.ToLower(name)]
	if !ok {
		return "", false
	}
	v, err := info.Generate(faker, buildParams(&info, params), &info)
	if err != nil {
		return "", false
	}
	return toString(v), true
}

// parseToken splits "name(field=value, field2='v2')" into the name and a param
// map. No parens → nil map. ponytail: scalar params only; gofakeit's few
// array-typed params (e.g. comma-joined lists) aren't addressable here yet.
func parseToken(token string) (string, map[string]string) {
	open := strings.IndexByte(token, '(')
	if open < 0 || !strings.HasSuffix(token, ")") {
		return token, nil
	}
	name := token[:open]
	params := map[string]string{}
	for _, pair := range strings.Split(token[open+1:len(token)-1], ",") {
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(pair[:eq])
		v := strings.Trim(strings.TrimSpace(pair[eq+1:]), `"'`)
		if k != "" {
			params[k] = v
		}
	}
	return name, params
}

// buildParams maps a generator's declared params to values: a caller-supplied
// value wins, else the documented default. Unknown caller keys are ignored.
// Returns nil when the generator takes none, satisfying the no-param path.
func buildParams(info *gofakeit.Info, user map[string]string) *gofakeit.MapParams {
	if len(info.Params) == 0 {
		return nil
	}
	m := gofakeit.NewMapParams()
	for _, p := range info.Params {
		if v, ok := user[p.Field]; ok {
			m.Add(p.Field, v)
		} else if p.Default != "" {
			m.Add(p.Field, p.Default)
		}
	}
	if m.Size() == 0 {
		return nil
	}
	return m
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// Token is one autocomplete entry: a namespaced faker category surfaced to the
// editor. Example is gofakeit's own sample value, shown as a live-ish preview.
type Token struct {
	Category string `json:"category"`
	Name     string `json:"name"`    // bare gofakeit key, e.g. "firstname"
	Example  string `json:"example"` // sample output for the dropdown detail
}

// Tokens lists every faker generator that resolves with no caller-supplied
// params, grouped/sorted by category then name. Each candidate is actually
// generated once here, so an advertised token is guaranteed to resolve at send
// time (params metadata alone isn't reliable — some defaulted generators still
// error without real values).
func Tokens() []Token {
	out := make([]Token, 0, len(gofakeit.FuncLookups))
	for name, info := range gofakeit.FuncLookups {
		if _, err := info.Generate(faker, buildParams(&info, nil), &info); err != nil {
			continue
		}
		out = append(out, Token{Category: info.Category, Name: name, Example: info.Example})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Thin wrappers used by the mock server's schema-driven faker.
func Name() string    { return faker.Name() }
func Email() string   { return faker.Email() }
func UUID() string    { return faker.UUID() }
func Phone() string   { return faker.Phone() }
func Word() string    { return faker.Word() }
func City() string    { return faker.City() }
func Country() string { return faker.Country() }
