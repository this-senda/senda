package vars

import (
	"reflect"
	"testing"

	"senda/internal/model"
)

func kv(k, v string) model.KV { return model.KV{Key: k, Value: v, Enabled: true} }

func TestPrecedence(t *testing.T) {
	coll := []model.KV{kv("baseUrl", "http://coll"), kv("token", "coll-tok")}
	env := []model.KV{kv("baseUrl", "http://env")}
	// Lowest precedence first: collection, then environment.
	sc := Build(coll, env)

	if got := sc.Apply("{{baseUrl}}/x"); got != "http://env/x" {
		t.Errorf("env should override collection: got %q", got)
	}
	if got := sc.Apply("{{token}}"); got != "coll-tok" {
		t.Errorf("fall back to collection: got %q", got)
	}
}

func TestUnresolved(t *testing.T) {
	sc := Build([]model.KV{kv("a", "1")})
	got := sc.Apply("{{a}}-{{missing}}-{{missing}}")
	if got != "1-{{missing}}-{{missing}}" {
		t.Errorf("unknown left verbatim: got %q", got)
	}
	if !reflect.DeepEqual(sc.Unresolved, []string{"missing"}) {
		t.Errorf("unresolved dedup: got %v", sc.Unresolved)
	}
}

func TestWhitespaceAndDisabled(t *testing.T) {
	sc := Build([]model.KV{
		kv("x", "ok"),
		{Key: "y", Value: "no", Enabled: false},
	})
	if got := sc.Apply("{{ x }}"); got != "ok" {
		t.Errorf("trim inside braces: got %q", got)
	}
	if got := sc.Apply("{{y}}"); got != "{{y}}" {
		t.Errorf("disabled var ignored: got %q", got)
	}
}

func TestApplyKVsDropsDisabled(t *testing.T) {
	sc := Build([]model.KV{kv("v", "X")})
	in := []model.KV{kv("k", "{{v}}"), {Key: "off", Value: "y", Enabled: false}}
	out := sc.ApplyKVs(in)
	if len(out) != 1 || out[0].Value != "X" {
		t.Errorf("interpolate + drop disabled: got %+v", out)
	}
}

func TestFakerTokens(t *testing.T) {
	sc := Build(nil)
	// Known faker token resolves to a non-verbatim value.
	if got := sc.Apply("{{$uuid}}"); got == "{{$uuid}}" || len(got) != 36 {
		t.Errorf("faker uuid not resolved: got %q", got)
	}
	// $-prefixed faker tokens are never recorded as unresolved scope vars.
	sc.Apply("{{$email}}")
	if len(sc.Unresolved) != 0 {
		t.Errorf("faker token leaked into Unresolved: %v", sc.Unresolved)
	}
	// Unknown faker token is left verbatim.
	if got := sc.Apply("{{$nope}}"); got != "{{$nope}}" {
		t.Errorf("unknown faker token: got %q", got)
	}
}

func TestFakerNamespacedAndParams(t *testing.T) {
	sc := Build([]model.KV{kv("greet", "hi")})
	// Namespaced token resolves (not left verbatim).
	if got := sc.Apply("{{$person.firstname}}"); got == "" || got == "{{$person.firstname}}" {
		t.Errorf("namespaced faker not resolved: got %q", got)
	}
	// Param token honours a pinned range, inline with a plain var and literals.
	if got := sc.Apply("{{greet}} n={{$number.number(min=4,max=4)}}"); got != "hi n=4" {
		t.Errorf("param faker + plain var: got %q", got)
	}
	// Grammar change didn't break plain vars or leak faker into Unresolved.
	if len(sc.Unresolved) != 0 {
		t.Errorf("unexpected unresolved: %v", sc.Unresolved)
	}
}
