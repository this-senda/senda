// Package store reads and writes collections as plain YAML files on disk.
// One request = one .yaml file; the folder tree mirrors the collection tree.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"senda/internal/model"
)

const (
	metaFile = "senda.meta.yaml"
	envDir   = "environments"
	mockDir  = "mocks"
)

// reserved paths that are not requests when walking the tree.
func isReserved(name string) bool {
	return name == metaFile || name == envDir || name == mockDir || strings.HasPrefix(name, ".") || isSecretFile(name)
}

// ReadMeta loads the senda.meta.yaml metadata for a directory (collection root or
// sub-folder) without building a tree. The root's metadata is read from
// .senda/senda.meta.yaml; sub-folders keep theirs inline. Name defaults to the
// directory's base name when unset. A missing senda.meta.yaml is not an error:
// the zero metadata (with Name defaulted) is returned, so every folder behaves
// as if it has metadata even before any is saved.
func ReadMeta(dir string) model.Collection {
	c := model.Collection{Path: dir, Name: filepath.Base(dir)}
	if data, err := os.ReadFile(metaReadPath(dir)); err == nil {
		var meta model.Collection
		if err := yaml.Unmarshal(data, &meta); err == nil {
			if meta.Name != "" {
				c.Name = meta.Name
			}
			c.Color = meta.Color
			c.Tags = meta.Tags
			c.Description = meta.Description
			c.Vars = meta.Vars
			c.Auth = meta.Auth
			c.Proxy = meta.Proxy
			c.TLS = meta.TLS
		}
	}
	return c
}

// OpenCollection loads collection metadata and builds its tree. If root points
// at a packed archive (.zip), it is extracted transparently and the collection
// is served from the extracted directory; edits are folded back via PackArchive.
func OpenCollection(root string) (model.Collection, error) {
	if IsArchive(root) {
		live, err := OpenArchive(root)
		if err != nil {
			return model.Collection{Path: root, Name: filepath.Base(root)}, err
		}
		root = live
	}

	// Normalise older collections into the .senda/ layout (best-effort,
	// idempotent) before reading anything back.
	Migrate(root)

	c := ReadMeta(root)
	tree, err := buildTree(root, root)
	if err != nil {
		return c, err
	}
	c.Tree = tree
	return c, nil
}

// SaveCollection writes a directory's metadata (name, color, tags,
// description, vars, auth) to its senda.meta.yaml. The collection root's
// metadata is written inside .senda/; sub-folders keep theirs inline next to
// their requests. The tree and path are runtime-only and never persisted.
func SaveCollection(c model.Collection) error {
	meta := model.Collection{
		Name:        c.Name,
		Color:       c.Color,
		Tags:        c.Tags,
		Description: c.Description,
		Vars:        c.Vars,
		Auth:        c.Auth,
		Proxy:       c.Proxy,
		TLS:         c.TLS,
	}
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	path := metaWritePath(c.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// FolderChain returns the directories between the collection root and the
// directory holding reqPath, ordered root-first (shallowest) to leaf-last
// (deepest), excluding the collection root itself. Used to resolve
// folder-level vars and auth, where deeper folders override shallower ones.
// Returns nil when reqPath is empty, outside collPath, or directly in the root.
func FolderChain(collPath, reqPath string) []string {
	if collPath == "" || reqPath == "" {
		return nil
	}
	root, err := filepath.Abs(collPath)
	if err != nil {
		return nil
	}
	dir, err := filepath.Abs(filepath.Dir(reqPath))
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	var chain []string
	cur := dir
	for cur != root && len(cur) > len(root) {
		chain = append(chain, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	// reverse to root-first order
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// buildTree walks dir and produces a sorted TreeNode (folders first).
func buildTree(root, dir string) (*model.TreeNode, error) {
	meta := ReadMeta(dir)
	node := &model.TreeNode{
		Name:        filepath.Base(dir),
		Path:        dir,
		IsDir:       true,
		Color:       meta.Color,
		Tags:        meta.Tags,
		Description: meta.Description,
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if isReserved(e.Name()) {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			child, err := buildTree(root, full)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)
		} else if isYAMLFile(e.Name()) {
			method := ""
			if req, err := ReadRequest(full); err == nil {
				method = req.Method
			}
			node.Children = append(node.Children, &model.TreeNode{
				Name:   trimYAMLSuffix(e.Name()),
				Path:   full,
				IsDir:  false,
				Method: method,
			})
		}
	}
	sort.SliceStable(node.Children, func(i, j int) bool {
		a, b := node.Children[i], node.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir // folders first
		}
		return a.Name < b.Name
	})
	return node, nil
}

// ReadRequest loads a single request YAML file.
func ReadRequest(path string) (model.Request, error) {
	var req model.Request
	data, err := os.ReadFile(path)
	if err != nil {
		return req, err
	}
	if err := yaml.Unmarshal(data, &req); err != nil {
		return req, fmt.Errorf("parse %s: %w", path, err)
	}
	return req, nil
}

// ReadRequests loads every request file in paths, in order, failing on the
// first read error.
func ReadRequests(paths []string) ([]model.Request, error) {
	reqs := make([]model.Request, 0, len(paths))
	for _, p := range paths {
		req, err := ReadRequest(p)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}

// SaveRequest serialises a request to YAML, creating parent dirs as needed.
func SaveRequest(path string, req model.Request) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(req)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// DeleteRequest removes a request file.
func DeleteRequest(path string) error {
	return os.Remove(path)
}

// DeleteNode removes a request file or a folder (recursively, including its
// senda.meta.yaml metadata and any nested requests).
func DeleteNode(path string) error {
	return os.RemoveAll(path)
}

// ListEnvironments loads every environment YAML under .senda/environments/.
func ListEnvironments(root string) ([]model.Environment, error) {
	dir := environmentsReadDir(root)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var envs []model.Environment
	for _, e := range entries {
		if e.IsDir() || !isYAMLFile(e.Name()) {
			continue
		}
		if isSecretFile(e.Name()) {
			continue // merged at send time, never listed or round-tripped
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var env model.Environment
		if err := yaml.Unmarshal(data, &env); err != nil {
			return nil, err
		}
		if env.Name == "" {
			env.Name = trimYAMLSuffix(e.Name())
		}
		envs = append(envs, env)
	}
	sort.Slice(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })
	return envs, nil
}

// SaveEnvironment writes one environment file under .senda/environments/.
func SaveEnvironment(root string, env model.Environment) error {
	dir := EnvironmentsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(env)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, env.Name+".yaml"), data, 0o644)
}

// CreateFolder makes a new folder inside the collection.
func CreateFolder(path string) error {
	return os.MkdirAll(path, 0o755)
}

// isSecretFile reports whether name is a secrets overlay (*.secret.yaml/yml).
func isSecretFile(name string) bool {
	return strings.HasSuffix(name, ".secret.yaml") || strings.HasSuffix(name, ".secret.yml")
}

// secretVars reads a vars-only YAML file, returning nil if it doesn't exist
// or doesn't parse. Secrets are best-effort overlays, never hard errors.
func secretVars(path string) []model.KV {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f struct {
		Vars []model.KV `yaml:"vars"`
	}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil
	}
	return f.Vars
}

// CollectionSecrets returns vars from .senda/senda.secret.yaml (gitignored
// sibling of senda.meta.yaml), falling back to a legacy root-level copy.
// Empty when absent.
func CollectionSecrets(root string) []model.KV {
	for _, p := range []string{
		filepath.Join(ConfigDir(root), secretFile),
		filepath.Join(ConfigDir(root), secretFileYml),
		filepath.Join(root, secretFile),
		filepath.Join(root, secretFileYml),
	} {
		if v := secretVars(p); v != nil {
			return v
		}
	}
	return nil
}

// EnvironmentSecrets returns vars from .senda/environments/<name>.secret.yaml.
// These overlay (and shadow) the plain environment's vars at send time.
func EnvironmentSecrets(root, name string) []model.KV {
	if name == "" {
		return nil
	}
	dir := environmentsReadDir(root)
	if v := secretVars(filepath.Join(dir, name+".secret.yaml")); v != nil {
		return v
	}
	return secretVars(filepath.Join(dir, name+".secret.yml"))
}
