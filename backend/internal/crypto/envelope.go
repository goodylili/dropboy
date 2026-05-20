// Package crypto provides client-side encryption primitives for dropboy.
//
// File payloads are encrypted with AES-256-GCM. Each file uses a fresh
// data-encryption key (DEK), itself wrapped with the user's master key —
// classic envelope encryption (see PRD §5.3).
//
// The master key is held by the daemon at runtime; persistence to the OS
// keychain happens in the daemon package. This package is keychain-agnostic.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	KeySize   = 32 // 256-bit
	NonceSize = 12 // GCM standard
)

// GenerateKey returns a freshly generated 256-bit key.
func GenerateKey() ([]byte, error) {
	k := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return k, nil
}

// SealedDEK is a DEK encrypted under the master key.
type SealedDEK struct {
	Nonce      []byte
	Ciphertext []byte
}

// WrapKey encrypts the data-encryption key with the master key.
func WrapKey(masterKey, dek []byte) (SealedDEK, error) {
	ct, nonce, err := seal(masterKey, dek)
	if err != nil {
		return SealedDEK{}, fmt.Errorf("wrap key: %w", err)
	}
	return SealedDEK{Nonce: nonce, Ciphertext: ct}, nil
}

// UnwrapKey decrypts a sealed DEK with the master key.
func UnwrapKey(masterKey []byte, sealed SealedDEK) ([]byte, error) {
	dek, err := open(masterKey, sealed.Nonce, sealed.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("unwrap key: %w", err)
	}
	return dek, nil
}

// SealPayload encrypts a file payload with the DEK and returns ciphertext + nonce.
func SealPayload(dek, plaintext []byte) (ciphertext, nonce []byte, err error) {
	return seal(dek, plaintext)
}

// OpenPayload decrypts a file payload.
func OpenPayload(dek, nonce, ciphertext []byte) ([]byte, error) {
	return open(dek, nonce, ciphertext)
}

func seal(key, plaintext []byte) ([]byte, []byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("nonce: %w", err)
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return ct, nonce, nil
}

func open(key, nonce, ciphertext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return pt, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, errors.New("invalid key size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
