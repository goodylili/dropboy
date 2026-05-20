package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
	"github.com/goodylili/dropboy/internal/daemon"
	"github.com/goodylili/dropboy/internal/service"
)

func newStartCmd() *cobra.Command {
	var foreground bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the dropboy daemon.",
		Long:  "Installs (if needed) and starts the dropboy service via launchd/systemd. Use --foreground to run inline (e.g. for debugging or when invoked by the service file itself).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if foreground {
				return runDaemonForeground(cmd)
			}
			mgr, err := service.New()
			if err != nil {
				info(cmd, "%v — falling back to foreground", err)
				return runDaemonForeground(cmd)
			}
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			if err := mgr.Install(exe); err != nil {
				return err
			}
			info(cmd, "service installed and started")
			return nil
		},
	}
	cmd.Flags().BoolVar(&foreground, "foreground", false, "run the daemon inline instead of via launchd/systemd")
	return cmd
}

func runDaemonForeground(cmd *cobra.Command) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	d := daemon.New(cfg, dir)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	info(cmd, "daemon running (Ctrl+C to stop)")
	return d.Run(ctx, daemon.Options{Passphrase: os.Getenv("DROPBOY_PASSPHRASE")})
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the dropboy daemon.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := service.New()
			if err != nil {
				return err
			}
			return mgr.Stop()
		},
	}
}

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the dropboy daemon.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := service.New()
			if err != nil {
				return err
			}
			return mgr.Restart()
		},
	}
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop the daemon and remove its launchd/systemd service file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := service.New()
			if err != nil {
				return err
			}
			return mgr.Uninstall()
		},
	}
}

func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause",
		Short: "Temporarily halt syncing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return callAPI("POST", "/api/v1/pause", nil)
		},
	}
}

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume syncing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return callAPI("POST", "/api/v1/resume", nil)
		},
	}
}

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [path]",
		Short: "Force an immediate reconciliation pass.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return callAPI("POST", "/api/v1/sync", nil)
		},
	}
}

func newLogsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail daemon logs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := logFilePath()
			if err != nil {
				return err
			}
			if follow {
				return tailFollow(cmd, path)
			}
			return tailOnce(cmd, path)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow new log lines")
	return cmd
}

// callAPI sends a request to the loopback daemon API using the session token
// at ~/.dropboy/ui.token. This lets CLI mutations target a running daemon
// rather than duplicating logic in-process.
func callAPI(method, path string, body []byte) error {
	cfg, _ := config.Load()
	port := cfg.UI.Port
	if port == 0 {
		port = 7777
	}
	tok, err := readUIToken()
	if err != nil {
		return err
	}
	return doAPIRequest(method, fmt.Sprintf("http://127.0.0.1:%d%s", port, path), tok, body)
}
