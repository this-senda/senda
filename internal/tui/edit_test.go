package tui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"

	"senda/internal/model"
)

func newTestInput(val string) textinput.Model {
	ti := textinput.New()
	ti.SetValue(val)
	ti.CursorEnd()
	return ti
}

func TestFilterCandidates(t *testing.T) {
	all := []acItem{
		{insert: "baseUrl", label: "baseUrl"},
		{insert: "$person.firstname", label: "$firstname"},
		{insert: "$internet.email", label: "$email"},
	}

	// Empty token returns everything.
	if got := filterCandidates(all, ""); len(got) != 3 {
		t.Fatalf("empty token: want 3, got %d", len(got))
	}
	// Leading "$" is stripped before matching, so "$ema" matches the email token.
	got := filterCandidates(all, "$ema")
	if len(got) != 1 || got[0].insert != "$internet.email" {
		t.Fatalf("$ema: want email token, got %+v", got)
	}
	// Case-insensitive substring match against both label and insert.
	if got := filterCandidates(all, "URL"); len(got) != 1 || got[0].insert != "baseUrl" {
		t.Fatalf("URL: want baseUrl, got %+v", got)
	}
	// No match → empty.
	if got := filterCandidates(all, "zzz"); len(got) != 0 {
		t.Fatalf("zzz: want 0, got %d", len(got))
	}
}

func TestOffsetLineColRoundTrip(t *testing.T) {
	val := "{\n  \"email\": \"x\"\n}"
	for off := 0; off <= len(val); off++ {
		ln, col := offsetToLineCol(val, off)
		if got := lineColToOffset(val, ln, col); got != off {
			t.Fatalf("off=%d → (%d,%d) → %d", off, ln, col, got)
		}
	}
}

func TestRefreshCompletionDetectsToken(t *testing.T) {
	m := tuiModel{buf: map[string]model.Request{}, dirty: map[string]bool{}}
	m.input = newTestInput("https://{{base")
	m.refreshCompletion()
	if !m.ac.open {
		t.Fatal("expected popup open for unclosed {{base")
	}
	if m.ac.start != len("https://{{") {
		t.Fatalf("start: want %d, got %d", len("https://{{"), m.ac.start)
	}

	// A closed token must not trigger the popup.
	m.input = newTestInput("https://{{base}}/x")
	m.refreshCompletion()
	if m.ac.open {
		t.Fatal("expected popup closed once {{base}} is closed")
	}
}
