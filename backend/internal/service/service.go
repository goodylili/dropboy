// Package service installs and controls the dropboy daemon as a user-level
// background service: launchd on macOS, systemd --user on Linux. The CLI's
// start/stop/restart subcommands delegate here so day-to-day operation
// matches PRD §5.6.
package service

// Label is the canonical label used by both launchd and systemd.
const (
	LaunchdLabel = "com.dropboy"
	SystemdUnit  = "dropboy"
)

// Manager is the cross-platform install/control surface.
type Manager interface {
	Install(binaryPath string) error
	Uninstall() error
	Start() error
	Stop() error
	Restart() error
	Status() (string, error)
}
