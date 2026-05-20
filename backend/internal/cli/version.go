package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date.",
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "dropboy %s\n", version.String())
			if version.Commit != "" {
				fmt.Fprintf(out, "  commit: %s\n", version.Commit)
			}
			if version.Date != "" {
				fmt.Fprintf(out, "  built:  %s\n", version.Date)
			}
		},
	}
}
