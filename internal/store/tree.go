package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListRequests walks dir recursively and returns every request file path,
// sorted (folders' contents grouped, depth-first, names ascending). Reserved
// entries (senda.meta.yaml, environments/, dotfiles) are skipped.
func ListRequests(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{dir}, nil
	}
	var out []string
	if err := walkRequests(dir, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func walkRequests(dir string, out *[]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		if isReserved(e.Name()) {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			// Skip subdirs we can't read (e.g. restricted /proc entries when
			// the collection root is "/") rather than aborting the whole walk.
			if err := walkRequests(full, out); err != nil {
				continue
			}
		} else if isYAMLFile(e.Name()) {
			*out = append(*out, full)
		}
	}
	return nil
}

// RenameNode renames a request file or folder to newName within the same
// parent directory and returns the new path. For request files the .yaml
// extension is preserved regardless of whether newName includes it.
func RenameNode(path, newName string) (string, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return "", fmt.Errorf("new name is empty")
	}
	if strings.ContainsAny(newName, `/\`) {
		return "", fmt.Errorf("name may not contain path separators")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(path)
	if !info.IsDir() {
		newName = trimYAMLSuffix(newName) + ".yaml"
	}
	dest := filepath.Join(parent, newName)
	if dest == path {
		return path, nil
	}
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("%q already exists", newName)
	}
	if err := os.Rename(path, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// MoveNode moves a request file or folder into destDir and returns the new
// path. destDir must be an existing directory; moving a folder into its own
// subtree is rejected.
func MoveNode(srcPath, destDir string) (string, error) {
	info, err := os.Stat(srcPath)
	if err != nil {
		return "", err
	}
	di, err := os.Stat(destDir)
	if err != nil {
		return "", err
	}
	if !di.IsDir() {
		return "", fmt.Errorf("destination is not a directory")
	}
	dest := filepath.Join(destDir, filepath.Base(srcPath))
	if dest == srcPath {
		return srcPath, nil
	}
	if info.IsDir() {
		rel, err := filepath.Rel(srcPath, destDir)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "." {
			return "", fmt.Errorf("cannot move a folder into itself")
		}
	}
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("%q already exists in destination", filepath.Base(srcPath))
	}
	if err := os.Rename(srcPath, dest); err != nil {
		return "", err
	}
	return dest, nil
}
