package app

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"senda/internal/model"
	"senda/internal/secretcrypt"
)

// EnableEncryption seals every secret file and decrypts transparently on read;
// DisableEncryption reverses it. keyring.MockInit keeps the OS keychain in-memory
// so the test is hermetic (no env var, no real keychain, no home-dir key file).
func TestEnableDisableEncryptionRoundTrip(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate the key-file fallback (Linux)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".senda"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := NewApp()

	if err := a.SaveCollectionSecrets(root, []model.KV{{Key: "TOKEN", Value: "s3cr3t", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveEnvironmentSecrets(root, "dev", []model.KV{{Key: "DB", Value: "pw", Enabled: true}}); err != nil {
		t.Fatal(err)
	}

	if err := a.EnableEncryption(root); err != nil {
		t.Fatal(err)
	}
	if st := a.EncryptionStatus(root); !st.Enabled || !st.KeyAvailable {
		t.Fatalf("status after enable = %+v", st)
	}

	collFile := filepath.Join(root, ".senda", "senda.secret.yaml")
	raw, err := os.ReadFile(collFile)
	if err != nil {
		t.Fatal(err)
	}
	if !secretcrypt.IsEnvelope(raw) {
		t.Fatal("collection secret not encrypted on disk")
	}
	if strings.Contains(string(raw), "s3cr3t") {
		t.Fatal("plaintext leaked to disk")
	}

	if got := a.ReadCollectionSecrets(root); len(got) != 1 || got[0].Value != "s3cr3t" {
		t.Fatalf("collection round-trip = %+v", got)
	}
	if got := a.ReadEnvironmentSecrets(root, "dev"); len(got) != 1 || got[0].Value != "pw" {
		t.Fatalf("env round-trip = %+v", got)
	}

	if err := a.DisableEncryption(root); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(collFile)
	if secretcrypt.IsEnvelope(raw) {
		t.Fatal("still encrypted after disable")
	}
	if got := a.ReadCollectionSecrets(root); len(got) != 1 || got[0].Value != "s3cr3t" {
		t.Fatalf("collection plaintext round-trip = %+v", got)
	}
}

// When SENDA_SECRET_KEY is set during enable, files must be sealed with THAT key
// and the same key persisted to the keychain — so reads still work once the env
// var is gone. Guards against the seal-key / stored-key mismatch.
func TestEnableEncryptionUsesEnvKey(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var k [32]byte // deterministic zero key is a valid 32-byte AES key
	t.Setenv("SENDA_SECRET_KEY", base64.StdEncoding.EncodeToString(k[:]))
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".senda"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	if err := a.SaveCollectionSecrets(root, []model.KV{{Key: "K", Value: "v", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	if err := a.EnableEncryption(root); err != nil {
		t.Fatal(err)
	}

	// Drop the env var: decryption must now fall back to the keychain, which has
	// to hold the very key we sealed with.
	os.Unsetenv("SENDA_SECRET_KEY")
	if got := a.ReadCollectionSecrets(root); len(got) != 1 || got[0].Value != "v" {
		t.Fatalf("env/keychain key mismatch after enable: %+v", got)
	}
}

// A locked collection (encrypted file, key gone) reads empty rather than crashing
// — the UI relies on this plus EncryptionStatus to block destructive saves.
func TestLockedReadIsEmptyNotPanic(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate the key-file fallback (Linux)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".senda"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	_ = a.SaveCollectionSecrets(root, []model.KV{{Key: "K", Value: "v", Enabled: true}})
	if err := a.EnableEncryption(root); err != nil {
		t.Fatal(err)
	}

	// Wipe the key: fresh in-memory keychain, no env, no key file.
	keyring.MockInit()
	if st := a.EncryptionStatus(root); !st.Enabled || st.KeyAvailable {
		t.Fatalf("expected locked status, got %+v", st)
	}
	if got := a.ReadCollectionSecrets(root); len(got) != 0 {
		t.Fatalf("locked read should be empty, got %+v", got)
	}
}
