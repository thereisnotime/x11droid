package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBg      = lipgloss.Color("#1a1a2e")
	colorSurface = lipgloss.Color("#16213e")
	colorBorder  = lipgloss.Color("#0f3460")
	colorAccent  = lipgloss.Color("#e94560")
	colorGreen   = lipgloss.Color("#4ade80")
	colorRed     = lipgloss.Color("#f87171")
	colorYellow  = lipgloss.Color("#facc15")
	colorMuted   = lipgloss.Color("#64748b")
	colorText    = lipgloss.Color("#e2e8f0")
	colorSubtext = lipgloss.Color("#94a3b8")

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			Background(colorSurface).
			Padding(0, 2)

	styleFooter = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(colorSurface).
			Padding(0, 2)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	styleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			Background(colorBorder).
			Padding(0, 1)

	styleNormal = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Padding(0, 1)

	styleRunning = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	styleStopped = lipgloss.NewStyle().
			Foreground(colorRed)

	styleInProgress = lipgloss.NewStyle().
			Foreground(colorYellow)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleLabel = lipgloss.NewStyle().
			Foreground(colorSubtext)

	styleValue = lipgloss.NewStyle().
			Foreground(colorText)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	styleAction = lipgloss.NewStyle().
			Foreground(colorText).
			Padding(0, 1)

	styleActionSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent).
				Background(colorBorder).
				Padding(0, 1)

	styleError = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorGreen)

	styleInput = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorSurface).
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	styleInputFocused = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorSurface).
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorAccent).
				Padding(0, 1)
)

func statusStyle(status string) lipgloss.Style {
	switch {
	case len(status) >= 2 && status[:2] == "Up":
		return styleRunning
	case status == "running":
		return styleRunning
	case status == "exited", status == "stopped":
		return styleStopped
	case len(status) >= 6 && status[:6] == "Exited":
		return styleStopped
	default:
		return styleInProgress
	}
}
