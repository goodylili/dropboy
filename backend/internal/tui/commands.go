package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/goodylili/dropboy/internal/config"
)

// Slash command registry for the interactive REPL.

type slashCmd struct {
	name string
	help string
	run  func(args []string) (string, error)
}

func builtinCommands() []slashCmd {
	cmds := []slashCmd{
		{
			name: "/help",
			help: "list available commands",
			run: func(args []string) (string, error) {
				var b strings.Builder
				b.WriteString(titleStyle.Render("Commands") + "\n")
				for _, c := range allCommands() {
					fmt.Fprintf(&b, "  %s  %s\n", cmdStyle.Render(c.name), hintStyle.Render(c.help))
				}
				b.WriteString("\n" + hintStyle.Render("Tip: tab to autocomplete, ↑/↓ for history, esc or /exit to quit"))
				return b.String(), nil
			},
		},
		{
			name: "/status",
			help: "show config + daemon status",
			run: func(args []string) (string, error) {
				cfg, err := config.Load()
				if err != nil {
					return "", err
				}
				return renderStatus(cfg), nil
			},
		},
		{
			name: "/list",
			help: "list watched folders",
			run: func(args []string) (string, error) {
				cfg, err := config.Load()
				if err != nil {
					return "", err
				}
				if len(cfg.Folders) == 0 {
					return hintStyle.Render("no folders watched yet — try /add <path>"), nil
				}
				var b strings.Builder
				for _, f := range cfg.Folders {
					fmt.Fprintf(&b, "  %s", argStyle.Render(f.Path))
					if len(f.Exclude) > 0 {
						fmt.Fprintf(&b, "  %s", hintStyle.Render("exclude="+strings.Join(f.Exclude, ",")))
					}
					b.WriteByte('\n')
				}
				return strings.TrimRight(b.String(), "\n"), nil
			},
		},
		{
			name: "/add",
			help: "watch a folder — /add <path>",
			run: func(args []string) (string, error) {
				if len(args) < 1 {
					return "", fmt.Errorf("usage: /add <path>")
				}
				cfg, err := config.Load()
				if err != nil {
					return "", err
				}
				cfg.Folders = append(cfg.Folders, config.Folder{Path: args[0]})
				if err := config.Save(cfg); err != nil {
					return "", err
				}
				return okStyle.Render("✓ watching " + args[0]), nil
			},
		},
		{
			name: "/remove",
			help: "stop watching a folder — /remove <path>",
			run: func(args []string) (string, error) {
				if len(args) < 1 {
					return "", fmt.Errorf("usage: /remove <path>")
				}
				cfg, err := config.Load()
				if err != nil {
					return "", err
				}
				next := cfg.Folders[:0]
				removed := false
				for _, f := range cfg.Folders {
					if f.Path == args[0] {
						removed = true
						continue
					}
					next = append(next, f)
				}
				if !removed {
					return "", fmt.Errorf("not watching: %s", args[0])
				}
				cfg.Folders = next
				if err := config.Save(cfg); err != nil {
					return "", err
				}
				return okStyle.Render("✓ removed " + args[0]), nil
			},
		},
		{
			name: "/sync",
			help: "force a sync pass (impl pending)",
			run: func(args []string) (string, error) {
				return warnStyle.Render("sync engine not yet implemented (M3)"), nil
			},
		},
		{
			name: "/conflicts",
			help: "list unresolved conflicts",
			run: func(args []string) (string, error) {
				return hintStyle.Render("no conflicts (sync engine pending)"), nil
			},
		},
		{
			name: "/doctor",
			help: "run preflight checks",
			run: func(args []string) (string, error) {
				return renderDoctor(), nil
			},
		},
		{
			name: "/ui",
			help: "print the local UI URL",
			run: func(args []string) (string, error) {
				cfg, _ := config.Load()
				port := cfg.UI.Port
				if port == 0 {
					port = 7777
				}
				return fmt.Sprintf("%s %s", okStyle.Render("→"), cmdStyle.Render(fmt.Sprintf("http://127.0.0.1:%d", port))), nil
			},
		},
		{
			name: "/clear",
			help: "clear the screen",
			run:  nil, // handled in update.go
		},
		{
			name: "/exit",
			help: "leave dropboy",
			run:  nil, // handled in update.go
		},
	}
	return cmds
}

func allCommands() []slashCmd {
	cmds := builtinCommands()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].name < cmds[j].name })
	return cmds
}

func renderStatus(cfg config.Config) string {
	var b strings.Builder
	row := func(k, v string) {
		fmt.Fprintf(&b, "  %s  %s\n", hintStyle.Render(fmt.Sprintf("%-10s", k)), argStyle.Render(v))
	}
	row("bucket", fallback(cfg.Bucket, "—"))
	row("region", fallback(cfg.Region, "—"))
	row("machine", fallback(cfg.MachineID, "—"))
	row("folders", fmt.Sprintf("%d", len(cfg.Folders)))
	row("encrypt", fmt.Sprintf("%s (%s)", cfg.Encryption.Scheme, cfg.Encryption.Keyring))
	row("daemon", warnStyle.Render("not running"))
	return strings.TrimRight(b.String(), "\n")
}

func renderDoctor() string {
	var b strings.Builder
	cfg, err := config.Load()
	if err != nil {
		b.WriteString(errorStyle.Render("• config: " + err.Error()))
		return b.String()
	}
	line := func(ok bool, label, val string) {
		mark := okStyle.Render("✓")
		if !ok {
			mark = warnStyle.Render("•")
		}
		fmt.Fprintf(&b, "  %s %-18s %s\n", mark, label+":", val)
	}
	line(true, "config", "loaded")
	line(cfg.Bucket != "", "bucket", fallback(cfg.Bucket, "missing"))
	line(cfg.Region != "", "region", fallback(cfg.Region, "missing"))
	line(cfg.MachineID != "", "machine id", fallback(cfg.MachineID, "missing"))
	line(len(cfg.Folders) > 0, "folders", fmt.Sprintf("%d watched", len(cfg.Folders)))
	line(false, "aws creds", "verification pending")
	line(false, "daemon", "not running")
	return strings.TrimRight(b.String(), "\n")
}

func fallback(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}
