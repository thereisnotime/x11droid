package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/thereisnotime/x11droid/internal/kernel"
)

func renderMain(m Model) string {
	if m.loading {
		return lipgloss.NewStyle().Padding(2, 3).Foreground(colorMuted).Render("Loading instances...")
	}

	var sb strings.Builder

	if w := m.session.Warning(); w != "" {
		warning := lipgloss.NewStyle().
			Foreground(colorYellow).
			Background(lipgloss.Color("#2d2000")).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorYellow).
			Padding(0, 2).
			Width(m.width - 4).
			Render("⚠  " + w)
		sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Render(warning))
		sb.WriteString("\n")
	}

	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Bold(true).Foreground(colorSubtext).Render(
		fmt.Sprintf("Instances (%d)", len(m.instances)),
	))
	sb.WriteString("\n")

	if len(m.instances) == 0 {
		sb.WriteString(lipgloss.NewStyle().Padding(1, 3).Foreground(colorMuted).Render(
			"No instances found. Press n to create one.",
		))
		return sb.String()
	}

	for i, inst := range m.instances {
		name := padRight(inst.Name, 24)
		id := padRight(inst.ID, 14)
		status := statusLabel(inst.Status)
		image := styleMuted.Render(truncate(inst.Image, 24))

		row := fmt.Sprintf(" %s  %s  %s  %s",
			styleLabel.Render(name),
			styleMuted.Render(id),
			status,
			image,
		)

		if i == m.cursor {
			sb.WriteString(styleSelected.Width(m.width - 2).Render(row))
		} else {
			sb.WriteString(styleNormal.Width(m.width - 2).Render(row))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderDetail(m Model) string {
	if m.showLogs {
		return renderLogs(m)
	}

	inst := m.selected
	var sb strings.Builder

	header := lipgloss.NewStyle().Padding(1, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			styleValue.Bold(true).Render(inst.Name),
			styleLabel.Render("ID:     ")+styleValue.Render(inst.ID),
			styleLabel.Render("Image:  ")+styleValue.Render(inst.Image),
			styleLabel.Render("Status: ")+statusStyle(inst.Status).Render(inst.Status),
		),
	)
	sb.WriteString(header)
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(colorSubtext).Bold(true).Render("Actions"))
	sb.WriteString("\n")

	for i, action := range detailActions {
		var line string
		if i == m.actionCursor {
			line = styleActionSelected.Render("  " + action + "  ")
		} else {
			line = styleAction.Foreground(colorSubtext).Render("  " + action + "  ")
		}
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(line))
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderLogs(m Model) string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Bold(true).Foreground(colorSubtext).Render(
		fmt.Sprintf("Logs: %s  (any key to close)", m.selected.Name),
	))
	sb.WriteString("\n")

	lines := strings.Split(m.logs, "\n")
	maxLines := m.height - 6
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	for _, l := range lines {
		sb.WriteString(lipgloss.NewStyle().Padding(0, 3).Foreground(colorSubtext).Render(l))
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderSpawn(m Model) string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Bold(true).Foreground(colorSubtext).Render("New Instance"))
	sb.WriteString("\n\n")

	// Name field.
	nameLabel := styleLabel.Render("Name:  ")
	nameInput := m.spawnName
	if m.spawnCursor == 0 {
		nameInput = styleInputFocused.Render(padRight(nameInput+"_", 24))
	} else {
		nameInput = styleInput.Render(padRight(nameInput, 24))
	}
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(nameLabel + nameInput))
	sb.WriteString("\n\n")

	// GApps toggle.
	gappsLabel := styleLabel.Render("GApps: ")
	gappsVal := "[ ]"
	if m.spawnGApps {
		gappsVal = "[x]"
	}
	var gappsWidget string
	if m.spawnCursor == 1 {
		gappsWidget = styleActionSelected.Render(gappsVal + " (space to toggle)")
	} else {
		gappsWidget = styleAction.Foreground(colorSubtext).Render(gappsVal + " (space to toggle)")
	}
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(gappsLabel + gappsWidget))
	sb.WriteString("\n\n")

	// Submit button.
	var submitBtn string
	if m.spawnCursor == 2 {
		submitBtn = styleActionSelected.Render("  Spawn  ")
	} else {
		submitBtn = styleAction.Foreground(colorSubtext).Render("  Spawn  ")
	}
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(submitBtn))
	sb.WriteString("\n")

	return sb.String()
}

func renderSetup(m Model) string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Bold(true).Foreground(colorSubtext).Render("System Setup"))
	sb.WriteString("\n\n")

	// Kernel modules section.
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(colorText).Render("Kernel Modules"))
	sb.WriteString("\n")
	for _, mod := range m.kernelStatus {
		var badge string
		if mod.Loaded {
			badge = styleRunning.Render("● loaded  ")
		} else {
			badge = styleStopped.Render("○ missing ")
		}
		sb.WriteString(lipgloss.NewStyle().Padding(0, 3).Render(badge + styleLabel.Render(mod.Name)))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Image section.
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(colorText).Render("Container Image"))
	sb.WriteString("\n")
	var imgBadge string
	if m.imageExists {
		imgBadge = styleRunning.Render("● built   ") + styleLabel.Render("x11droid:latest")
	} else {
		imgBadge = styleStopped.Render("○ missing ") + styleLabel.Render("x11droid:latest")
	}
	sb.WriteString(lipgloss.NewStyle().Padding(0, 3).Render(imgBadge))
	sb.WriteString("\n\n")

	// Actions.
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(colorText).Render("Actions"))
	sb.WriteString("\n")
	for i, action := range setupActions {
		var line string
		if i == m.setupCursor {
			line = styleActionSelected.Render("  " + action + "  ")
		} else {
			line = styleAction.Foreground(colorSubtext).Render("  " + action + "  ")
		}
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(line))
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderHelp(m Model) string {
	var sb strings.Builder
	pad := lipgloss.NewStyle().Padding(0, 2)
	heading := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Padding(0, 2)
	key := lipgloss.NewStyle().Foreground(colorAccent).Width(18)
	desc := lipgloss.NewStyle().Foreground(colorSubtext)

	row := func(k, d string) string {
		return pad.Render(key.Render(k) + desc.Render(d)) + "\n"
	}

	sb.WriteString("\n")

	// Global
	sb.WriteString(heading.Render("Global"))
	sb.WriteString("\n")
	sb.WriteString(row("?", "open / close this help screen"))
	sb.WriteString(row("q  ctrl+c", "quit"))
	sb.WriteString(row("esc", "go back / close"))
	sb.WriteString("\n")

	// Dashboard
	sb.WriteString(heading.Render("Dashboard"))
	sb.WriteString("\n")
	sb.WriteString(row("↑ / ↓  or  k / j", "navigate instance list"))
	sb.WriteString(row("enter", "open instance detail"))
	sb.WriteString(row("n", "new instance (spawn form)"))
	sb.WriteString(row("s", "open setup screen"))
	sb.WriteString(row("r", "refresh instance list"))
	sb.WriteString("\n")

	// Instance detail
	sb.WriteString(heading.Render("Instance Detail"))
	sb.WriteString("\n")
	sb.WriteString(row("↑ / ↓  or  k / j", "navigate actions"))
	sb.WriteString(row("enter", "run selected action"))
	sb.WriteString(row("", "  Start   — start a stopped container"))
	sb.WriteString(row("", "  Stop    — stop a running container"))
	sb.WriteString(row("", "  Remove  — force-remove the container"))
	sb.WriteString(row("", "  Shell   — open bash inside container"))
	sb.WriteString(row("", "  Logs    — tail last 100 log lines"))
	sb.WriteString("\n")

	// Spawn
	sb.WriteString(heading.Render("New Instance"))
	sb.WriteString("\n")
	sb.WriteString(row("tab  /  ↑↓", "move between fields"))
	sb.WriteString(row("space", "toggle GApps"))
	sb.WriteString(row("enter", "submit (when Spawn is focused)"))
	sb.WriteString(row("ctrl+u", "clear name input"))
	sb.WriteString("\n")

	// Setup
	sb.WriteString(heading.Render("Setup"))
	sb.WriteString("\n")
	sb.WriteString(row("↑ / ↓", "navigate actions"))
	sb.WriteString(row("enter", "run selected action"))
	sb.WriteString(row("", "  Load Modules   — sudo modprobe binder_linux"))
	sb.WriteString(row("", "  Unload Modules — sudo rmmod binder_linux"))
	sb.WriteString(row("", "  Build Image    — podman build x11droid:latest"))
	sb.WriteString(row("", "  Refresh        — re-check status"))
	sb.WriteString("\n")

	// System info
	sb.WriteString(heading.Render("System"))
	sb.WriteString("\n")
	infoKey := lipgloss.NewStyle().Foreground(colorSubtext).Width(20)
	infoVal := func(v, good, bad string) string {
		s := lipgloss.NewStyle()
		if v == good {
			s = s.Foreground(colorGreen)
		} else if v == bad || v == "" {
			s = s.Foreground(colorRed)
		} else {
			s = s.Foreground(colorYellow)
		}
		return pad.Render(infoKey.Render("session type") + s.Render(v)) + "\n"
	}
	sb.WriteString(infoVal(m.session.KindLabel(), "X11", "Wayland"))

	dispStyle := lipgloss.NewStyle().Foreground(colorGreen)
	if m.session.Display == "" {
		dispStyle = dispStyle.Foreground(colorRed)
	}
	disp := m.session.Display
	if disp == "" {
		disp = "(not set)"
	}
	sb.WriteString(pad.Render(infoKey.Render("$DISPLAY") + dispStyle.Render(disp)) + "\n")

	for _, mod := range kernel.Status() {
		var badge string
		if mod.Loaded {
			badge = lipgloss.NewStyle().Foreground(colorGreen).Render("loaded")
		} else {
			badge = lipgloss.NewStyle().Foreground(colorRed).Render("missing")
		}
		sb.WriteString(pad.Render(infoKey.Render(mod.Name) + badge) + "\n")
	}

	imgStyle := lipgloss.NewStyle()
	imgLabel := "not built"
	if m.imageExists {
		imgStyle = imgStyle.Foreground(colorGreen)
		imgLabel = "x11droid:latest"
	} else {
		imgStyle = imgStyle.Foreground(colorRed)
	}
	sb.WriteString(pad.Render(infoKey.Render("container image") + imgStyle.Render(imgLabel)) + "\n")

	if w := m.session.Warning(); w != "" {
		sb.WriteString("\n")
		sb.WriteString(pad.Render(lipgloss.NewStyle().Foreground(colorYellow).Render("⚠  " + w)) + "\n")
	}

	return sb.String()
}

// helpers

func statusLabel(status string) string {
	return statusStyle(status).Render(padRight(status, 20))
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

