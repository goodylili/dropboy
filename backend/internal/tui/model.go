package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/goodylili/dropboy/internal/config"
)

type lineKind int

const (
	lineInput lineKind = iota
	lineOutput
	lineError
	lineInfo
)

type historyLine struct {
	kind lineKind
	text string
}

type model struct {
	input       textinput.Model
	history     []historyLine
	pastCmds    []string
	pastCursor  int // -1 = editing fresh
	suggest     string
	width       int
	height      int
	queueUp     int
	queueDown   int
	bandwidth   string
	paused      bool
	bucketLabel string
	quit        bool // set by /exit /quit; checked in Update
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "/help for commands"
	ti.Prompt = ""
	ti.Focus()
	ti.CharLimit = 1024
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(accent)

	cfg, err := config.Load()
	bucket := "not configured — /help"
	if err == nil && cfg.Bucket != "" {
		bucket = fmt.Sprintf("%s · %s · %s", cfg.Bucket, cfg.Region, cfg.MachineID)
	}

	return model{
		input:       ti,
		pastCursor:  -1,
		bandwidth:   "0 KB/s",
		bucketLabel: bucket,
		history: []historyLine{
			{kind: lineInfo, text: "Welcome. " + cfg.Bucket},
		},
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			line := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			m.suggest = ""
			if line == "" {
				return m, nil
			}
			m.pastCmds = append(m.pastCmds, line)
			m.pastCursor = -1
			next := m.handleLine(line)
			if next.quit {
				return next, tea.Quit
			}
			return next, nil
		case tea.KeyTab:
			if m.suggest != "" {
				m.input.SetValue(m.suggest)
				m.input.CursorEnd()
				m.suggest = ""
			}
			return m, nil
		case tea.KeyUp:
			if len(m.pastCmds) == 0 {
				return m, nil
			}
			if m.pastCursor == -1 {
				m.pastCursor = len(m.pastCmds) - 1
			} else if m.pastCursor > 0 {
				m.pastCursor--
			}
			m.input.SetValue(m.pastCmds[m.pastCursor])
			m.input.CursorEnd()
			return m, nil
		case tea.KeyDown:
			if m.pastCursor == -1 {
				return m, nil
			}
			m.pastCursor++
			if m.pastCursor >= len(m.pastCmds) {
				m.pastCursor = -1
				m.input.SetValue("")
				return m, nil
			}
			m.input.SetValue(m.pastCmds[m.pastCursor])
			m.input.CursorEnd()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.suggest = computeSuggestion(m.input.Value())
	return m, cmd
}

func (m model) handleLine(line string) model {
	m.history = append(m.history, historyLine{kind: lineInput, text: line})

	if !strings.HasPrefix(line, "/") {
		m.history = append(m.history, historyLine{
			kind: lineInfo,
			text: "I only speak in slash commands today. Try " + cmdStyle.Render("/help") + ".",
		})
		return m
	}

	fields := strings.Fields(line)
	name := fields[0]
	args := fields[1:]

	if name == "/exit" || name == "/quit" {
		m.history = append(m.history, historyLine{kind: lineInfo, text: "bye"})
		m.quit = true
		return m
	}
	if name == "/clear" {
		m.history = m.history[:0]
		return m
	}

	for _, c := range builtinCommands() {
		if c.name == name {
			if c.run == nil {
				return m
			}
			out, err := c.run(args)
			if err != nil {
				m.history = append(m.history, historyLine{kind: lineError, text: err.Error()})
				return m
			}
			m.history = append(m.history, historyLine{kind: lineOutput, text: out})
			return m
		}
	}

	m.history = append(m.history, historyLine{
		kind: lineError,
		text: "unknown command: " + name + "  (try " + cmdStyle.Render("/help") + ")",
	})
	return m
}

func computeSuggestion(prefix string) string {
	if !strings.HasPrefix(prefix, "/") || prefix == "/" {
		return ""
	}
	for _, c := range allCommands() {
		if strings.HasPrefix(c.name, prefix) && c.name != prefix {
			return c.name
		}
	}
	return ""
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	body := renderHistory(m.history, m.width)

	prompt := promptStyle.Render("›") + " " + m.input.View()
	if m.suggest != "" && strings.HasPrefix(m.suggest, m.input.Value()) {
		tail := strings.TrimPrefix(m.suggest, m.input.Value())
		prompt += hintStyle.Render(tail)
	}

	footer := hintStyle.Render("? for shortcuts · tab to autocomplete · esc to exit")
	status := m.statusLine()

	parts := []string{banner(m.width), ""}
	if body != "" {
		parts = append(parts, body, "")
	}
	parts = append(parts,
		boxStyle.Width(m.width-2).Render(prompt),
		footer,
		status,
	)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) statusLine() string {
	left := fmt.Sprintf(" %s ", m.bucketLabel)
	pauseTag := "running"
	if m.paused {
		pauseTag = warnStyle.Render("paused")
	}
	right := fmt.Sprintf("↑%d ↓%d · %s · %s ", m.queueUp, m.queueDown, m.bandwidth, pauseTag)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return statusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}
