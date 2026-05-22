package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newRecoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recover",
		Short: "Unlock the daemon with your recovery code (use when the passphrase is lost).",
		Long: `Recover unlocks dropboy using the recovery code that was printed once
during 'dropboy init'. The recovery code is an independent secret that wraps
the same master key as the passphrase, so either can unlock the engine.

This unlocks the engine for the current daemon session. To restore normal
boot, either set a new passphrase via the UI or re-run 'dropboy init' (the
existing master key is preserved).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in := bufio.NewReader(os.Stdin)
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Enter the 20-character recovery code printed at init.")
			code := strings.TrimSpace(prompt(in, out, "Recovery code", "", true))
			body, _ := json.Marshal(map[string]string{"code": code})
			if err := callAPI("POST", "/api/v1/recover", body); err != nil {
				return err
			}
			fmt.Fprintln(out, "✓ unlocked via recovery code. Engine is running for this session.")
			fmt.Fprintln(out, "  To restore seamless boot, set a new passphrase via the UI.")
			return nil
		},
	}
}
