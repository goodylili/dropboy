package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/goodylili/dropboy/internal/config"
)

func newAddCmd() *cobra.Command {
	var excludes []string
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Register a folder to watch and sync.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			p, err := absExpand(args[0])
			if err != nil {
				return err
			}
			if st, err := os.Stat(p); err != nil {
				return die("cannot read %s: %v", p, err)
			} else if !st.IsDir() {
				return die("%s is not a directory", p)
			}
			for _, f := range cfg.Folders {
				if f.Path == p {
					return die("folder already watched: %s", p)
				}
			}
			cfg.Folders = append(cfg.Folders, config.Folder{Path: p, Exclude: excludes})
			if err := config.Save(cfg); err != nil {
				return err
			}
			info(cmd, "watching %s", p)
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&excludes, "exclude", "e", nil, "glob to exclude (repeatable)")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <path>",
		Aliases: []string{"rm"},
		Short:   "Stop watching a folder (does not delete remote data).",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			p, err := absExpand(args[0])
			if err != nil {
				return err
			}
			next := cfg.Folders[:0]
			removed := false
			for _, f := range cfg.Folders {
				if f.Path == p {
					removed = true
					continue
				}
				next = append(next, f)
			}
			if !removed {
				return die("not watching: %s", p)
			}
			cfg.Folders = next
			if err := config.Save(cfg); err != nil {
				return err
			}
			info(cmd, "stopped watching %s", p)
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List watched folders.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Folders) == 0 {
				info(cmd, "no folders watched yet — add one with `dropboy add <path>`")
				return nil
			}
			out := cmd.OutOrStdout()
			for _, f := range cfg.Folders {
				ex := ""
				if len(f.Exclude) > 0 {
					ex = "  exclude=" + strings.Join(f.Exclude, ",")
				}
				fmt.Fprintf(out, "%s%s\n", f.Path, ex)
			}
			return nil
		},
	}
}

func absExpand(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return filepath.Abs(p)
}
