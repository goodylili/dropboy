// Package keychain stores and retrieves secrets in the user's OS keychain.
//
// Implementation strategy: shell out to the platform's native CLI tool —
// `security` on macOS, `secret-tool` (libsecret) on Linux. This keeps the
// dropboy binary CGO-free (the static-binary goal in PRD §6) at the cost of
// requiring those tools to be present. If they aren't, callers fall back to
// asking the user for the passphrase each boot.
package keychain

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const Service = "com.dropboy"

// ErrUnavailable is returned when the platform keychain tool is missing.
var ErrUnavailable = errors.New("keychain not available on this platform")

// ErrNotFound is returned when the requested entry does not exist.
var ErrNotFound = errors.New("keychain entry not found")

// Set stores secret under (service, account). Overwrites any existing entry.
func Set(account, secret string) error {
	switch runtime.GOOS {
	case "darwin":
		return darwinSet(account, secret)
	case "linux":
		return linuxSet(account, secret)
	}
	return ErrUnavailable
}

// Get reads the secret for (service, account).
func Get(account string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return darwinGet(account)
	case "linux":
		return linuxGet(account)
	}
	return "", ErrUnavailable
}

// Delete removes the secret for (service, account). No-op if absent.
func Delete(account string) error {
	switch runtime.GOOS {
	case "darwin":
		return darwinDelete(account)
	case "linux":
		return linuxDelete(account)
	}
	return ErrUnavailable
}

// ---- darwin ----

func darwinSet(account, secret string) error {
	if _, err := exec.LookPath("security"); err != nil {
		return ErrUnavailable
	}
	// -U updates if the item already exists.
	cmd := exec.Command("security", "add-generic-password",
		"-a", account, "-s", Service, "-w", secret, "-U")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("security add-generic-password: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func darwinGet(account string) (string, error) {
	if _, err := exec.LookPath("security"); err != nil {
		return "", ErrUnavailable
	}
	cmd := exec.Command("security", "find-generic-password",
		"-a", account, "-s", Service, "-w")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "could not be found") {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("security find-generic-password: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

func darwinDelete(account string) error {
	if _, err := exec.LookPath("security"); err != nil {
		return ErrUnavailable
	}
	cmd := exec.Command("security", "delete-generic-password",
		"-a", account, "-s", Service)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "could not be found") {
			return nil
		}
		return fmt.Errorf("security delete-generic-password: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ---- linux ----

func linuxSet(account, secret string) error {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return ErrUnavailable
	}
	cmd := exec.Command("secret-tool", "store", "--label=dropboy "+account,
		"service", Service, "account", account)
	cmd.Stdin = strings.NewReader(secret)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("secret-tool store: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func linuxGet(account string) (string, error) {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return "", ErrUnavailable
	}
	cmd := exec.Command("secret-tool", "lookup", "service", Service, "account", account)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// secret-tool exits non-zero with empty output when no match.
		if out.Len() == 0 {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("secret-tool lookup: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

func linuxDelete(account string) error {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return ErrUnavailable
	}
	cmd := exec.Command("secret-tool", "clear", "service", Service, "account", account)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("secret-tool clear: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
