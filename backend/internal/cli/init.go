package cli

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
	dcrypto "github.com/goodylili/dropboy/internal/crypto"
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

	dataDir, err := config.Dir()
	if err != nil {
		return die("data dir: %v", err)
	}

	if dcrypto.HasMasterKey(dataDir) {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Encryption already initialized — keeping existing master key.")
	} else {
		passphrase, err := promptPassphrase(in, out)
		if err != nil {
			return err
		}

		recoveryCode, err := dcrypto.GenerateRecoveryCode()
		if err != nil {
			return die("generate recovery code: %v", err)
		}

		if _, err := dcrypto.CreateMasterKey(dataDir, passphrase, recoveryCode); err != nil {
			return die("create master key: %v", err)
		}

		showRecoveryCode(in, out, recoveryCode)
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

// promptPassphrase asks the user to generate or supply a passphrase. The
// returned string is what the master key will be sealed under.
func promptPassphrase(in *bufio.Reader, out io.Writer) (string, error) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Choose an encryption passphrase. It will be hashed with Argon2id and used to")
	fmt.Fprintln(out, "wrap a fresh master key. A recovery code is also generated so you can still")
	fmt.Fprintln(out, "decrypt your data if you ever forget the passphrase.")
	fmt.Fprintln(out)

	gen := strings.ToLower(prompt(in, out, "Generate a strong passphrase for you? [Y/n]", "Y", false))
	if gen == "" || strings.HasPrefix(gen, "y") {
		generated, err := generatePassphrase(20)
		if err != nil {
			return "", die("generate passphrase: %v", err)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  Generated passphrase: "+generated)
		fmt.Fprintln(out, "  ⚠ Copy it now to a password manager.")
		fmt.Fprintln(out)
		ack := strings.ToLower(prompt(in, out, "Type 'saved' to confirm you've stored it", "", true))
		if ack != "saved" {
			return "", die("aborted — passphrase not confirmed as saved")
		}
		return generated, nil
	}

	pass1 := prompt(in, out, "Passphrase (visible — for first-run setup only)", "", true)
	pass2 := prompt(in, out, "Confirm passphrase", "", true)
	if pass1 != pass2 {
		return "", die("passphrases do not match")
	}
	if len(pass1) < 12 {
		fmt.Fprintln(out, "  ⚠ Passphrase is shorter than 12 characters. Consider a longer one.")
	}
	return pass1, nil
}

func showRecoveryCode(in *bufio.Reader, out io.Writer, code string) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "──────────────────────────────────────────────────────────────")
	fmt.Fprintln(out, "  RECOVERY CODE")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "    "+code)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Store this somewhere safe (password manager, paper in a safe).")
	fmt.Fprintln(out, "  If you forget your passphrase, `dropboy recover` will use this code.")
	fmt.Fprintln(out, "  Losing BOTH the passphrase AND this code means losing the data.")
	fmt.Fprintln(out, "  This code is shown only once.")
	fmt.Fprintln(out, "──────────────────────────────────────────────────────────────")
	fmt.Fprintln(out)
	for {
		ack := strings.ToLower(prompt(in, out, "Type 'saved' to confirm you've stored the recovery code", "", true))
		if ack == "saved" {
			return
		}
	}
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

// generatePassphrase returns a cryptographically random string of n characters
// drawn from an unambiguous alphabet (no 0/O/1/l/I). 20 chars ≈ 100 bits entropy.
func generatePassphrase(n int) (string, error) {
	const alphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	max := big.NewInt(int64(len(alphabet)))
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[idx.Int64()]
	}
	return string(b), nil
}

func sanitizeMachineID(host string) string {
	host = strings.ToLower(host)
	host = strings.TrimSuffix(host, ".local")
	host = strings.ReplaceAll(host, " ", "-")
	return host
}
