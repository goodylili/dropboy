package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
	"github.com/goodylili/dropboy/internal/daemon"
)

func newUICmd() *cobra.Command {
	var port int
	var open bool
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Start the loopback web UI server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if port != 0 {
				cfg.UI.Port = port
			}
			if cfg.UI.Port == 0 {
				cfg.UI.Port = 7777
			}

			dir, err := config.Dir()
			if err != nil {
				return err
			}
			d := daemon.New(cfg, dir)

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			errc := make(chan error, 1)
			go func() {
				errc <- d.Run(ctx, daemon.Options{Passphrase: os.Getenv("DROPBOY_PASSPHRASE")})
			}()

			tok := waitForToken(d.Server())
			url := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", cfg.UI.Port, tok)
			info(cmd, "UI listening on http://127.0.0.1:%d", cfg.UI.Port)
			info(cmd, "open: %s", url)
			if open {
				if err := openBrowser(url); err != nil {
					return die("open browser: %v", err)
				}
			}

			select {
			case <-ctx.Done():
				return nil
			case err := <-errc:
				return err
			}
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "loopback port (default from config or 7777)")
	cmd.Flags().BoolVar(&open, "open", false, "open the URL in the default browser")
	return cmd
}

func waitForToken(srv interface{ Token() string }) string {
	for i := 0; i < 50; i++ {
		if t := srv.Token(); t != "" {
			return t
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ""
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
