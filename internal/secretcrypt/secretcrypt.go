// Package secretcrypt encrypts Senda's local secret files at rest.
//
// Secret YAML (collection senda.secret.yaml, per-env *.secret.yaml) is sealed
// with AES-256-GCM into an envelope that is itself valid YAML, so the store's
// file detection, gitguard globs and tree-walk exclusion keep working unchanged:
//
//	enc: aes-256-gcm
//	nonce: <base64>
//	data: <base64 ciphertext>
//
// The 32-byte key never touches the collection on disk. It is resolved (for both
// seal and open) in this order, keyed by a per-collection opaque keyID:
//
//  1. SENDA_SECRET_KEY env var (base64 key) — explicit override for CI/servers.
//  2. OS keychain (service "senda", account = keyID) — transparent on desktop.
//  3. ~/.config/senda/keys/<keyID>.key — headless fallback when no keychain.
//
// If none resolve, Open/ResolveKey return ErrLocked so callers can surface a
// clear "secrets locked" state instead of silently reading empty secrets.
package secretcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	keyringService = "senda"
	envKeyVar      = "SENDA_SECRET_KEY"
	encAlgo        = "aes-256-gcm"
)

// ErrLocked is returned when an encrypted secret file is read but no key can be
// resolved on this machine.
var ErrLocked = errors.New("secrets locked: no decryption key (set " + envKeyVar + ", unlock keychain, or import the key)")

// envelope is the on-disk shape of an encrypted secret file.
type envelope struct {
	Enc   string `yaml:"enc"`
	Nonce string `yaml:"nonce"`
	Data  string `yaml:"data"`
}

// IsEnvelope reports whether data is an encrypted secret file (vs legacy
// plaintext vars). Cheap prefix check, then a real parse to be safe.
func IsEnvelope(data []byte) bool {
	if !strings.Contains(string(data), "enc:") {
		return false
	}
	var e envelope
	return yaml.Unmarshal(data, &e) == nil && e.Enc == encAlgo && e.Data != ""
}

// GenerateKey returns a fresh random 32-byte AES-256 key.
func GenerateKey() ([32]byte, error) {
	var k [32]byte
	if _, err := io.ReadFull(rand.Reader, k[:]); err != nil {
		return k, err
	}
	return k, nil
}

// Seal encrypts plaintextYAML and marshals the envelope.
func Seal(key [32]byte, plaintextYAML []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintextYAML, nil)
	return yaml.Marshal(envelope{
		Enc:   encAlgo,
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		Data:  base64.StdEncoding.EncodeToString(ct),
	})
}

// Open decrypts an envelope produced by Seal.
func Open(key [32]byte, env []byte) ([]byte, error) {
	var e envelope
	if err := yaml.Unmarshal(env, &e); err != nil {
		return nil, err
	}
	if e.Enc != encAlgo {
		return nil, fmt.Errorf("unsupported secret encryption %q", e.Enc)
	}
	nonce, err := base64.StdEncoding.DecodeString(e.Nonce)
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(e.Data)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("bad nonce length")
	}
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed (wrong key?): %w", err)
	}
	return pt, nil
}

func newGCM(key [32]byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// ResolveKey finds the decryption key for keyID. Returns ErrLocked if none of
// the sources (env, keychain, key file) yield a key. source names which won.
func ResolveKey(keyID string) (key [32]byte, source string, err error) {
	if v := strings.TrimSpace(os.Getenv(envKeyVar)); v != "" {
		k, derr := decodeKey(v)
		if derr != nil {
			return key, "", fmt.Errorf("%s: %w", envKeyVar, derr)
		}
		return k, "env", nil
	}
	if v, kerr := keyring.Get(keyringService, keyID); kerr == nil && v != "" {
		k, derr := decodeKey(v)
		if derr != nil {
			return key, "", fmt.Errorf("keychain: %w", derr)
		}
		return k, "keychain", nil
	}
	if data, ferr := os.ReadFile(keyFilePath(keyID)); ferr == nil {
		k, derr := decodeKey(strings.TrimSpace(string(data)))
		if derr != nil {
			return key, "", fmt.Errorf("key file: %w", derr)
		}
		return k, "keyfile", nil
	}
	return key, "", ErrLocked
}

// StoreKey persists key for keyID, preferring the OS keychain and falling back
// to a 0600 key file when no keychain is available (headless).
func StoreKey(keyID string, key [32]byte) error {
	enc := base64.StdEncoding.EncodeToString(key[:])
	if err := keyring.Set(keyringService, keyID, enc); err == nil {
		return nil
	}
	path := keyFilePath(keyID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(enc+"\n"), 0o600)
}

// ExportKey returns the base64 key for keyID (to set SENDA_SECRET_KEY elsewhere).
func ExportKey(keyID string) (string, error) {
	k, _, err := ResolveKey(keyID)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(k[:]), nil
}

// ImportKey stores a base64 key (e.g. exported from another machine) for keyID.
func ImportKey(keyID, b64 string) error {
	k, err := decodeKey(strings.TrimSpace(b64))
	if err != nil {
		return err
	}
	return StoreKey(keyID, k)
}

func decodeKey(s string) ([32]byte, error) {
	var k [32]byte
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return k, fmt.Errorf("key is not valid base64: %w", err)
	}
	if len(raw) != 32 {
		return k, fmt.Errorf("key must be 32 bytes, got %d", len(raw))
	}
	copy(k[:], raw)
	return k, nil
}

func keyFilePath(keyID string) string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "senda", "keys", keyID+".key")
}
