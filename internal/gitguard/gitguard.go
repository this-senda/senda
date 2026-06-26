// Package gitguard checks, when a collection opens, whether its secret and
// history files are safe from accidental git commits — and offers to fix it.
// Collections are plain folders users keep in git; secrets (*.secret.yaml) and
// the local history log must not be pushed. This nudges the user before the
// mishap instead of after.
package gitguard

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"senda/internal/store"
)

// ignoreMarker leads the block we append to .gitignore so we can detect (and
// skip re-adding) a block we wrote on a previous open.
const ignoreMarker = "# senda: keep secrets and local history out of git"

// ignoreLines is the block appended to a collection's .gitignore. Patterns are
// relative to the collection root (where the .gitignore lives): a slash-free
// pattern matches at any depth, so *.secret.yaml covers both the root secret
// overlay and per-environment secret files under .senda/environments.
var ignoreLines = []string{
	ignoreMarker,
	"*.secret.yaml",
	"*.secret.yml",
	".senda/history.jsonl",
}

// Status is the git-hygiene snapshot for a collection, surfaced to the UI right
// after the collection opens.
type Status struct {
	InGit     bool     `json:"inGit"`
	Unignored []string `json:"unignored"` // present, not git-ignored — fixable by adding to .gitignore
	Tracked   []string `json:"tracked"`   // already committed — .gitignore can't help (needs git rm --cached)
}

// Check reports whether collPath sits inside a git repo and whether its secret
// and history files are protected from accidental commits. A collection not in
// git, or with no sensitive files on disk, yields a zero Status (InGit false).
func Check(collPath string) (Status, error) {
	var st Status

	files := sensitiveFiles(collPath)
	if len(files) == 0 {
		return st, nil // nothing sensitive on disk → nothing to guard
	}

	repo, err := git.PlainOpenWithOptions(collPath, &git.PlainOpenOptions{DetectDotGit: true})
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return st, nil // not in git
	}
	if err != nil {
		return st, err
	}
	st.InGit = true

	wt, err := repo.Worktree()
	if err != nil {
		return st, err
	}
	root := wt.Filesystem.Root()
	prefix, err := filepath.Rel(root, collPath)
	if err != nil {
		return st, err
	}
	prefix = filepath.ToSlash(prefix)

	// ReadPatterns walks the whole worktree reading every .gitignore plus
	// .git/info/exclude. ponytail: full-tree walk, but it prunes already-ignored
	// dirs (node_modules etc.), so cost is bounded in practice; swap for a
	// root→collection chain reader if a giant un-ignored monorepo proves slow.
	patterns, _ := gitignore.ReadPatterns(wt.Filesystem, nil)
	matcher := gitignore.NewMatcher(patterns)
	tracked := indexPaths(repo)

	for _, rel := range files {
		repoRel := rel
		if prefix != "." && prefix != "" {
			repoRel = prefix + "/" + rel
		}
		if tracked[repoRel] {
			st.Tracked = append(st.Tracked, rel)
			continue
		}
		if !matcher.Match(strings.Split(repoRel, "/"), false) {
			st.Unignored = append(st.Unignored, rel)
		}
	}
	sort.Strings(st.Unignored)
	sort.Strings(st.Tracked)
	return st, nil
}

// WriteIgnore appends the senda ignore block to <collPath>/.gitignore, creating
// the file if absent. Idempotent: a no-op once the marker line is present.
func WriteIgnore(collPath string) error {
	p := filepath.Join(collPath, ".gitignore")
	existing, _ := os.ReadFile(p)
	if strings.Contains(string(existing), ignoreMarker) {
		return nil
	}

	var b strings.Builder
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		b.WriteByte('\n')
	}
	if len(existing) > 0 {
		b.WriteByte('\n') // blank line before our block
	}
	b.WriteString(strings.Join(ignoreLines, "\n"))
	b.WriteByte('\n')

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(b.String())
	return err
}

// sensitiveFiles returns collection-relative (slash-separated) paths of the
// secret and history files that actually exist under collPath.
func sensitiveFiles(collPath string) []string {
	var out []string
	add := func(abs string) {
		if rel, err := filepath.Rel(collPath, abs); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
	}

	cfg := store.ConfigDir(collPath)
	for _, n := range []string{"senda.secret.yaml", "senda.secret.yml", "history.jsonl"} {
		if p := filepath.Join(cfg, n); fileExists(p) {
			add(p)
		}
	}
	for _, pat := range []string{"*.secret.yaml", "*.secret.yml"} {
		matches, _ := filepath.Glob(filepath.Join(store.EnvironmentsDir(collPath), pat))
		for _, m := range matches {
			add(m)
		}
	}
	return out
}

// indexPaths returns the set of repo-relative (slash-separated) paths in the
// git index — i.e. files already tracked. Empty on any read error.
func indexPaths(repo *git.Repository) map[string]bool {
	m := map[string]bool{}
	idx, err := repo.Storer.Index()
	if err != nil {
		return m
	}
	for _, e := range idx.Entries {
		m[e.Name] = true
	}
	return m
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
