//go:build darwin

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

type launchd struct {
	plistPath string
	logDir    string
}

func newLaunchd() (Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	logDir := filepath.Join(home, "Library", "Logs", "dropboy")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	return &launchd{plistPath: filepath.Join(dir, LaunchdLabel+".plist"), logDir: logDir}, nil
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTD/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.Binary}}</string>
        <string>start</string>
        <string>--foreground</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>ProcessType</key><string>Background</string>
    <key>StandardOutPath</key><string>{{.LogDir}}/dropboy.log</string>
    <key>StandardErrorPath</key><string>{{.LogDir}}/dropboy.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key><string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin</string>
    </dict>
</dict>
</plist>
`

func (l *launchd) Install(binaryPath string) error {
	abs, err := filepath.Abs(binaryPath)
	if err != nil {
		return err
	}
	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"Label": LaunchdLabel, "Binary": abs, "LogDir": l.logDir,
	}); err != nil {
		return err
	}
	if err := os.WriteFile(l.plistPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	return l.bootstrap()
}

func (l *launchd) bootstrap() error {
	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)
	// `bootstrap` is idempotent-ish: try, fall back to `bootout` + retry.
	if out, err := exec.Command("launchctl", "bootstrap", domain, l.plistPath).CombinedOutput(); err != nil {
		// Already loaded → reload it.
		_ = exec.Command("launchctl", "bootout", domain, l.plistPath).Run()
		if out2, err2 := exec.Command("launchctl", "bootstrap", domain, l.plistPath).CombinedOutput(); err2 != nil {
			return fmt.Errorf("launchctl bootstrap: %w: %s | initial: %s", err2, strings.TrimSpace(string(out2)), strings.TrimSpace(string(out)))
		}
	}
	_ = exec.Command("launchctl", "enable", fmt.Sprintf("%s/%s", domain, LaunchdLabel)).Run()
	return nil
}

func (l *launchd) Uninstall() error {
	uid := os.Getuid()
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", uid), l.plistPath).Run()
	if err := os.Remove(l.plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (l *launchd) Start() error {
	uid := os.Getuid()
	out, err := exec.Command("launchctl", "kickstart", "-k",
		fmt.Sprintf("gui/%d/%s", uid, LaunchdLabel)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl kickstart: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (l *launchd) Stop() error {
	uid := os.Getuid()
	target := fmt.Sprintf("gui/%d/%s", uid, LaunchdLabel)
	out, err := exec.Command("launchctl", "kill", "SIGTERM", target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl kill: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (l *launchd) Restart() error {
	if err := l.Stop(); err != nil {
		// not fatal — continue to start
	}
	return l.Start()
}

func (l *launchd) Status() (string, error) {
	uid := os.Getuid()
	out, err := exec.Command("launchctl", "print", fmt.Sprintf("gui/%d/%s", uid, LaunchdLabel)).CombinedOutput()
	if err != nil {
		return "not loaded", nil
	}
	s := string(out)
	state := "loaded"
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "state =") {
			state = strings.TrimSpace(strings.TrimPrefix(line, "state ="))
			break
		}
	}
	return state, nil
}
