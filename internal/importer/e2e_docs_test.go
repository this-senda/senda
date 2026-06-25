package importer

import (
	"path/filepath"
	"strings"
	"testing"

	"senda/internal/docgen"
	"senda/internal/store"
)

// TestOpenAPIDocsRoundTrip is a throwaway end-to-end check: import -> save YAML
// -> generate markdown. Confirms operation summary/description survive to disk
// and into rendered docs.
func TestOpenAPIDocsRoundTrip(t *testing.T) {
	out, err := OpenAPI([]byte(openapiSample))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, im := range out {
		sub := filepath.Join(im.Dir...)
		path := filepath.Join(dir, sub, im.Request.Name+".yaml")
		if err := store.SaveRequest(path, im.Request); err != nil {
			t.Fatal(err)
		}
	}
	md, err := docgen.GenerateMarkdown(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "List all users") || !strings.Contains(md, "paginated list") {
		t.Errorf("markdown missing operation docs:\n%s", md)
	}
}
