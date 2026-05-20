//go:build linux

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type systemd struct {
	unitPath string
}

func newSystemd() (Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &systemd{unitPath: filepath.Join(dir, SystemdUnit+".service")}, nil
}

const unitTemplate = `[Unit]
Description=dropboy continuous S3 sync
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.Binary}} start --foreground
Restart=always
RestartSec=5
Environment=PATH=/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=default.target
`

func (s *systemd) Install(binaryPath string) error {
	abs, err := filepath.Abs(binaryPath)
	if err != nil {
		return err
	}
	tmpl, err := template.New("unit").Parse(unitTemplate)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"Binary": abs}); err != nil {
		return err
	}
	if err := os.WriteFile(s.unitPath, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := s.run("daemon-reload"); err != nil {
		return err
	}
	return s.run("enable", "--now", SystemdUnit+".service")
}

func (s *systemd) Uninstall() error {
	_ = s.run("disable", "--now", SystemdUnit+".service")
	if err := os.Remove(s.unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return s.run("daemon-reload")
}

func (s *systemd) Start() error   { return s.run("start", SystemdUnit+".service") }
func (s *systemd) Stop() error    { return s.run("stop", SystemdUnit+".service") }
func (s *systemd) Restart() error { return s.run("restart", SystemdUnit+".service") }

func (s *systemd) Status() (string, error) {
	out, err := exec.Command("systemctl", "--user", "is-active", SystemdUnit+".service").CombinedOutput()
	state := strings.TrimSpace(string(out))
	if state == "" {
		state = "unknown"
	}
	if err != nil {
		return state, nil // systemctl exits non-zero for inactive units
	}
	return state, nil
}

func (s *systemd) run(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
