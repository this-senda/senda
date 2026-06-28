package store

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"senda/internal/model"
)

// isFlowFile reports whether name is a flow definition (*.flow.yaml/.yml).
func isFlowFile(name string) bool {
	return strings.HasSuffix(name, ".flow.yaml") || strings.HasSuffix(name, ".flow.yml")
}

// flowStem strips the .flow.yaml/.flow.yml suffix, yielding the bare flow name.
func flowStem(name string) string {
	return strings.TrimSuffix(strings.TrimSuffix(name, ".flow.yaml"), ".flow.yml")
}

// ListFlows returns the flows defined under .senda/flows/, identity only.
func ListFlows(root string) ([]model.FlowInfo, error) {
	dir := FlowsDir(root)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var flows []model.FlowInfo
	for _, e := range entries {
		if e.IsDir() || !isFlowFile(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		name := flowStem(e.Name())
		if f, err := ReadFlow(p); err == nil && f.Name != "" {
			name = f.Name
		}
		flows = append(flows, model.FlowInfo{Name: name, Path: p})
	}
	sort.Slice(flows, func(i, j int) bool { return flows[i].Name < flows[j].Name })
	return flows, nil
}

// ReadFlow loads one flow YAML file. Name defaults to the file stem when unset.
func ReadFlow(path string) (model.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Flow{}, err
	}
	var f model.Flow
	if err := yaml.Unmarshal(data, &f); err != nil {
		return model.Flow{}, err
	}
	f.Path = path
	if f.Name == "" {
		f.Name = flowStem(filepath.Base(path))
	}
	return f, nil
}

// ReadFlowRaw returns a flow file's verbatim text, for an editor buffer.
// (ReadFlow round-trips through the struct and would drop comments/formatting.)
func ReadFlowRaw(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveFlowRaw writes content verbatim to a flow file, creating its dir. Used by
// the in-app YAML editor so user comments and layout survive a round-trip.
func SaveFlowRaw(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// flowFileName sanitizes a flow name into a slug-safe file stem (letters,
// digits, '.', '-', '_'); anything else collapses to '-'.
func flowFileName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = "flow"
	}
	return s
}

// CreateFlow writes a starter flow under .senda/flows/<slug>.flow.yaml and
// returns its path. Errors if a file with that slug already exists.
func CreateFlow(root, name string) (string, error) {
	dir := FlowsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, flowFileName(name)+".flow.yaml")
	if pathExists(path) {
		return "", os.ErrExist
	}
	tmpl := "name: " + name + "\nstart: step1\nnodes:\n  step1:\n    type: request\n    request: path/to/request.yaml\n"
	if err := os.WriteFile(path, []byte(tmpl), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// SaveFlow writes one flow file under .senda/flows/<name>.flow.yaml.
func SaveFlow(root string, f model.Flow) error {
	dir := FlowsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, f.Name+".flow.yaml"), data, 0o644)
}

// DeleteFlow removes a flow file by path.
func DeleteFlow(path string) error {
	return os.Remove(path)
}

// ResolveFlow turns a flow name or path into a concrete file path: an existing
// path is used as-is, otherwise it's looked up as a name under .senda/flows/.
func ResolveFlow(root, nameOrPath string) (string, error) {
	if pathExists(nameOrPath) {
		return nameOrPath, nil
	}
	name := flowStem(filepath.Base(nameOrPath))
	for _, ext := range []string{".flow.yaml", ".flow.yml"} {
		p := filepath.Join(FlowsDir(root), name+ext)
		if pathExists(p) {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}
