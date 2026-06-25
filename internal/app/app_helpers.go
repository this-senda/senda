package app

import (
	"fmt"
	"time"

	"senda/internal/history"
	"senda/internal/model"
)

// emit sends a runtime event to the frontend, a no-op when running headless
// (tests, or before the Wails app is wired up).
func (a *App) emit(name string, data any) {
	if a.wails != nil {
		a.wails.Event.Emit(name, data)
	}
}

// requireWails reports an error when no Wails app is attached, used to guard the
// native dialog methods that have no headless fallback.
func (a *App) requireWails() error {
	if a.wails == nil {
		return fmt.Errorf("native dialogs unavailable")
	}
	return nil
}

// recordSend appends one history entry for a completed send. It is a no-op for
// ad-hoc sends outside a collection (collPath == "").
func recordSend(collPath string, req model.Request, resp model.Response, appliedURL string) {
	if collPath == "" {
		return
	}
	_ = history.Append(collPath, model.HistoryEntry{
		At:         time.Now().UTC().Format(time.RFC3339),
		Method:     req.Method,
		URL:        appliedURL,
		Status:     resp.Status,
		DurationMs: resp.DurationMs,
		SizeBytes:  resp.SizeBytes,
		Error:      resp.Error,
	})
}
