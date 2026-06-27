// Package history records recently sent requests as an append-only JSONL log
// under a collection's .senda/ directory, and reads back the most recent
// entries for the UI's history panel.
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"senda/internal/model"
)

// appendMu serializes Append's read-modify-write (List → rebuild → tmp → rename)
// so concurrent callers — e.g. a flow's parallel request nodes — can't clobber
// the shared history.jsonl(.tmp) or lose entries.
// ponytail: one global lock; history volume is tiny, so per-collection locks
// aren't worth it.
var appendMu sync.Mutex

const (
	dir  = ".senda"
	file = "history.jsonl"
)

// max entries kept on disk; older lines are dropped on append.
const maxEntries = 500

func logPath(collPath string) string {
	return filepath.Join(collPath, dir, file)
}

// Append records one sent request. Failures are returned but callers may treat
// history as best-effort. The timestamp is set if absent.
func Append(collPath string, e model.HistoryEntry) error {
	if collPath == "" {
		return nil
	}
	appendMu.Lock()
	defer appendMu.Unlock()
	if e.At == "" {
		e.At = time.Now().UTC().Format(time.RFC3339)
	}
	entries, _ := List(collPath, maxEntries)
	// List returns newest-first; rebuild oldest-first with the new entry last.
	all := make([]model.HistoryEntry, 0, len(entries)+1)
	for i := len(entries) - 1; i >= 0; i-- {
		all = append(all, entries[i])
	}
	all = append(all, e)
	if len(all) > maxEntries {
		all = all[len(all)-maxEntries:]
	}

	if err := os.MkdirAll(filepath.Join(collPath, dir), 0o755); err != nil {
		return err
	}
	tmp := logPath(collPath) + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, en := range all {
		if err := enc.Encode(en); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, logPath(collPath))
}

// List returns up to limit entries, newest first. A missing log is not an
// error — it yields an empty slice.
func List(collPath string, limit int) ([]model.HistoryEntry, error) {
	f, err := os.Open(logPath(collPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []model.HistoryEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e model.HistoryEntry
		if json.Unmarshal(line, &e) == nil {
			entries = append(entries, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	// Reverse to newest-first.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

// Clear removes the history log for a collection.
func Clear(collPath string) error {
	err := os.Remove(logPath(collPath))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
