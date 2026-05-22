package crypto

// keystore handles master-key persistence and KEK derivation.
//
// The master key is wrapped under two independent KEKs:
//   - master.key           — wrapped under Argon2id(passphrase, salt)
//   - master.recovery.key  — wrapped under Argon2id(recovery code, salt)
//
// Either secret can unwrap the master key, so the user can lose one without
// losing access to their data. Both files live under <data-dir>/.

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

const (
	keyFile         = "master.key"
	recoveryKeyFile = "master.recovery.key"
	saltSize        = 16
	argonTime       = 3
	argonMem        = 64 * 1024 // KiB
	argonThread     = 4

	recoveryCodeLen = 20
)

// Unambiguous alphabet for recovery codes (no 0/O/1/l/I).
const recoveryAlphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type sealedMaster struct {
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
	Time       uint32 `json:"time"`
	Memory     uint32 `json:"memory"`
	Threads    uint8  `json:"threads"`
}

// DeriveKEK turns any secret (passphrase or recovery code) + salt into a
// 32-byte key-encryption key.
func DeriveKEK(secret string, salt []byte) []byte {
	return argon2.IDKey([]byte(secret), salt, argonTime, argonMem, argonThread, KeySize)
}

// GenerateRecoveryCode returns a fresh recoveryCodeLen-char code drawn from
// recoveryAlphabet (~118 bits of entropy).
func GenerateRecoveryCode() (string, error) {
	max := big.NewInt(int64(len(recoveryAlphabet)))
	b := make([]byte, recoveryCodeLen)
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("recovery code: %w", err)
		}
		b[i] = recoveryAlphabet[idx.Int64()]
	}
	return string(b), nil
}

// CreateMasterKey generates a fresh master key, seals it under BOTH the
// passphrase-derived KEK and the recovery-code-derived KEK, writes both
// sealed blobs, and returns the plaintext master key for in-memory use.
func CreateMasterKey(dataDir, passphrase, recoveryCode string) ([]byte, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	master, err := GenerateKey()
	if err != nil {
		return nil, err
	}
	if err := sealAndWrite(filepath.Join(dataDir, keyFile), passphrase, master); err != nil {
		return nil, err
	}
	if err := sealAndWrite(filepath.Join(dataDir, recoveryKeyFile), recoveryCode, master); err != nil {
		return nil, err
	}
	return master, nil
}

// LoadMasterKey decrypts the passphrase-sealed master key.
func LoadMasterKey(dataDir, passphrase string) ([]byte, error) {
	return loadAndOpen(filepath.Join(dataDir, keyFile), passphrase)
}

// LoadMasterKeyWithRecovery decrypts the recovery-code-sealed master key.
func LoadMasterKeyWithRecovery(dataDir, recoveryCode string) ([]byte, error) {
	return loadAndOpen(filepath.Join(dataDir, recoveryKeyFile), recoveryCode)
}

// HasMasterKey reports whether the passphrase-sealed master-key file exists.
func HasMasterKey(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, keyFile))
	return err == nil
}

// HasRecoveryKey reports whether the recovery-sealed master-key file exists.
// Older installs (pre-recovery-code) won't have one.
func HasRecoveryKey(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, recoveryKeyFile))
	return err == nil
}

// AddRecoveryKey wraps an already-known master key under a new recovery code
// and writes master.recovery.key. Used to retro-fit recovery onto pre-existing
// installs that were initialized without one.
func AddRecoveryKey(dataDir, recoveryCode string, master []byte) error {
	return sealAndWrite(filepath.Join(dataDir, recoveryKeyFile), recoveryCode, master)
}

// ErrBadPassphrase is returned when the supplied secret doesn't decrypt the
// sealed master key (covers both passphrase and recovery-code paths).
var ErrBadPassphrase = errors.New("invalid passphrase or recovery code")

func sealAndWrite(path, secret string, master []byte) error {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("salt: %w", err)
	}
	kek := DeriveKEK(secret, salt)
	ct, nonce, err := seal(kek, master)
	if err != nil {
		return err
	}
	blob := sealedMaster{
		Salt: salt, Nonce: nonce, Ciphertext: ct,
		Time: argonTime, Memory: argonMem, Threads: argonThread,
	}
	data, err := json.Marshal(blob)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func loadAndOpen(path, secret string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read sealed key: %w", err)
	}
	var blob sealedMaster
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, fmt.Errorf("decode sealed key: %w", err)
	}
	kek := argon2.IDKey([]byte(secret), blob.Salt, blob.Time, blob.Memory, blob.Threads, KeySize)
	master, err := open(kek, blob.Nonce, blob.Ciphertext)
	if err != nil {
		return nil, ErrBadPassphrase
	}
	return master, nil
}
