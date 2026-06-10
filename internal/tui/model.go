package tui

import (
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thereisnotime/x11droid/internal/config"
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
	viewConfig
	viewHelp
)

// refreshInterval is how often the dashboard re-fetches the instance list.
const refreshInterval = 3 * time.Second

type errMsg struct{ err error }
type instancesMsg []container.Instance
type logsMsg string
type actionDoneMsg struct{ err error }
type buildDoneMsg struct {
	buildErr    error
	imageExists bool
}
type kernelStatusMsg []kernel.ModuleStatus
type imageStatusMsg struct {
	exists bool
	valid  bool // false when the check itself failed
}
type tickMsg struct{}

// bg* messages are silent background refreshes — they update data without
// touching loading/error/status state, so periodic polling stays unobtrusive.
type bgInstancesMsg struct {
	list []container.Instance
	ok   bool
}
type bgLogsMsg string

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

	// spawn view — cursor: 0=name 1=gapps 2=hidearm 3=pv 4=submit
	spawnInput   textinput.Model
	spawnGApps   bool
	spawnHideARM bool
	spawnPV      bool
	spawnCursor  int

	// setup view
	kernelStatus    []kernel.ModuleStatus
	imageExists     bool
	setupCursor     int
	podmanInstalled bool
	prereqsChecked  bool

	// config view
	cfg          config.Config
	configCursor int

	isRoot bool
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
		view:            viewMain,
		loading:         true,
		session:         sess,
		spawnInput:      newSpawnInput(),
		podmanInstalled: container.PodmanInstalled(),
		cfg:             config.Load(),
		isRoot:          system.IsRoot(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchInstances, fetchImageStatus, fetchKernelStatus, tick)
}

func tick() tea.Msg {
	time.Sleep(refreshInterval)
	return tickMsg{}
}

func fetchInstances() tea.Msg {
	list, err := container.List()
	if err != nil {
		return errMsg{err}
	}
	return instancesMsg(list)
}

func fetchInstancesBg() tea.Msg {
	list, err := container.List()
	return bgInstancesMsg{list: list, ok: err == nil}
}

func fetchLogsBg(name string) tea.Cmd {
	return func() tea.Msg {
		out, err := container.Logs(name)
		if err != nil {
			return bgLogsMsg("")
		}
		return bgLogsMsg(out)
	}
}

func fetchKernelStatus() tea.Msg { return kernelStatusMsg(kernel.Status()) }
func fetchImageStatus() tea.Msg {
	exists, ok := container.ImageExistsChecked("x11droid:latest")
	return imageStatusMsg{exists: exists, valid: ok}
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

	case kernelStatusMsg:
		m.kernelStatus = []kernel.ModuleStatus(msg)
		return m, nil

	case imageStatusMsg:
		if msg.valid {
			m.imageExists = msg.exists
		}
		m.prereqsChecked = true
		return m, nil

	case tickMsg:
		// Keep ticking and quietly refresh whatever the current view shows.
		cmds := []tea.Cmd{tick}
		switch m.view {
		case viewMain:
			if !m.loading {
				cmds = append(cmds, fetchInstancesBg)
			}
		case viewDetail:
			cmds = append(cmds, fetchInstancesBg) // keep selected status live
			if m.showLogs {
				cmds = append(cmds, fetchLogsBg(m.selected.Name))
			}
		}
		return m, tea.Batch(cmds...)

	case bgInstancesMsg:
		if !msg.ok {
			return m, nil // ignore transient errors during background polls
		}
		m.instances = msg.list
		if m.cursor >= len(m.instances) && len(m.instances) > 0 {
			m.cursor = len(m.instances) - 1
		}
		// Keep the detail view's selected instance status fresh.
		for _, inst := range m.instances {
			if inst.Name == m.selected.Name {
				m.selected = inst
				break
			}
		}
		return m, nil

	case bgLogsMsg:
		if m.view == viewDetail && m.showLogs {
			m.logs = string(msg)
		}
		return m, nil

	case buildDoneMsg:
		if msg.buildErr != nil {
			m.err = msg.buildErr
			m.statusMsg = ""
		} else if msg.imageExists {
			m.statusMsg = "image built successfully"
			m.imageExists = true
		} else {
			m.err = &simpleErr{"build exited cleanly but image not found — check logs"}
			m.statusMsg = ""
		}
		return m, fetchKernelStatus

	case actionDoneMsg:
		m.statusMsg = ""
		if msg.err != nil {
			m.err = msg.err
		}
		// Refresh in the background (bgInstancesMsg) so the list updates without
		// clobbering an error we just set — fetchInstances would clear m.err
		// before it could be read.
		if m.view == viewSetup {
			return m, tea.Batch(fetchInstancesBg, fetchKernelStatus, fetchImageStatus)
		}
		return m, fetchInstancesBg

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			return m.handleMouseWheel(msg), nil
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return m.handleMouseClick(msg)
		}
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
			return m, nil
		}
		m.prevView = m.view
		m.view = viewHelp
		return m, tea.Batch(fetchKernelStatus, fetchImageStatus)
	}

	// Clear transient messages on any key.
	m.err = nil
	m.statusMsg = ""

	switch m.view {
	case viewMain:
		return m.handleMain(msg)
	case viewDetail:
		return m.handleDetail(msg)
	case viewSpawn:
		return m.handleSpawn(msg)
	case viewSetup:
		return m.handleSetup(msg)
	case viewConfig:
		return m.handleConfig(msg)
	case viewHelp:
		if key.Matches(msg, keys.Esc) {
			m.view = m.prevView
		}
	}
	return m, nil
}

func (m Model) handleMouseWheel(msg tea.MouseMsg) Model {
	up := msg.Button == tea.MouseButtonWheelUp
	switch m.view {
	case viewMain, viewHelp:
		if up && m.cursor > 0 {
			m.cursor--
		} else if !up && m.cursor < len(m.instances)-1 {
			m.cursor++
		}
	case viewDetail:
		if up && m.actionCursor > 0 {
			m.actionCursor--
		} else if !up && m.actionCursor < len(detailActions)-1 {
			m.actionCursor++
		}
	case viewSetup:
		if up && m.setupCursor > 0 {
			m.setupCursor--
		} else if !up && m.setupCursor < len(setupActions)-1 {
			m.setupCursor++
		}
	case viewSpawn:
		var next int
		if up {
			next = (m.spawnCursor + spawnFields - 1) % spawnFields
		} else {
			next = (m.spawnCursor + 1) % spawnFields
		}
		if next == 0 {
			m.spawnInput.Focus()
		} else {
			m.spawnInput.Blur()
		}
		m.spawnCursor = next
	}
	return m
}

func (m Model) handleMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	y := msg.Y

	switch m.view {
	case viewMain:
		// list items start after: header(1) + instances_header_with_padding(3) + newline(1) = y5
		// add 4 more if warning is shown
		offset := 5
		if m.session.Warning() != "" {
			offset += 4
		}
		idx := y - offset
		if idx >= 0 && idx < len(m.instances) {
			m.cursor = idx
			// double-click equivalent: single click opens detail
			m.selected = m.instances[m.cursor]
			m.actionCursor = 0
			m.showLogs = false
			m.logs = ""
			m.view = viewDetail
		}

	case viewDetail:
		// actions start after: header(1) + detail_info(7) + actions_label(2) = y10
		offset := 10
		idx := y - offset
		if idx >= 0 && idx < len(detailActions) {
			m.actionCursor = idx
			return m.execDetailAction()
		}

	case viewSetup:
		offset := 14
		idx := y - offset
		if idx >= 0 && idx < len(setupActions) {
			m.setupCursor = idx
			nm, cmd := m.execSetupAction()
			return nm, cmd
		}

	case viewSpawn:
		// name~y4, gapps~y7, hidearm~y10, persist~y13, spawn~y16
		prev := m.spawnCursor
		switch {
		case y >= 3 && y <= 5:
			m.spawnCursor = 0
		case y >= 6 && y <= 8:
			m.spawnCursor = 1
		case y >= 9 && y <= 11:
			m.spawnCursor = 2
		case y >= 12 && y <= 14:
			m.spawnCursor = 3
		case y >= 15 && y <= 17:
			m.spawnCursor = 4
		}
		if m.spawnCursor == 0 && prev != 0 {
			m.spawnInput.Focus()
		} else if m.spawnCursor != 0 && prev == 0 {
			m.spawnInput.Blur()
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
		m.spawnHideARM = false
		m.spawnPV = true
		m.spawnCursor = 0
		m.view = viewSpawn
	case key.Matches(msg, keys.Setup):
		m.setupCursor = 0
		m.view = viewSetup
		return m, tea.Batch(fetchKernelStatus, fetchImageStatus)
	case key.Matches(msg, keys.Config):
		m.cfg = config.Load()
		m.configCursor = 0
		m.view = viewConfig
	case key.Matches(msg, keys.Refresh):
		m.loading = true
		return m, fetchInstances
	}
	return m, nil
}

// resolutionPresets are the portrait (tall) dimensions offered in the config
// view; orientation swaps them for landscape.
var resolutionPresets = [][2]int{
	{480, 800},
	{540, 960},
	{720, 1280},
	{1080, 1920},
}

const configFields = 4 // resolution, orientation, compositor, save

var compositorChoices = []string{config.CompositorAuto, config.CompositorWeston, config.CompositorCage}

// cycleResolution moves the current width/height to the next/previous preset.
func (m *Model) cycleResolution(delta int) {
	idx := 0
	for i, p := range resolutionPresets {
		if p[0] == m.cfg.Width && p[1] == m.cfg.Height {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(resolutionPresets)) % len(resolutionPresets)
	m.cfg.Width = resolutionPresets[idx][0]
	m.cfg.Height = resolutionPresets[idx][1]
}

func (m *Model) cycleCompositor(delta int) {
	idx := 0
	for i, c := range compositorChoices {
		if c == m.cfg.Compositor {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(compositorChoices)) % len(compositorChoices)
	m.cfg.Compositor = compositorChoices[idx]
}

func (m Model) handleConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Esc):
		m.view = viewMain
	case key.Matches(msg, keys.Up):
		if m.configCursor > 0 {
			m.configCursor--
		}
	case key.Matches(msg, keys.Down), key.Matches(msg, keys.Tab):
		if m.configCursor < configFields-1 {
			m.configCursor++
		} else {
			m.configCursor = 0
		}
	case key.Matches(msg, keys.Space), key.Matches(msg, keys.Right):
		m.applyConfigChange(1)
	case key.Matches(msg, keys.Left):
		m.applyConfigChange(-1)
	case key.Matches(msg, keys.Enter):
		if m.configCursor == configFields-1 {
			if err := m.cfg.Save(); err != nil {
				m.err = err
			} else {
				m.statusMsg = "config saved"
			}
		} else {
			m.applyConfigChange(1)
		}
	}
	return m, nil
}

// applyConfigChange cycles the value of the focused field.
func (m *Model) applyConfigChange(delta int) {
	switch m.configCursor {
	case 0:
		m.cycleResolution(delta)
	case 1:
		if m.cfg.Orientation == config.Portrait {
			m.cfg.Orientation = config.Landscape
		} else {
			m.cfg.Orientation = config.Portrait
		}
	case 2:
		m.cycleCompositor(delta)
	}
}

var detailActions = []string{"Show UI", "Start", "Stop", "Remove", "Purge", "Shell", "Logs"}

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
	case "Show UI":
		m.statusMsg = "Opening GUI..."
		return m, func() tea.Msg {
			return actionDoneMsg{container.ShowUI(name)}
		}
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
	case "Purge":
		m.statusMsg = "Purging (container + data)..."
		m.view = viewMain
		return m, func() tea.Msg {
			return actionDoneMsg{container.Purge(name)}
		}
	case "Shell":
		return m, tea.ExecProcess(
			exec.Command("sudo", "podman", "exec", "-it", name, "bash"),
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

const spawnFields = 5 // name, gapps, hidearm, pv, submit

func (m Model) handleSpawn(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Esc):
		m.spawnInput.Blur()
		m.view = viewMain
		return m, nil

	case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Down):
		m.spawnCursor = (m.spawnCursor + 1) % spawnFields
		if m.spawnCursor == 0 {
			m.spawnInput.Focus()
		} else {
			m.spawnInput.Blur()
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		m.spawnCursor = (m.spawnCursor + spawnFields - 1) % spawnFields
		if m.spawnCursor == 0 {
			m.spawnInput.Focus()
		} else {
			m.spawnInput.Blur()
		}
		return m, nil

	case key.Matches(msg, keys.Space):
		switch m.spawnCursor {
		case 1:
			m.spawnGApps = !m.spawnGApps
		case 2:
			m.spawnHideARM = !m.spawnHideARM
		case 3:
			m.spawnPV = !m.spawnPV
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		if m.spawnCursor == spawnFields-1 {
			name := strings.TrimSpace(m.spawnInput.Value())
			if name == "" {
				m.err = &simpleErr{"instance name cannot be empty"}
				return m, nil
			}
			w, h := m.cfg.EffectiveDims()
			opts := container.SpawnOpts{
				Name:       name,
				GApps:      m.spawnGApps,
				HideARM:    m.spawnHideARM,
				PV:         m.spawnPV,
				Width:      w,
				Height:     h,
				Compositor: m.cfg.Compositor,
			}
			m.spawnInput.Blur()
			m.statusMsg = "Spawning..."
			m.view = viewMain
			return m, func() tea.Msg {
				return actionDoneMsg{container.Spawn(opts)}
			}
		}
		if m.spawnCursor == 0 {
			m.spawnCursor = 1
			m.spawnInput.Blur()
		}
		return m, nil
	}

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
		m.statusMsg = "Loading modules..."
		return m, func() tea.Msg { return actionDoneMsg{kernel.Load()} }
	case "Unload Modules":
		m.statusMsg = "Unloading modules..."
		return m, func() tea.Msg { return actionDoneMsg{kernel.Unload()} }
	case "Build Image":
		cmd, err := container.BuildImageCmd()
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return buildDoneMsg{
				buildErr:    err,
				imageExists: err == nil,
			}
		})
	case "Refresh":
		return m, tea.Batch(fetchKernelStatus, fetchImageStatus)
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
		viewConfig: "Config",
		viewHelp:   "Help",
	}
	title := styleTitle.Render("x11droid")
	viewName := styleMuted.Render("  /  " + viewNames[m.view])
	left := lipgloss.JoinHorizontal(lipgloss.Left, title, viewName)

	status := ""
	if m.statusMsg != "" {
		status = styleInProgress.Render(m.statusMsg)
	} else if m.err != nil {
		maxErrWidth := m.width - lipgloss.Width(left) - 10
		if maxErrWidth < 10 {
			maxErrWidth = 10
		}
		errStr := "error: " + m.err.Error()
		runes := []rune(errStr)
		for lipgloss.Width(string(runes)) > maxErrWidth && len(runes) > 1 {
			runes = runes[:len(runes)-1]
		}
		if lipgloss.Width(string(runes)) < lipgloss.Width(errStr) {
			runes[len(runes)-1] = '…'
		}
		status = styleError.Render(string(runes))
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
		viewMain:   "↑↓ navigate  enter select  n new  s setup  c config  r refresh  ? help  q quit",
		viewDetail: "↑↓ navigate  enter action  esc back  ? help  q quit",
		viewSpawn:  "tab/↑↓ navigate  space toggle  enter confirm  esc back  ? help",
		viewSetup:  "↑↓ navigate  enter action  esc back  ? help  q quit",
		viewConfig: "↑↓ navigate  ←→/space change  enter save  esc back  ? help",
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
	case viewConfig:
		return renderConfig(m)
	case viewHelp:
		return renderHelp(m)
	}
	return ""
}

// prereqWarning returns a non-empty string when required setup is incomplete.
func (m Model) prereqWarning() string {
	if !m.isRoot {
		return "not running as root — quit and restart with: sudo x11droid"
	}
	if !m.prereqsChecked {
		return ""
	}
	var issues []string
	if !m.podmanInstalled {
		issues = append(issues, "podman not found")
	}
	for _, s := range m.kernelStatus {
		if s.Required && !s.OK() {
			issues = append(issues, s.Name+" not loaded")
		}
	}
	if !m.imageExists {
		issues = append(issues, "image not built")
	}
	if len(issues) == 0 {
		return ""
	}
	return strings.Join(issues, " · ") + " — press s to open Setup"
}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }
