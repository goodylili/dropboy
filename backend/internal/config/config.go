// Package config loads and saves the user's dropboy configuration.
//
// The on-disk format is YAML at ~/.dropboy/config.yaml. The structure mirrors
// the example in PRD §5.7.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DirName  = ".dropboy"
	FileName = "config.yaml"
)

type Config struct {
	Bucket     string      `yaml:"bucket"`
	Region     string      `yaml:"region"`
	AWSProfile string      `yaml:"aws_profile"`
	MachineID  string      `yaml:"machine_id"`
	Encryption Encryption  `yaml:"encryption"`
	Folders    []Folder    `yaml:"folders"`
	Limits     Limits      `yaml:"limits"`
	Poll       Poll        `yaml:"poll"`
	UI         UISettings  `yaml:"ui"`
}

type Encryption struct {
	Scheme  string `yaml:"scheme"`  // aes-256-gcm
	Keyring string `yaml:"keyring"` // os | file
}

type Folder struct {
	Path    string   `yaml:"path"`
	Exclude []string `yaml:"exclude,omitempty"`
}

type Limits struct {
	MaxUploadMbps    int `yaml:"max_upload_mbps"`
	MaxConcurrent    int `yaml:"max_concurrent"`
	DeleteGraceHours int `yaml:"delete_grace_hours"`
}

type Poll struct {
	RemoteSeconds    int `yaml:"remote_seconds"`
	FullScanMinutes  int `yaml:"full_scan_minutes"`
}

type UISettings struct {
	Port int `yaml:"port"`
}

func Default() Config {
	return Config{
		Region:     "us-east-1",
		AWSProfile: "default",
		Encryption: Encryption{Scheme: "aes-256-gcm", Keyring: "os"},
		Limits:     Limits{MaxUploadMbps: 0, MaxConcurrent: 4, DeleteGraceHours: 24},
		Poll:       Poll{RemoteSeconds: 60, FullScanMinutes: 15},
		UI:         UISettings{Port: 7777},
	}
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DirName), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

func Load() (Config, error) {
	p, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, ErrNotInitialized
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func Save(cfg Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	p := filepath.Join(dir, FileName)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

var ErrNotInitialized = errors.New("dropboy is not initialized — run `dropboy init`")
