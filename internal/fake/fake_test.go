package fake

import "testing"

func TestFuncNamespaceAndBare(t *testing.T) {
	// Namespaced and bare names resolve to the same generator key.
	if v, ok := Func("person.firstname"); !ok || v == "" {
		t.Errorf("namespaced firstname: %q ok=%v", v, ok)
	}
	if v, ok := Func("firstname"); !ok || v == "" {
		t.Errorf("bare firstname: %q ok=%v", v, ok)
	}
	if _, ok := Func("person.definitelynotareal"); ok {
		t.Error("unknown token should be unresolved")
	}
}

func TestFuncParams(t *testing.T) {
	// A pinned range must produce exactly that value.
	if v, ok := Func("number.number(min=7,max=7)"); !ok || v != "7" {
		t.Errorf("number(min=7,max=7): %q ok=%v", v, ok)
	}
	// A param-required generator resolves once params are supplied.
	if _, ok := Func("number.number(min=1,max=100)"); !ok {
		t.Error("number with params should resolve")
	}
}

func TestTokensAllResolvable(t *testing.T) {
	toks := Tokens()
	if len(toks) < 100 {
		t.Fatalf("expected a large catalog, got %d", len(toks))
	}
	// Every advertised token must actually resolve (zero-param filter holds).
	for _, tok := range toks {
		if _, ok := Func(tok.Category + "." + tok.Name); !ok {
			t.Errorf("advertised token %s.%s does not resolve", tok.Category, tok.Name)
		}
	}
}
