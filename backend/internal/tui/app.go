// Package tui hosts the interactive REPL — the Claude-style shell that boots
// when the user runs `dropboy` with no arguments. It uses Bubble Tea.
package tui

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/goodylili/dropboy/internal/config"
	"github.com/goodylili/dropboy/internal/version"
)

// Run launches the interactive REPL. It writes nothing to w on a clean exit —
// the TUI takes over the terminal directly.
func Run(_ io.Writer) error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func banner(width int) string {
	left := leftPanel()
	right := rightPanel()

	gap := lipgloss.NewStyle().Width(4).Render("")
	inner := lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)

	box := welcomeBoxStyle
	if width > 4 {
		box = box.Width(width - 2)
	}
	return box.Render(inner)
}

func leftPanel() string {
	username := "there"
	if u, err := user.Current(); err == nil && u.Username != "" {
		username = u.Username
	}

	mascot := mascotStyle.Render(strings.Join([]string{
		"  ______   ",
		" /     /|  ",
		"/_____/ |  ",
		"|     | /  ",
		"|_____|/   ",
	}, "\n"))

	header := sectionTitleStyle.Render("dropboy ") + subtitleStyle.Render("v"+version.String())
	welcome := titleStyle.Render("Welcome back " + username + "!")

	modelLine := subtitleStyle.Render("self-hosted cloud drive · your bucket, your keys")

	cwd, _ := os.Getwd()
	cwd = compactPath(cwd)

	bucket := "not configured"
	if cfg, err := config.Load(); err == nil && cfg.Bucket != "" {
		bucket = cfg.Bucket + " · " + cfg.Region
	}

	bucketLine := hintStyle.Render("bucket: ") + argStyle.Render(bucket)
	cwdLine := hintStyle.Render(cwd)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		mascot,
		"",
		welcome,
		modelLine,
		"",
		bucketLine,
		cwdLine,
	)
}

func rightPanel() string {
	tipsTitle := sectionTitleStyle.Render("Tips for getting started")
	tips := strings.Join([]string{
		"Run " + cmdStyle.Render("/init") + " to scaffold a config",
		"Use " + cmdStyle.Render("/add <path>") + " to watch a folder",
		"Run " + cmdStyle.Render("/doctor") + " to verify your setup",
	}, "\n")

	whatsNewTitle := sectionTitleStyle.Render("What's new")
	whatsNew := strings.Join([]string{
		"Interactive shell with slash commands and tab-complete",
		"Envelope encryption (AES-256-GCM) for files at rest",
		"Local UI on " + cmdStyle.Render("http://127.0.0.1:7777"),
		hintStyle.Render("/help for the full command list"),
	}, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		tipsTitle,
		tips,
		"",
		whatsNewTitle,
		whatsNew,
	)
}

func compactPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func renderHistory(lines []historyLine, width int) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	for _, l := range lines {
		switch l.kind {
		case lineInput:
			fmt.Fprintf(&b, "%s %s\n", promptStyle.Render("›"), argStyle.Render(l.text))
		case lineOutput:
			b.WriteString(l.text + "\n")
		case lineError:
			b.WriteString(errorStyle.Render("✗ "+l.text) + "\n")
		case lineInfo:
			b.WriteString(hintStyle.Render(l.text) + "\n")
		}
	}
	_ = width
	return strings.TrimRight(b.String(), "\n")
}
