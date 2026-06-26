package app

import "senda/internal/gitguard"

// GitGuardStatus reports whether collPath is inside a git repo and whether its
// secret/history files are protected from accidental commits. Called by the
// frontend right after a collection opens.
func (a *App) GitGuardStatus(collPath string) (gitguard.Status, error) {
	return gitguard.Check(collPath)
}

// GitGuardIgnore appends senda's secret/history ignore block to the
// collection's .gitignore (idempotent).
func (a *App) GitGuardIgnore(collPath string) error {
	return gitguard.WriteIgnore(collPath)
}
