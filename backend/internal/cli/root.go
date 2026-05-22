// Package cli wires the Cobra command tree for the dropboy binary.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/goodylili/dropboy/internal/logging"
	"github.com/goodylili/dropboy/internal/tui"
	"github.com/goodylili/dropboy/internal/version"
)

var (
	flagVerbose bool
	flagConfig  string
)

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "dropboy",
		Short: "Self-hosted cloud drive on top of your own AWS S3 bucket.",
		Long: strings.TrimSpace(`
dropboy continuously syncs the folders you choose to an S3 bucket you own.
It is iCloud Drive for people who'd rather hold their own keys.

Run with no arguments to enter the interactive shell.
`),
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logging.Setup(flagVerbose)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(cmd.OutOrStdout())
		},
	}

	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose (debug) logging to stderr")
	root.PersistentFlags().StringVar(&flagConfig, "config", "", "override config file path")

	viper.SetEnvPrefix("DROPBOY")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	root.SetVersionTemplate(fmt.Sprintf("dropboy %s\n", version.String()))

	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newListCmd(),
		newStatusCmd(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newUninstallCmd(),
		newSyncCmd(),
		newPauseCmd(),
		newResumeCmd(),
		newRestoreCmd(),
		newConflictsCmd(),
		newDoctorCmd(),
		newLogsCmd(),
		newRecoverCmd(),
		newUICmd(),
		newVersionCmd(),
	)

	return root
}

// Execute runs the root command.
func Execute() error {
	return newRoot().Execute()
}

// die is a convenience for "user-visible error and exit".
func die(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// info writes a one-line user-facing message to stdout.
func info(cmd *cobra.Command, format string, args ...any) {
	fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(fmt.Sprintf(format, args...), "\n"))
}

// _ pin os import in case downstream commands need it.
var _ = os.Stdout
