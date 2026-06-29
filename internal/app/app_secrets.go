package app

import (
	"crypto/rand"
	"encoding/hex"

	"senda/internal/gitguard"
	"senda/internal/model"
	"senda/internal/secretcrypt"
	"senda/internal/store"
)

// EncryptionStatus reports whether a collection's secret files are encrypted at
// rest, and whether a decryption key is reachable on this machine.
type EncryptionStatus struct {
	Enabled      bool   `json:"enabled"`
	KeyAvailable bool   `json:"keyAvailable"`
	Source       string `json:"source"` // env | keychain | keyfile | ""
}

// EncryptionStatus returns the secret-encryption state for a collection.
func (a *App) EncryptionStatus(collPath string) EncryptionStatus {
	m := store.ReadMeta(collPath)
	st := EncryptionStatus{Enabled: m.SecretsEncrypted}
	if m.SecretsEncrypted {
		if _, src, err := secretcrypt.ResolveKey(m.SecretsKeyID); err == nil {
			st.KeyAvailable = true
			st.Source = src
		}
	}
	return st
}

// EnableEncryption turns on at-rest encryption for a collection's secret files:
// it generates a key, stores it (keychain, else key file), records the opaque
// keyID in senda.meta.yaml, and re-writes every existing secret file as an
// AES-GCM envelope. Idempotent — a no-op if already enabled.
func (a *App) EnableEncryption(collPath string) error {
	m := store.ReadMeta(collPath)
	if m.SecretsEncrypted {
		return nil
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	keyID := hex.EncodeToString(id)

	// If the environment already supplies a key (SENDA_SECRET_KEY), seal with
	// THAT — otherwise the files would be sealed with the env key (ResolveKey
	// prefers it) while the keychain held a different generated one, silently
	// locking everything once the env var goes away. Persist it under keyID so
	// reads still resolve when the env var is absent.
	key, _, rerr := secretcrypt.ResolveKey(keyID)
	if rerr != nil {
		k, err := secretcrypt.GenerateKey()
		if err != nil {
			return err
		}
		key = k
	}
	if err := secretcrypt.StoreKey(keyID, key); err != nil {
		return err
	}

	m.SecretsKeyID = keyID
	m.SecretsEncrypted = true
	return a.rewriteSecrets(collPath, m)
}

// DisableEncryption decrypts the secret files back to plaintext and clears the
// encryption flag. Requires the key to be reachable. Idempotent.
func (a *App) DisableEncryption(collPath string) error {
	m := store.ReadMeta(collPath)
	if !m.SecretsEncrypted {
		return nil
	}
	m.SecretsEncrypted = false
	// Keep SecretsKeyID: the reader detects format from file CONTENT, not this
	// flag, so if a rewrite below is interrupted the still-encrypted files remain
	// decryptable via the retained key. A stale keyID over plaintext is harmless.
	return a.rewriteSecrets(collPath, m)
}

// rewriteSecrets reads every secret file with the CURRENT on-disk format, flips
// the collection meta to target, then re-writes each file in the new format.
// Reads happen first because store.Save* honours the persisted meta flag.
func (a *App) rewriteSecrets(collPath string, target model.Collection) error {
	coll := store.CollectionSecrets(collPath)
	envNames := store.SecretEnvNames(collPath)
	envVars := make(map[string][]model.KV, len(envNames))
	for _, n := range envNames {
		envVars[n] = store.EnvironmentSecrets(collPath, n)
	}

	if err := store.SaveCollection(target); err != nil {
		return err
	}
	if err := gitguard.WriteIgnore(collPath); err != nil {
		return err
	}
	if err := store.SaveCollectionSecrets(collPath, coll); err != nil {
		return err
	}
	for n, v := range envVars {
		if err := store.SaveEnvironmentSecrets(collPath, n, v); err != nil {
			return err
		}
	}
	return nil
}

// ExportKey returns the base64 decryption key for the collection so it can be
// set as SENDA_SECRET_KEY on a server or imported on another machine.
func (a *App) ExportKey(collPath string) (string, error) {
	return secretcrypt.ExportKey(store.ReadMeta(collPath).SecretsKeyID)
}

// ImportKey stores a base64 key for the collection (e.g. exported elsewhere) so
// the local machine can decrypt its secrets.
func (a *App) ImportKey(collPath, b64 string) error {
	return secretcrypt.ImportKey(store.ReadMeta(collPath).SecretsKeyID, b64)
}
