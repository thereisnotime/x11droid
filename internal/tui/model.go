package tui

import (
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thereisnotime/x11droid/internal/container"
	"github.com/thereisnotime/x11droid/internal/kernel"
	"github.com/thereisnotime/x11droid/internal/system"
)

type view int

const (
	viewMain view = iota
	viewDetail
	viewSpawn
	viewSetup
	viewHelp
)

type errMsg struct{ err error }
type instancesMsg []container.Instance
type logsMsg string
type actionDoneMsg struct{ err error }

type Model struct {
	view      view
	prevView  view
	width     int
	height    int
	err       error
	statusMsg string
	session   system.Info

	// main view
	instances []container.Instance
	cursor    int
	loading   bool

	// detail view
	selected     container.Instance
	actionCursor int
	logs         string
	showLogs     bool

	// spawn view
	spawnInput  textinput.Model
	spawnGApps  bool
	spawnCursor int // 0=name, 1=gapps, 2=submit

	// setup view
	kernelStatus []kernel.ModuleStatus
	imageExists  bool
	setupCursor  int
}

func newSpawnInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "instance-name"
	ti.CharLimit = 64
	ti.Width = 28
	ti.Prompt = ""
	return ti
}

func New(sess system.Info) Model {
	return Model{
		view:       viewMain,
		loading:    true,
		session:    sess,
		spawnInput: newSpawnInput(),
	}
}

func (m Model) Init() tea.Cmd {
	return fetchInstances
}

func fetchInstances() tea.Msg {
	list, err := container.List()
	if err != nil {
		return errMsg{err}
	}
	return instancesMsg(list)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		m.statusMsg = ""
		return m, nil

	case instancesMsg:
		m.loading = false
		m.err = nil
		m.instances = []container.Instance(msg)
		if m.cursor >= len(m.instances) && len(m.instances) > 0 {
			m.cursor = len(m.instances) - 1
		}
		return m, nil

	case logsMsg:
		m.logs = string(msg)
		m.showLogs = true
		return m, nil

	case actionDoneMsg:
		m.statusMsg = ""
		if msg.err != nil {
			m.err = msg.err
		}
		// Refresh setup state if in setup view.
		if m.view == viewSetup {
			m.kernelStatus = kernel.Status()
			m.imageExists = container.ImageExists("x11droid:latest")
		}
		return m, fetchInstances

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// forward non-key messages to textinput (cursor blink etc.)
	if m.view == viewSpawn && m.spawnCursor == 0 {
		var cmd tea.Cmd
		m.spawnInput, cmd = m.spawnInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Quit) {
		return m, tea.Quit
	}

	// ? toggles help from any view
	if key.Matches(msg, keys.Help) {
		if m.view == viewHelp {
			m.view = m.prevView
		} else {
			m.prevView = m.view
			m.view = viewHelp
		}
		return m, nil
	}

	// Clear error on any key.
	if m.err != nil {
		m.err = nil
	}

	switch m.view {
	case viewMain:
		return m.handleMain(msg)
	case viewDetail:
		return m.handleDetail(msg)
	case viewSpawn:
		return m.handleSpawn(msg)
	case viewSetup:
		return m.handleSetup(msg)
	case viewHelp:
		if key.Matches(msg, keys.Esc) {
			m.view = m.prevView
		}
	}
	return m, nil
}

func (m Model) handleMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.instances)-1 {
			m.cursor++
		}
	case key.Matches(msg, keys.Enter):
		if len(m.instances) > 0 {
			m.selected = m.instances[m.cursor]
			m.actionCursor = 0
			m.showLogs = false
			m.logs = ""
			m.view = viewDetail
		}
	case key.Matches(msg, keys.New):
		m.spawnInput = newSpawnInput()
		m.spawnInput.Focus()
		m.spawnGApps = false
		m.spawnCursor = 0
		m.view = viewSpawn
	case key.Matches(msg, keys.Setup):
		m.kernelStatus = kernel.Status()
		m.imageExists = container.ImageExists("x11droid:latest")
		m.setupCursor = 0
		m.view = viewSetup
	case key.Matches(msg, keys.Refresh):
		m.loading = true
		return m, fetchInstances
	}
	return m, nil
}

var detailActions = []string{"Start", "Stop", "Remove", "Shell", "Logs"}

func (m Model) handleDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showLogs {
		m.showLogs = false
		return m, nil
	}
	switch {
	case key.Matches(msg, keys.Esc):
		m.view = viewMain
	case key.Matches(msg, keys.Up):
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.actionCursor < len(detailActions)-1 {
			m.actionCursor++
		}
	case key.Matches(msg, keys.Enter):
		return m.execDetailAction()
	}
	return m, nil
}

func (m Model) execDetailAction() (Model, tea.Cmd) {
	name := m.selected.Name
	switch detailActions[m.actionCursor] {
	case "Start":
		m.statusMsg = "Starting..."
		return m, func() tea.Msg {
			return actionDoneMsg{container.Start(name)}
		}
	case "Stop":
		m.statusMsg = "Stopping..."
		return m, func() tea.Msg {
			return actionDoneMsg{container.Stop(name)}
		}
	case "Remove":
		m.statusMsg = "Removing..."
		m.view = viewMain
		return m, func() tea.Msg {
			return actionDoneMsg{container.Remove(name)}
		}
	case "Shell":
		return m, tea.ExecProcess(
			exec.Command("podman", "exec", "-it", name, "bash"),
			func(err error) tea.Msg { return actionDoneMsg{err} },
		)
	case "Logs":
		return m, func() tea.Msg {
			logs, err := container.Logs(name)
			if err != nil {
				return errMsg{err}
			}
			return logsMsg(logs)
		}
	}
	return m, nil
}

func (m Model) handleSpawn(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Esc):
		m.spawnInput.Blur()
		m.view = viewMain
		return m, nil

	case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Down):
		m.spawnCursor = (m.spawnCursor + 1) % 3
		if m.spawnCursor == 0 {
			m.spawnInput.Focus()
		} else {
			m.spawnInput.Blur()
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		m.spawnCursor = (m.spawnCursor + 2) % 3
		if m.spawnCursor == 0 {
			m.spawnInput.Focus()
		} else {
			m.spawnInput.Blur()
		}
		return m, nil

	case key.Matches(msg, keys.Space):
		if m.spawnCursor == 1 {
			m.spawnGApps = !m.spawnGApps
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		if m.spawnCursor == 2 {
			name := strings.TrimSpace(m.spawnInput.Value())
			if name == "" {
				m.err = &simpleErr{"instance name cannot be empty"}
				return m, nil
			}
			gapps := m.spawnGApps
			m.spawnInput.Blur()
			m.statusMsg = "Spawning..."
			m.view = viewMain
			return m, func() tea.Msg {
				return actionDoneMsg{container.Spawn(name, gapps)}
			}
		}
		// enter on name field moves to next
		if m.spawnCursor == 0 {
			m.spawnCursor = 1
			m.spawnInput.Blur()
		}
		return m, nil
	}

	// route remaining keys to textinput when on name field
	if m.spawnCursor == 0 {
		var cmd tea.Cmd
		m.spawnInput, cmd = m.spawnInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

var setupActions = []string{"Load Modules", "Unload Modules", "Build Image", "Refresh"}

func (m Model) handleSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Esc):
		m.view = viewMain
	case key.Matches(msg, keys.Up):
		if m.setupCursor > 0 {
			m.setupCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.setupCursor < len(setupActions)-1 {
			m.setupCursor++
		}
	case key.Matches(msg, keys.Enter):
		return m.execSetupAction()
	}
	return m, nil
}

func (m Model) execSetupAction() (Model, tea.Cmd) {
	switch setupActions[m.setupCursor] {
	case "Load Modules":
		return m, tea.ExecProcess(
			exec.Command("sudo", "modprobe", "binder_linux"),
			func(err error) tea.Msg { return actionDoneMsg{err} },
		)
	case "Unload Modules":
		return m, tea.ExecProcess(
			exec.Command("sudo", "rmmod", "binder_linux"),
			func(err error) tea.Msg { return actionDoneMsg{err} },
		)
	case "Build Image":
		m.statusMsg = "Building image (this may take a while)..."
		return m, func() tea.Msg {
			return actionDoneMsg{container.BuildImage()}
		}
	case "Refresh":
		m.kernelStatus = kernel.Status()
		m.imageExists = container.ImageExists("x11droid:latest")
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	header := m.renderHeader()
	footer := m.renderFooter()
	body := m.renderBody()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) renderHeader() string {
	viewNames := map[view]string{
		viewMain:   "Dashboard",
		viewDetail: "Instance",
		viewSpawn:  "New Instance",
		viewSetup:  "Setup",
		viewHelp:   "Help",
	}
	title := styleTitle.Render("x11droid")
	viewName := styleMuted.Render("  /  " + viewNames[m.view])
	left := lipgloss.JoinHorizontal(lipgloss.Left, title, viewName)

	status := ""
	if m.statusMsg != "" {
		status = styleInProgress.Render(m.statusMsg)
	} else if m.err != nil {
		status = styleError.Render("error: " + m.err.Error())
	}

	padding := m.width - lipgloss.Width(left) - lipgloss.Width(status)
	if padding < 0 {
		padding = 0
	}
	row := left + strings.Repeat(" ", padding) + status
	return styleHeader.Width(m.width).Render(row)
}

func (m Model) renderFooter() string {
	hints := map[view]string{
		viewMain:   "↑↓ navigate  enter select  n new  s setup  r refresh  ? help  q quit",
		viewDetail: "↑↓ navigate  enter action  esc back  ? help  q quit",
		viewSpawn:  "tab/↑↓ navigate  space toggle  enter confirm  esc back  ? help",
		viewSetup:  "↑↓ navigate  enter action  esc back  ? help  q quit",
		viewHelp:   "esc/? back  q quit",
	}
	return styleFooter.Width(m.width).Render(hints[m.view])
}

func (m Model) renderBody() string {
	switch m.view {
	case viewMain:
		return renderMain(m)
	case viewDetail:
		return renderDetail(m)
	case viewSpawn:
		return renderSpawn(m)
	case viewSetup:
		return renderSetup(m)
	case viewHelp:
		return renderHelp(m)
	}
	return ""
}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }
