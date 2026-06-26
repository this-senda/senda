package gitguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
)

// initRepo makes a real git repo at dir with a senda collection holding a
// secret overlay and a history log.
func initRepo(t *testing.T) (dir string, repo *git.Repository) {
	t.Helper()
	dir = t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, ".senda")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(cfg, "senda.secret.yaml"), "vars:\n  - {key: token, value: hunter2}\n")
	write(t, filepath.Join(cfg, "history.jsonl"), "{}\n")
	return dir, repo
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheck_unignored(t *testing.T) {
	dir, _ := initRepo(t)
	st, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !st.InGit {
		t.Fatal("expected InGit")
	}
	if len(st.Unignored) != 2 {
		t.Fatalf("want 2 unignored secret/history files, got %v", st.Unignored)
	}
	if len(st.Tracked) != 0 {
		t.Fatalf("want 0 tracked, got %v", st.Tracked)
	}
}

func TestCheck_notInGit(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".senda")
	os.MkdirAll(cfg, 0o755)
	write(t, filepath.Join(cfg, "senda.secret.yaml"), "vars: []\n")
	st, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.InGit {
		t.Fatal("plain folder must not report InGit")
	}
}

func TestWriteIgnore_thenIgnored(t *testing.T) {
	dir, _ := initRepo(t)
	if err := WriteIgnore(dir); err != nil {
		t.Fatal(err)
	}
	st, _ := Check(dir)
	if len(st.Unignored) != 0 {
		t.Fatalf("after WriteIgnore nothing should be unignored, got %v", st.Unignored)
	}
	// Idempotent: second call adds no duplicate block.
	WriteIgnore(dir)
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if c := strings.Count(string(data), ignoreMarker); c != 1 {
		t.Fatalf("want 1 marker line, got %d", c)
	}
}

func TestCheck_tracked(t *testing.T) {
	dir, repo := initRepo(t)
	wt, _ := repo.Worktree()
	if _, err := wt.Add(filepath.ToSlash(filepath.Join(".senda", "senda.secret.yaml"))); err != nil {
		t.Fatal(err)
	}
	st, _ := Check(dir)
	if len(st.Tracked) != 1 || st.Tracked[0] != ".senda/senda.secret.yaml" {
		t.Fatalf("staged secret should be Tracked, got %v", st.Tracked)
	}
	// history.jsonl still only unignored, not tracked.
	if len(st.Unignored) != 1 || st.Unignored[0] != ".senda/history.jsonl" {
		t.Fatalf("want history unignored, got %v", st.Unignored)
	}
}
