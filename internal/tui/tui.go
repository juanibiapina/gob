package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/juanibiapina/gob/internal/telemetry"
)

// Panel focus
type panel int

const (
	panelJobs panel = iota
	panelStdout
	panelStderr
)

// Modal mode
type modalMode int

const (
	modalNone modalMode = iota
	modalNewJob
	modalHelp
)

// Job represents a job with its runtime status
type Job struct {
	ID         string
	PID        int
	Command    string
	Workdir    string
	Running    bool
	StdoutPath string
	StderrPath string
	ExitCode   *int
	StartedAt  time.Time
	StoppedAt  time.Time
}

// logTickMsg is sent periodically to refresh log content
type logTickMsg time.Time

// jobsUpdatedMsg is sent when jobs are refreshed
type jobsUpdatedMsg struct {
	jobs []Job
}

// logUpdateMsg is sent when log content is updated
type logUpdateMsg struct {
	stdout string
	stderr string
}

// actionResultMsg is sent after an action completes
type actionResultMsg struct {
	message string
	isError bool
}

// subscriptionStartedMsg is sent when subscription is established
type subscriptionStartedMsg struct {
	client *daemon.Client
	events <-chan daemon.Event
	errs   <-chan error
}

// daemonEventMsg wraps an event from the daemon
type daemonEventMsg struct {
	event daemon.Event
}

// subscriptionErrorMsg is sent when subscription fails
type subscriptionErrorMsg struct {
	err error
}

// reconnectMsg triggers a reconnection attempt
type reconnectMsg struct{}

// Model is the main TUI model
type Model struct {
	// State
	jobs         []Job
	cursor       int
	showAll      bool
	expandedView bool
	activePanel  panel
	modal        modalMode
	width        int
	height       int
	ready        bool
	message     string
	messageTime time.Time
	isError     bool
	cwd         string

	// Components
	help        help.Model
	textInput   textinput.Model
	stdoutView  viewport.Model
	stderrView  viewport.Model
	jobListView viewport.Model

	// Log viewer state
	followLogs    bool
	stdoutContent string
	stderrContent string

	// Subscription state
	subscribed bool
	subClient  *daemon.Client
	eventChan  <-chan daemon.Event
	errChan    <-chan error
}

// New creates a new TUI model
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "command args..."
	ti.CharLimit = 256
	ti.Width = 50

	h := help.New()
	h.ShowAll = true

	cwd, _ := os.Getwd()

	return Model{
		jobs:        []Job{},
		cursor:      0,
		showAll:     false,
		activePanel: panelJobs,
		modal:       modalNone,
		help:        h,
		textInput:   ti,
		cwd:         cwd,
		followLogs:  true,
	}
}

// connectClient creates a new daemon client connection
func connectClient() (*daemon.Client, error) {
	client, err := daemon.NewClient()
	if err != nil {
		return nil, err
	}
	if err := client.Connect(); err != nil {
		return nil, err
	}
	return client, nil
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshJobs(),
		m.startSubscription(),
		logTickCmd(),
	)
}

// logTickCmd returns a command that sends a tick every 500ms for log updates
func logTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return logTickMsg(t)
	})
}

// startSubscription attempts to connect and subscribe to daemon events
func (m Model) startSubscription() tea.Cmd {
	return func() tea.Msg {
		client, err := daemon.NewClient()
		if err != nil {
			return subscriptionErrorMsg{err: err}
		}
		if err := client.Connect(); err != nil {
			return subscriptionErrorMsg{err: err}
		}

		workdir := ""
		if !m.showAll {
			workdir = m.cwd
		}

		events, errs := client.SubscribeChan(workdir)
		return subscriptionStartedMsg{
			client: client,
			events: events,
			errs:   errs,
		}
	}
}

// waitForEvent waits for an event or error from the subscription
func waitForEvent(events <-chan daemon.Event, errs <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-events:
			if !ok {
				return subscriptionErrorMsg{err: fmt.Errorf("event channel closed")}
			}
			return daemonEventMsg{event: event}
		case err, ok := <-errs:
			if !ok {
				return subscriptionErrorMsg{err: fmt.Errorf("error channel closed")}
			}
			return subscriptionErrorMsg{err: err}
		}
	}
}

// reconnectCmd triggers reconnection after a delay
func reconnectCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return reconnectMsg{}
	})
}

// refreshJobs fetches the current job list
func (m Model) refreshJobs() tea.Cmd {
	return func() tea.Msg {
		client, err := connectClient()
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to connect: %v", err), isError: true}
		}
		defer client.Close()

		workdir := ""
		if !m.showAll {
			workdir = m.cwd
		}

		jobResponses, err := client.List(workdir)
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to list jobs: %v", err), isError: true}
		}

		jobs := make([]Job, len(jobResponses))
		for i, jr := range jobResponses {
			jobs[i] = Job{
				ID:         jr.ID,
				PID:        jr.PID,
				Command:    strings.Join(jr.Command, " "),
				Workdir:    jr.Workdir,
				Running:    jr.Status == "running",
				StdoutPath: jr.StdoutPath,
				StderrPath: jr.StderrPath,
				ExitCode:   jr.ExitCode,
				StartedAt:  parseTime(jr.StartedAt),
				StoppedAt:  parseTime(jr.StoppedAt),
			}
		}

		return jobsUpdatedMsg{jobs: jobs}
	}
}

// readLogs reads the log files for the selected job
func (m Model) readLogs() tea.Cmd {
	return func() tea.Msg {
		if len(m.jobs) == 0 || m.cursor >= len(m.jobs) {
			return logUpdateMsg{stdout: "", stderr: ""}
		}

		job := m.jobs[m.cursor]
		stdout, _ := os.ReadFile(job.StdoutPath)
		stderr, _ := os.ReadFile(job.StderrPath)

		return logUpdateMsg{
			stdout: string(stdout),
			stderr: string(stderr),
		}
	}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.help.Width = msg.Width

		// Calculate panel sizes
		logWidth := m.width - m.jobPanelWidth()
		totalLogHeight := m.height - 2 // header + status bar
		
		// Stderr gets 20% of height, stdout gets 80%
		stderrHeight := totalLogHeight * 20 / 100
		if stderrHeight < 4 {
			stderrHeight = 4
		}
		stdoutHeight := totalLogHeight - stderrHeight

		m.stdoutView = viewport.New(logWidth-4, stdoutHeight-3)
		m.stderrView = viewport.New(logWidth-4, stderrHeight-3)
		m.jobListView = viewport.New(m.jobPanelWidth()-4, totalLogHeight-3)

	case logTickMsg:
		// Update logs only - job status is handled by events
		cmds = append(cmds, m.readLogs(), logTickCmd())

	case subscriptionStartedMsg:
		m.subscribed = true
		m.subClient = msg.client
		m.eventChan = msg.events
		m.errChan = msg.errs
		// Start waiting for events
		cmds = append(cmds, waitForEvent(m.eventChan, m.errChan))

	case daemonEventMsg:
		// Handle the event by updating the job list
		m.handleDaemonEvent(msg.event)
		// Continue waiting for more events
		if m.subscribed && m.eventChan != nil {
			cmds = append(cmds, waitForEvent(m.eventChan, m.errChan))
		}

	case subscriptionErrorMsg:
		// Subscription failed - close old client and schedule reconnect
		if m.subClient != nil {
			m.subClient.Close()
			m.subClient = nil
		}
		m.subscribed = false
		m.eventChan = nil
		m.errChan = nil
		// Schedule reconnection attempt
		cmds = append(cmds, reconnectCmd())

	case reconnectMsg:
		// Try to reconnect
		cmds = append(cmds, m.startSubscription())

	case jobsUpdatedMsg:
		m.jobs = msg.jobs
		if m.cursor >= len(m.jobs) {
			m.cursor = len(m.jobs) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}

	case logUpdateMsg:
		m.stdoutContent = msg.stdout
		m.stderrContent = msg.stderr
		m.stdoutView.SetContent(m.formatStdout())
		m.stderrView.SetContent(m.formatStderr())
		if m.followLogs {
			m.stdoutView.GotoBottom()
			m.stderrView.GotoBottom()
		}

	case actionResultMsg:
		m.message = msg.message
		m.isError = msg.isError
		m.messageTime = time.Now()
		// No need to refresh jobs - events will handle that

	case tea.KeyMsg:
		// Clear old messages
		if time.Since(m.messageTime) > 3*time.Second {
			m.message = ""
		}

		// Handle modal input first
		if m.modal != modalNone {
			return m.updateModal(msg)
		}

		return m.updateMain(msg)
	}

	return m, tea.Batch(cmds...)
}

// handleDaemonEvent updates the job list based on a daemon event
func (m *Model) handleDaemonEvent(event daemon.Event) {
	switch event.Type {
	case daemon.EventTypeJobAdded:
		// Add job to the beginning of the list (newest first)
		newJob := Job{
			ID:         event.Job.ID,
			PID:        event.Job.PID,
			Command:    strings.Join(event.Job.Command, " "),
			Workdir:    event.Job.Workdir,
			Running:    event.Job.Status == "running",
			StdoutPath: event.Job.StdoutPath,
			StderrPath: event.Job.StderrPath,
			ExitCode:   event.Job.ExitCode,
			StartedAt:  parseTime(event.Job.StartedAt),
			StoppedAt:  parseTime(event.Job.StoppedAt),
		}
		// Only adjust cursor if there are existing jobs (keep selection on same job)
		// When list was empty, cursor stays at 0 to select the new job
		if len(m.jobs) > 0 {
			m.cursor++
		}
		m.jobs = append([]Job{newJob}, m.jobs...)

	case daemon.EventTypeJobStarted:
		// Update job status to running
		for i := range m.jobs {
			if m.jobs[i].ID == event.JobID {
				m.jobs[i].Running = true
				m.jobs[i].PID = event.Job.PID
				m.jobs[i].StartedAt = parseTime(event.Job.StartedAt)
				m.jobs[i].StoppedAt = time.Time{}
				break
			}
		}

	case daemon.EventTypeJobStopped:
		// Update job status to stopped
		for i := range m.jobs {
			if m.jobs[i].ID == event.JobID {
				m.jobs[i].Running = false
				m.jobs[i].ExitCode = event.Job.ExitCode
				m.jobs[i].StoppedAt = parseTime(event.Job.StoppedAt)
				break
			}
		}

	case daemon.EventTypeJobRemoved:
		// Remove job from list
		for i := range m.jobs {
			if m.jobs[i].ID == event.JobID {
				m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
				// Adjust cursor if needed
				if m.cursor >= len(m.jobs) {
					m.cursor = len(m.jobs) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
				break
			}
		}
	}
}

func (m Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalNewJob:
		switch msg.String() {
		case "esc":
			m.modal = modalNone
			return m, nil
		case "enter":
			cmd := m.textInput.Value()
			if cmd != "" {
				m.modal = modalNone
				telemetry.TUIActionExecute("new_job")
				return m, m.addJob(cmd)
			}
		case "ctrl+c":
			if m.subClient != nil {
				m.subClient.Close()
			}
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd

	case modalHelp:
		switch msg.String() {
		case "esc", "?", "q":
			m.modal = modalNone
		case "ctrl+c":
			if m.subClient != nil {
				m.subClient.Close()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		// Close subscription client on quit
		if m.subClient != nil {
			m.subClient.Close()
		}
		return m, tea.Quit

	case "1":
		m.activePanel = panelJobs
		telemetry.TUIActionExecute("switch_panel")

	case "2":
		m.activePanel = panelStdout
		telemetry.TUIActionExecute("switch_panel")

	case "3":
		m.activePanel = panelStderr
		telemetry.TUIActionExecute("switch_panel")

	case "tab":
		switch m.activePanel {
		case panelJobs:
			m.activePanel = panelStdout
		case panelStdout:
			m.activePanel = panelStderr
		case panelStderr:
			m.activePanel = panelJobs
		}
		telemetry.TUIActionExecute("switch_panel")

	case "?":
		m.modal = modalHelp

	case "n":
		m.modal = modalNewJob
		m.textInput.Reset()
		m.textInput.Focus()
		return m, textinput.Blink

	case "a":
		m.showAll = !m.showAll
		m.cursor = 0
		telemetry.TUIActionExecute("toggle_all_dirs")
		// Restart subscription with new filter
		if m.subClient != nil {
			m.subClient.Close()
			m.subClient = nil
		}
		m.subscribed = false
		m.eventChan = nil
		m.errChan = nil
		return m, tea.Batch(m.refreshJobs(), m.startSubscription())

	case "i":
		m.expandedView = !m.expandedView
		telemetry.TUIActionExecute("toggle_details")
	}

	// Panel-specific keys
	if m.activePanel == panelJobs {
		return m.updateJobsPanel(msg)
	}
	return m.updateLogsPanel(msg)
}

func (m Model) updateJobsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.followLogs = true
			return m, m.readLogs()
		}

	case "down", "j":
		if m.cursor < len(m.jobs)-1 {
			m.cursor++
			m.followLogs = true
			return m, m.readLogs()
		}

	case "g":
		m.cursor = 0
		m.followLogs = true
		return m, m.readLogs()

	case "G":
		if len(m.jobs) > 0 {
			m.cursor = len(m.jobs) - 1
			m.followLogs = true
			return m, m.readLogs()
		}

	case "s":
		if len(m.jobs) > 0 && m.jobs[m.cursor].Running {
			telemetry.TUIActionExecute("stop_job")
			return m, m.stopJob(m.jobs[m.cursor].ID, false)
		}

	case "S":
		if len(m.jobs) > 0 && m.jobs[m.cursor].Running {
			telemetry.TUIActionExecute("kill_job")
			return m, m.stopJob(m.jobs[m.cursor].ID, true)
		}

	case "r":
		if len(m.jobs) > 0 {
			telemetry.TUIActionExecute("restart_job")
			return m, m.restartJob(m.jobs[m.cursor].ID)
		}

	case "d":
		if len(m.jobs) > 0 && !m.jobs[m.cursor].Running {
			telemetry.TUIActionExecute("remove_job")
			return m, m.removeJob(m.jobs[m.cursor].ID)
		}

	case "c":
		if len(m.jobs) > 0 {
			telemetry.TUIActionExecute("copy_command")
			err := clipboard.WriteAll(m.jobs[m.cursor].Command)
			if err != nil {
				m.message = fmt.Sprintf("Failed to copy: %v", err)
				m.isError = true
				m.messageTime = time.Now()
			} else {
				m.message = "Command copied to clipboard"
				m.isError = false
				m.messageTime = time.Now()
			}
		}

	case "K":
		m.stdoutView.LineUp(1)
		m.followLogs = false

	case "J":
		m.stdoutView.LineDown(1)
		if m.stdoutView.AtBottom() {
			m.followLogs = true
		}

	case "f":
		m.followLogs = !m.followLogs
		telemetry.TUIActionExecute("toggle_follow")
		if m.followLogs {
			m.stdoutView.GotoBottom()
			m.stderrView.GotoBottom()
		}
	}

	return m, nil
}

func (m Model) updateLogsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Get the active viewport
	activeView := &m.stdoutView
	if m.activePanel == panelStderr {
		activeView = &m.stderrView
	}

	switch msg.String() {
	case "up", "k":
		activeView.LineUp(1)
		m.followLogs = false

	case "down", "j":
		activeView.LineDown(1)
		if activeView.AtBottom() {
			m.followLogs = true
		}

	case "pgup", "ctrl+u":
		activeView.HalfViewUp()
		m.followLogs = false

	case "pgdown", "ctrl+d":
		activeView.HalfViewDown()
		if activeView.AtBottom() {
			m.followLogs = true
		}

	case "g":
		activeView.GotoTop()
		m.followLogs = false

	case "G":
		activeView.GotoBottom()
		m.followLogs = true

	case "f":
		m.followLogs = !m.followLogs
		telemetry.TUIActionExecute("toggle_follow")
		if m.followLogs {
			m.stdoutView.GotoBottom()
			m.stderrView.GotoBottom()
		}
	}

	var cmd tea.Cmd
	*activeView, cmd = activeView.Update(msg)
	return m, cmd
}

// Actions

func (m Model) stopJob(jobID string, force bool) tea.Cmd {
	return func() tea.Msg {
		client, err := connectClient()
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to connect: %v", err), isError: true}
		}
		defer client.Close()

		pid, err := client.Stop(jobID, force)
		if err != nil {
			return actionResultMsg{
				message: fmt.Sprintf("Failed to stop: %v", err),
				isError: true,
			}
		}

		action := "Stopped"
		if force {
			action = "Killed"
		}
		return actionResultMsg{
			message: fmt.Sprintf("%s PID %d", action, pid),
			isError: false,
		}
	}
}

func (m Model) restartJob(jobID string) tea.Cmd {
	return func() tea.Msg {
		client, err := connectClient()
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to connect: %v", err), isError: true}
		}
		defer client.Close()

		job, err := client.Restart(jobID)
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to restart: %v", err), isError: true}
		}

		return actionResultMsg{
			message: fmt.Sprintf("Restarted %s (PID %d)", job.ID, job.PID),
			isError: false,
		}
	}
}

func (m Model) removeJob(jobID string) tea.Cmd {
	return func() tea.Msg {
		client, err := connectClient()
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to connect: %v", err), isError: true}
		}
		defer client.Close()

		_, err = client.Remove(jobID)
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to remove: %v", err), isError: true}
		}

		return actionResultMsg{
			message: fmt.Sprintf("Removed %s", jobID),
			isError: false,
		}
	}
}

func (m Model) addJob(command string) tea.Cmd {
	return func() tea.Msg {
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return actionResultMsg{message: "Empty command", isError: true}
		}

		client, err := connectClient()
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to connect: %v", err), isError: true}
		}
		defer client.Close()

		job, err := client.Add(parts, m.cwd)
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to add: %v", err), isError: true}
		}

		return actionResultMsg{
			message: fmt.Sprintf("Started %s (PID %d)", job.ID, job.PID),
			isError: false,
		}
	}
}

// Formatting

func (m Model) formatStdout() string {
	if len(m.jobs) == 0 || m.cursor >= len(m.jobs) {
		return mutedStyle.Render("No job selected")
	}

	if len(m.stdoutContent) == 0 {
		return mutedStyle.Render("(no output yet)")
	}

	var result strings.Builder
	lines := strings.Split(m.stdoutContent, "\n")
	for _, line := range lines {
		if line != "" {
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}

func (m Model) formatStderr() string {
	if len(m.jobs) == 0 || m.cursor >= len(m.jobs) {
		return ""
	}

	if len(m.stderrContent) == 0 {
		return mutedStyle.Render("(no errors)")
	}

	var result strings.Builder
	stderrStyle := lipgloss.NewStyle().Foreground(warningColor)
	lines := strings.Split(m.stderrContent, "\n")
	for _, line := range lines {
		if line != "" {
			result.WriteString(stderrStyle.Render(line) + "\n")
		}
	}
	return result.String()
}

// Layout helpers

func (m Model) jobPanelWidth() int {
	// 40% of screen or min 40 chars
	w := m.width * 40 / 100
	if w < 40 {
		w = 40
	}
	if w > 60 {
		w = 60
	}
	return w
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	var s strings.Builder

	// Header
	s.WriteString(m.renderHeader())
	s.WriteString("\n")

	// Main panels
	s.WriteString(m.renderPanels())

	// Status bar
	s.WriteString("\n")
	s.WriteString(m.renderStatusBar())

	// Modal overlay
	if m.modal != modalNone {
		return m.renderModal(s.String())
	}

	return s.String()
}

func (m Model) renderHeader() string {
	title := " gob "
	
	// Directory info
	dir := m.shortenPath(m.cwd)
	if m.showAll {
		dir = "all directories"
	}
	
	// Running count
	running := 0
	for _, j := range m.jobs {
		if j.Running {
			running++
		}
	}
	
	stats := fmt.Sprintf(" %d jobs (%d running) ", len(m.jobs), running)
	
	// Build header
	titlePart := headerStyle.Render(title)
	dirPart := statusTextStyle.Render(" " + dir + " ")
	statsPart := headerStyle.Render(stats)
	
	// Fill remaining space
	usedWidth := lipgloss.Width(titlePart) + lipgloss.Width(dirPart) + lipgloss.Width(statsPart)
	gap := m.width - usedWidth
	if gap < 0 {
		gap = 0
	}
	filler := statusBarStyle.Render(strings.Repeat(" ", gap))
	
	return titlePart + dirPart + filler + statsPart
}

func (m Model) renderPanels() string {
	jobPanelW := m.jobPanelWidth()
	logPanelW := m.width - jobPanelW
	totalH := m.height - 2 // height - header - status bar

	// Ensure minimum height
	if totalH < 8 {
		totalH = 8
	}

	// Stderr gets 20% of height, stdout gets 80%
	stderrH := totalH * 20 / 100
	if stderrH < 4 {
		stderrH = 4
	}
	stdoutH := totalH - stderrH

	// Job panel (full height)
	jobContent := m.renderJobList(jobPanelW-4, totalH-2)
	jobPanel := m.renderPanel("1 Jobs", jobContent, jobPanelW, totalH, m.activePanel == panelJobs)

	// Build titles for log panels
	var stdoutTitle, stderrTitle string
	if len(m.jobs) > 0 && m.cursor < len(m.jobs) {
		job := m.jobs[m.cursor]
		var status string
		if job.Running {
			status = "◉"
		} else if job.ExitCode != nil {
			if *job.ExitCode == 0 {
				status = "✓"
			} else {
				status = fmt.Sprintf("✗ %d", *job.ExitCode)
			}
		} else {
			status = "◼"
		}

		// Calculate duration
		var durationStr string
		if !job.StartedAt.IsZero() {
			var d time.Duration
			if job.Running {
				d = time.Since(job.StartedAt)
			} else if !job.StoppedAt.IsZero() {
				d = job.StoppedAt.Sub(job.StartedAt)
			}
			if d > 0 {
				durationStr = " " + formatDuration(d)
			}
		}

		stdoutTitle = fmt.Sprintf("2 stdout: %s %s%s", job.ID, status, durationStr)
		stderrTitle = "3 stderr"
		if m.followLogs {
			stdoutTitle += " [following]"
			stderrTitle += " [following]"
		}
	} else {
		stdoutTitle = "2 stdout"
		stderrTitle = "3 stderr"
	}

	// Stdout panel
	m.stdoutView.Width = logPanelW - 4
	m.stdoutView.Height = stdoutH - 3
	stdoutContent := m.stdoutView.View()
	stdoutPanel := m.renderPanel(stdoutTitle, stdoutContent, logPanelW, stdoutH, m.activePanel == panelStdout)

	// Stderr panel
	m.stderrView.Width = logPanelW - 4
	m.stderrView.Height = stderrH - 3
	stderrContent := m.stderrView.View()
	stderrPanel := m.renderPanel(stderrTitle, stderrContent, logPanelW, stderrH, m.activePanel == panelStderr)

	// Stack stdout and stderr vertically
	logPanels := lipgloss.JoinVertical(lipgloss.Left, stdoutPanel, stderrPanel)

	return lipgloss.JoinHorizontal(lipgloss.Top, jobPanel, logPanels)
}

func (m Model) renderPanel(title, content string, width, height int, active bool) string {
	borderColor := colorBrightBlack
	titleBg := colorBrightBlack
	titleFg := colorWhite
	if active {
		borderColor = primaryColor
		titleBg = primaryColor
		titleFg = colorBlack
	}

	// Border characters
	tl, tr, bl, br := "╭", "╮", "╰", "╯"
	h, v := "─", "│"

	// Title with background
	titleText := " " + title + " "
	styledTitle := lipgloss.NewStyle().
		Background(titleBg).
		Foreground(titleFg).
		Bold(true).
		Render(titleText)
	titleWidth := lipgloss.Width(titleText)

	// Top border with title
	topBorderRight := width - 3 - titleWidth
	if topBorderRight < 0 {
		topBorderRight = 0
	}
	topLine := lipgloss.NewStyle().Foreground(borderColor).Render(tl+h) +
		styledTitle +
		lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat(h, topBorderRight)+tr)

	// Bottom border
	bottomLine := lipgloss.NewStyle().Foreground(borderColor).Render(bl + strings.Repeat(h, width-2) + br)

	// Side borders
	vBorder := lipgloss.NewStyle().Foreground(borderColor).Render(v)

	// Content area
	contentWidth := width - 4  // 2 for borders, 2 for padding
	contentHeight := height - 2

	// Split content into lines and pad/truncate
	contentLines := strings.Split(content, "\n")
	var paddedLines []string
	for i := 0; i < contentHeight; i++ {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		// Truncate if too long (accounting for ANSI codes is tricky, so we use lipgloss)
		lineWidth := lipgloss.Width(line)
		if lineWidth > contentWidth {
			// Simple truncation - may cut ANSI codes but better than overflow
			runes := []rune(line)
			if len(runes) > contentWidth {
				line = string(runes[:contentWidth-1]) + "…"
			}
		}
		// Pad to full width
		padding := contentWidth - lipgloss.Width(line)
		if padding > 0 {
			line = line + strings.Repeat(" ", padding)
		}
		paddedLines = append(paddedLines, vBorder+" "+line+" "+vBorder)
	}

	// Combine all parts
	result := topLine + "\n" + strings.Join(paddedLines, "\n") + "\n" + bottomLine
	return result
}

func (m Model) renderJobList(width, height int) string {
	if len(m.jobs) == 0 {
		return mutedStyle.Render("No jobs. Press 'n' to start one.")
	}

	var lines []string
	for i, job := range m.jobs {
		isSelected := i == m.cursor

		// Status indicator with semantic symbols
		var status string
		if job.Running {
			if isSelected {
				status = jobRunningSelectedStyle.Render("◉")
			} else {
				status = jobRunningStyle.Render("◉")
			}
		} else if job.ExitCode != nil {
			if *job.ExitCode == 0 {
				if isSelected {
					status = jobSuccessSelectedStyle.Render("✓")
				} else {
					status = jobSuccessStyle.Render("✓")
				}
			} else {
				if isSelected {
					status = jobFailedSelectedStyle.Render("✗")
				} else {
					status = jobFailedStyle.Render("✗")
				}
			}
		} else {
			if isSelected {
				status = jobStoppedSelectedStyle.Render("◼")
			} else {
				status = jobStoppedStyle.Render("◼")
			}
		}

		// Exit code (only for failures)
		var exitInfo string
		if job.ExitCode != nil && *job.ExitCode != 0 {
			exitStr := fmt.Sprintf("(%d) ", *job.ExitCode)
			if isSelected {
				exitInfo = jobFailedSelectedStyle.Render(exitStr)
			} else {
				exitInfo = jobFailedStyle.Render(exitStr)
			}
		}

		// Command (truncated)
		maxCmdLen := width - 5 - len(exitInfo)
		if maxCmdLen < 10 {
			maxCmdLen = 10
		}
		cmd := m.truncate(job.Command, maxCmdLen)
		var cmdStyled string
		if isSelected {
			cmdStyled = jobCommandSelectedStyle.Render(cmd)
		} else {
			cmdStyled = cmd
		}

		// Build line 1: symbol + command
		var line string
		if isSelected {
			sp := jobSelectedBgStyle.Render(" ")
			line = sp + status + sp + exitInfo + cmdStyled
			// Pad with styled spaces to fill width
			padding := width - lipgloss.Width(line)
			if padding > 0 {
				line = line + jobSelectedBgStyle.Render(strings.Repeat(" ", padding))
			}
		} else {
			line = fmt.Sprintf(" %s %s%s", status, exitInfo, cmd)
		}

		lines = append(lines, line)

		// Expanded view: add detail lines
		if m.expandedView {
			// Line 2: ID + PID + workdir
			wdStr := m.shortenPath(job.Workdir)

			var detail2 string
			if isSelected {
				sp := jobSelectedBgStyle.Render("   ")
				idStyled := jobIDSelectedStyle.Render(job.ID)
				pidStyled := jobPIDSelectedStyle.Render(fmt.Sprintf("PID %d", job.PID))
				wdStyled := workdirSelectedStyle.Render(wdStr)
				detail2 = sp + idStyled + jobSelectedBgStyle.Render("  ") + pidStyled + jobSelectedBgStyle.Render("  ") + wdStyled
				padding := width - lipgloss.Width(detail2)
				if padding > 0 {
					detail2 = detail2 + jobSelectedBgStyle.Render(strings.Repeat(" ", padding))
				}
			} else {
				idStyled := jobIDStyle.Render(job.ID)
				pidStyled := jobPIDStyle.Render(fmt.Sprintf("PID %d", job.PID))
				wdStyled := workdirStyle.Render(wdStr)
				detail2 = "   " + idStyled + "  " + pidStyled + "  " + wdStyled
			}
			lines = append(lines, detail2)

			// Line 3: timing info
			timeLine := m.formatJobTiming(job)
			var detail3 string
			if isSelected {
				sp := jobSelectedBgStyle.Render("   ")
				timeStyled := jobTimeSelectedStyle.Render(timeLine)
				detail3 = sp + timeStyled
				padding := width - lipgloss.Width(detail3)
				if padding > 0 {
					detail3 = detail3 + jobSelectedBgStyle.Render(strings.Repeat(" ", padding))
				}
			} else {
				detail3 = "   " + jobTimeStyle.Render(timeLine)
			}
			lines = append(lines, detail3)

			// Empty line between jobs (except for last job)
			if i < len(m.jobs)-1 {
				lines = append(lines, "")
			}
		}
	}

	return strings.Join(lines, "\n")
}

// formatJobTiming returns a formatted timing string based on job status
func (m Model) formatJobTiming(job Job) string {
	const timeFmt = "2006-01-02 15:04:05"
	startTime := job.StartedAt.Format(timeFmt)

	if job.Running {
		// Running: "2025-06-12 14:32:05 (2m 30s)" - duration so far
		duration := formatDuration(time.Since(job.StartedAt))
		return fmt.Sprintf("%s (%s)", startTime, duration)
	}
	// Completed or stopped: "2025-06-12 14:30:00 (1m 23s)" - total duration
	duration := formatDuration(job.StoppedAt.Sub(job.StartedAt))
	return fmt.Sprintf("%s (%s)", startTime, duration)
}

func (m Model) renderStatusBar() string {
	var content string

	// Show message instead of shortcuts when there's an active message
	if m.message != "" && time.Since(m.messageTime) < 3*time.Second {
		var styledMessage string
		if m.isError {
			styledMessage = errorStyle.Render(m.message)
		} else {
			styledMessage = successStyle.Render(m.message)
		}
		msgWidth := lipgloss.Width(styledMessage)
		gap := m.width - msgWidth - 2
		if gap < 0 {
			gap = 0
		}
		content = " " + styledMessage + strings.Repeat(" ", gap) + " "
	} else {
		// Show shortcuts
		var parts []string
		switch m.activePanel {
		case panelJobs:
			parts = append(parts,
				m.renderKey("↑↓", "navigate"),
				m.renderKey("s", "stop"),
				m.renderKey("S", "kill"),
				m.renderKey("r", "restart"),
				m.renderKey("d", "delete"),
				m.renderKey("c", "copy"),
				m.renderKey("n", "new"),
				m.renderKey("a", "all dirs"),
				m.renderKey("1/2/3", "panels"),
			)
		case panelStdout:
			parts = append(parts,
				m.renderKey("↑↓", "scroll"),
				m.renderKey("g/G", "top/bottom"),
				m.renderKey("f", "follow"),
				m.renderKey("1/2/3", "panels"),
			)
		case panelStderr:
			parts = append(parts,
				m.renderKey("↑↓", "scroll"),
				m.renderKey("g/G", "top/bottom"),
				m.renderKey("f", "follow"),
				m.renderKey("1/2/3", "panels"),
			)
		}
		parts = append(parts, m.renderKey("?", "help"), m.renderKey("q", "quit"))

		leftSide := strings.Join(parts, " ")
		leftWidth := lipgloss.Width(leftSide)
		gap := m.width - leftWidth - 2
		if gap < 0 {
			gap = 0
		}
		content = " " + leftSide + strings.Repeat(" ", gap) + " "
	}

	return statusBarStyle.Render(content)
}

func (m Model) renderKey(key, desc string) string {
	return helpKeyStyle.Render(key) + " " + helpDescStyle.Render(desc)
}

func (m Model) renderModal(background string) string {
	var content string

	switch m.modal {
	case modalNewJob:
		content = m.renderNewJobModal()
	case modalHelp:
		content = m.renderHelpModal()
	}

	// Center modal on screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderNewJobModal() string {
	title := dialogTitleStyle.Render("Start New Job")
	input := m.textInput.View()
	help := helpDescStyle.Render("enter: start • esc: cancel")

	content := title + "\n\n" + input + "\n\n" + help

	return dialogStyle.Render(content)
}

func (m Model) renderHelpModal() string {
	title := dialogTitleStyle.Render("Keyboard Shortcuts")

	sections := []string{
		helpKeyStyle.Render("Navigation"),
		"  " + m.renderKey("↑/k ↓/j", "move cursor"),
		"  " + m.renderKey("g/G", "first/last"),
		"  " + m.renderKey("tab", "switch panel"),
		"",
		helpKeyStyle.Render("Job Actions"),
		"  " + m.renderKey("s", "stop (SIGTERM)"),
		"  " + m.renderKey("S", "kill (SIGKILL)"),
		"  " + m.renderKey("r", "restart"),
		"  " + m.renderKey("d", "delete stopped"),
		"  " + m.renderKey("c", "copy command"),
		"  " + m.renderKey("n", "new job"),
		"",
		helpKeyStyle.Render("Log Viewer"),
		"  " + m.renderKey("↑/k ↓/j", "scroll"),
		"  " + m.renderKey("pgup/pgdn", "page scroll"),
		"  " + m.renderKey("g/G", "top/bottom"),
		"  " + m.renderKey("f", "toggle follow"),
		"",
		helpKeyStyle.Render("Other"),
		"  " + m.renderKey("i", "toggle details"),
		"  " + m.renderKey("a", "toggle all dirs"),
		"  " + m.renderKey("?", "this help"),
		"  " + m.renderKey("q", "quit"),
	}

	help := helpDescStyle.Render("\npress esc or ? to close")

	content := title + "\n\n" + strings.Join(sections, "\n") + help

	return dialogStyle.Width(45).Render(content)
}

// Helpers

func (m Model) truncate(s string, max int) string {
	if max <= 0 {
		max = 10
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func (m Model) shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// parseTime parses an RFC3339 time string, returning zero time on error
func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05Z07:00", s)
	return t
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// Run starts the TUI
func Run() error {
	telemetry.TUISessionStart()
	defer telemetry.TUISessionEnd()

	p := tea.NewProgram(New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
