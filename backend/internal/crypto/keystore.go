package crypto

// keystore handles master-key persistence and passphrase-based derivation.
//
// PRD §5.3 calls for storing the master key in the OS keychain. To keep v1
// portable across macOS, Linux, and headless CI environments we instead
// persist a file at <data-dir>/master.key that contains:
//
//   - the Argon2id parameters (kdf salt + cost knobs)
//   - the master key sealed under a passphrase-derived KEK
//
// The OS keychain integration is an isolated backend swap once we add the
// platform-specific deps; the surface here (Derive / Load / Save) does not
// change.

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

const (
	keyFile     = "master.key"
	saltSize    = 16
	argonTime   = 3
	argonMem    = 64 * 1024 // KiB
	argonThread = 4
)

type sealedMaster struct {
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
	Time       uint32 `json:"time"`
	Memory     uint32 `json:"memory"`
	Threads    uint8  `json:"threads"`
}

// DeriveKEK turns a passphrase + salt into a 32-byte key-encryption key.
func DeriveKEK(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argonTime, argonMem, argonThread, KeySize)
}

// CreateMasterKey generates a fresh master key, seals it under the
// passphrase-derived KEK, and writes the sealed blob to <dataDir>/master.key.
// The plaintext master key is returned for the caller to hold in memory.
func CreateMasterKey(dataDir, passphrase string) ([]byte, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	master, err := GenerateKey()
	if err != nil {
		return nil, err
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}
	kek := DeriveKEK(passphrase, salt)
	ct, nonce, err := seal(kek, master)
	if err != nil {
		return nil, err
	}
	blob := sealedMaster{
		Salt: salt, Nonce: nonce, Ciphertext: ct,
		Time: argonTime, Memory: argonMem, Threads: argonThread,
	}
	data, err := json.Marshal(blob)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dataDir, keyFile), data, 0o600); err != nil {
		return nil, err
	}
	return master, nil
}

// LoadMasterKey reads and decrypts the master key with the passphrase.
func LoadMasterKey(dataDir, passphrase string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, keyFile))
	if err != nil {
		return nil, fmt.Errorf("read master key: %w", err)
	}
	var blob sealedMaster
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, fmt.Errorf("decode master key: %w", err)
	}
	kek := argon2.IDKey([]byte(passphrase), blob.Salt, blob.Time, blob.Memory, blob.Threads, KeySize)
	master, err := open(kek, blob.Nonce, blob.Ciphertext)
	if err != nil {
		return nil, ErrBadPassphrase
	}
	return master, nil
}

// HasMasterKey reports whether the master-key file exists.
func HasMasterKey(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, keyFile))
	return err == nil
}

// ErrBadPassphrase is returned when the supplied passphrase does not decrypt
// the master key.
var ErrBadPassphrase = errors.New("invalid passphrase")
