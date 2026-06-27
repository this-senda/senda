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
