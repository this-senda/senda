package pipeline

import (
	"testing"

	"senda/internal/model"
)

// TestResolveRes covers {{res.<slug>.<target>}} resolution against responses
// stored in the session, and the misses that must stay unresolved.
func TestResolveRes(t *testing.T) {
	s := NewSession()
	s.storeResponse("auth/login.yaml", model.Response{
		Status:  200,
		Body:    `{"token":"abc123","user":{"id":42}}`,
		Headers: map[string][]string{"X-Trace": {"t-1"}},
	})

	cases := []struct {
		name string // the placeholder inner name
		want string
		ok   bool
	}{
		{"res.login.json.token", "abc123", true},
		{"res.login.json.user.id", "42", true},
		{"res.login.status", "200", true},
		{"res.login.body", `{"token":"abc123","user":{"id":42}}`, true},
		{"res.login.header.X-Trace", "t-1", true},
		{"res.missing.json.token", "", false}, // unknown slug
		{"res.login.json.nope", "", false},    // missing json path
		{"res.login", "", false},              // no target
		{"token", "", false},                  // not a res ref
	}
	for _, c := range cases {
		got, ok := s.resolveRes(c.name)
		if ok != c.ok || got != c.want {
			t.Errorf("resolveRes(%q) = (%q,%v), want (%q,%v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestResRefViaApply verifies the Dynamic hook is wired into the scope so
// interpolation of {{res...}} works the same as a normal variable.
func TestResRefViaApply(t *testing.T) {
	s := NewSession()
	s.storeResponse("login.yaml", model.Response{Body: `{"token":"xyz"}`})

	scope := s.Scope("", "", "")
	scope.Dynamic = s.resolveRes

	got := scope.Apply("Bearer {{res.login.json.token}}")
	if got != "Bearer xyz" {
		t.Errorf("Apply = %q, want %q", got, "Bearer xyz")
	}
	if len(scope.Unresolved) != 0 {
		t.Errorf("unexpected unresolved: %v", scope.Unresolved)
	}

	// An unknown reference is left verbatim and recorded as unresolved.
	got = scope.Apply("{{res.login.json.missing}}")
	if got != "{{res.login.json.missing}}" {
		t.Errorf("missing ref should stay verbatim, got %q", got)
	}
	if len(scope.Unresolved) == 0 {
		t.Error("missing ref should be recorded as unresolved")
	}
}

func TestSlugOf(t *testing.T) {
	cases := map[string]string{
		"auth/login.yaml": "login",
		"login.yml":       "login",
		"a/b/create.yaml": "create",
		"":                "",
	}
	for in, want := range cases {
		if got := slugOf(in); got != want {
			t.Errorf("slugOf(%q) = %q, want %q", in, got, want)
		}
	}
}
