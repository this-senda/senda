package scm

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// git runs a git command in dir, failing the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// initRepo makes a temp git repo with one committed request, then mutates it so
// status/diff have something to report.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.email", "t@t.t")
	git(t, dir, "config", "user.name", "t")

	write(t, filepath.Join(dir, "login.yaml"), "name: Login\nmethod: GET\nurl: https://api.test/v1/login\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "init")
	return dir
}

func TestStatusAndSemanticDiff(t *testing.T) {
	dir := initRepo(t)

	// change the URL + method of the committed request, add an untracked file
	write(t, filepath.Join(dir, "login.yaml"), "name: Login\nmethod: POST\nurl: https://api.test/v2/login\n")
	write(t, filepath.Join(dir, "new.yaml"), "name: New\nmethod: GET\nurl: https://api.test/new\n")

	st, err := GetStatus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Repo {
		t.Fatal("expected Repo=true")
	}
	got := map[string]string{} // display -> status
	for _, f := range st.Files {
		got[f.Display] = f.Status
	}
	if got["Login"] != "modified" {
		t.Errorf("Login: want modified, got %q", got["Login"])
	}
	if got["New"] != "untracked" {
		t.Errorf("New: want untracked, got %q", got["New"])
	}

	// semantic diff of the modified request: Method + URL changed, nothing else
	d, err := GetDiff(dir, "login.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if d.Display != "Login" {
		t.Errorf("display: want Login, got %q", d.Display)
	}
	changed := map[string]FieldDiff{}
	for _, f := range d.Fields {
		changed[f.Label] = f
	}
	if len(changed) != 2 {
		t.Fatalf("want 2 changed fields, got %d: %+v", len(changed), d.Fields)
	}
	if m := changed["Method"]; m.Old != "GET" || m.New != "POST" || m.Kind != "changed" {
		t.Errorf("Method diff wrong: %+v", m)
	}
	if u := changed["URL"]; u.Kind != "changed" || u.New != "https://api.test/v2/login" {
		t.Errorf("URL diff wrong: %+v", u)
	}
}

// TestCollectionInSubdir locks the bug where porcelain paths (repo-root-
// relative) were joined against collPath: when the opened collection is a
// subdirectory of the repo, status shows files but the diff reads the wrong
// path and reports "no field changes". Here collPath is one level below root.
func TestCollectionInSubdir(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.email", "t@t.t")
	git(t, dir, "config", "user.name", "t")

	sub := filepath.Join(dir, "examples-collection")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	reqPath := filepath.Join(sub, "get-booking.yaml")
	write(t, reqPath, "name: get-booking\nmethod: GET\nurl: https://api.test/bookings/1\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "init")

	// modify the request inside the subdir
	write(t, reqPath, "name: get-booking\nmethod: GET\nurl: https://api.test/bookings/2\n")

	// status is queried with collPath = the subdir, not the repo root
	st, err := GetStatus(sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Files) != 1 || st.Files[0].Status != "modified" {
		t.Fatalf("want 1 modified file, got %+v", st.Files)
	}
	if st.Files[0].Display != "get-booking" {
		t.Errorf("display: want get-booking, got %q", st.Files[0].Display)
	}

	// the diff must resolve the repo-root-relative path and find the URL change
	d, err := GetDiff(sub, st.Files[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Fields) != 1 || d.Fields[0].Label != "URL" {
		t.Fatalf("want 1 URL field diff, got %+v (raw=%q)", d.Fields, d.Raw)
	}
	if d.Fields[0].New != "https://api.test/bookings/2" {
		t.Errorf("URL new: got %q", d.Fields[0].New)
	}
}

func TestNotARepo(t *testing.T) {
	st, err := GetStatus(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if st.Repo {
		t.Error("want Repo=false outside a git repo")
	}
}
