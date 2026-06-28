package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"senda/internal/importer"
	"senda/internal/model"
	"senda/internal/schemaval"
	"senda/internal/store"
)

// specPath resolves a spec basename to its file under .senda/openapi, guarding
// against path traversal by stripping any directory components.
func specPath(collPath, file string) string {
	return filepath.Join(store.SpecsDir(collPath), filepath.Base(file))
}

// ListSpecs returns the basenames of the OpenAPI specs stored in the
// collection's .senda/openapi directory.
func (a *App) ListSpecs(collPath string) ([]string, error) {
	if collPath == "" {
		return nil, fmt.Errorf("no collection open")
	}
	entries, err := os.ReadDir(store.SpecsDir(collPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml") || strings.HasSuffix(n, ".json") {
			out = append(out, n)
		}
	}
	return out, nil
}

// ReadSpec returns the raw text of a stored spec file for the editor.
func (a *App) ReadSpec(collPath, file string) (string, error) {
	if collPath == "" {
		return "", fmt.Errorf("no collection open")
	}
	data, err := os.ReadFile(specPath(collPath, file))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteSpec saves raw spec text back to its file (creating .senda/openapi if
// needed). Used by the spec editor's Save.
func (a *App) WriteSpec(collPath, file, data string) error {
	if collPath == "" {
		return fmt.Errorf("no collection open")
	}
	if err := os.MkdirAll(store.SpecsDir(collPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(specPath(collPath, file), []byte(data), 0o644)
}

// ValidateSpec parses spec text and returns any structural problems, so the
// editor can flag an invalid document as the user types.
func (a *App) ValidateSpec(data string) []importer.SpecError {
	return importer.ValidateSpec([]byte(data))
}

// SpecOperations lists a spec's operations for a request's relink dropdown.
func (a *App) SpecOperations(data string) ([]importer.SpecOp, error) {
	return importer.SpecOperations([]byte(data))
}

// RequestBodySchema returns the JSON Schema of the linked operation's JSON
// request body (refs inlined), read from the stored spec. Feeds the body
// editor's validation + key autocomplete. Empty string = no JSON request body.
func (a *App) RequestBodySchema(collPath, file, operationID string) (string, error) {
	if collPath == "" {
		return "", fmt.Errorf("no collection open")
	}
	data, err := os.ReadFile(specPath(collPath, file))
	if err != nil {
		return "", err
	}
	return importer.RequestBodySchema(data, operationID)
}

// ValidateJSONSchema validates a JSON body against an inline JSON Schema string,
// reusing the response-schema validator for editor-time request-body checks.
func (a *App) ValidateJSONSchema(schema, body string) []model.AssertResult {
	return schemaval.Validate(schema, body)
}

// SpecOperationDetail returns the form-editable view of one operation in a spec.
func (a *App) SpecOperationDetail(data, operationID string) (importer.SpecOpDetail, error) {
	return importer.SpecOperationDetail([]byte(data), operationID)
}

// UpdateSpecOperation applies a structured edit to one operation and writes the
// spec back, preserving everything the form doesn't touch. Returns the new raw
// text so the editor can re-sync.
func (a *App) UpdateSpecOperation(collPath, file, operationID string, detail importer.SpecOpDetail) (string, error) {
	if collPath == "" {
		return "", fmt.Errorf("no collection open")
	}
	path := specPath(collPath, file)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	out, err := importer.UpdateSpecOperation(data, operationID, detail)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return "", err
	}
	return string(out), nil
}
