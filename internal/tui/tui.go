package tui

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/juanibiapina/gob/internal/telemetry"
	"github.com/juanibiapina/gob/internal/version"
)

// Panel focus
type panel int

const (
	panelJobs panel = iota
	panelPorts
	panelRuns
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
	ID          string
	PID         int
	Command     string
	Description string
	Workdir     string
	Running     bool
	ExitCode    *int
	StartedAt   time.Time
	StoppedAt   time.Time
	Ports       []daemon.PortInfo // Listening ports (only for running jobs)
}

// Run represents a single execution of a job
type Run struct {
	ID         string
	JobID      string
	PID        int
	Status     string
	ExitCode   *int
	StdoutPath string
	StderrPath string
	StartedAt  time.Time
	StoppedAt  time.Time
	DurationMs int64
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

// runsUpdatedMsg is sent when runs are fetched for a job
type runsUpdatedMsg struct {
	jobID string
	runs  []Run
	stats *daemon.StatsResponse
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
	jobs        []Job
	showAll     bool
	activePanel panel
	modal       modalMode
	width       int
	height      int
	ready       bool
	message     string
	messageTime time.Time
	isError     bool
	cwd         string
	env         []string

	// Components
	help        help.Model
	textInput   textinput.Model
	stdoutView  viewport.Model
	stderrView  viewport.Model
	jobListView viewport.Model

	// Log viewer state
	followLogs    bool
	wrapLines     bool
	logPanelWidth int
	stdoutContent string
	stderrContent string

	// Run history state
	runs         []Run
	stats        *daemon.StatsResponse
	runsForJobID string // tracks which job the runs are for

	// Scrollable list states
	jobScroll  ScrollState
	portScroll ScrollState
	runScroll  ScrollState

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
	env := os.Environ()

	return Model{
		jobs:        []Job{},
		showAll:     false,
		activePanel: panelJobs,
		modal:       modalNone,
		help:        h,
		textInput:   ti,
		cwd:         cwd,
		env:         env,
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
				ID:          jr.ID,
				PID:         jr.PID,
				Command:     strings.Join(jr.Command, " "),
				Description: jr.Description,
				Workdir:     jr.Workdir,
				Running:     jr.Status == "running",
				ExitCode:    jr.ExitCode,
				StartedAt:   parseTime(jr.StartedAt),
				StoppedAt:   parseTime(jr.StoppedAt),
				Ports:       jr.Ports,
			}
		}

		return jobsUpdatedMsg{jobs: jobs}
	}
}

// readLogs reads the log files for the selected run
func (m Model) readLogs() tea.Cmd {
	return func() tea.Msg {
		if len(m.runs) == 0 || m.runScroll.Cursor < 0 || m.runScroll.Cursor >= len(m.runs) {
			return logUpdateMsg{stdout: "", stderr: ""}
		}

		run := m.runs[m.runScroll.Cursor]
		stdout, _ := os.ReadFile(run.StdoutPath)
		stderr, _ := os.ReadFile(run.StderrPath)

		return logUpdateMsg{
			stdout: string(stdout),
			stderr: string(stderr),
		}
	}
}

// fetchRuns fetches runs and stats for a job
func (m Model) fetchRuns(jobID string) tea.Cmd {
	return func() tea.Msg {
		client, err := connectClient()
		if err != nil {
			return runsUpdatedMsg{jobID: jobID, runs: nil, stats: nil}
		}
		defer client.Close()

		// Fetch runs
		runsResp, err := client.Runs(jobID)
		if err != nil {
			return runsUpdatedMsg{jobID: jobID, runs: nil, stats: nil}
		}

		runs := make([]Run, len(runsResp))
		for i, r := range runsResp {
			runs[i] = Run{
				ID:         r.ID,
				JobID:      r.JobID,
				PID:        r.PID,
				Status:     r.Status,
				ExitCode:   r.ExitCode,
				StdoutPath: r.StdoutPath,
				StderrPath: r.StderrPath,
				StartedAt:  parseTime(r.StartedAt),
				StoppedAt:  parseTime(r.StoppedAt),
				DurationMs: r.DurationMs,
			}
		}

		// Fetch stats
		stats, _ := client.Stats(jobID)

		return runsUpdatedMsg{jobID: jobID, runs: runs, stats: stats}
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

		// Store log panel width for line wrapping
		m.logPanelWidth = logWidth - 4

		m.stdoutView = viewport.New(logWidth-4, stdoutHeight-3)
		m.stderrView = viewport.New(logWidth-4, stderrHeight-3)
		m.jobListView = viewport.New(m.jobPanelWidth()-4, totalLogHeight-3)

		// Enable horizontal scrolling on log viewports
		m.stdoutView.SetHorizontalStep(4)
		m.stderrView.SetHorizontalStep(4)

		// Calculate visible rows for scrollable panels (matches renderPanels layout)
		totalH := m.height - 1 // height - status bar
		if totalH < 8 {
			totalH = 8
		}
		infoH := 3
		leftH := totalH - infoH
		portsH := leftH * 20 / 100
		if portsH < 4 {
			portsH = 4
		}
		runsH := leftH * 30 / 100
		if runsH < 5 {
			runsH = 5
		}
		jobsH := leftH - portsH - runsH

		// Set visible rows for each scroll state
		m.jobScroll.VisibleRows = jobsH - 2   // panel height - border (2)
		m.portScroll.VisibleRows = portsH - 3 // panel height - border (2) - header (1)
		m.runScroll.VisibleRows = runsH - 4   // panel height - border (2) - stats (1) - empty line (1)
		if m.jobScroll.VisibleRows < 1 {
			m.jobScroll.VisibleRows = 1
		}
		if m.portScroll.VisibleRows < 1 {
			m.portScroll.VisibleRows = 1
		}
		if m.runScroll.VisibleRows < 1 {
			m.runScroll.VisibleRows = 1
		}

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
		// Handle the event by updating the job list and runs
		m.handleDaemonEvent(msg.event)
		// Fetch runs if the selected job changed (e.g., first job added)
		if len(m.jobs) > 0 && m.jobScroll.Cursor < len(m.jobs) {
			jobID := m.jobs[m.jobScroll.Cursor].ID
			if jobID != m.runsForJobID {
				m.runsForJobID = jobID
				m.runScroll.Reset()
				cmds = append(cmds, m.fetchRuns(jobID))
			}
		}
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
		m.jobScroll.ClampToCount(len(m.jobs))
		// Fetch runs for the selected job (ports come from events)
		if len(m.jobs) > 0 {
			jobID := m.jobs[m.jobScroll.Cursor].ID
			if jobID != m.runsForJobID {
				m.runsForJobID = jobID
				m.runScroll.Reset()
				m.portScroll.Reset()
				cmds = append(cmds, m.fetchRuns(jobID))
			}
		}

	case runsUpdatedMsg:
		// Only update if this is for the currently selected job
		if msg.jobID == m.runsForJobID {
			m.runs = msg.runs
			m.stats = msg.stats
			m.runScroll.ClampToCount(len(m.runs))
			// Read logs now that runs are loaded
			cmds = append(cmds, m.readLogs())
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
			ID:          event.Job.ID,
			PID:         event.Job.PID,
			Command:     strings.Join(event.Job.Command, " "),
			Description: event.Job.Description,
			Workdir:     event.Job.Workdir,
			Running:     event.Job.Status == "running",
			ExitCode:    event.Job.ExitCode,
			StartedAt:   parseTime(event.Job.StartedAt),
			StoppedAt:   parseTime(event.Job.StoppedAt),
			Ports:       event.Job.Ports,
		}
		m.jobs = append([]Job{newJob}, m.jobs...)
		// Select the new job and scroll to top
		m.jobScroll.First()

	case daemon.EventTypeJobStarted:
		// Find the job, update its status, and move it to the top
		for i := range m.jobs {
			if m.jobs[i].ID == event.JobID {
				// Update job status
				m.jobs[i].Running = true
				m.jobs[i].PID = event.Job.PID
				m.jobs[i].StartedAt = parseTime(event.Job.StartedAt)
				m.jobs[i].StoppedAt = time.Time{}

				// Move job to the top of the list (most recently run first)
				if i > 0 {
					job := m.jobs[i]
					// Remove from current position
					m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
					// Prepend to list
					m.jobs = append([]Job{job}, m.jobs...)
					// Adjust cursor to keep selection on same job
					// Note: We only shift cursor, not offset, so the moved job is visible at top
					if m.jobScroll.Cursor == i {
						// Selected job moved to top
						m.jobScroll.SetCursorTo(0)
					} else if m.jobScroll.Cursor < i {
						// Selected job was above the moved job, shift cursor down by 1
						m.jobScroll.Cursor++
					}
					// If cursor > i, the selected job stays at same index
				}
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
				m.jobs[i].Ports = nil // Clear ports when job stops
				break
			}
		}

	case daemon.EventTypeJobRemoved:
		// Remove job from list
		for i := range m.jobs {
			if m.jobs[i].ID == event.JobID {
				m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
				m.jobScroll.ClampToCount(len(m.jobs))
				// Clear runs if this was the selected job
				if event.JobID == m.runsForJobID {
					m.runs = nil
					m.stats = nil
					m.runScroll.Reset()
					m.runsForJobID = ""
				}
				break
			}
		}

	case daemon.EventTypeRunStarted:
		// Add new run to the runs list if it's for the selected job
		if event.Run != nil && event.JobID == m.runsForJobID {
			newRun := Run{
				ID:         event.Run.ID,
				JobID:      event.Run.JobID,
				PID:        event.Run.PID,
				Status:     event.Run.Status,
				ExitCode:   event.Run.ExitCode,
				StdoutPath: event.Run.StdoutPath,
				StderrPath: event.Run.StderrPath,
				StartedAt:  parseTime(event.Run.StartedAt),
				StoppedAt:  parseTime(event.Run.StoppedAt),
				DurationMs: event.Run.DurationMs,
			}
			// Prepend new run to the list (newest first)
			m.runs = append([]Run{newRun}, m.runs...)
			// Update stats from event
			if event.Stats != nil {
				m.stats = event.Stats
			}
		}

	case daemon.EventTypeRunStopped:
		// Update run status in the runs list if it's for the selected job
		if event.Run != nil && event.JobID == m.runsForJobID {
			for i := range m.runs {
				if m.runs[i].ID == event.Run.ID {
					m.runs[i].Status = event.Run.Status
					m.runs[i].ExitCode = event.Run.ExitCode
					m.runs[i].StoppedAt = parseTime(event.Run.StoppedAt)
					m.runs[i].DurationMs = event.Run.DurationMs
					break
				}
			}
			// Update stats from event
			if event.Stats != nil {
				m.stats = event.Stats
			}
		}

	case daemon.EventTypePortsUpdated:
		// Update ports for a specific job
		for i := range m.jobs {
			if m.jobs[i].ID == event.JobID {
				m.jobs[i].Ports = event.Ports
				// Bounds check port cursor if this is the selected job
				if i == m.jobScroll.Cursor {
					m.portScroll.ClampToCount(len(event.Ports))
				}
				break
			}
		}

	case daemon.EventTypeJobUpdated:
		// Update job properties (e.g., description change)
		for i := range m.jobs {
			if m.jobs[i].ID == event.JobID {
				m.jobs[i].Description = event.Job.Description
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
		m.activePanel = panelPorts
		telemetry.TUIActionExecute("switch_panel")

	case "3":
		m.activePanel = panelRuns
		telemetry.TUIActionExecute("switch_panel")

	case "4":
		m.activePanel = panelStdout
		telemetry.TUIActionExecute("switch_panel")

	case "5":
		m.activePanel = panelStderr
		telemetry.TUIActionExecute("switch_panel")

	case "tab":
		switch m.activePanel {
		case panelJobs:
			m.activePanel = panelPorts
		case panelPorts:
			m.activePanel = panelRuns
		case panelRuns:
			m.activePanel = panelStdout
		case panelStdout:
			m.activePanel = panelStderr
		case panelStderr:
			m.activePanel = panelJobs
		}
		telemetry.TUIActionExecute("switch_panel")

	case "shift+tab":
		switch m.activePanel {
		case panelJobs:
			m.activePanel = panelStderr
		case panelPorts:
			m.activePanel = panelJobs
		case panelRuns:
			m.activePanel = panelPorts
		case panelStdout:
			m.activePanel = panelRuns
		case panelStderr:
			m.activePanel = panelStdout
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
		m.jobScroll.Reset()
		m.runScroll.Reset()
		m.portScroll.Reset()
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
	}

	// Panel-specific keys
	switch m.activePanel {
	case panelJobs:
		return m.updateJobsPanel(msg)
	case panelPorts:
		return m.updatePortsPanel(msg)
	case panelRuns:
		return m.updateRunsPanel(msg)
	default:
		return m.updateLogsPanel(msg)
	}
}

func (m Model) updateJobsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.jobScroll.Up() {
			m.followLogs = true
			m.runScroll.Reset()
			m.runs = nil
			m.stats = nil
			m.stdoutContent = ""
			m.stderrContent = ""
			m.portScroll.Reset()
			if len(m.jobs) > 0 {
				m.runsForJobID = m.jobs[m.jobScroll.Cursor].ID
			}
			return m, m.fetchRunsForSelectedJob()
		}

	case "down", "j":
		if m.jobScroll.Down(len(m.jobs)) {
			m.followLogs = true
			m.runScroll.Reset()
			m.runs = nil
			m.stats = nil
			m.stdoutContent = ""
			m.stderrContent = ""
			m.portScroll.Reset()
			if len(m.jobs) > 0 {
				m.runsForJobID = m.jobs[m.jobScroll.Cursor].ID
			}
			return m, m.fetchRunsForSelectedJob()
		}

	case "g":
		m.jobScroll.First()
		m.followLogs = true
		m.runScroll.Reset()
		m.runs = nil
		m.stats = nil
		m.stdoutContent = ""
		m.stderrContent = ""
		m.portScroll.Reset()
		if len(m.jobs) > 0 {
			m.runsForJobID = m.jobs[m.jobScroll.Cursor].ID
		}
		return m, m.fetchRunsForSelectedJob()

	case "G":
		if len(m.jobs) > 0 {
			m.jobScroll.Last(len(m.jobs))
			m.followLogs = true
			m.runScroll.Reset()
			m.runs = nil
			m.stats = nil
			m.stdoutContent = ""
			m.stderrContent = ""
			m.portScroll.Reset()
			m.runsForJobID = m.jobs[m.jobScroll.Cursor].ID
			return m, m.fetchRunsForSelectedJob()
		}

	case "s":
		if len(m.jobs) > 0 && m.jobs[m.jobScroll.Cursor].Running {
			telemetry.TUIActionExecute("stop_job")
			return m, m.stopJob(m.jobs[m.jobScroll.Cursor].ID, false)
		}

	case "S":
		if len(m.jobs) > 0 && m.jobs[m.jobScroll.Cursor].Running {
			telemetry.TUIActionExecute("kill_job")
			return m, m.stopJob(m.jobs[m.jobScroll.Cursor].ID, true)
		}

	case "r":
		if len(m.jobs) > 0 {
			telemetry.TUIActionExecute("restart_job")
			return m, m.restartJob(m.jobs[m.jobScroll.Cursor].ID)
		}

	case "d":
		if len(m.jobs) > 0 && !m.jobs[m.jobScroll.Cursor].Running {
			telemetry.TUIActionExecute("remove_job")
			return m, m.removeJob(m.jobs[m.jobScroll.Cursor].ID)
		}

	case "c":
		if len(m.jobs) > 0 {
			telemetry.TUIActionExecute("copy_command")
			err := clipboard.WriteAll(m.jobs[m.jobScroll.Cursor].Command)
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

	case "H":
		m.stdoutView.ScrollLeft(4)
		m.stderrView.ScrollLeft(4)

	case "L":
		m.stdoutView.ScrollRight(4)
		m.stderrView.ScrollRight(4)

	case "f":
		m.followLogs = !m.followLogs
		telemetry.TUIActionExecute("toggle_follow")
		if m.followLogs {
			m.stdoutView.GotoBottom()
			m.stderrView.GotoBottom()
		}

	case "w":
		m.wrapLines = !m.wrapLines
		telemetry.TUIActionExecute("toggle_wrap")
		m.stdoutView.SetContent(m.formatStdout())
		m.stderrView.SetContent(m.formatStderr())
		m.stdoutView.SetXOffset(0)
		m.stderrView.SetXOffset(0)
	}

	return m, nil
}

// fetchRunsForSelectedJob returns a command to fetch runs for the selected job
// Note: caller must set m.runsForJobID before calling this
// Ports are received via daemon events (EventTypePortsUpdated)
func (m Model) fetchRunsForSelectedJob() tea.Cmd {
	if len(m.jobs) == 0 || m.jobScroll.Cursor >= len(m.jobs) {
		return nil
	}
	jobID := m.jobs[m.jobScroll.Cursor].ID
	return m.fetchRuns(jobID)
}

func (m Model) updatePortsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Get current job's port count for bounds checking
	portCount := 0
	if len(m.jobs) > 0 && m.jobScroll.Cursor < len(m.jobs) && m.jobs[m.jobScroll.Cursor].Running {
		portCount = len(m.jobs[m.jobScroll.Cursor].Ports)
	}

	switch msg.String() {
	case "up", "k":
		m.portScroll.Up()

	case "down", "j":
		m.portScroll.Down(portCount)

	case "g":
		m.portScroll.First()

	case "G":
		m.portScroll.Last(portCount)

	case "f":
		m.followLogs = !m.followLogs
		telemetry.TUIActionExecute("toggle_follow")
		if m.followLogs {
			m.stdoutView.GotoBottom()
			m.stderrView.GotoBottom()
		}

	case "H":
		m.stdoutView.ScrollLeft(4)
		m.stderrView.ScrollLeft(4)

	case "L":
		m.stdoutView.ScrollRight(4)
		m.stderrView.ScrollRight(4)

	case "w":
		m.wrapLines = !m.wrapLines
		telemetry.TUIActionExecute("toggle_wrap")
		m.stdoutView.SetContent(m.formatStdout())
		m.stderrView.SetContent(m.formatStderr())
		m.stdoutView.SetXOffset(0)
		m.stderrView.SetXOffset(0)
	}

	return m, nil
}

func (m Model) updateRunsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.runScroll.Up() {
			m.followLogs = true
			return m, m.readLogs()
		}

	case "down", "j":
		if m.runScroll.Down(len(m.runs)) {
			m.followLogs = true
			return m, m.readLogs()
		}

	case "g":
		m.runScroll.First()
		m.followLogs = true
		return m, m.readLogs()

	case "G":
		if len(m.runs) > 0 {
			m.runScroll.Last(len(m.runs))
			m.followLogs = true
			return m, m.readLogs()
		}

	case "f":
		m.followLogs = !m.followLogs
		telemetry.TUIActionExecute("toggle_follow")
		if m.followLogs {
			m.stdoutView.GotoBottom()
			m.stderrView.GotoBottom()
		}

	case "H":
		m.stdoutView.ScrollLeft(4)
		m.stderrView.ScrollLeft(4)

	case "L":
		m.stdoutView.ScrollRight(4)
		m.stderrView.ScrollRight(4)

	case "w":
		m.wrapLines = !m.wrapLines
		telemetry.TUIActionExecute("toggle_wrap")
		m.stdoutView.SetContent(m.formatStdout())
		m.stderrView.SetContent(m.formatStderr())
		m.stdoutView.SetXOffset(0)
		m.stderrView.SetXOffset(0)
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

	case "left", "h":
		activeView.ScrollLeft(4)

	case "right", "l":
		activeView.ScrollRight(4)

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

	case "w":
		m.wrapLines = !m.wrapLines
		telemetry.TUIActionExecute("toggle_wrap")
		// Re-apply content with new wrap setting
		m.stdoutView.SetContent(m.formatStdout())
		m.stderrView.SetContent(m.formatStderr())
		// Reset horizontal scroll when toggling wrap
		m.stdoutView.SetXOffset(0)
		m.stderrView.SetXOffset(0)
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

		job, err := client.Restart(jobID, m.env)
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

		result, err := client.Add(parts, m.cwd, m.env, "")
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to add: %v", err), isError: true}
		}

		return actionResultMsg{
			message: fmt.Sprintf("Started %s (PID %d)", result.Job.ID, result.Job.PID),
			isError: false,
		}
	}
}

// Formatting

func (m Model) formatStdout() string {
	if len(m.jobs) == 0 || m.jobScroll.Cursor >= len(m.jobs) {
		return mutedStyle.Render("No job selected")
	}

	if len(m.stdoutContent) == 0 {
		return mutedStyle.Render("(no output yet)")
	}

	// Strip cursor movement sequences that break TUI rendering
	content := StripCursorSequences(m.stdoutContent)

	// Apply line wrapping if enabled
	if m.wrapLines && m.logPanelWidth > 0 {
		content = ansi.Wrap(content, m.logPanelWidth, " ")
	}

	var result strings.Builder
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}

func (m Model) formatStderr() string {
	if len(m.jobs) == 0 || m.jobScroll.Cursor >= len(m.jobs) {
		return ""
	}

	if len(m.stderrContent) == 0 {
		return mutedStyle.Render("(no errors)")
	}

	// Strip cursor movement sequences that break TUI rendering
	content := StripCursorSequences(m.stderrContent)

	// Apply line wrapping if enabled
	if m.wrapLines && m.logPanelWidth > 0 {
		content = ansi.Wrap(content, m.logPanelWidth, " ")
	}

	var result strings.Builder
	stderrStyle := lipgloss.NewStyle().Foreground(warningColor)
	lines := strings.Split(content, "\n")
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

func (m Model) renderPanels() string {
	leftPanelW := m.jobPanelWidth()
	rightPanelW := m.width - leftPanelW
	totalH := m.height - 1 // height - status bar

	// Ensure minimum height
	if totalH < 8 {
		totalH = 8
	}

	// Info panel is fixed at 3 lines (border + 1 content + border)
	infoH := 3

	// Description panel is fixed at 3 lines when shown
	hasDescription := m.selectedJobHasDescription()
	descH := 0
	if hasDescription {
		descH = 3
	}

	leftH := totalH - infoH - descH

	// Left side: Jobs (50%) + Ports (20%) + Runs (30%) of remaining height
	portsH := leftH * 20 / 100
	if portsH < 4 {
		portsH = 4
	}
	runsH := leftH * 30 / 100
	if runsH < 5 {
		runsH = 5
	}
	jobsH := leftH - portsH - runsH

	// Right side: Stdout (80%) + Stderr (20%)
	stderrH := totalH * 20 / 100
	if stderrH < 4 {
		stderrH = 4
	}
	stdoutH := totalH - stderrH

	// Info panel (logo + directory + version)
	dir := m.shortenPath(m.cwd)
	if m.showAll {
		dir = "all directories"
	}
	infoPanel := m.renderInfoPanel("gob", dir, version.Version, leftPanelW, infoH)

	// Jobs panel
	jobContent := m.renderJobList(leftPanelW - 4)
	jobPanel := m.renderPanel(1, "Jobs", jobContent, leftPanelW, jobsH, m.activePanel == panelJobs)

	// Description panel (only if selected job has a description)
	var descPanel string
	if hasDescription {
		descContent := m.renderDescriptionContent(leftPanelW - 4)
		descPanel = m.renderDescriptionPanel(descContent, leftPanelW, descH)
	}

	// Ports panel
	portsTitle := "Ports"
	if len(m.jobs) > 0 && m.jobScroll.Cursor < len(m.jobs) {
		portsTitle = fmt.Sprintf("Ports: %s", m.jobs[m.jobScroll.Cursor].ID)
	}
	portsContent := m.renderPortsList(leftPanelW - 4)
	portsPanel := m.renderPanel(2, portsTitle, portsContent, leftPanelW, portsH, m.activePanel == panelPorts)

	// Runs panel
	runsTitle := "Runs"
	if len(m.jobs) > 0 && m.jobScroll.Cursor < len(m.jobs) {
		runsTitle = fmt.Sprintf("Runs: %s", m.jobs[m.jobScroll.Cursor].ID)
	}
	runsContent := m.renderRunsList(leftPanelW - 4)
	runsPanel := m.renderPanel(3, runsTitle, runsContent, leftPanelW, runsH, m.activePanel == panelRuns)

	// Build titles for log panels
	var stdoutTitle, stderrTitle string
	if len(m.jobs) > 0 && m.jobScroll.Cursor < len(m.jobs) {
		job := m.jobs[m.jobScroll.Cursor]

		// Check if showing a specific run
		var showingRunID string
		var runStatus string
		var durationStr string

		if len(m.runs) > 0 && m.runScroll.Cursor >= 0 && m.runScroll.Cursor < len(m.runs) {
			// Showing a specific run
			run := m.runs[m.runScroll.Cursor]
			showingRunID = run.ID

			if run.Status == "running" {
				runStatus = "◉"
				if !run.StartedAt.IsZero() {
					durationStr = " " + formatDuration(time.Since(run.StartedAt))
				}
			} else if run.ExitCode != nil {
				if *run.ExitCode == 0 {
					runStatus = "✓"
				} else {
					runStatus = fmt.Sprintf("✗ %d", *run.ExitCode)
				}
				durationStr = " " + formatDuration(time.Duration(run.DurationMs)*time.Millisecond)
			} else {
				runStatus = "◼"
			}
		} else {
			// Showing current/latest run (the job's current state)
			showingRunID = job.ID

			if job.Running {
				runStatus = "◉"
			} else if job.ExitCode != nil {
				if *job.ExitCode == 0 {
					runStatus = "✓"
				} else {
					runStatus = fmt.Sprintf("✗ %d", *job.ExitCode)
				}
			} else {
				runStatus = "◼"
			}

			// Calculate duration
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
		}

		stdoutTitle = fmt.Sprintf("stdout: %s %s%s", showingRunID, runStatus, durationStr)
		stderrTitle = "stderr"
		if m.followLogs {
			stdoutTitle += " [following]"
			stderrTitle += " [following]"
		}
		if m.wrapLines {
			stdoutTitle += " [wrap]"
			stderrTitle += " [wrap]"
		}
	} else {
		stdoutTitle = "stdout"
		stderrTitle = "stderr"
		if m.wrapLines {
			stdoutTitle += " [wrap]"
			stderrTitle += " [wrap]"
		}
	}

	// Stdout panel
	m.stdoutView.Width = rightPanelW - 4
	m.stdoutView.Height = stdoutH - 3
	stdoutContent := m.stdoutView.View()
	stdoutPanel := m.renderPanel(4, stdoutTitle, stdoutContent, rightPanelW, stdoutH, m.activePanel == panelStdout)

	// Stderr panel
	m.stderrView.Width = rightPanelW - 4
	m.stderrView.Height = stderrH - 3
	stderrContent := m.stderrView.View()
	stderrPanel := m.renderPanel(5, stderrTitle, stderrContent, rightPanelW, stderrH, m.activePanel == panelStderr)

	// Stack panels
	var leftPanels string
	if hasDescription {
		leftPanels = lipgloss.JoinVertical(lipgloss.Left, infoPanel, jobPanel, descPanel, portsPanel, runsPanel)
	} else {
		leftPanels = lipgloss.JoinVertical(lipgloss.Left, infoPanel, jobPanel, portsPanel, runsPanel)
	}
	rightPanels := lipgloss.JoinVertical(lipgloss.Left, stdoutPanel, stderrPanel)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanels, rightPanels)
}

// renderInfoPanel renders the info panel with logo, directory (left) and version (right)
func (m Model) renderInfoPanel(logo, dir, ver string, width, height int) string {
	borderColor := colorBlue
	textColor := colorWhite

	// Border characters
	tl, tr, bl, br := "╭", "╮", "╰", "╯"
	h, v := "─", "│"

	// Top border
	topLine := lipgloss.NewStyle().Foreground(borderColor).Render(tl + strings.Repeat(h, width-2) + tr)

	// Bottom border
	bottomLine := lipgloss.NewStyle().Foreground(borderColor).Render(bl + strings.Repeat(h, width-2) + br)

	// Side borders
	vBorder := lipgloss.NewStyle().Foreground(borderColor).Render(v)

	// Content area with logo+dir left-aligned and version right-aligned
	contentWidth := width - 4 // 2 for borders, 2 for padding
	styledLogo := lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render(logo)
	styledDir := lipgloss.NewStyle().Foreground(textColor).Render(dir)
	styledVer := lipgloss.NewStyle().Foreground(textColor).Render(ver)

	leftPart := styledLogo + "  " + styledDir
	leftWidth := lipgloss.Width(logo) + 2 + lipgloss.Width(dir)

	gap := contentWidth - leftWidth - lipgloss.Width(ver)
	if gap < 1 {
		gap = 1
	}
	line := leftPart + strings.Repeat(" ", gap) + styledVer
	contentLine := vBorder + " " + FitToWidth(line, contentWidth) + " " + vBorder

	return topLine + "\n" + contentLine + "\n" + bottomLine
}

// renderDescriptionContent renders the description text for the selected job
func (m Model) renderDescriptionContent(width int) string {
	if len(m.jobs) == 0 || m.jobScroll.Cursor >= len(m.jobs) {
		return ""
	}

	job := m.jobs[m.jobScroll.Cursor]
	if job.Description == "" {
		return ""
	}

	// Truncate the description to fit the panel width
	desc := job.Description
	if len(desc) > width {
		desc = desc[:width-1] + "…"
	}
	return desc
}

// selectedJobHasDescription returns true if the selected job has a description
func (m Model) selectedJobHasDescription() bool {
	if len(m.jobs) == 0 || m.jobScroll.Cursor >= len(m.jobs) {
		return false
	}
	return m.jobs[m.jobScroll.Cursor].Description != ""
}

// renderDescriptionPanel renders a small panel showing the job description
func (m Model) renderDescriptionPanel(content string, width, height int) string {
	borderColor := colorBlue

	// Border characters
	tl, tr, bl, br := "╭", "╮", "╰", "╯"
	h, v := "─", "│"

	// Title
	title := "Description"
	styledTitle := lipgloss.NewStyle().Foreground(borderColor).Render(title)

	// Top border with title
	topBorderRight := width - 2 - len(title) - 1
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
	contentWidth := width - 4 // 2 for borders, 2 for padding
	contentHeight := height - 2

	// Description text uses normal foreground color for visibility
	styledContent := content

	// Build content lines
	var paddedLines []string
	for i := 0; i < contentHeight; i++ {
		var line string
		if i == 0 {
			line = styledContent
		}
		line = FitToWidth(line, contentWidth)
		paddedLines = append(paddedLines, vBorder+" "+line+" "+vBorder)
	}

	return topLine + "\n" + strings.Join(paddedLines, "\n") + "\n" + bottomLine
}

// renderRunsList renders the runs list for the selected job
func (m Model) renderRunsList(width int) string {
	if len(m.jobs) == 0 {
		return mutedStyle.Render("No job selected")
	}

	if m.stats == nil {
		return mutedStyle.Render("Loading...")
	}

	if len(m.runs) == 0 {
		return mutedStyle.Render("No runs yet")
	}

	var lines []string

	// Stats summary line
	statsLine := m.formatStatsLine()
	lines = append(lines, mutedStyle.Render(statsLine))
	lines = append(lines, "")

	// Calculate column widths based on panel width
	// Layout: [space][status 3][space][id 30%][space][time 35%][space][duration 35%]
	// Account for spacing: 1 leading space + 3 separating spaces = 4 total
	const statusWidth = 3
	const numSpaces = 4
	remainingWidth := width - statusWidth - numSpaces
	if remainingWidth < 9 {
		remainingWidth = 9 // Minimum: 3 chars per column
	}
	idWidth := remainingWidth * 30 / 100
	timeWidth := remainingWidth * 35 / 100
	durationWidth := remainingWidth - idWidth - timeWidth // Give remainder to duration

	// Ensure minimum widths
	if idWidth < 3 {
		idWidth = 3
	}
	if timeWidth < 3 {
		timeWidth = 3
	}
	if durationWidth < 3 {
		durationWidth = 3
	}

	// Run list (only visible runs)
	start, end := m.runScroll.VisibleRange(len(m.runs))
	for i := start; i < end; i++ {
		run := m.runs[i]
		isSelected := i == m.runScroll.Cursor && m.activePanel == panelRuns
		runLine := m.formatRunListLine(run, isSelected, width, statusWidth, idWidth, timeWidth, durationWidth)
		lines = append(lines, runLine)
	}

	return strings.Join(lines, "\n")
}

// renderPortsList renders the ports list for the selected job
func (m Model) renderPortsList(width int) string {
	if len(m.jobs) == 0 {
		return mutedStyle.Render("No job selected")
	}

	job := m.jobs[m.jobScroll.Cursor]

	if !job.Running {
		return mutedStyle.Render("Job is not running")
	}

	if len(job.Ports) == 0 {
		return mutedStyle.Render("No open ports")
	}

	// Sort ports by port number (lower first)
	ports := make([]daemon.PortInfo, len(job.Ports))
	copy(ports, job.Ports)
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})

	// Table header
	header := fmt.Sprintf("%-6s %-6s %-15s %s", "PORT", "PROTO", "ADDRESS", "PID")
	lines := []string{mutedStyle.Render(header)}

	// Port rows (only visible ones)
	start, end := m.portScroll.VisibleRange(len(ports))
	for i := start; i < end; i++ {
		p := ports[i]
		isSelected := i == m.portScroll.Cursor && m.activePanel == panelPorts
		line := m.formatPortLine(p, isSelected, width)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// formatPortLine formats a single port line with optional selection highlighting
func (m Model) formatPortLine(p daemon.PortInfo, isSelected bool, width int) string {
	if isSelected {
		sp := jobSelectedBgStyle.Render(" ")
		sp2 := jobSelectedBgStyle.Render("  ")
		portStr := jobSelectedBgStyle.Render(fmt.Sprintf("%-5d", p.Port))
		protoStr := jobSelectedBgStyle.Render(fmt.Sprintf("%-6s", p.Protocol))
		addrStr := jobSelectedBgStyle.Render(fmt.Sprintf("%-15s", p.Address))
		pidStr := jobSelectedBgStyle.Render(fmt.Sprintf("%d", p.PID))
		line := sp + portStr + sp2 + protoStr + sp2 + addrStr + sp2 + pidStr
		// Pad to fill width
		padding := width - lipgloss.Width(line)
		if padding > 0 {
			line = line + jobSelectedBgStyle.Render(strings.Repeat(" ", padding))
		}
		return line
	}
	return fmt.Sprintf(" %-5d  %-6s  %-15s  %d", p.Port, p.Protocol, p.Address, p.PID)
}

// formatRunListLine formats a single run line for the runs panel
func (m Model) formatRunListLine(run Run, isSelected bool, width, statusWidth, idWidth, timeWidth, durationWidth int) string {
	// Status indicator (3 chars: icon padded, or right-aligned exit code)
	var statusText string
	var statusStyle, statusSelectedStyle lipgloss.Style

	if run.Status == "running" {
		statusText = "◉"
		statusStyle = jobRunningStyle
		statusSelectedStyle = jobRunningSelectedStyle
	} else if run.ExitCode != nil {
		if *run.ExitCode == 0 {
			statusText = "✓"
			statusStyle = jobSuccessStyle
			statusSelectedStyle = jobSuccessSelectedStyle
		} else {
			// Right-align exit code in 3 chars
			statusText = fmt.Sprintf("%3d", *run.ExitCode)
			if *run.ExitCode > 999 {
				statusText = "999" // Cap display at 999
			}
			statusStyle = jobFailedStyle
			statusSelectedStyle = jobFailedSelectedStyle
		}
	} else {
		statusText = "◼"
		statusStyle = jobStoppedStyle
		statusSelectedStyle = jobStoppedSelectedStyle
	}

	// Pad status to statusWidth (for non-exit-code statuses)
	if len(statusText) < statusWidth {
		statusText = statusText + strings.Repeat(" ", statusWidth-len(statusText))
	}

	// Relative time
	relTime := formatRelativeTime(run.StartedAt)

	// Duration
	var duration string
	if run.Status == "running" {
		duration = formatDuration(time.Since(run.StartedAt))
	} else {
		duration = formatDuration(time.Duration(run.DurationMs) * time.Millisecond)
	}

	// Build the line with fixed-width columns
	if isSelected {
		sp := jobSelectedBgStyle.Render(" ")
		statusStyled := statusSelectedStyle.Render(statusText)
		idStyled := jobIDSelectedStyle.Render(FitCellContent(run.ID, idWidth))
		timeStyled := jobTimeSelectedStyle.Render(FitCellContent(relTime, timeWidth))
		durationStyled := jobTimeSelectedStyle.Render(FitCellContent(duration, durationWidth))
		line := sp + statusStyled + sp + idStyled + sp + timeStyled + sp + durationStyled
		lineWidth := lipgloss.Width(line)
		if lineWidth < width {
			line = line + jobSelectedBgStyle.Render(strings.Repeat(" ", width-lineWidth))
		}
		return line
	}

	statusStyled := statusStyle.Render(statusText)
	idStyled := jobIDStyle.Render(FitCellContent(run.ID, idWidth))
	timeStyled := jobTimeStyle.Render(FitCellContent(relTime, timeWidth))
	durationStyled := jobTimeStyle.Render(FitCellContent(duration, durationWidth))
	return " " + statusStyled + " " + idStyled + " " + timeStyled + " " + durationStyled
}

func (m Model) renderPanel(num int, title, content string, width, height int, active bool) string {
	borderColor := colorBlue
	titleFg := colorBlue
	if active {
		borderColor = primaryColor
		titleFg = primaryColor
	}

	// Border characters
	tl, tr, bl, br := "╭", "╮", "╰", "╯"
	h, v := "─", "│"

	// Panel number and title, styled separately
	numText := fmt.Sprintf("[%d]", num)
	styledNum := lipgloss.NewStyle().
		Foreground(titleFg).
		Bold(active).
		Render(numText)
	styledTitle := lipgloss.NewStyle().
		Foreground(titleFg).
		Bold(active).
		Render(title)

	// Border dash between number and title (styled as border)
	styledDash := lipgloss.NewStyle().Foreground(borderColor).Render(h)

	// Calculate widths
	numWidth := lipgloss.Width(numText)
	titleWidth := lipgloss.Width(title)

	// Top border with number and title
	// Format: ╭─[num]─title─────...─╮
	topBorderRight := width - 2 - numWidth - 1 - titleWidth - 1
	if topBorderRight < 0 {
		topBorderRight = 0
	}
	topLine := lipgloss.NewStyle().Foreground(borderColor).Render(tl+h) +
		styledNum +
		styledDash +
		styledTitle +
		lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat(h, topBorderRight)+tr)

	// Bottom border
	bottomLine := lipgloss.NewStyle().Foreground(borderColor).Render(bl + strings.Repeat(h, width-2) + br)

	// Side borders
	vBorder := lipgloss.NewStyle().Foreground(borderColor).Render(v)

	// Content area
	contentWidth := width - 4 // 2 for borders, 2 for padding
	contentHeight := height - 2

	// Split content into lines and ensure exact width using ANSI-aware functions
	contentLines := strings.Split(content, "\n")
	var paddedLines []string
	for i := 0; i < contentHeight; i++ {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		// Ensure line is exactly contentWidth using ANSI-aware truncation/padding
		line = FitToWidth(line, contentWidth)
		paddedLines = append(paddedLines, vBorder+" "+line+" "+vBorder)
	}

	// Combine all parts
	result := topLine + "\n" + strings.Join(paddedLines, "\n") + "\n" + bottomLine
	return result
}

func (m Model) renderJobList(width int) string {
	if len(m.jobs) == 0 {
		return mutedStyle.Render("No jobs. Press 'n' to start one.")
	}

	var lines []string
	start, end := m.jobScroll.VisibleRange(len(m.jobs))
	for i := start; i < end; i++ {
		job := m.jobs[i]
		isSelected := i == m.jobScroll.Cursor

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

		// Build line: symbol + command
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

// formatStatsLine returns a formatted stats summary for the expanded view
func (m Model) formatStatsLine() string {
	if m.stats == nil {
		return "loading..."
	}
	// Format: "5 runs | 80% success | avg: 2m30s"
	avgDuration := formatDuration(time.Duration(m.stats.AvgDurationMs) * time.Millisecond)
	return fmt.Sprintf("%d runs | %.0f%% success | avg: %s",
		m.stats.RunCount,
		m.stats.SuccessRate,
		avgDuration)
}

// formatRelativeTime formats a time as a relative duration from now
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "1 hr ago"
		}
		return fmt.Sprintf("%d hr ago", h)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
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
				m.renderKey("H/L", "scroll log"),
				m.renderKey("w", "wrap"),
				m.renderKey("a", "all dirs"),
			)
		case panelPorts:
			parts = append(parts,
				m.renderKey("↑↓", "navigate"),
				m.renderKey("g/G", "first/last"),
				m.renderKey("H/L", "scroll log"),
				m.renderKey("f", "follow"),
				m.renderKey("w", "wrap"),
				m.renderKey("1-5", "panels"),
			)
		case panelRuns:
			parts = append(parts,
				m.renderKey("↑↓", "select run"),
				m.renderKey("g/G", "first/last"),
				m.renderKey("H/L", "scroll log"),
				m.renderKey("f", "follow"),
				m.renderKey("w", "wrap"),
				m.renderKey("1-5", "panels"),
			)
		case panelStdout:
			parts = append(parts,
				m.renderKey("↑↓", "scroll"),
				m.renderKey("h/l", "left/right"),
				m.renderKey("g/G", "top/bottom"),
				m.renderKey("f", "follow"),
				m.renderKey("w", "wrap"),
				m.renderKey("1-5", "panels"),
			)
		case panelStderr:
			parts = append(parts,
				m.renderKey("↑↓", "scroll"),
				m.renderKey("h/l", "left/right"),
				m.renderKey("g/G", "top/bottom"),
				m.renderKey("f", "follow"),
				m.renderKey("w", "wrap"),
				m.renderKey("1-5", "panels"),
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

	// Calculate center position for overlay
	modalWidth := lipgloss.Width(content)
	modalHeight := lipgloss.Height(content)
	x := (m.width - modalWidth) / 2
	y := (m.height - modalHeight) / 2

	// Overlay modal on top of background
	return placeOverlay(x, y, content, background)
}

// placeOverlay places the foreground string on top of the background string
// at position (x, y). Characters from fg replace characters in bg.
func placeOverlay(x, y int, fg, bg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	for i, fgLine := range fgLines {
		bgY := y + i
		if bgY < 0 || bgY >= len(bgLines) {
			continue
		}

		bgLine := bgLines[bgY]
		bgLineWidth := ansi.StringWidth(bgLine)

		// Build the new line: left part + fg line + right part
		var newLine strings.Builder

		// Left part of background (before overlay)
		if x > 0 {
			left := ansi.Truncate(bgLine, x, "")
			newLine.WriteString(left)
			// Pad if background line is shorter than x
			leftWidth := ansi.StringWidth(left)
			if leftWidth < x {
				newLine.WriteString(strings.Repeat(" ", x-leftWidth))
			}
		}

		// Foreground content
		newLine.WriteString(fgLine)
		fgLineWidth := ansi.StringWidth(fgLine)

		// Right part of background (after overlay)
		rightStart := x + fgLineWidth
		if rightStart < bgLineWidth {
			// We need to skip 'rightStart' visual columns and take the rest
			right := truncateLeft(bgLine, rightStart)
			newLine.WriteString(right)
		}

		bgLines[bgY] = newLine.String()
	}

	return strings.Join(bgLines, "\n")
}

// truncateLeft removes the first n visual columns from a string,
// preserving ANSI escape sequences.
func truncateLeft(s string, n int) string {
	if n <= 0 {
		return s
	}

	var result strings.Builder
	width := 0
	inEscape := false
	escapeSeq := strings.Builder{}

	for _, r := range s {
		if inEscape {
			escapeSeq.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
				// If we're past the truncation point, include this escape sequence
				if width >= n {
					result.WriteString(escapeSeq.String())
				}
				escapeSeq.Reset()
			}
			continue
		}

		if r == '\x1b' {
			inEscape = true
			escapeSeq.WriteRune(r)
			continue
		}

		charWidth := 1
		if r > 127 {
			charWidth = ansi.StringWidth(string(r))
		}

		if width >= n {
			result.WriteRune(r)
		}
		width += charWidth
	}

	return result.String()
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
		"  " + m.renderKey("1-5", "panels"),
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
		"  " + m.renderKey("↑/k ↓/j", "scroll vertical"),
		"  " + m.renderKey("h/l", "scroll horizontal"),
		"  " + m.renderKey("H/L", "scroll log (from jobs)"),
		"  " + m.renderKey("pgup/pgdn", "page scroll"),
		"  " + m.renderKey("g/G", "top/bottom"),
		"  " + m.renderKey("f", "toggle follow"),
		"  " + m.renderKey("w", "toggle wrap"),
		"",
		helpKeyStyle.Render("Other"),
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

// Start starts the TUI
func Start() error {
	telemetry.TUISessionStart()
	defer telemetry.TUISessionEnd()

	cwd, _ := os.Getwd()
	env := os.Environ()

	// Read gobfile
	commands, _ := ReadGobfile(cwd)

	// Cleanup function for gobfile jobs
	cleanup := func() {
		if commands != nil {
			StopGobfileJobs(cwd, commands)
		}
	}

	// Handle signals to ensure cleanup runs even on SIGHUP (tmux kill)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanup()
		os.Exit(0)
	}()
	defer signal.Stop(sigChan)

	// Auto-start gobfile jobs (async, don't block TUI startup)
	if commands != nil {
		go StartGobfileJobs(cwd, commands, env)
	}

	// Run TUI
	p := tea.NewProgram(New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()

	// Auto-stop gobfile jobs (after TUI exits normally)
	cleanup()

	return err
}
