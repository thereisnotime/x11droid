package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/thereisnotime/x11droid/internal/config"
	"github.com/thereisnotime/x11droid/internal/kernel"
	"github.com/thereisnotime/x11droid/internal/version"
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

	if pw := m.prereqWarning(); pw != "" {
		banner := lipgloss.NewStyle().
			Foreground(colorRed).
			Background(lipgloss.Color("#2d0000")).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorRed).
			Padding(0, 2).
			Width(m.width - 4).
			Render("✖  " + pw)
		sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Render(banner))
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

	// Show the full (untruncated) error from the last action — the header only
	// has room for one truncated line, and these are often multi-line.
	if m.err != nil {
		w := m.width - 6
		if w < 20 {
			w = 20
		}
		box := lipgloss.NewStyle().
			Foreground(colorRed).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorRed).
			Padding(0, 2).
			Width(w).
			Render("error\n" + m.err.Error())
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(box))
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderLogs(m Model) string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Bold(true).Foreground(colorSubtext).Render(
		fmt.Sprintf("Logs: %s  (live · any key to close)", m.selected.Name),
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
	pad := lipgloss.NewStyle().Padding(0, 3)
	label := lipgloss.NewStyle().Foreground(colorSubtext).Width(10)

	toggle := func(on bool, cursor, idx int, name, hint string) string {
		val := "[ ] off"
		if on {
			val = "[x] on "
		}
		text := val
		if hint != "" {
			text = val + "  " + lipgloss.NewStyle().Foreground(colorMuted).Render(hint)
		}
		if cursor == idx {
			return pad.Render(lipgloss.JoinHorizontal(lipgloss.Center,
				label.Render(name), styleActionSelected.Render(" "+val+" "),
				lipgloss.NewStyle().Foreground(colorMuted).Render("  "+hint)))
		}
		_ = text
		return pad.Render(lipgloss.JoinHorizontal(lipgloss.Center,
			label.Render(name), styleAction.Foreground(colorSubtext).Render(" "+val+" "),
			lipgloss.NewStyle().Foreground(colorMuted).Render("  "+hint)))
	}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Bold(true).Foreground(colorSubtext).Render("New Instance"))
	sb.WriteString("\n\n")

	// Name
	inputView := m.spawnInput.View()
	if m.spawnCursor == 0 {
		inputView = styleInputFocused.Render(inputView)
	} else {
		inputView = styleInput.Render(inputView)
	}
	sb.WriteString(pad.Render(lipgloss.JoinHorizontal(lipgloss.Center, label.Render("Name"), inputView)))
	sb.WriteString("\n\n")

	sb.WriteString(toggle(m.spawnGApps, m.spawnCursor, 1, "GApps", "Google Play Store"))
	sb.WriteString("\n\n")
	sb.WriteString(toggle(m.spawnHideARM, m.spawnCursor, 2, "ARM", "libndk ARM translation — needed for GApps (adds minutes to first boot)"))
	sb.WriteString("\n\n")
	sb.WriteString(toggle(m.spawnApps, m.spawnCursor, 3, "App stores", "install F-Droid + Aurora after first boot"))
	sb.WriteString("\n\n")
	sb.WriteString(toggle(m.spawnPV, m.spawnCursor, 4, "Persist", "keep Android data between container restarts"))
	sb.WriteString("\n\n")

	var submitBtn string
	if m.spawnCursor == 5 {
		submitBtn = styleActionSelected.Render("  Spawn  ")
	} else {
		submitBtn = styleAction.Foreground(colorSubtext).Render("  Spawn  ")
	}
	sb.WriteString(pad.Render(lipgloss.JoinHorizontal(lipgloss.Center, label.Render(""), submitBtn)))
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
		sb.WriteString(lipgloss.NewStyle().Padding(0, 3).Render(moduleStateBadge(mod) + styleLabel.Render(mod.Name)))
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

func renderConfig(m Model) string {
	var sb strings.Builder
	pad := lipgloss.NewStyle().Padding(0, 3)
	label := lipgloss.NewStyle().Foreground(colorSubtext).Width(13)

	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Bold(true).Foreground(colorSubtext).Render("Instance Defaults"))
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(colorMuted).Render(
		"Applied to every new instance. ←→ or space to change, enter on Save to persist."))
	sb.WriteString("\n\n")

	field := func(idx int, name, value, hint string) string {
		valStyle := styleAction.Foreground(colorSubtext)
		if m.configCursor == idx {
			valStyle = styleActionSelected
		}
		row := lipgloss.JoinHorizontal(lipgloss.Center,
			label.Render(name),
			valStyle.Render(" "+value+" "),
		)
		if hint != "" {
			row = lipgloss.JoinHorizontal(lipgloss.Center, row,
				lipgloss.NewStyle().Foreground(colorMuted).Render("  "+hint))
		}
		return pad.Render(row)
	}

	w, h := m.cfg.EffectiveDims()
	sb.WriteString(field(0, "Resolution", fmt.Sprintf("%dx%d", m.cfg.Width, m.cfg.Height),
		fmt.Sprintf("portrait dimensions (window opens %dx%d)", w, h)))
	sb.WriteString("\n\n")
	sb.WriteString(field(1, "Orientation", m.cfg.Orientation, ""))
	sb.WriteString("\n\n")
	sb.WriteString(field(2, "Compositor", m.cfg.Compositor, compositorHint(m.cfg.Compositor)))
	sb.WriteString("\n\n")

	var saveBtn string
	if m.configCursor == 3 {
		saveBtn = styleActionSelected.Render("  Save  ")
	} else {
		saveBtn = styleAction.Foreground(colorSubtext).Render("  Save  ")
	}
	sb.WriteString(pad.Render(lipgloss.JoinHorizontal(lipgloss.Center, label.Render(""), saveBtn)))
	sb.WriteString("\n")

	return sb.String()
}

func compositorHint(c string) string {
	switch c {
	case config.CompositorAuto:
		return "try cage, fall back to weston (recommended)"
	case config.CompositorWeston:
		return "pixman X11 backend — works on NVIDIA"
	case config.CompositorCage:
		return "kiosk wlroots — needs mesa GPU"
	}
	return ""
}

func renderHelp(m Model) string {
	var sb strings.Builder
	pad := lipgloss.NewStyle().Padding(0, 2)
	heading := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Padding(0, 2)
	key := lipgloss.NewStyle().Foreground(colorAccent).Width(18)
	desc := lipgloss.NewStyle().Foreground(colorSubtext)

	row := func(k, d string) string {
		return pad.Render(key.Render(k)+desc.Render(d)) + "\n"
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
	sb.WriteString(row("c", "open config screen"))
	sb.WriteString(row("r", "refresh instance list"))
	sb.WriteString(row("", "  (the list also auto-refreshes every few seconds)"))
	sb.WriteString("\n")

	// Instance detail
	sb.WriteString(heading.Render("Instance Detail"))
	sb.WriteString("\n")
	sb.WriteString(row("↑ / ↓  or  k / j", "navigate actions"))
	sb.WriteString(row("enter", "run selected action"))
	sb.WriteString(row("", "  Show UI — (re)open the Android window"))
	sb.WriteString(row("", "  Start   — start a stopped container"))
	sb.WriteString(row("", "  Stop    — stop a running container"))
	sb.WriteString(row("", "  Remove  — force-remove the container"))
	sb.WriteString(row("", "  Purge   — remove container + delete its Android data"))
	sb.WriteString(row("", "  Shell   — open bash inside container"))
	sb.WriteString(row("", "  Logs    — tail logs (auto-refreshes while open)"))
	sb.WriteString("\n")

	// Spawn
	sb.WriteString(heading.Render("New Instance"))
	sb.WriteString("\n")
	sb.WriteString(row("tab  /  ↑↓", "move between fields"))
	sb.WriteString(row("space", "toggle GApps"))
	sb.WriteString(row("enter", "submit (when Spawn is focused)"))
	sb.WriteString(row("ctrl+u", "clear name input"))
	sb.WriteString("\n")

	// Config
	sb.WriteString(heading.Render("Config"))
	sb.WriteString("\n")
	sb.WriteString(row("↑ / ↓", "navigate fields"))
	sb.WriteString(row("← / →  or  space", "change focused value"))
	sb.WriteString(row("enter", "save (when Save is focused)"))
	sb.WriteString(row("", "  Resolution  — portrait window size"))
	sb.WriteString(row("", "  Orientation — portrait / landscape"))
	sb.WriteString(row("", "  Compositor  — auto / weston / cage"))
	sb.WriteString("\n")

	// Setup
	sb.WriteString(heading.Render("Setup"))
	sb.WriteString("\n")
	sb.WriteString(row("↑ / ↓", "navigate actions"))
	sb.WriteString(row("enter", "run selected action"))
	sb.WriteString(row("", "  Authenticate sudo — cache sudo creds (rootful podman needs it)"))
	sb.WriteString(row("", "  Load Modules      — ensure binder_linux is loaded"))
	sb.WriteString(row("", "  Unload Modules    — rmmod binder_linux"))
	sb.WriteString(row("", "  Build Image       — sudo podman build x11droid:latest"))
	sb.WriteString(row("", "  Refresh           — re-check status"))
	sb.WriteString("\n")

	// System info
	sb.WriteString(heading.Render("System"))
	sb.WriteString("\n")
	infoKey := lipgloss.NewStyle().Foreground(colorSubtext).Width(20)
	infoVal := func(v, good, bad string) string {
		var color lipgloss.Color
		switch v {
		case good:
			color = colorGreen
		case bad, "":
			color = colorRed
		default:
			color = colorYellow
		}
		s := lipgloss.NewStyle().Foreground(color)
		return pad.Render(infoKey.Render("session type")+s.Render(v)) + "\n"
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
	sb.WriteString(pad.Render(infoKey.Render("$DISPLAY")+dispStyle.Render(disp)) + "\n")

	for _, mod := range m.kernelStatus {
		sb.WriteString(pad.Render(infoKey.Render(mod.Name)+moduleStateBadge(mod)) + "\n")
	}

	imgStyle := lipgloss.NewStyle()
	imgLabel := "not built"
	if m.imageExists {
		imgStyle = imgStyle.Foreground(colorGreen)
		imgLabel = "x11droid:latest"
	} else {
		imgStyle = imgStyle.Foreground(colorRed)
	}
	sb.WriteString(pad.Render(infoKey.Render("container image")+imgStyle.Render(imgLabel)) + "\n")

	sb.WriteString(pad.Render(infoKey.Render("version")+lipgloss.NewStyle().Foreground(colorMuted).Render(version.String())) + "\n")

	if w := m.session.Warning(); w != "" {
		sb.WriteString("\n")
		sb.WriteString(pad.Render(lipgloss.NewStyle().Foreground(colorYellow).Render("⚠  "+w)) + "\n")
	}

	return sb.String()
}

func moduleStateBadge(mod kernel.ModuleStatus) string {
	switch mod.State {
	case kernel.StateLoaded:
		return styleRunning.Render("● loaded   ")
	case kernel.StateBuiltIn:
		return styleRunning.Render("● built-in ")
	case kernel.StateOptional:
		return styleMuted.Render("○ built-in ")
	default:
		return styleStopped.Render("○ missing  ")
	}
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
