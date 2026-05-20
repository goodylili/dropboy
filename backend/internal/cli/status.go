package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon health, queue depth, last sync time, conflicts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "bucket:     %s (%s)\n", cfg.Bucket, cfg.Region)
			fmt.Fprintf(out, "machine:    %s\n", cfg.MachineID)
			fmt.Fprintf(out, "folders:    %d\n", len(cfg.Folders))
			fmt.Fprintf(out, "encryption: %s (%s)\n", cfg.Encryption.Scheme, cfg.Encryption.Keyring)
			fmt.Fprintln(out, "daemon:     not running (daemon impl pending)")
			return nil
		},
	}
}

func newConflictsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "conflicts",
		Short: "List unresolved conflicts and resolve interactively.",
		RunE: func(cmd *cobra.Command, args []string) error {
			info(cmd, "no conflicts (sync engine pending)")
			return nil
		},
	}
}

func newRestoreCmd() *cobra.Command {
	var machine, into string
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Pull a remote machine's tree onto this device.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if machine == "" {
				return die("--machine is required")
			}
			info(cmd, "restore not yet implemented (machine=%s into=%s)", machine, into)
			return nil
		},
	}
	cmd.Flags().StringVar(&machine, "machine", "", "machine ID to restore from")
	cmd.Flags().StringVar(&into, "into", "", "destination directory (defaults to original paths)")
	_ = cmd.MarkFlagRequired("machine")
	return cmd
}
