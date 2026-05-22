package tui

import "github.com/charmbracelet/lipgloss"

// Vercel-inspired monochrome palette + a single accent.
var (
	accent      = lipgloss.Color("#22C55E") // dropboy green — file/folder themed
	fg          = lipgloss.Color("#FAFAFA")
	dim         = lipgloss.Color("#A1A1AA") // zinc-400
	dimmer      = lipgloss.Color("#71717A") // zinc-500
	border      = lipgloss.Color("#22C55E") // green border to match welcome box
	successFg   = lipgloss.Color("#10B981")
	warningFg   = lipgloss.Color("#F59E0B")
	dangerFg    = lipgloss.Color("#EF4444")

	titleStyle    = lipgloss.NewStyle().Foreground(fg).Bold(true)
	subtitleStyle = lipgloss.NewStyle().Foreground(dim)
	promptStyle   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	hintStyle     = lipgloss.NewStyle().Foreground(dimmer)
	cmdStyle      = lipgloss.NewStyle().Foreground(accent)
	argStyle      = lipgloss.NewStyle().Foreground(fg)
	errorStyle    = lipgloss.NewStyle().Foreground(dangerFg)
	okStyle       = lipgloss.NewStyle().Foreground(successFg)
	warnStyle     = lipgloss.NewStyle().Foreground(warningFg)
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3F3F46")).
			Padding(0, 1)
	welcomeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1, 2)
	sectionTitleStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	mascotStyle       = lipgloss.NewStyle().Foreground(accent)
	statusBarStyle = lipgloss.NewStyle().
			Foreground(dim).
			Background(lipgloss.Color("#18181B")).
			Padding(0, 1)
)
