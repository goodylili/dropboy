package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultHasSafeFallbacks(t *testing.T) {
	d := Default()
	if d.Region == "" {
		t.Error("Default.Region should be set")
	}
	if d.Encryption.Scheme != "aes-256-gcm" {
		t.Errorf("Default.Encryption.Scheme = %q, want aes-256-gcm", d.Encryption.Scheme)
	}
	if d.UI.Port == 0 {
		t.Error("Default.UI.Port should be non-zero")
	}
	if d.Limits.DeleteGraceHours == 0 {
		t.Error("Default.Limits.DeleteGraceHours should be non-zero")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := Default()
	cfg.Bucket = "test-bucket"
	cfg.MachineID = "test-machine"
	cfg.Folders = []Folder{{Path: "/tmp/x", Exclude: []string{".DS_Store"}}}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File should be 0600.
	p := filepath.Join(tmp, DirName, FileName)
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config perms = %v, want 0600", info.Mode().Perm())
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Bucket != "test-bucket" || got.MachineID != "test-machine" {
		t.Errorf("round trip lost fields: %+v", got)
	}
	if len(got.Folders) != 1 || got.Folders[0].Path != "/tmp/x" {
		t.Errorf("folder lost in round trip: %+v", got.Folders)
	}
}

func TestLoadMissingReturnsSentinel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := Load()
	if err != ErrNotInitialized {
		t.Errorf("want ErrNotInitialized, got %v", err)
	}
}
