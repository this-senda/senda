package docgen

import (
	"strings"
	"testing"
)

func TestRenderFragmentEmphasis(t *testing.T) {
	got := RenderFragment("**bold** and *italic* and `code` and <script>")
	for _, want := range []string{
		"<strong>bold</strong>",
		"<em>italic</em>",
		"<code>code</code>",
		"&lt;script&gt;", // raw HTML escaped, not executable
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<script>") {
		t.Errorf("unescaped HTML leaked:\n%s", got)
	}
}
