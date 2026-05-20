package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
)

func readUIToken() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "ui.token"))
	if err != nil {
		return "", fmt.Errorf("read ui.token (is the daemon running?): %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func doAPIRequest(method, url, token string, body []byte) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("X-Dropboy-CSRF", token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: %s: %s", method, url, resp.Status, strings.TrimSpace(string(out)))
	}
	if len(out) > 0 {
		fmt.Println(strings.TrimSpace(string(out)))
	}
	return nil
}

// logFilePath returns the platform-conventional daemon log file.
func logFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "dropboy", "dropboy.log"), nil
	default:
		// systemd captures stdout/stderr; users typically `journalctl --user -u dropboy`.
		// For convenience, also write to ~/.dropboy/dropboy.log if present.
		return filepath.Join(home, ".dropboy", "dropboy.log"), nil
	}
}

func tailOnce(cmd *cobra.Command, path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			info(cmd, "no log file yet at %s — on Linux, try: journalctl --user -u dropboy", path)
			return nil
		}
		return err
	}
	defer f.Close()
	_, _ = io.Copy(cmd.OutOrStdout(), f)
	return nil
}

func tailFollow(cmd *cobra.Command, path string) error {
	for {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return err
		}
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			return err
		}
		r := bufio.NewReader(f)
		for {
			line, err := r.ReadString('\n')
			if len(line) > 0 {
				fmt.Fprint(cmd.OutOrStdout(), line)
			}
			if err == io.EOF {
				time.Sleep(300 * time.Millisecond)
				continue
			}
			if err != nil {
				_ = f.Close()
				return err
			}
		}
	}
}
