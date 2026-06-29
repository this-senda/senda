package secretcrypt

import (
	"bytes"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("vars:\n- key: TOKEN\n  value: s3cr3t\n")

	env, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !IsEnvelope(env) {
		t.Fatal("sealed output not detected as envelope")
	}
	if bytes.Contains(env, []byte("s3cr3t")) {
		t.Fatal("plaintext leaked into envelope")
	}

	got, err := Open(key, env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round-trip mismatch: %q != %q", got, plain)
	}
}

func TestOpenWrongKeyFails(t *testing.T) {
	k1, _ := GenerateKey()
	k2, _ := GenerateKey()
	env, err := Seal(k1, []byte("vars: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(k2, env); err == nil {
		t.Fatal("expected GCM auth failure with wrong key, got nil")
	}
}

func TestIsEnvelopePlaintext(t *testing.T) {
	if IsEnvelope([]byte("vars:\n- key: A\n  value: b\n")) {
		t.Fatal("legacy plaintext misdetected as envelope")
	}
}
