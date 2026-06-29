package store

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"senda/internal/model"
	"senda/internal/secretcrypt"
)

// Encrypted collection secrets round-trip through the store, and the on-disk
// file is an AES-GCM envelope (not plaintext). Uses the SENDA_SECRET_KEY env
// path so it runs headless without a keychain.
func TestEncryptedCollectionSecretsRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}

	key, _ := secretcrypt.GenerateKey()
	t.Setenv("SENDA_SECRET_KEY", base64.StdEncoding.EncodeToString(key[:]))

	if err := SaveCollection(model.Collection{Path: root, SecretsEncrypted: true, SecretsKeyID: "test-id"}); err != nil {
		t.Fatal(err)
	}

	vars := []model.KV{{Key: "TOKEN", Value: "s3cr3t"}}
	if err := SaveCollectionSecrets(root, vars); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(ConfigDir(root), secretFile))
	if err != nil {
		t.Fatal(err)
	}
	if !secretcrypt.IsEnvelope(raw) {
		t.Fatal("secret file is not encrypted on disk")
	}
	if strings.Contains(string(raw), "s3cr3t") {
		t.Fatal("plaintext secret leaked to disk")
	}

	got := CollectionSecrets(root)
	if len(got) != 1 || got[0].Key != "TOKEN" || got[0].Value != "s3cr3t" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// Writing a secret overlay drops a stale .secret.yml sibling, so enabling
// encryption can't leave a plaintext .yml orphan beside the .yaml envelope.
func TestWriteSecretsDropsYmlSibling(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	yml := filepath.Join(cfg, "senda.secret.yml")
	if err := os.WriteFile(yml, []byte("vars:\n- key: A\n  value: b\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SaveCollectionSecrets(root, []model.KV{{Key: "A", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(yml); !os.IsNotExist(err) {
		t.Fatal(".secret.yml orphan not removed")
	}
	if _, err := os.Stat(filepath.Join(cfg, secretFile)); err != nil {
		t.Fatalf(".secret.yaml not written: %v", err)
	}
}

// A plaintext (legacy) secret file still reads when encryption is off.
func TestPlaintextSecretsStillRead(t *testing.T) {
	root := t.TempDir()
	if err := SaveCollectionSecrets(root, []model.KV{{Key: "A", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(ConfigDir(root), secretFile))
	if secretcrypt.IsEnvelope(raw) {
		t.Fatal("expected plaintext, got envelope")
	}
	if got := CollectionSecrets(root); len(got) != 1 || got[0].Value != "b" {
		t.Fatalf("plaintext read failed: %+v", got)
	}
}
