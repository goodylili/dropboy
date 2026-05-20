package cli

import (
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose config, AWS perms, clock skew, watcher limits.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			check(out, "OS / Arch", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), true)

			cfg, err := config.Load()
			if err != nil {
				check(out, "Config", err.Error(), false)
				return nil
			}
			check(out, "Config", "loaded", true)
			check(out, "Bucket", cfg.Bucket, cfg.Bucket != "")
			check(out, "Region", cfg.Region, cfg.Region != "")
			check(out, "Machine ID", cfg.MachineID, cfg.MachineID != "")
			check(out, "Folders", fmt.Sprintf("%d watched", len(cfg.Folders)), len(cfg.Folders) > 0)
			check(out, "AWS credentials", "verification pending (needs S3 client wiring)", false)
			check(out, "Daemon", "not running (daemon impl pending)", false)

			if runtime.GOOS == "linux" {
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Tip: bump inotify watcher limit with")
				fmt.Fprintln(out, "  sudo sysctl fs.inotify.max_user_watches=524288")
			}
			return nil
		},
	}
}

func check(w io.Writer, label, value string, ok bool) {
	mark := "✓"
	if !ok {
		mark = "•"
	}
	_, _ = fmt.Fprintf(w, "  %s %-18s %s\n", mark, label+":", value)
}
