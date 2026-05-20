package crypto

import (
	"bytes"
	"testing"
)

func TestSealOpenPayloadRoundTrip(t *testing.T) {
	dek, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	plain := []byte("the quick brown fox jumps over the lazy dog")
	ct, nonce, err := SealPayload(dek, plain)
	if err != nil {
		t.Fatalf("SealPayload: %v", err)
	}
	if bytes.Equal(ct, plain) {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := OpenPayload(dek, nonce, ct)
	if err != nil {
		t.Fatalf("OpenPayload: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypt mismatch: %q vs %q", got, plain)
	}
}

func TestWrapUnwrapKey(t *testing.T) {
	master, _ := GenerateKey()
	dek, _ := GenerateKey()
	sealed, err := WrapKey(master, dek)
	if err != nil {
		t.Fatalf("WrapKey: %v", err)
	}
	got, err := UnwrapKey(master, sealed)
	if err != nil {
		t.Fatalf("UnwrapKey: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("DEK mismatch after unwrap")
	}
}

func TestUnwrapKeyWrongMasterFails(t *testing.T) {
	master, _ := GenerateKey()
	other, _ := GenerateKey()
	dek, _ := GenerateKey()
	sealed, _ := WrapKey(master, dek)
	if _, err := UnwrapKey(other, sealed); err == nil {
		t.Fatal("unwrap with wrong master should fail")
	}
}

func TestOpenTamperedCiphertextFails(t *testing.T) {
	dek, _ := GenerateKey()
	ct, nonce, _ := SealPayload(dek, []byte("hello"))
	ct[0] ^= 0xff
	if _, err := OpenPayload(dek, nonce, ct); err == nil {
		t.Fatal("tampered ciphertext should fail to open")
	}
}

func TestInvalidKeySize(t *testing.T) {
	if _, err := newGCM(make([]byte, 16)); err == nil {
		t.Fatal("16-byte key should be rejected")
	}
}

func TestDeriveKEKDeterministic(t *testing.T) {
	salt := []byte("0123456789abcdef")
	k1 := DeriveKEK("hunter2", salt)
	k2 := DeriveKEK("hunter2", salt)
	if !bytes.Equal(k1, k2) {
		t.Fatal("DeriveKEK should be deterministic for same pass+salt")
	}
	k3 := DeriveKEK("hunter3", salt)
	if bytes.Equal(k1, k3) {
		t.Fatal("DeriveKEK should differ for different passphrase")
	}
}
