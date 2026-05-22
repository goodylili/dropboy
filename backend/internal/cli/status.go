package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
)

type apiStatus struct {
	Running        bool   `json:"running"`
	Locked         bool   `json:"locked"`
	Paused         bool   `json:"paused"`
	QueueUploads   int    `json:"queueUploads"`
	QueueDownloads int    `json:"queueDownloads"`
	BytesUp        int64  `json:"bytesUp"`
	BytesDown      int64  `json:"bytesDown"`
	Conflicts      int    `json:"conflicts"`
	LastSyncAt     string `json:"lastSyncAt"`
	Bucket         string `json:"bucket"`
	Region         string `json:"region"`
	MachineID      string `json:"machineId"`
}

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

			st, err := fetchDaemonStatus(cfg)
			if err != nil {
				fmt.Fprintf(out, "daemon:     not running (%v)\n", err)
				return nil
			}
			daemonLine := "running"
			if st.Locked {
				daemonLine = "running · locked (no passphrase)"
			} else if st.Paused {
				daemonLine = "running · paused"
			}
			fmt.Fprintf(out, "daemon:     %s\n", daemonLine)
			fmt.Fprintf(out, "queue:      %d up / %d down\n", st.QueueUploads, st.QueueDownloads)
			fmt.Fprintf(out, "transfer:   %s up / %s down\n", humanBytes(st.BytesUp), humanBytes(st.BytesDown))
			fmt.Fprintf(out, "conflicts:  %d\n", st.Conflicts)
			if st.LastSyncAt != "" {
				fmt.Fprintf(out, "last sync:  %s\n", st.LastSyncAt)
			}
			return nil
		},
	}
}

func fetchDaemonStatus(cfg config.Config) (apiStatus, error) {
	port := cfg.UI.Port
	if port == 0 {
		port = 7777
	}
	tok, err := readUIToken()
	if err != nil {
		return apiStatus{}, fmt.Errorf("daemon not reachable")
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/v1/status", port), nil)
	if err != nil {
		return apiStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return apiStatus{}, fmt.Errorf("daemon not reachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return apiStatus{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var st apiStatus
	if err := json.Unmarshal(body, &st); err != nil {
		return apiStatus{}, err
	}
	return st, nil
}

func humanBytes(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%d B", n)
	case n < k*k:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/(k*k*k))
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
