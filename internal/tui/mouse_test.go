package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/juanibiapina/gob/internal/daemon"
)

// newTestModel creates a Model with standard dimensions for mouse testing.
// height=51 → totalH=50, width=200 → leftW=60 (200*40/100=80 capped at 60)
func newTestModel() Model {
	m := New()
	m.width = 200
	m.height = 51
	m.ready = true
	return m
}

func newTestModelWithJobs(jobs ...Job) Model {
	m := newTestModel()
	m.jobs = jobs
	m.jobScroll.VisibleRows = 10
	m.runScroll.VisibleRows = 10
	m.portScroll.VisibleRows = 10
	return m
}

// --- panelAtPosition tests ---

func TestPanelAtPosition_LeftSide(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Command: "cmd1"})
	l := m.leftPanelLayout()

	tests := []struct {
		name string
		x, y int
		want panel
	}{
		{"info area", 10, 0, panelJobs},
		{"jobs panel top", 10, l.jobsStart, panelJobs},
		{"jobs panel mid", 10, l.jobsStart + 3, panelJobs},
		{"ports panel top", 10, l.portsStart, panelPorts},
		{"ports panel mid", 10, l.portsStart + 2, panelPorts},
		{"runs panel top", 10, l.runsStart, panelRuns},
		{"runs panel bottom", 10, l.runsStart + 5, panelRuns},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.panelAtPosition(tt.x, tt.y)
			if got != tt.want {
				t.Errorf("panelAtPosition(%d, %d) = %d, want %d", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestPanelAtPosition_RightSide(t *testing.T) {
	m := newTestModel()
	m.activePanel = panelJobs // stderr gets 20%
	leftW := m.jobPanelWidth()
	stdoutH, _ := m.logPanelHeights()

	tests := []struct {
		name string
		x, y int
		want panel
	}{
		{"stdout top", leftW + 5, 0, panelStdout},
		{"stdout just before boundary", leftW + 5, stdoutH - 1, panelStdout},
		{"stderr at boundary", leftW + 5, stdoutH, panelStderr},
		{"stderr bottom", leftW + 5, stdoutH + 5, panelStderr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.panelAtPosition(tt.x, tt.y)
			if got != tt.want {
				t.Errorf("panelAtPosition(%d, %d) = %d, want %d", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

// --- handleMouseScroll tests ---

func TestHandleMouseScroll_JobsPanel(t *testing.T) {
	m := newTestModelWithJobs(
		Job{ID: "j1", Command: "cmd1"},
		Job{ID: "j2", Command: "cmd2"},
		Job{ID: "j3", Command: "cmd3"},
	)
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()
	x := 10
	y := l.jobsStart + 2 // inside jobs panel

	// Scroll down — should move cursor from 0 to 1
	m2, _, handled := m.handleMouseScroll(x, y, 1)
	model := m2.(Model)
	if !handled {
		t.Fatal("scroll down not handled")
	}
	if model.jobScroll.Cursor != 1 {
		t.Errorf("after scroll down: cursor = %d, want 1", model.jobScroll.Cursor)
	}

	// Scroll up from position 1 — should go back to 0
	m3, _, handled := model.handleMouseScroll(x, y, -1)
	model = m3.(Model)
	if !handled {
		t.Fatal("scroll up not handled")
	}
	if model.jobScroll.Cursor != 0 {
		t.Errorf("after scroll up: cursor = %d, want 0", model.jobScroll.Cursor)
	}
}

func TestHandleMouseScroll_EmptyJobsList(t *testing.T) {
	m := newTestModel()
	l := m.leftPanelLayout()
	x := 10
	y := l.jobsStart + 2

	// Scroll on empty list should not panic
	_, _, handled := m.handleMouseScroll(x, y, 1)
	if handled {
		t.Error("scroll on empty jobs should not be handled (no cursor change)")
	}
}

func TestHandleMouseScroll_AtBounds(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Command: "cmd1"})
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()
	x := 10
	y := l.jobsStart + 2

	// Scroll up at top — should not change
	_, _, handled := m.handleMouseScroll(x, y, -1)
	if handled {
		t.Error("scroll up at top should not be handled")
	}

	// Scroll down at bottom (single item) — should not change
	_, _, handled = m.handleMouseScroll(x, y, 1)
	if handled {
		t.Error("scroll down at bottom should not be handled")
	}
}

func TestHandleMouseScroll_RunsPanel(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Command: "cmd1"})
	m.runs = []Run{
		{ID: "r1", JobID: "j1"},
		{ID: "r2", JobID: "j1"},
	}
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()
	x := 10
	y := l.runsStart + 2 // inside runs panel

	m2, _, handled := m.handleMouseScroll(x, y, 1)
	if !handled {
		t.Fatal("scroll down on runs not handled")
	}
	model := m2.(Model)
	if model.runScroll.Cursor != 1 {
		t.Errorf("after scroll down: run cursor = %d, want 1", model.runScroll.Cursor)
	}
}

func TestHandleMouseScroll_StdoutPanel(t *testing.T) {
	m := newTestModel()
	m.followLogs = true
	leftW := m.jobPanelWidth()
	x := leftW + 5
	y := 5 // inside stdout

	m2, _, handled := m.handleMouseScroll(x, y, -1)
	if !handled {
		t.Fatal("scroll up on stdout not handled")
	}
	model := m2.(Model)
	if model.followLogs {
		t.Error("scroll up on stdout should disable followLogs")
	}
}

func TestHandleMouseScroll_StderrPanel(t *testing.T) {
	m := newTestModel()
	m.activePanel = panelJobs // stderr gets 20%
	m.followLogs = true
	leftW := m.jobPanelWidth()
	stdoutH, _ := m.logPanelHeights()
	x := leftW + 5
	y := stdoutH + 2 // inside stderr

	m2, _, handled := m.handleMouseScroll(x, y, -1)
	if !handled {
		t.Fatal("scroll up on stderr not handled")
	}
	model := m2.(Model)
	if model.followLogs {
		t.Error("scroll up on stderr should disable followLogs")
	}
}

func TestHandleMouseScroll_TargetsPanelUnderCursor(t *testing.T) {
	// Active panel is jobs, but cursor is over runs — scroll should go to runs
	m := newTestModelWithJobs(Job{ID: "j1", Command: "cmd1"})
	m.runs = []Run{
		{ID: "r1", JobID: "j1"},
		{ID: "r2", JobID: "j1"},
	}
	m.runsForJobID = "j1"
	m.activePanel = panelJobs // focused on jobs

	l := m.leftPanelLayout()
	x := 10
	y := l.runsStart + 2 // cursor over runs panel

	m2, _, handled := m.handleMouseScroll(x, y, 1)
	if !handled {
		t.Fatal("scroll not handled")
	}
	model := m2.(Model)
	// Run cursor should move, not job cursor
	if model.runScroll.Cursor != 1 {
		t.Errorf("run cursor = %d, want 1 (scroll should target panel under cursor)", model.runScroll.Cursor)
	}
	if model.jobScroll.Cursor != 0 {
		t.Errorf("job cursor = %d, want 0 (should not change)", model.jobScroll.Cursor)
	}
}

// --- handleMouseClick tests ---

func TestHandleMouseClick_FocusesJobsPanel(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Command: "cmd1"})
	m.activePanel = panelStdout // start focused on stdout
	l := m.leftPanelLayout()

	m2, _, handled := m.handleLeftPanelClick(10, l.jobsStart+1)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.activePanel != panelJobs {
		t.Errorf("activePanel = %d, want panelJobs (%d)", model.activePanel, panelJobs)
	}
}

func TestHandleMouseClick_SelectsJobItem(t *testing.T) {
	m := newTestModelWithJobs(
		Job{ID: "j1", Command: "cmd1"},
		Job{ID: "j2", Command: "cmd2"},
		Job{ID: "j3", Command: "cmd3"},
	)
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()

	// Click on the third item (index 2): jobsStart + 1 (border) + 2 (item index)
	clickY := l.jobsStart + 1 + 2
	m2, _, handled := m.handleLeftPanelClick(10, clickY)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.jobScroll.Cursor != 2 {
		t.Errorf("job cursor = %d, want 2", model.jobScroll.Cursor)
	}
}

func TestHandleMouseClick_ClickOnSameJobReselects(t *testing.T) {
	m := newTestModelWithJobs(
		Job{ID: "j1", Command: "cmd1"},
		Job{ID: "j2", Command: "cmd2"},
	)
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()

	// Click on the first item (already selected) — should still trigger
	clickY := l.jobsStart + 1 + 0
	m2, _, handled := m.handleLeftPanelClick(10, clickY)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.jobScroll.Cursor != 0 {
		t.Errorf("cursor = %d, want 0", model.jobScroll.Cursor)
	}
}

func TestHandleMouseClick_EmptyJobsList(t *testing.T) {
	m := newTestModel()
	l := m.leftPanelLayout()

	// Click inside jobs panel area when list is empty — should focus but not panic
	_, _, handled := m.handleLeftPanelClick(10, l.jobsStart+2)
	if !handled {
		t.Fatal("click not handled")
	}
}

func TestHandleMouseClick_ClickBeyondJobsList(t *testing.T) {
	m := newTestModelWithJobs(
		Job{ID: "j1", Command: "cmd1"},
	)
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()

	// Click way below the single item — should focus but not change selection
	clickY := l.jobsStart + 1 + 5 // well beyond item 0
	m2, cmd, handled := m.handleLeftPanelClick(10, clickY)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.jobScroll.Cursor != 0 {
		t.Errorf("cursor = %d, want 0 (should not change)", model.jobScroll.Cursor)
	}
	if cmd != nil {
		t.Error("click beyond list should not trigger a command")
	}
}

func TestHandleMouseClick_SelectsJobWithDescription(t *testing.T) {
	// Regression: description panel is between jobs and ports, not before jobs.
	// Clicks must work regardless of whether the selected job has a description.
	m := newTestModelWithJobs(
		Job{ID: "j1", Command: "cmd1", Description: "has description"},
		Job{ID: "j2", Command: "cmd2", Description: "also has description"},
		Job{ID: "j3", Command: "cmd3"},
	)
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()

	// jobsStart should be right after info panel (3), NOT offset by descH
	if l.jobsStart != 3 {
		t.Fatalf("jobsStart = %d, want 3 (description goes after jobs, not before)", l.jobsStart)
	}

	// Click on third item: jobsStart + 1 (border) + 2 (third item)
	clickY := l.jobsStart + 1 + 2
	m2, _, handled := m.handleLeftPanelClick(10, clickY)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.jobScroll.Cursor != 2 {
		t.Errorf("job cursor = %d, want 2", model.jobScroll.Cursor)
	}
}

func TestHandleMouseClick_PortsPanel(t *testing.T) {
	m := newTestModelWithJobs(Job{
		ID:      "j1",
		Command: "cmd1",
		Running: true,
		Ports: []daemon.PortInfo{
			{Port: 3000, Protocol: "tcp", Address: "127.0.0.1", PID: 123},
			{Port: 8080, Protocol: "tcp", Address: "127.0.0.1", PID: 123},
		},
	})
	m.activePanel = panelJobs
	l := m.leftPanelLayout()

	// Click on second port: portsStart + 2 (border + header) + 1 (second item)
	clickY := l.portsStart + 2 + 1
	m2, _, handled := m.handleLeftPanelClick(10, clickY)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.activePanel != panelPorts {
		t.Errorf("activePanel = %d, want panelPorts (%d)", model.activePanel, panelPorts)
	}
	if model.portScroll.Cursor != 1 {
		t.Errorf("port cursor = %d, want 1", model.portScroll.Cursor)
	}
}

func TestHandleMouseClick_RunsPanel(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Command: "cmd1"})
	m.runs = []Run{
		{ID: "r1", JobID: "j1"},
		{ID: "r2", JobID: "j1"},
		{ID: "r3", JobID: "j1"},
	}
	m.stats = &daemon.JobResponse{RunCount: 3}
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()

	// Header rows = 2 (stats + empty), no progress bar since job not running
	// Click on second run: runsStart + 1 (border) + 2 (headers) + 1 (second item)
	clickY := l.runsStart + 1 + 2 + 1
	m2, _, handled := m.handleLeftPanelClick(10, clickY)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.activePanel != panelRuns {
		t.Errorf("activePanel = %d, want panelRuns (%d)", model.activePanel, panelRuns)
	}
	if model.runScroll.Cursor != 1 {
		t.Errorf("run cursor = %d, want 1", model.runScroll.Cursor)
	}
}

func TestHandleMouseClick_StdoutPanel(t *testing.T) {
	m := newTestModel()
	m.activePanel = panelJobs
	leftW := m.jobPanelWidth()

	m2, _, handled := m.handleLogPanelPress(leftW+5, 5, panelStdout)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.activePanel != panelStdout {
		t.Errorf("activePanel = %d, want panelStdout (%d)", model.activePanel, panelStdout)
	}
}

func TestHandleMouseClick_StderrPanel(t *testing.T) {
	m := newTestModel()
	m.activePanel = panelJobs
	leftW := m.jobPanelWidth()
	stdoutH, _ := m.logPanelHeights()

	m2, _, handled := m.handleLogPanelPress(leftW+5, stdoutH+2, panelStderr)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	if model.activePanel != panelStderr {
		t.Errorf("activePanel = %d, want panelStderr (%d)", model.activePanel, panelStderr)
	}
}

func TestHandleMouseClick_InfoAreaNoAction(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Command: "cmd1"})

	// Click on info area (y=0..2)
	_, _, handled := m.handleLeftPanelClick(10, 1)
	if handled {
		t.Error("click on info area should not be handled")
	}
}

func TestHandleMouseClick_OnBorder(t *testing.T) {
	m := newTestModelWithJobs(
		Job{ID: "j1", Command: "cmd1"},
		Job{ID: "j2", Command: "cmd2"},
	)
	m.runsForJobID = "j1"
	l := m.leftPanelLayout()

	// Click on the top border of jobs panel (contentY = -1)
	m2, cmd, handled := m.handleLeftPanelClick(10, l.jobsStart)
	if !handled {
		t.Fatal("click not handled")
	}
	model := m2.(Model)
	// Should focus the panel but not change selection
	if model.activePanel != panelJobs {
		t.Errorf("activePanel = %d, want panelJobs", model.activePanel)
	}
	if cmd != nil {
		t.Error("clicking border should not trigger a command")
	}
}

// --- Modal guard tests ---

func TestHandleMouseEvent_IgnoredDuringModal(t *testing.T) {
	m := newTestModelWithJobs(
		Job{ID: "j1", Command: "cmd1"},
		Job{ID: "j2", Command: "cmd2"},
	)
	m.modal = modalHelp

	msg := tea.MouseMsg{
		X:      10,
		Y:      10,
		Button: tea.MouseButtonWheelDown,
	}

	_, _, handled := m.handleMouseEvent(msg)
	if handled {
		t.Error("mouse events should be ignored when modal is open")
	}
}

func TestHandleMouseEvent_IgnoredDuringNewJobModal(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Command: "cmd1"})
	m.modal = modalNewJob

	msg := tea.MouseMsg{
		X:      10,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}

	_, _, handled := m.handleMouseEvent(msg)
	if handled {
		t.Error("mouse clicks should be ignored when new job modal is open")
	}
}

// --- Text selection tests ---

func TestHandleLogPanelPress_StartsSelection(t *testing.T) {
	m := newTestModel()
	m.activePanel = panelJobs

	m2, _, handled := m.handleLogPanelPress(70, 5, panelStdout)
	if !handled {
		t.Fatal("press not handled")
	}
	model := m2.(Model)
	if model.activePanel != panelStdout {
		t.Errorf("activePanel = %d, want panelStdout", model.activePanel)
	}
	if !model.selection.active {
		t.Error("selection should be active after press")
	}
	if model.selection.panel != panelStdout {
		t.Errorf("selection.panel = %d, want panelStdout", model.selection.panel)
	}
}

func TestHandleSelectionDrag_UpdatesEndLine(t *testing.T) {
	m := newTestModel()
	m.updateLogViewportSizes() // set viewport dimensions
	m.selection = textSelection{
		active:    true,
		panel:     panelStdout,
		startLine: 2,
		endLine:   2,
	}

	// Drag to a different line (y=8 is within stdout panel content area)
	m2, _, handled := m.handleSelectionDrag(70, 8)
	if !handled {
		t.Fatal("drag not handled")
	}
	model := m2.(Model)
	if !model.selection.dragged {
		t.Error("selection.dragged should be true after drag")
	}
	if model.selection.endLine == model.selection.startLine {
		t.Error("endLine should differ from startLine after drag")
	}
}

func TestHandleSelectionRelease_ClearsSelection(t *testing.T) {
	m := newTestModel()
	m.selection = textSelection{
		active:    true,
		dragged:   true,
		panel:     panelStdout,
		startLine: 0,
		endLine:   2,
	}

	m2, _, handled := m.handleSelectionRelease()
	if !handled {
		t.Fatal("release not handled")
	}
	model := m2.(Model)
	if model.selection.active {
		t.Error("selection should be cleared after release")
	}
}

func TestSelectionRange_OrdersCorrectly(t *testing.T) {
	// Forward selection
	m := newTestModel()
	m.selection = textSelection{startLine: 2, endLine: 5}
	s, e := m.selectionRange()
	if s != 2 || e != 5 {
		t.Errorf("forward: got (%d, %d), want (2, 5)", s, e)
	}

	// Backward selection (drag upward)
	m.selection = textSelection{startLine: 5, endLine: 2}
	s, e = m.selectionRange()
	if s != 2 || e != 5 {
		t.Errorf("backward: got (%d, %d), want (2, 5)", s, e)
	}
}

func TestHighlightedLogView_NoSelectionPassthrough(t *testing.T) {
	m := newTestModel()
	// No selection — should return viewport view unchanged
	view := m.highlightedLogView(panelStdout)
	expected := m.stdoutView.View()
	if view != expected {
		t.Error("with no selection, highlightedLogView should return viewport view unchanged")
	}
}

func TestClickWithoutDrag_DoesNotCopy(t *testing.T) {
	// Press and release without drag should NOT copy to clipboard
	m := newTestModel()
	m.selection = textSelection{
		active:    true,
		dragged:   false, // no drag
		panel:     panelStdout,
		startLine: 0,
		endLine:   0,
	}

	m2, _, _ := m.handleSelectionRelease()
	model := m2.(Model)
	// Should clear selection without setting a message
	if model.selection.active {
		t.Error("selection should be cleared")
	}
	if model.message != "" {
		t.Errorf("message should be empty (no copy), got %q", model.message)
	}
}

// --- selectedJobPortCount tests ---

func TestSelectedJobPortCount_Running(t *testing.T) {
	m := newTestModelWithJobs(Job{
		ID:      "j1",
		Running: true,
		Ports: []daemon.PortInfo{
			{Port: 3000},
			{Port: 8080},
		},
	})
	if got := m.selectedJobPortCount(); got != 2 {
		t.Errorf("selectedJobPortCount() = %d, want 2", got)
	}
}

func TestSelectedJobPortCount_NotRunning(t *testing.T) {
	m := newTestModelWithJobs(Job{
		ID:      "j1",
		Running: false,
		Ports:   []daemon.PortInfo{{Port: 3000}},
	})
	if got := m.selectedJobPortCount(); got != 0 {
		t.Errorf("selectedJobPortCount() = %d, want 0 (job not running)", got)
	}
}

func TestSelectedJobPortCount_EmptyJobs(t *testing.T) {
	m := newTestModel()
	if got := m.selectedJobPortCount(); got != 0 {
		t.Errorf("selectedJobPortCount() = %d, want 0 (no jobs)", got)
	}
}

// --- runsHeaderRows tests ---

func TestRunsHeaderRows_NoProgressBar(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Running: false})
	m.stats = &daemon.JobResponse{AvgDurationMs: 0}
	if got := m.runsHeaderRows(); got != 2 {
		t.Errorf("runsHeaderRows() = %d, want 2 (no progress bar)", got)
	}
}

func TestRunsHeaderRows_WithProgressBar(t *testing.T) {
	m := newTestModelWithJobs(Job{ID: "j1", Running: true})
	m.stats = &daemon.JobResponse{AvgDurationMs: 5000}
	if got := m.runsHeaderRows(); got != 3 {
		t.Errorf("runsHeaderRows() = %d, want 3 (with progress bar)", got)
	}
}
