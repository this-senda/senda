package store

import (
	"os"
	"path/filepath"
	"strings"
)

// isYAMLFile reports whether name has a .yaml or .yml extension.
func isYAMLFile(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}

// trimYAMLSuffix strips a trailing .yaml or .yml extension, yielding the bare
// stem used as a request or environment name.
func trimYAMLSuffix(name string) string {
	return strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
}

// ConfigDirName is the per-collection directory that holds every non-request
// file — metadata, environments, mock definitions, security templates and the
// history log. Consolidating them under one dotfolder keeps the collection
// root clean: only request YAML and request folders live at the top level.
const ConfigDirName = ".senda"

// securityDirName is the security-templates folder inside the config dir. The
// leading dot of the legacy ".security" location is dropped — everything under
// .senda/ is already hidden.
const securityDirName = "security"

// collection-level secret overlay filenames (siblings of senda.meta.yaml).
const (
	secretFile    = "senda.secret.yaml"
	secretFileYml = "senda.secret.yml"
)

// legacy (pre-.senda) root-level locations, kept only so Migrate can move
// older collections into the .senda/ layout on open.
const (
	legacyEnvDir      = "environments"
	legacyMockDir     = "mocks"
	legacySecurityDir = ".security"
)

// ConfigDir returns the .senda directory for a collection root.
func ConfigDir(root string) string { return filepath.Join(root, ConfigDirName) }

// EnvironmentsDir returns the canonical environments directory (.senda/environments).
func EnvironmentsDir(root string) string { return filepath.Join(root, ConfigDirName, envDir) }

// MocksDir returns the canonical mock-definitions directory (.senda/mocks).
func MocksDir(root string) string { return filepath.Join(root, ConfigDirName, mockDir) }

// SecurityDir returns the canonical security-templates directory (.senda/security).
func SecurityDir(root string) string { return filepath.Join(root, ConfigDirName, securityDirName) }

// FlowsDir returns the canonical flows directory (.senda/flows). Flows live
// under .senda/ so the request tree-walk (which skips dotfiles) never mistakes a
// *.flow.yaml for a request.
func FlowsDir(root string) string { return filepath.Join(root, ConfigDirName, flowDir) }

// pathExists reports whether anything (file or dir) exists at path.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// dirExists reports whether path exists and is a directory.
func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// isCollectionRoot reports whether dir is a collection root, identified by the
// presence of its .senda config directory. Sub-folders in the request tree
// never have one, so this distinguishes the root (whose metadata lives inside
// .senda/) from folders (whose metadata stays inline next to their requests).
func isCollectionRoot(dir string) bool {
	return dirExists(ConfigDir(dir))
}

// metaReadPath resolves where to read a directory's senda.meta.yaml from,
// preferring the .senda/ copy (collection root) and falling back to the inline
// file (sub-folders, or a root not yet migrated).
func metaReadPath(dir string) string {
	if p := filepath.Join(ConfigDir(dir), metaFile); pathExists(p) {
		return p
	}
	return filepath.Join(dir, metaFile)
}

// metaWritePath resolves where to write a directory's senda.meta.yaml: inside
// .senda/ for the collection root, inline for sub-folders.
func metaWritePath(dir string) string {
	if isCollectionRoot(dir) {
		return filepath.Join(ConfigDir(dir), metaFile)
	}
	return filepath.Join(dir, metaFile)
}

// environmentsReadDir resolves the environments directory to read from,
// preferring .senda/environments and falling back to a legacy root-level
// environments/ folder when present.
func environmentsReadDir(root string) string {
	if d := EnvironmentsDir(root); dirExists(d) {
		return d
	}
	if legacy := filepath.Join(root, legacyEnvDir); dirExists(legacy) {
		return legacy
	}
	return EnvironmentsDir(root)
}

// Migrate moves a collection's root-level config (metadata, secrets,
// environments, mocks and security templates) into the .senda/ directory,
// leaving the root holding only request YAML and folders. It is idempotent and
// best-effort: items already migrated or absent are skipped, and a failure to
// move one item does not stop the others. Sub-folder senda.meta.yaml files are
// part of the request tree and are deliberately left untouched.
//
// Creating .senda/ also marks the directory as a collection root, so later
// metadata writes land inside it rather than back at the top level.
func Migrate(root string) {
	if root == "" {
		return
	}
	cfg := ConfigDir(root)
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		return
	}

	move := func(oldPath, newPath string) {
		if !pathExists(oldPath) || pathExists(newPath) {
			return
		}
		_ = os.MkdirAll(filepath.Dir(newPath), 0o755)
		_ = os.Rename(oldPath, newPath)
	}

	move(filepath.Join(root, metaFile), filepath.Join(cfg, metaFile))
	move(filepath.Join(root, secretFile), filepath.Join(cfg, secretFile))
	move(filepath.Join(root, secretFileYml), filepath.Join(cfg, secretFileYml))
	move(filepath.Join(root, legacyEnvDir), EnvironmentsDir(root))
	move(filepath.Join(root, legacyMockDir), MocksDir(root))
	move(filepath.Join(root, legacySecurityDir), SecurityDir(root))
}
