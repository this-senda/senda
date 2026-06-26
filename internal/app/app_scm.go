package app

import (
	"fmt"

	"senda/internal/scm"
)

// GitStatus returns the working-tree-vs-HEAD comparison for the collection dir:
// the current branch and the list of changed requests/files. When the dir is
// not a git repo it returns scm.Status{Repo: false} (no error) so the UI shows
// an "initialise git" hint rather than failing.
func (a *App) GitStatus(collPath string) (scm.Status, error) {
	if collPath == "" {
		return scm.Status{}, fmt.Errorf("no collection open")
	}
	return scm.GetStatus(collPath)
}

// GitDiff returns the semantic per-field comparison of one changed file (path
// relative to the collection dir). Non-request files fall back to a raw text
// diff carried in the Raw field.
func (a *App) GitDiff(collPath, path string) (scm.Diff, error) {
	if collPath == "" {
		return scm.Diff{}, fmt.Errorf("no collection open")
	}
	return scm.GetDiff(collPath, path)
}
