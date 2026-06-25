package app

import (
	"context"
	"fmt"

	"senda/internal/model"
	"senda/internal/security"
	"senda/internal/store"
)

// RunSecurityScan runs the embedded security check pack (plus any
// nuclei-compatible http templates under <collection>/.senda/security) against the
// unique resolved URLs of every request under folderPath. Variables resolve
// against the named environment. Progress streams to the frontend as
// "security:start" / "security:check" events (one check per template×target,
// whether it matched, passed or errored); the returned summary aggregates
// the run.
//
// Scans send real probe traffic — the UI warns the user to only scan APIs
// they own or are authorized to test.
func (a *App) RunSecurityScan(ctx context.Context, folderPath, collPath, envName string, opts model.SecurityOptions) (model.SecuritySummary, error) {
	targets, err := a.scanTargets(folderPath, collPath, envName)
	if err != nil {
		return model.SecuritySummary{}, err
	}

	a.emit("security:start", map[string]any{"targets": targets})
	return security.Run(ctx, targets, a.scanTemplateDir(collPath, opts), opts, func(c model.SecurityCheck) {
		a.emit("security:check", c)
	})
}

// SecurityScanPlan previews how large a scan would be for the given options —
// the unique target count, the number of templates that match the filter, and
// the resulting template×target check count — without sending any traffic. The
// UI calls this as the user tweaks scope/severity/tags so it can show how many
// tests will run before they press Scan.
func (a *App) SecurityScanPlan(folderPath, collPath, envName string, opts model.SecurityOptions) (model.ScanPlan, error) {
	targets, err := a.scanTargets(folderPath, collPath, envName)
	if err != nil {
		return model.ScanPlan{}, err
	}
	templates := security.CountTemplates(a.scanTemplateDir(collPath, opts), opts)
	return model.ScanPlan{
		Targets:   len(targets),
		Templates: templates,
		Checks:    templates * len(targets),
	}, nil
}

// scanTargets resolves the unique probe URLs for every request under folderPath
// against the named environment.
func (a *App) scanTargets(folderPath, collPath, envName string) ([]string, error) {
	paths, err := store.ListRequests(folderPath)
	if err != nil {
		return nil, err
	}
	reqs, err := store.ReadRequests(paths)
	if err != nil {
		return nil, err
	}
	scope := a.session.Scope(collPath, "", envName)
	return security.Targets(reqs, scope.Apply), nil
}

// scanTemplateDir is the extra-template directory a scan loads from, or "" when
// the run is restricted to the embedded builtin pack (opts.Builtin) or no
// collection is open.
func (a *App) scanTemplateDir(collPath string, opts model.SecurityOptions) string {
	if collPath == "" || opts.Builtin {
		return ""
	}
	return store.SecurityDir(collPath)
}

// SyncSecurityTemplates clones or pulls a nuclei-compatible template repo into
// the collection's .senda/security/templates folder, so new checks can be pulled
// from GitHub without an app update. Returns the resulting sync state (source,
// commit, time, count of supported templates found).
func (a *App) SyncSecurityTemplates(ctx context.Context, collPath, url, ref string) (security.SyncState, error) {
	if collPath == "" {
		return security.SyncState{}, fmt.Errorf("no collection open")
	}
	return security.SyncTemplates(ctx, collPath, url, ref)
}

// SecurityTemplatesState returns the last template-sync record for a
// collection (zero value if never synced).
func (a *App) SecurityTemplatesState(collPath string) (security.SyncState, error) {
	if collPath == "" {
		return security.SyncState{}, nil
	}
	return security.ReadSyncState(collPath)
}
