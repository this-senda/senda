// Package scm is a read-only git "what changed" view over a collection dir.
// The collection root is a plain folder of one-YAML-per-request, so git already
// tracks it cleanly; this package surfaces the working-tree-vs-HEAD comparison
// the way Yaak does — a list of changed requests plus a per-field semantic diff
// (URL changed, header added, …) rather than raw YAML line noise.
//
// It shells out to the system `git` binary (cwd = collection dir) so credential
// helpers, SSH and signing config all just work; go-git stays reserved for the
// security-template sync. Read-only: no add/commit/push here yet.
package scm

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"senda/internal/model"

	"gopkg.in/yaml.v3"
)

// ChangedFile is one entry in the source-control list. Path is relative to the
// collection dir; Display is the request name (or the filename for non-request
// files). Other marks files outside the request tree (dotfiles, .senda config,
// .gitignore) so the UI can bucket them under "External file changes".
type ChangedFile struct {
	Path    string `json:"path"`
	Display string `json:"display"`
	Status  string `json:"status"` // modified | added | deleted | renamed | untracked
	Other   bool   `json:"other"`
}

// Status is the working-tree-vs-HEAD comparison for a collection dir.
type Status struct {
	Repo   bool          `json:"repo"`   // false when collPath is not inside a git repo
	Branch string        `json:"branch"` // current branch (empty on detached HEAD)
	Files  []ChangedFile `json:"files"`
}

// FieldDiff is one changed field between the HEAD and working versions of a
// request. Old/New are rendered strings ("" = absent on that side, which makes
// Kind added/removed).
type FieldDiff struct {
	Label string `json:"label"`
	Old   string `json:"old"`
	New   string `json:"new"`
	Kind  string `json:"kind"` // added | removed | changed
}

// Diff is the semantic comparison of a single changed file. For request YAML it
// is a list of FieldDiff; for anything else (config, dotfiles, unparseable
// YAML) Raw carries the plain unified `git diff` text and Fields is nil.
type Diff struct {
	Display string      `json:"display"`
	Fields  []FieldDiff `json:"fields"`
	Raw     string      `json:"raw"`
}

// runGit runs git in the collection dir and returns stdout. cwd-based (not -C)
// so `HEAD:./path` pathspecs resolve relative to the collection even when it is
// a subdirectory of the repo.
func runGit(collPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = collPath
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if errBuf.Len() > 0 {
			return "", &gitError{strings.TrimSpace(errBuf.String())}
		}
		return "", err
	}
	return out.String(), nil
}

type gitError struct{ msg string }

func (e *gitError) Error() string { return e.msg }

// repoRoot returns the absolute git work-tree root for collPath, falling back to
// collPath itself if rev-parse fails. Porcelain status and `git show` pathspecs
// are repo-root-relative, so all file/blob access anchors here — collPath may be
// any subdirectory of the repo.
func repoRoot(collPath string) string {
	out, err := runGit(collPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return collPath
	}
	return strings.TrimSpace(out)
}

// GetStatus lists what changed in the collection dir versus HEAD. When the dir
// is not a git repo it returns Status{Repo: false} with no error, so the UI can
// show an "initialise git to compare" hint rather than an error.
func GetStatus(collPath string) (Status, error) {
	if _, err := runGit(collPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return Status{Repo: false}, nil
	}
	st := Status{Repo: true}
	if b, err := runGit(collPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		st.Branch = strings.TrimSpace(b)
	}
	root := repoRoot(collPath)

	// Limit to the collection subtree (`-- .`, cwd = collPath), list untracked
	// files individually (-uall), NUL-separated so paths with spaces are safe.
	// Porcelain paths come back repo-root-relative regardless of cwd.
	out, err := runGit(collPath, "status", "--porcelain", "-z", "-uall", "--", ".")
	if err != nil {
		return st, err
	}
	for _, entry := range parsePorcelain(out) {
		f := ChangedFile{Path: entry.path, Status: entry.status, Other: isOther(entry.path)}
		f.Display = displayName(root, entry)
		st.Files = append(st.Files, f)
	}
	return st, nil
}

type entry struct {
	path   string
	status string
	gone   bool // working copy absent (deleted) → read HEAD side for the name
}

// parsePorcelain decodes `git status --porcelain -z` records into entries with a
// friendly status word. Rename records carry the original path as a trailing
// NUL field, which we consume but don't display.
func parsePorcelain(out string) []entry {
	parts := strings.Split(out, "\x00")
	var entries []entry
	for i := 0; i < len(parts); i++ {
		rec := parts[i]
		if len(rec) < 3 {
			continue
		}
		xy, path := rec[:2], rec[3:]
		if xy[0] == 'R' || xy[1] == 'R' {
			i++ // rename: next NUL field is the original path, skip it
		}
		entries = append(entries, entry{path: path, status: statusWord(xy)})
		if statusWord(xy) == "deleted" {
			entries[len(entries)-1].gone = true
		}
	}
	return entries
}

// statusWord collapses the two-char XY porcelain code to one label. Untracked
// (`??`) and deletions are special-cased; otherwise the most meaningful of the
// staged/worktree columns wins.
func statusWord(xy string) string {
	switch {
	case xy == "??":
		return "untracked"
	case xy[0] == 'D' || xy[1] == 'D':
		return "deleted"
	case xy[0] == 'A':
		return "added"
	case xy[0] == 'R' || xy[1] == 'R':
		return "renamed"
	default:
		return "modified"
	}
}

// isOther reports whether a path is outside the request tree — dotfiles, the
// .senda config dir, anything not a .yaml. Those bucket under "External
// file changes" and get a raw text diff rather than a field diff.
func isOther(path string) bool {
	if strings.HasPrefix(path, ".") || strings.Contains(path, "/.") {
		return true
	}
	return !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")
}

// displayName resolves a request file to its `name:` field, reading the working
// copy (or, for deletions, the HEAD blob). root is the repo root; paths are
// repo-root-relative. Falls back to the base filename.
func displayName(root string, e entry) string {
	base := strings.TrimSuffix(filepath.Base(e.path), filepath.Ext(e.path))
	if isOther(e.path) {
		return e.path
	}
	var data []byte
	if e.gone {
		data = headBlob(root, e.path)
	} else {
		data, _ = os.ReadFile(filepath.Join(root, e.path))
	}
	if r, err := parseRequest(data); err == nil && r.Name != "" {
		return r.Name
	}
	return base
}

// headBlob returns the file's contents at HEAD, or nil if it didn't exist there
// (added/untracked). path is repo-root-relative; with no `./` prefix `git show`
// anchors it to the repo root, so cwd doesn't matter.
func headBlob(root, path string) []byte {
	out, err := runGit(root, "show", "HEAD:"+path)
	if err != nil {
		return nil
	}
	return []byte(out)
}

func parseRequest(data []byte) (model.Request, error) {
	var r model.Request
	err := yaml.Unmarshal(data, &r)
	return r, err
}

// GetDiff compares the HEAD and working versions of one changed file. Request
// YAML yields a per-field diff; everything else falls back to raw `git diff`.
func GetDiff(collPath, path string) (Diff, error) {
	d := Diff{Display: path}
	root := repoRoot(collPath)
	head := headBlob(root, path)
	work, _ := os.ReadFile(filepath.Join(root, path))

	// rawDiff is the unified text diff, anchored at the repo root so the pathspec
	// resolves regardless of where the collection sits in the repo.
	rawDiff := func() string {
		out, _ := runGit(root, "diff", "HEAD", "--", path)
		if strings.TrimSpace(out) == "" { // untracked: absent from HEAD
			out, _ = runGit(root, "diff", "--no-index", "/dev/null", filepath.Join(root, path))
		}
		return out
	}

	if isOther(path) {
		d.Raw = rawDiff()
		return d, nil
	}

	oldReq, errOld := parseRequest(head)
	newReq, errNew := parseRequest(work)
	// If either side won't parse as a request, don't pretend — show raw text.
	if (len(head) > 0 && errOld != nil) || (len(work) > 0 && errNew != nil) {
		d.Raw = rawDiff()
		return d, nil
	}
	if name := firstNonEmpty(newReq.Name, oldReq.Name); name != "" {
		d.Display = name
	}
	d.Fields = diffRequests(oldReq, newReq, len(head) == 0, len(work) == 0)
	// A request file whose tracked fields are identical still differs on disk
	// (whitespace, comments, key order) — fall back to the raw text diff so the
	// user sees the actual change instead of a misleading "nothing changed".
	if len(d.Fields) == 0 && !bytes.Equal(head, work) {
		d.Raw = rawDiff()
	}
	return d, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// requestFields is the ordered, labelled view of a request used for diffing.
// Scalars render directly; structured fields render as trimmed YAML so a
// changed header or assert shows as a readable block.
func requestFields(r model.Request) []struct{ label, val string } {
	return []struct{ label, val string }{
		{"Name", r.Name},
		{"Method", r.Method},
		{"URL", r.URL},
		{"Query params", yamlBlock(r.Params)},
		{"Path params", yamlBlock(r.PathParams)},
		{"Headers", yamlBlock(r.Headers)},
		{"Body", yamlBlock(r.Body)},
		{"Auth", yamlBlock(r.Auth)},
		{"Assertions", yamlBlock(r.Asserts)},
		{"Pre-request script", r.PreScript},
		{"Post-response script", r.PostScript},
		{"Docs", r.Docs},
		{"Response schema", r.ResponseSchema},
		{"Response example", r.ResponseExample},
		{"On fail", r.OnFail},
	}
}

// diffRequests emits a FieldDiff for every field whose rendered value differs.
// addedFile/removedFile force every present field to read as added/removed so a
// brand-new or deleted request shows its full contents, not "changed from
// nothing".
func diffRequests(oldR, newR model.Request, addedFile, removedFile bool) []FieldDiff {
	oldF, newF := requestFields(oldR), requestFields(newR)
	var diffs []FieldDiff
	for i := range newF {
		o, n := oldF[i].val, newF[i].val
		if o == n {
			continue
		}
		kind := "changed"
		switch {
		case addedFile || o == "":
			kind = "added"
		case removedFile || n == "":
			kind = "removed"
		}
		diffs = append(diffs, FieldDiff{Label: newF[i].label, Old: o, New: n, Kind: kind})
	}
	return diffs
}

// yamlBlock renders a structured field as trimmed YAML, normalising the empty
// shapes (null/[]/{}) to "" so an absent slice doesn't read as a change.
func yamlBlock(v any) string {
	b, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(b))
	switch s {
	case "null", "[]", "{}", "":
		return ""
	}
	return s
}
