package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
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
	ID      string
	PID     int
	Command string
	Workdir string
	Running bool
}

// tickMsg is sent periodically to refresh job status
type tickMsg time.Time

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
}

// New creates a new TUI model
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "command args..."
	ti.CharLimit = 256
	ti.Width = 50

	h := help.New()
	h.ShowAll = true

	cwd, _ := storage.GetCurrentWorkdir()

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

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshJobs(),
		tickCmd(),
	)
}

// tickCmd returns a command that sends a tick every 500ms
func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// refreshJobs fetches the current job list
func (m Model) refreshJobs() tea.Cmd {
	return func() tea.Msg {
		var jobInfos []storage.JobInfo
		var err error

		if m.showAll {
			jobInfos, err = storage.ListAllJobMetadata()
		} else {
			jobInfos, err = storage.ListJobMetadata()
		}

		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to list jobs: %v", err), isError: true}
		}

		jobs := make([]Job, len(jobInfos))
		for i, ji := range jobInfos {
			jobs[i] = Job{
				ID:      ji.ID,
				PID:     ji.Metadata.PID,
				Command: strings.Join(ji.Metadata.Command, " "),
				Workdir: ji.Metadata.Workdir,
				Running: process.IsProcessRunning(ji.Metadata.PID),
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

		jobID := m.jobs[m.cursor].ID
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return logUpdateMsg{stdout: "", stderr: fmt.Sprintf("Error: %v", err)}
		}

		stdoutPath := filepath.Join(jobDir, fmt.Sprintf("%s.stdout.log", jobID))
		stderrPath := filepath.Join(jobDir, fmt.Sprintf("%s.stderr.log", jobID))

		stdout, _ := os.ReadFile(stdoutPath)
		stderr, _ := os.ReadFile(stderrPath)

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

	case tickMsg:
		cmds = append(cmds, m.refreshJobs(), m.readLogs(), tickCmd())

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
		cmds = append(cmds, m.refreshJobs())

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
				return m, m.startJob(cmd)
			}
		case "ctrl+c":
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
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "1":
		m.activePanel = panelJobs

	case "2":
		m.activePanel = panelStdout

	case "3":
		m.activePanel = panelStderr

	case "tab":
		switch m.activePanel {
		case panelJobs:
			m.activePanel = panelStdout
		case panelStdout:
			m.activePanel = panelStderr
		case panelStderr:
			m.activePanel = panelJobs
		}

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
		return m, m.refreshJobs()
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
			return m, m.stopJob(m.jobs[m.cursor].PID, false)
		}

	case "S":
		if len(m.jobs) > 0 && m.jobs[m.cursor].Running {
			return m, m.stopJob(m.jobs[m.cursor].PID, true)
		}

	case "r":
		if len(m.jobs) > 0 {
			return m, m.restartJob(m.jobs[m.cursor])
		}

	case "d":
		if len(m.jobs) > 0 && !m.jobs[m.cursor].Running {
			return m, m.removeJob(m.jobs[m.cursor].ID)
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

func (m Model) stopJob(pid int, force bool) tea.Cmd {
	return func() tea.Msg {
		// Check if already stopped
		if !process.IsProcessRunning(pid) {
			action := "Stopped"
			if force {
				action = "Killed"
			}
			return actionResultMsg{
				message: fmt.Sprintf("%s PID %d", action, pid),
				isError: false,
			}
		}

		var err error
		if force {
			// Send SIGKILL immediately
			err = process.KillProcess(pid)
			if err != nil {
				return actionResultMsg{
					message: fmt.Sprintf("Failed to kill: %v", err),
					isError: true,
				}
			}
		} else {
			// Graceful shutdown with timeout (same as CLI)
			err = process.StopProcessWithTimeout(pid, 10*time.Second, 5*time.Second)
			if err != nil {
				return actionResultMsg{
					message: fmt.Sprintf("Failed to stop: %v", err),
					isError: true,
				}
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

func (m Model) restartJob(job Job) tea.Cmd {
	return func() tea.Msg {
		// Load job metadata to get the original command
		metadata, err := storage.LoadJobMetadata(job.ID + ".json")
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Job not found: %v", err), isError: true}
		}

		// Stop if running (same timeouts as CLI)
		if process.IsProcessRunning(metadata.PID) {
			err := process.StopProcessWithTimeout(metadata.PID, 10*time.Second, 5*time.Second)
			if err != nil {
				return actionResultMsg{message: fmt.Sprintf("Failed to stop: %v", err), isError: true}
			}
		}

		// Get job directory
		jobDir, err := storage.EnsureJobDir()
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed: %v", err), isError: true}
		}

		// Start with the same job ID
		command := metadata.Command[0]
		args := []string{}
		if len(metadata.Command) > 1 {
			args = metadata.Command[1:]
		}

		pid, err := process.StartDetached(command, args, metadata.ID, jobDir)
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to start: %v", err), isError: true}
		}

		// Update PID in metadata
		metadata.PID = pid
		_, err = storage.SaveJobMetadata(metadata)
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed to save: %v", err), isError: true}
		}

		return actionResultMsg{
			message: fmt.Sprintf("Restarted %s (PID %d)", job.ID, pid),
			isError: false,
		}
	}
}

func (m Model) removeJob(jobID string) tea.Cmd {
	return func() tea.Msg {
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed: %v", err), isError: true}
		}

		files := []string{
			filepath.Join(jobDir, fmt.Sprintf("%s.json", jobID)),
			filepath.Join(jobDir, fmt.Sprintf("%s.stdout.log", jobID)),
			filepath.Join(jobDir, fmt.Sprintf("%s.stderr.log", jobID)),
		}

		for _, f := range files {
			os.Remove(f)
		}

		return actionResultMsg{
			message: fmt.Sprintf("Removed %s", jobID),
			isError: false,
		}
	}
}

func (m Model) startJob(command string) tea.Cmd {
	return func() tea.Msg {
		jobDir, err := storage.EnsureJobDir()
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed: %v", err), isError: true}
		}

		parts := strings.Fields(command)
		if len(parts) == 0 {
			return actionResultMsg{message: "Empty command", isError: true}
		}

		newID := storage.GenerateJobID()
		pid, err := process.StartDetached(parts[0], parts[1:], newID, jobDir)
		if err != nil {
			return actionResultMsg{message: fmt.Sprintf("Failed: %v", err), isError: true}
		}

		cwd, _ := storage.GetCurrentWorkdir()
		metadata := &storage.JobMetadata{
			ID:        newID,
			Command:   parts,
			PID:       pid,
			Workdir:   cwd,
			CreatedAt: time.Now(),
		}
		storage.SaveJobMetadata(metadata)

		return actionResultMsg{
			message: fmt.Sprintf("Started %s (PID %d)", newID, pid),
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
		status := "○"
		if job.Running {
			status = "●"
		}
		stdoutTitle = fmt.Sprintf("2 stdout: %s %s", job.ID, status)
		stderrTitle = "3 stderr"
		if m.followLogs && m.activePanel == panelStdout {
			stdoutTitle += " [following]"
		}
		if m.followLogs && m.activePanel == panelStderr {
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
	borderColor := fgOyster
	titleBg := bgIron
	titleFg := fgAsh
	if active {
		borderColor = primaryColor
		titleBg = primaryColor
		titleFg = bgPepper
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

		// Status indicator
		var status string
		if job.Running {
			if isSelected {
				status = "●" // No color when selected
			} else {
				status = jobRunningStyle.Render("●")
			}
		} else {
			if isSelected {
				status = "○"
			} else {
				status = jobStoppedStyle.Render("○")
			}
		}

		// Job ID
		var id string
		if isSelected {
			id = job.ID
		} else {
			id = jobIDStyle.Render(job.ID)
		}

		// PID
		var pid string
		pidStr := fmt.Sprintf("[%d]", job.PID)
		if isSelected {
			pid = pidStr
		} else {
			pid = jobPIDStyle.Render(pidStr)
		}

		// Command (truncated)
		maxCmdLen := width - 25
		if maxCmdLen < 10 {
			maxCmdLen = 10
		}
		cmd := m.truncate(job.Command, maxCmdLen)

		line := fmt.Sprintf(" %s %s %s %s", status, id, pid, cmd)

		// Pad to full width and apply selection style
		if isSelected {
			// Pad with spaces to fill width
			padding := width - lipgloss.Width(line)
			if padding > 0 {
				line = line + strings.Repeat(" ", padding)
			}
			line = jobSelectedStyle.Render(line)
		}

		lines = append(lines, line)

		// Show workdir if showing all
		if m.showAll && job.Workdir != "" {
			var wd string
			wdStr := m.shortenPath(job.Workdir)
			if isSelected {
				wd = "   " + wdStr
				padding := width - lipgloss.Width(wd)
				if padding > 0 {
					wd = wd + strings.Repeat(" ", padding)
				}
				wd = jobSelectedStyle.Render(wd)
			} else {
				wd = "   " + workdirStyle.Render(wdStr)
			}
			lines = append(lines, wd)
		}
	}

	return strings.Join(lines, "\n")
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
		"  " + m.renderKey("n", "new job"),
		"",
		helpKeyStyle.Render("Log Viewer"),
		"  " + m.renderKey("↑/k ↓/j", "scroll"),
		"  " + m.renderKey("pgup/pgdn", "page scroll"),
		"  " + m.renderKey("g/G", "top/bottom"),
		"  " + m.renderKey("f", "toggle follow"),
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

// Run starts the TUI
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
