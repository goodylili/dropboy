package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
)

func newInitCmd() *cobra.Command {
	var nonInteractive bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up dropboy: bucket, region, AWS profile, machine ID, encryption passphrase.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrDefault()
			if err != nil {
				return err
			}
			if nonInteractive {
				return config.Save(cfg)
			}
			return runInteractiveInit(cmd, &cfg)
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "write defaults without prompting")
	return cmd
}

func loadOrDefault() (config.Config, error) {
	cfg, err := config.Load()
	if err == nil {
		return cfg, nil
	}
	if err == config.ErrNotInitialized {
		return config.Default(), nil
	}
	return config.Default(), err
}

func runInteractiveInit(cmd *cobra.Command, cfg *config.Config) error {
	in := bufio.NewReader(os.Stdin)
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "Welcome to dropboy. We'll set up your bucket and encryption.")
	fmt.Fprintln(out)

	cfg.Bucket = prompt(in, out, "S3 bucket name", cfg.Bucket, true)
	cfg.Region = prompt(in, out, "AWS region", cfg.Region, false)
	cfg.AWSProfile = prompt(in, out, "AWS profile (from ~/.aws/credentials)", cfg.AWSProfile, false)

	host, _ := os.Hostname()
	if cfg.MachineID == "" {
		cfg.MachineID = sanitizeMachineID(host)
	}
	cfg.MachineID = prompt(in, out, "Machine ID (used in S3 layout)", cfg.MachineID, true)

	// Passphrase: we won't store it. We just confirm the user has chosen one.
	// Real keychain integration lives in the daemon — see PRD §5.3.
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Choose an encryption passphrase. It will be hashed with Argon2id and the resulting")
	fmt.Fprintln(out, "master key will be saved in your OS keychain when the daemon first starts.")
	fmt.Fprintln(out, "If you forget it AND lose your keychain entry, your data is unrecoverable.")
	fmt.Fprintln(out)

	pass1 := prompt(in, out, "Passphrase (visible — for first-run setup only)", "", true)
	pass2 := prompt(in, out, "Confirm passphrase", "", true)
	if pass1 != pass2 {
		return die("passphrases do not match")
	}
	if len(pass1) < 12 {
		fmt.Fprintln(out, "  ⚠ Passphrase is shorter than 12 characters. Consider a longer one.")
	}

	if err := config.Save(*cfg); err != nil {
		return err
	}

	p, _ := config.Path()
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Saved %s\n", p)
	fmt.Fprintln(out, "Next: add a folder to watch — `dropboy add ~/Documents`")
	return nil
}

func prompt(in *bufio.Reader, writer io.Writer, label, def string, required bool) string {
	for {
		if def != "" {
			fmt.Fprintf(writer, "%s [%s]: ", label, def)
		} else {
			fmt.Fprintf(writer, "%s: ", label)
		}
		line, err := in.ReadString('\n')
		if err != nil {
			return def
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if def != "" || !required {
				return def
			}
			fmt.Fprintln(writer, "  required.")
			continue
		}
		return line
	}
}


func sanitizeMachineID(host string) string {
	host = strings.ToLower(host)
	host = strings.TrimSuffix(host, ".local")
	host = strings.ReplaceAll(host, " ", "-")
	return host
}
