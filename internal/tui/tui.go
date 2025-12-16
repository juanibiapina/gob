package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"
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
	Ports      []daemon.PortInfo // Listening ports (only for running jobs)
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
	cursor      int
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
	runCursor    int
	runsForJobID string // tracks which job the runs are for

	// Port list state
	portCursor int // selected port index
	portOffset int // first visible port index (for scrolling)

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
		cursor:      0,
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

// readLogs reads the log files for the selected job or run
func (m Model) readLogs() tea.Cmd {
	return func() tea.Msg {
		if len(m.jobs) == 0 || m.cursor >= len(m.jobs) {
			return logUpdateMsg{stdout: "", stderr: ""}
		}

		// Use the selected run's log paths if available
		var stdoutPath, stderrPath string
		if len(m.runs) > 0 && m.runCursor >= 0 && m.runCursor < len(m.runs) {
			run := m.runs[m.runCursor]
			stdoutPath = run.StdoutPath
			stderrPath = run.StderrPath
		} else {
			job := m.jobs[m.cursor]
			stdoutPath = job.StdoutPath
			stderrPath = job.StderrPath
		}

		stdout, _ := os.ReadFile(stdoutPath)
		stderr, _ := os.ReadFile(stderrPath)

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
		if len(m.jobs) > 0 && m.cursor < len(m.jobs) {
			jobID := m.jobs[m.cursor].ID
			if jobID != m.runsForJobID {
				m.runsForJobID = jobID
				m.runCursor = 0
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
		if m.cursor >= len(m.jobs) {
			m.cursor = len(m.jobs) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		// Fetch runs for the selected job (ports come from events)
		if len(m.jobs) > 0 {
			jobID := m.jobs[m.cursor].ID
			if jobID != m.runsForJobID {
				m.runsForJobID = jobID
				m.runCursor = 0
				m.portCursor = 0
				m.portOffset = 0
				cmds = append(cmds, m.fetchRuns(jobID))
			}
		}

	case runsUpdatedMsg:
		// Only update if this is for the currently selected job
		if msg.jobID == m.runsForJobID {
			m.runs = msg.runs
			m.stats = msg.stats
			if m.runCursor >= len(m.runs) {
				m.runCursor = len(m.runs) - 1
			}
			if m.runCursor < 0 {
				m.runCursor = 0
			}
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
			Ports:      event.Job.Ports,
		}
		// Only adjust cursor if there are existing jobs (keep selection on same job)
		// When list was empty, cursor stays at 0 to select the new job
		if len(m.jobs) > 0 {
			m.cursor++
		}
		m.jobs = append([]Job{newJob}, m.jobs...)

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
					if m.cursor == i {
						// Selected job moved to top
						m.cursor = 0
					} else if m.cursor < i {
						// Selected job was above the moved job, shift down by 1
						m.cursor++
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
				// Adjust cursor if needed
				if m.cursor >= len(m.jobs) {
					m.cursor = len(m.jobs) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
				// Clear runs if this was the selected job
				if event.JobID == m.runsForJobID {
					m.runs = nil
					m.stats = nil
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
				if i == m.cursor {
					portCount := len(event.Ports)
					if m.portCursor >= portCount {
						m.portCursor = portCount - 1
					}
					if m.portCursor < 0 {
						m.portCursor = 0
					}
					if m.portOffset > m.portCursor {
						m.portOffset = m.portCursor
					}
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
		if m.cursor > 0 {
			m.cursor--
			m.followLogs = true
			m.runCursor = 0
			m.runs = nil
			m.stats = nil
			m.portCursor = 0
			m.portOffset = 0
			if len(m.jobs) > 0 {
				m.runsForJobID = m.jobs[m.cursor].ID
			}
			return m, m.fetchRunsForSelectedJob()
		}

	case "down", "j":
		if m.cursor < len(m.jobs)-1 {
			m.cursor++
			m.followLogs = true
			m.runCursor = 0
			m.runs = nil
			m.stats = nil
			m.portCursor = 0
			m.portOffset = 0
			if len(m.jobs) > 0 {
				m.runsForJobID = m.jobs[m.cursor].ID
			}
			return m, m.fetchRunsForSelectedJob()
		}

	case "g":
		m.cursor = 0
		m.followLogs = true
		m.runCursor = 0
		m.runs = nil
		m.stats = nil
		m.portCursor = 0
		m.portOffset = 0
		if len(m.jobs) > 0 {
			m.runsForJobID = m.jobs[m.cursor].ID
		}
		return m, m.fetchRunsForSelectedJob()

	case "G":
		if len(m.jobs) > 0 {
			m.cursor = len(m.jobs) - 1
			m.followLogs = true
			m.runCursor = 0
			m.runs = nil
			m.stats = nil
			m.portCursor = 0
			m.portOffset = 0
			m.runsForJobID = m.jobs[m.cursor].ID
			return m, m.fetchRunsForSelectedJob()
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

// fetchRunsForSelectedJob returns commands to read logs and fetch runs for the selected job
// Note: caller must set m.runsForJobID before calling this
// Ports are received via daemon events (EventTypePortsUpdated)
func (m Model) fetchRunsForSelectedJob() tea.Cmd {
	if len(m.jobs) == 0 || m.cursor >= len(m.jobs) {
		return m.readLogs()
	}
	jobID := m.jobs[m.cursor].ID
	return tea.Batch(m.readLogs(), m.fetchRuns(jobID))
}

func (m Model) updatePortsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Get current job's port count for bounds checking
	portCount := 0
	if len(m.jobs) > 0 && m.cursor < len(m.jobs) && m.jobs[m.cursor].Running {
		portCount = len(m.jobs[m.cursor].Ports)
	}

	switch msg.String() {
	case "up", "k":
		if m.portCursor > 0 {
			m.portCursor--
			// Scroll up if cursor goes above visible area
			if m.portCursor < m.portOffset {
				m.portOffset = m.portCursor
			}
		}

	case "down", "j":
		if m.portCursor < portCount-1 {
			m.portCursor++
			// Scroll down if cursor goes below visible area
			visibleRows := m.portsVisibleRows()
			if m.portCursor >= m.portOffset+visibleRows {
				m.portOffset = m.portCursor - visibleRows + 1
			}
		}

	case "g":
		m.portCursor = 0
		m.portOffset = 0

	case "G":
		if portCount > 0 {
			m.portCursor = portCount - 1
			visibleRows := m.portsVisibleRows()
			if m.portCursor >= visibleRows {
				m.portOffset = m.portCursor - visibleRows + 1
			} else {
				m.portOffset = 0
			}
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

func (m Model) updateRunsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.runCursor > 0 {
			m.runCursor--
			m.followLogs = true
			return m, m.readLogs()
		}

	case "down", "j":
		if m.runCursor < len(m.runs)-1 {
			m.runCursor++
			m.followLogs = true
			return m, m.readLogs()
		}

	case "g":
		m.runCursor = 0
		m.followLogs = true
		return m, m.readLogs()

	case "G":
		if len(m.runs) > 0 {
			m.runCursor = len(m.runs) - 1
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

		result, err := client.Add(parts, m.cwd, m.env)
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
	if len(m.jobs) == 0 || m.cursor >= len(m.jobs) {
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
	if len(m.jobs) == 0 || m.cursor >= len(m.jobs) {
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

// portsVisibleRows returns the number of port rows visible in the ports panel
func (m Model) portsVisibleRows() int {
	totalH := m.height - 2 // header + status bar
	portsH := totalH * 20 / 100
	if portsH < 4 {
		portsH = 4
	}
	// Panel height - border (2) - header row (1) = content rows for ports
	visibleRows := portsH - 3
	if visibleRows < 1 {
		visibleRows = 1
	}
	return visibleRows
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
	leftPanelW := m.jobPanelWidth()
	rightPanelW := m.width - leftPanelW
	totalH := m.height - 2 // height - header - status bar

	// Ensure minimum height
	if totalH < 8 {
		totalH = 8
	}

	// Left side: Jobs (50%) + Ports (20%) + Runs (30%)
	portsH := totalH * 20 / 100
	if portsH < 4 {
		portsH = 4
	}
	runsH := totalH * 30 / 100
	if runsH < 5 {
		runsH = 5
	}
	jobsH := totalH - portsH - runsH

	// Right side: Stdout (80%) + Stderr (20%)
	stderrH := totalH * 20 / 100
	if stderrH < 4 {
		stderrH = 4
	}
	stdoutH := totalH - stderrH

	// Jobs panel
	jobContent := m.renderJobList(leftPanelW-4, jobsH-2)
	jobPanel := m.renderPanel("1 Jobs", jobContent, leftPanelW, jobsH, m.activePanel == panelJobs)

	// Ports panel
	portsTitle := "2 Ports"
	if len(m.jobs) > 0 && m.cursor < len(m.jobs) {
		portsTitle = fmt.Sprintf("2 Ports: %s", m.jobs[m.cursor].ID)
	}
	portsContent := m.renderPortsList(leftPanelW-4, portsH-2)
	portsPanel := m.renderPanel(portsTitle, portsContent, leftPanelW, portsH, m.activePanel == panelPorts)

	// Runs panel
	runsTitle := "3 Runs"
	if len(m.jobs) > 0 && m.cursor < len(m.jobs) {
		runsTitle = fmt.Sprintf("3 Runs: %s", m.jobs[m.cursor].ID)
	}
	runsContent := m.renderRunsList(leftPanelW-4, runsH-2)
	runsPanel := m.renderPanel(runsTitle, runsContent, leftPanelW, runsH, m.activePanel == panelRuns)

	// Build titles for log panels
	var stdoutTitle, stderrTitle string
	if len(m.jobs) > 0 && m.cursor < len(m.jobs) {
		job := m.jobs[m.cursor]

		// Check if showing a specific run
		var showingRunID string
		var runStatus string
		var durationStr string

		if len(m.runs) > 0 && m.runCursor >= 0 && m.runCursor < len(m.runs) {
			// Showing a specific run
			run := m.runs[m.runCursor]
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

		stdoutTitle = fmt.Sprintf("4 stdout: %s %s%s", showingRunID, runStatus, durationStr)
		stderrTitle = "5 stderr"
		if m.followLogs {
			stdoutTitle += " [following]"
			stderrTitle += " [following]"
		}
		if m.wrapLines {
			stdoutTitle += " [wrap]"
			stderrTitle += " [wrap]"
		}
	} else {
		stdoutTitle = "4 stdout"
		stderrTitle = "5 stderr"
		if m.wrapLines {
			stdoutTitle += " [wrap]"
			stderrTitle += " [wrap]"
		}
	}

	// Stdout panel
	m.stdoutView.Width = rightPanelW - 4
	m.stdoutView.Height = stdoutH - 3
	stdoutContent := m.stdoutView.View()
	stdoutPanel := m.renderPanel(stdoutTitle, stdoutContent, rightPanelW, stdoutH, m.activePanel == panelStdout)

	// Stderr panel
	m.stderrView.Width = rightPanelW - 4
	m.stderrView.Height = stderrH - 3
	stderrContent := m.stderrView.View()
	stderrPanel := m.renderPanel(stderrTitle, stderrContent, rightPanelW, stderrH, m.activePanel == panelStderr)

	// Stack panels
	leftPanels := lipgloss.JoinVertical(lipgloss.Left, jobPanel, portsPanel, runsPanel)
	rightPanels := lipgloss.JoinVertical(lipgloss.Left, stdoutPanel, stderrPanel)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanels, rightPanels)
}

// renderRunsList renders the runs list for the selected job
func (m Model) renderRunsList(width, height int) string {
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

	// Run list
	for i, run := range m.runs {
		isSelected := i == m.runCursor && m.activePanel == panelRuns
		runLine := m.formatRunListLine(run, isSelected, width)
		lines = append(lines, runLine)
	}

	return strings.Join(lines, "\n")
}

// renderPortsList renders the ports list for the selected job
func (m Model) renderPortsList(width, height int) string {
	if len(m.jobs) == 0 {
		return mutedStyle.Render("No job selected")
	}

	job := m.jobs[m.cursor]

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

	// Calculate visible range (height - 1 for header row)
	visibleRows := height - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	startIdx := m.portOffset
	endIdx := m.portOffset + visibleRows
	if endIdx > len(ports) {
		endIdx = len(ports)
	}

	// Port rows (only visible ones)
	for i := startIdx; i < endIdx; i++ {
		p := ports[i]
		isSelected := i == m.portCursor && m.activePanel == panelPorts
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
func (m Model) formatRunListLine(run Run, isSelected bool, width int) string {
	// Status indicator
	var status string
	if run.Status == "running" {
		if isSelected {
			status = jobRunningSelectedStyle.Render("◉")
		} else {
			status = jobRunningStyle.Render("◉")
		}
	} else if run.ExitCode != nil {
		if *run.ExitCode == 0 {
			if isSelected {
				status = jobSuccessSelectedStyle.Render("✓")
			} else {
				status = jobSuccessStyle.Render("✓")
			}
		} else {
			exitStr := fmt.Sprintf("✗ (%d)", *run.ExitCode)
			if isSelected {
				status = jobFailedSelectedStyle.Render(exitStr)
			} else {
				status = jobFailedStyle.Render(exitStr)
			}
		}
	} else {
		if isSelected {
			status = jobStoppedSelectedStyle.Render("◼")
		} else {
			status = jobStoppedStyle.Render("◼")
		}
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

	// Build the line
	if isSelected {
		sp := jobSelectedBgStyle.Render(" ")
		idStyled := jobIDSelectedStyle.Render(run.ID)
		timeStyled := jobTimeSelectedStyle.Render(relTime)
		durationStyled := jobTimeSelectedStyle.Render(duration)
		line := sp + status + sp + idStyled + jobSelectedBgStyle.Render("  ") + timeStyled + jobSelectedBgStyle.Render("  ") + durationStyled
		padding := width - lipgloss.Width(line)
		if padding > 0 {
			line = line + jobSelectedBgStyle.Render(strings.Repeat(" ", padding))
		}
		return line
	}

	return fmt.Sprintf(" %s %s  %s  %s", status, jobIDStyle.Render(run.ID), jobTimeStyle.Render(relTime), jobTimeStyle.Render(duration))
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

	p := tea.NewProgram(New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
