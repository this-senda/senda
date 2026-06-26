package store

import (
	"os"
	"path/filepath"
	"testing"

	"senda/internal/model"
)

func TestRequestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "create-user.yaml")

	req := model.Request{
		Name:   "Create user",
		Method: "POST",
		URL:    "{{baseUrl}}/users",
		Headers: []model.KV{
			{Key: "Content-Type", Value: "application/json", Enabled: true},
		},
		Body: model.Body{Type: model.BodyJSON, Raw: "{\n  \"name\": \"Ada\"\n}"},
	}
	if err := SaveRequest(path, req); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRequest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != req.Name || got.Method != req.Method || got.URL != req.URL {
		t.Errorf("scalar mismatch: %+v", got)
	}
	if got.Body.Raw != req.Body.Raw {
		t.Errorf("multiline body not preserved: got %q", got.Body.Raw)
	}
	if len(got.Headers) != 1 || got.Headers[0].Key != "Content-Type" {
		t.Errorf("headers not preserved: %+v", got.Headers)
	}
}

func TestBuildTreeFoldersFirst(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "users"), 0o755)
	os.WriteFile(filepath.Join(root, "zeta.yaml"), []byte("name: z"), 0o644)
	os.WriteFile(filepath.Join(root, "users", "list.yaml"), []byte("name: l"), 0o644)
	os.WriteFile(filepath.Join(root, "senda.meta.yaml"), []byte("name: meta"), 0o644) // reserved
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644)                // ignored

	c, err := OpenCollection(root)
	if err != nil {
		t.Fatal(err)
	}
	kids := c.Tree.Children
	if len(kids) != 2 {
		t.Fatalf("expected folder + 1 request (meta & txt excluded), got %d: %+v", len(kids), kids)
	}
	if !kids[0].IsDir || kids[0].Name != "users" {
		t.Errorf("folder should sort first: %+v", kids[0])
	}
	if kids[1].Name != "zeta" {
		t.Errorf("request name should strip ext: %+v", kids[1])
	}
}

func TestEnvironmentsRoundTrip(t *testing.T) {
	root := t.TempDir()
	env := model.Environment{
		Name: "dev",
		Vars: []model.KV{{Key: "baseUrl", Value: "http://localhost", Enabled: true}},
	}
	if err := SaveEnvironment(root, env); err != nil {
		t.Fatal(err)
	}
	envs, err := ListEnvironments(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 1 || envs[0].Name != "dev" || envs[0].Vars[0].Value != "http://localhost" {
		t.Errorf("env round-trip failed: %+v", envs)
	}
}

func TestSecretsOverlay(t *testing.T) {
	root := t.TempDir()
	envs := filepath.Join(root, "environments")
	os.MkdirAll(envs, 0o755)
	os.WriteFile(filepath.Join(envs, "dev.yaml"),
		[]byte("name: dev\nvars:\n  - {key: baseUrl, value: http://localhost, enabled: true}\n"), 0o644)
	os.WriteFile(filepath.Join(envs, "dev.secret.yaml"),
		[]byte("vars:\n  - {key: token, value: hunter2, enabled: true}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "senda.secret.yaml"),
		[]byte("vars:\n  - {key: apiKey, value: shh, enabled: true}\n"), 0o644)

	// secret files never appear as environments…
	list, err := ListEnvironments(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "dev" {
		t.Fatalf("secret file leaked into env list: %+v", list)
	}
	// …or in the request tree.
	c, err := OpenCollection(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Tree.Children) != 0 {
		t.Errorf("senda.secret.yaml leaked into tree: %+v", c.Tree.Children)
	}

	if v := EnvironmentSecrets(root, "dev"); len(v) != 1 || v[0].Key != "token" || v[0].Value != "hunter2" {
		t.Errorf("env secrets = %+v", v)
	}
	if v := EnvironmentSecrets(root, "prod"); v != nil {
		t.Errorf("missing env secrets should be nil, got %+v", v)
	}
	if v := CollectionSecrets(root); len(v) != 1 || v[0].Key != "apiKey" {
		t.Errorf("collection secrets = %+v", v)
	}
	if v := CollectionSecrets(t.TempDir()); v != nil {
		t.Errorf("missing collection secrets should be nil, got %+v", v)
	}
}

func TestListEnvironmentsMissingDir(t *testing.T) {
	envs, err := ListEnvironments(t.TempDir())
	if err != nil {
		t.Fatalf("missing env dir should be nil error: %v", err)
	}
	if envs != nil {
		t.Errorf("expected nil envs, got %+v", envs)
	}
}

func TestFolderMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "payments")
	os.MkdirAll(sub, 0o755)
	meta := model.Collection{
		Path:        sub,
		Name:        "Payments",
		Color:       "#46a758",
		Tags:        []string{"team-billing", "critical"},
		Description: "money endpoints",
		Vars:        []model.KV{{Key: "baseUrl", Value: "https://pay.test", Enabled: true}},
		Auth:        model.Auth{Type: model.AuthBearer, Token: "{{payToken}}"},
	}
	if err := SaveCollection(meta); err != nil {
		t.Fatal(err)
	}
	got := ReadMeta(sub)
	if got.Color != meta.Color || got.Description != meta.Description {
		t.Errorf("scalar mismatch: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "team-billing" {
		t.Errorf("tags not preserved: %+v", got.Tags)
	}
	if got.Auth.Type != model.AuthBearer || got.Auth.Token != "{{payToken}}" {
		t.Errorf("auth not preserved: %+v", got.Auth)
	}
	if len(got.Vars) != 1 || got.Vars[0].Key != "baseUrl" {
		t.Errorf("vars not preserved: %+v", got.Vars)
	}
}

func TestProxyTLSRoundTrip(t *testing.T) {
	dir := t.TempDir()
	meta := model.Collection{
		Path:  dir,
		Name:  "Root",
		Proxy: "{{corpProxy}}",
		TLS:   model.TLSConfig{CertFile: "{{cert}}", KeyFile: "/k.pem", CAFile: "/ca.pem", Insecure: true},
	}
	if err := SaveCollection(meta); err != nil {
		t.Fatal(err)
	}
	got := ReadMeta(dir)
	if got.Proxy != "{{corpProxy}}" {
		t.Errorf("proxy not preserved: %q", got.Proxy)
	}
	if got.TLS != meta.TLS {
		t.Errorf("tls not preserved: %+v", got.TLS)
	}
}

func TestMigrateMovesRootConfigIntoSenda(t *testing.T) {
	root := t.TempDir()
	// Legacy layout: config sits at the collection root.
	os.WriteFile(filepath.Join(root, "senda.meta.yaml"), []byte("name: api\n"), 0o644)
	os.WriteFile(filepath.Join(root, "senda.secret.yaml"), []byte("vars:\n  - {key: apiKey, value: shh, enabled: true}\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "environments"), 0o755)
	os.WriteFile(filepath.Join(root, "environments", "dev.yaml"), []byte("name: dev\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "mocks"), 0o755)
	os.WriteFile(filepath.Join(root, "mocks", "m.yaml"), []byte("name: m\n"), 0o644)
	os.MkdirAll(filepath.Join(root, ".security"), 0o755)
	os.WriteFile(filepath.Join(root, ".security", "check.yaml"), []byte("id: c\n"), 0o644)
	// A sub-folder's inline meta must NOT be touched by migration.
	sub := filepath.Join(root, "users")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "senda.meta.yaml"), []byte("color: \"#fff\"\n"), 0o644)

	c, err := OpenCollection(root) // triggers Migrate
	if err != nil {
		t.Fatal(err)
	}

	// Config moved under .senda/, gone from the root.
	for _, moved := range []string{
		filepath.Join(root, ConfigDirName, "senda.meta.yaml"),
		filepath.Join(root, ConfigDirName, "senda.secret.yaml"),
		filepath.Join(root, ConfigDirName, "environments", "dev.yaml"),
		filepath.Join(root, ConfigDirName, "mocks", "m.yaml"),
		filepath.Join(root, ConfigDirName, "security", "check.yaml"),
	} {
		if !pathExists(moved) {
			t.Errorf("expected migrated file at %s", moved)
		}
	}
	for _, gone := range []string{
		filepath.Join(root, "senda.meta.yaml"),
		filepath.Join(root, "environments"),
		filepath.Join(root, "mocks"),
		filepath.Join(root, ".security"),
	} {
		if pathExists(gone) {
			t.Errorf("expected %s to be gone after migration", gone)
		}
	}
	// Sub-folder meta stays inline.
	if !pathExists(filepath.Join(sub, "senda.meta.yaml")) {
		t.Error("sub-folder senda.meta.yaml should be left in place")
	}

	// Metadata, environments and secrets still resolve after migration.
	if c.Name != "api" {
		t.Errorf("name = %q, want api", c.Name)
	}
	if envs, _ := ListEnvironments(root); len(envs) != 1 || envs[0].Name != "dev" {
		t.Errorf("env list after migration = %+v", envs)
	}
	if v := CollectionSecrets(root); len(v) != 1 || v[0].Key != "apiKey" {
		t.Errorf("collection secrets after migration = %+v", v)
	}
}

func TestSaveCollectionRootVsFolder(t *testing.T) {
	root := t.TempDir()
	Migrate(root) // marks root with .senda/

	if err := SaveCollection(model.Collection{Path: root, Name: "api"}); err != nil {
		t.Fatal(err)
	}
	if !pathExists(filepath.Join(root, ConfigDirName, "senda.meta.yaml")) {
		t.Error("root metadata should be written inside .senda/")
	}
	if pathExists(filepath.Join(root, "senda.meta.yaml")) {
		t.Error("root metadata should not be written at the collection root")
	}

	sub := filepath.Join(root, "users")
	os.MkdirAll(sub, 0o755)
	if err := SaveCollection(model.Collection{Path: sub, Name: "users"}); err != nil {
		t.Fatal(err)
	}
	if !pathExists(filepath.Join(sub, "senda.meta.yaml")) {
		t.Error("sub-folder metadata should be written inline")
	}
	if ReadMeta(sub).Name != "users" || ReadMeta(root).Name != "api" {
		t.Error("metadata did not round-trip for root/folder")
	}
}

func TestReadMetaMissingDefaultsName(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "no-meta")
	os.MkdirAll(sub, 0o755)
	if got := ReadMeta(sub); got.Name != "no-meta" || got.Color != "" {
		t.Errorf("missing meta should default name and leave color empty: %+v", got)
	}
}

func TestBuildTreePopulatesFolderColor(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "users")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "senda.meta.yaml"), []byte("color: \"#e5484d\"\ntags: [auth]\n"), 0o644)
	os.WriteFile(filepath.Join(sub, "list.yaml"), []byte("name: l"), 0o644)
	c, err := OpenCollection(root)
	if err != nil {
		t.Fatal(err)
	}
	folder := c.Tree.Children[0]
	if folder.Color != "#e5484d" || len(folder.Tags) != 1 || folder.Tags[0] != "auth" {
		t.Errorf("folder color/tags not surfaced on tree node: %+v", folder)
	}
}

func TestFolderChain(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	os.MkdirAll(deep, 0o755)
	reqPath := filepath.Join(deep, "req.yaml")

	chain := FolderChain(root, reqPath)
	if len(chain) != 2 {
		t.Fatalf("want 2 ancestors, got %d: %+v", len(chain), chain)
	}
	if filepath.Base(chain[0]) != "a" || filepath.Base(chain[1]) != "b" {
		t.Errorf("chain should be root-first (a then b): %+v", chain)
	}

	// request directly in root: no folder ancestors.
	if got := FolderChain(root, filepath.Join(root, "x.yaml")); got != nil {
		t.Errorf("root-level request should have empty chain, got %+v", got)
	}
	// empty inputs.
	if got := FolderChain("", reqPath); got != nil {
		t.Errorf("empty collPath should yield nil, got %+v", got)
	}
	// reqPath outside collPath.
	if got := FolderChain(root, filepath.Join(t.TempDir(), "y.yaml")); got != nil {
		t.Errorf("out-of-tree request should yield nil, got %+v", got)
	}
}
