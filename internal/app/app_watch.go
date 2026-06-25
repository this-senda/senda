package app

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"senda/internal/store"
)

// collection file-watch: external edits (git pull, $EDITOR) emit a debounced
// "collection:changed" event so the frontend re-reads the tree.

var watchMu sync.Mutex
var watcher *fsnotify.Watcher

// watchCollection replaces any existing watcher with one rooted at root.
func (a *App) watchCollection(root string) {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watcher != nil {
		watcher.Close()
		watcher = nil
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return // watching is best-effort
	}
	watcher = w
	addDirs(w, root)

	go func() {
		var timer *time.Timer
		fire := func() {
			a.emit("collection:changed", nil)
		}
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ignoreEvent(ev.Name) {
					continue // history log churn, git internals, editor dotfiles
				}
				// New directories need watching too.
				if ev.Op.Has(fsnotify.Create) {
					addDirs(w, ev.Name)
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(300*time.Millisecond, fire)
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

// addDirs walks p (if it is a directory) and watches every subdirectory. The
// .senda config dir is watched (so external edits to environments/mocks/security
// still refresh the collection); other dot-dirs — .git, synced-template repos —
// are skipped, and noisy events within .senda are filtered by ignoreEvent.
func addDirs(w *fsnotify.Watcher, p string) {
	_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != store.ConfigDirName && path != p {
			return filepath.SkipDir
		}
		_ = w.Add(path)
		return nil
	})
}

// ignoreEvent reports whether a watch event should not trigger a refresh: the
// history log (rewritten on every send, including its .tmp), git internals, and
// hidden/editor files (.DS_Store, *.swp, .senda-sync.yaml). Config files under
// .senda/ (environments, mocks, security templates) still refresh.
func ignoreEvent(name string) bool {
	base := filepath.Base(name)
	if strings.HasPrefix(base, "history.jsonl") {
		return true
	}
	if strings.HasPrefix(base, ".") {
		return true
	}
	sep := string(filepath.Separator)
	return strings.Contains(name, sep+".git"+sep)
}
