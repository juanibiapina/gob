package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// handleMouseEvent dispatches mouse events to scroll, click, or selection handlers.
// Returns true and the updated model/cmd if the event was handled.
func (m Model) handleMouseEvent(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	// Ignore mouse when a modal is open
	if m.modal != modalNone {
		return m, nil, false
	}

	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		return m.handleMouseScroll(msg.X, msg.Y, -1)
	case msg.Button == tea.MouseButtonWheelDown:
		return m.handleMouseScroll(msg.X, msg.Y, 1)

	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		target := m.panelAtPosition(msg.X, msg.Y)
		if target == panelStdout || target == panelStderr {
			return m.handleLogPanelPress(msg.X, msg.Y, target)
		}
		// Left panels: immediate click-to-select
		m.selection = textSelection{} // clear any selection
		return m.handleLeftPanelClick(msg.X, msg.Y)

	case msg.Action == tea.MouseActionMotion:
		if m.selection.active {
			return m.handleSelectionDrag(msg.X, msg.Y)
		}

	case msg.Action == tea.MouseActionRelease:
		if m.selection.active {
			return m.handleSelectionRelease()
		}
	}

	return m, nil, false
}

// panelAtPosition returns which panel is at the given (x, y) terminal position.
func (m Model) panelAtPosition(x, y int) panel {
	leftW := m.jobPanelWidth()

	if x < leftW {
		l := m.leftPanelLayout()
		switch {
		case y < l.jobsStart:
			return panelJobs // info/desc area — treat as jobs
		case y < l.portsStart:
			return panelJobs
		case y < l.runsStart:
			return panelPorts
		default:
			return panelRuns
		}
	}

	// Right side
	stdoutH, _ := m.logPanelHeights()
	if y < stdoutH {
		return panelStdout
	}
	return panelStderr
}

// handleMouseScroll scrolls the panel under the cursor.
// delta is -1 for scroll up, +1 for scroll down.
func (m Model) handleMouseScroll(x, y, delta int) (tea.Model, tea.Cmd, bool) {
	target := m.panelAtPosition(x, y)

	switch target {
	case panelJobs:
		if delta < 0 {
			if m.jobScroll.Up() {
				return m, m.onJobChanged(), true
			}
		} else {
			if m.jobScroll.Down(len(m.jobs)) {
				return m, m.onJobChanged(), true
			}
		}

	case panelPorts:
		portCount := m.selectedJobPortCount()
		if delta < 0 {
			m.portScroll.Up()
		} else {
			m.portScroll.Down(portCount)
		}
		return m, nil, true

	case panelRuns:
		if delta < 0 {
			if m.runScroll.Up() {
				return m, m.onRunChanged(), true
			}
		} else {
			if m.runScroll.Down(len(m.runs)) {
				return m, m.onRunChanged(), true
			}
		}

	case panelStdout:
		if delta < 0 {
			m.stdoutView.LineUp(3)
			m.followLogs = false
		} else {
			m.stdoutView.LineDown(3)
			if m.stdoutView.AtBottom() {
				m.followLogs = true
			}
		}
		return m, nil, true

	case panelStderr:
		if delta < 0 {
			m.stderrView.LineUp(3)
			m.followLogs = false
		} else {
			m.stderrView.LineDown(3)
			if m.stderrView.AtBottom() {
				m.followLogs = true
			}
		}
		return m, nil, true
	}

	return m, nil, false
}

// handleMouseClick handles clicks on left-side panels (jobs, ports, runs).
// Right-side log panels are handled by handleLogPanelPress instead.
func (m Model) handleLeftPanelClick(x, y int) (tea.Model, tea.Cmd, bool) {
	l := m.leftPanelLayout()

	switch {
	case y < l.jobsStart:
		// Info / description panels — no action
		return m, nil, false

	case y < l.portsStart:
		// Jobs panel clicked
		m.activePanel = panelJobs
		m.updateLogViewportSizes()

		contentY := y - l.jobsStart - 1 // subtract top border
		if contentY >= 0 {
			itemIndex := m.jobScroll.Offset + contentY
			if itemIndex >= 0 && itemIndex < len(m.jobs) {
				m.jobScroll.SetCursorTo(itemIndex)
				return m, m.onJobChanged(), true
			}
		}
		return m, nil, true

	case y < l.runsStart:
		// Ports panel clicked
		m.activePanel = panelPorts
		m.updateLogViewportSizes()

		// Row 0 = top border, row 1 = header — items start at row 2
		contentY := y - l.portsStart - 2
		if contentY >= 0 {
			portCount := m.selectedJobPortCount()
			itemIndex := m.portScroll.Offset + contentY
			if itemIndex >= 0 && itemIndex < portCount {
				m.portScroll.SetCursorTo(itemIndex)
			}
		}
		return m, nil, true

	default:
		// Runs panel clicked
		m.activePanel = panelRuns
		m.updateLogViewportSizes()

		// Header rows inside the panel content: stats(1) + progress(0-1) + empty(1)
		headerRows := m.runsHeaderRows()

		// +1 for top border
		contentY := y - l.runsStart - 1 - headerRows
		if contentY >= 0 {
			itemIndex := m.runScroll.Offset + contentY
			if itemIndex >= 0 && itemIndex < len(m.runs) {
				m.runScroll.SetCursorTo(itemIndex)
				return m, m.onRunChanged(), true
			}
		}
		return m, nil, true
	}
}

// handleLogPanelPress handles a mouse press on a stdout/stderr panel.
// It focuses the panel and starts a text selection.
func (m Model) handleLogPanelPress(x, y int, target panel) (tea.Model, tea.Cmd, bool) {
	m.activePanel = target
	m.updateLogViewportSizes()

	contentLine := m.logContentLineAt(y, target)
	if contentLine >= 0 {
		m.selection = textSelection{
			active:    true,
			panel:     target,
			startLine: contentLine,
			endLine:   contentLine,
		}
	} else {
		m.selection = textSelection{}
	}
	return m, nil, true
}

// handleSelectionDrag updates the selection end position during mouse drag.
func (m Model) handleSelectionDrag(x, y int) (tea.Model, tea.Cmd, bool) {
	contentLine := m.logContentLineAt(y, m.selection.panel)
	if contentLine >= 0 {
		m.selection.endLine = contentLine
		m.selection.dragged = true
	}
	return m, nil, true
}

// handleSelectionRelease finalizes the selection: copies text to clipboard
// if the user dragged, then clears the selection.
func (m Model) handleSelectionRelease() (tea.Model, tea.Cmd, bool) {
	if m.selection.dragged {
		text := m.extractSelectedText()
		if text != "" {
			err := clipboard.WriteAll(text)
			if err != nil {
				m.message = fmt.Sprintf("Failed to copy: %v", err)
				m.isError = true
			} else {
				lineCount := m.selectionLineCount()
				m.message = fmt.Sprintf("Copied %d line(s) to clipboard", lineCount)
				m.isError = false
			}
			m.messageTime = time.Now()
		}
	}
	m.selection = textSelection{}
	return m, nil, true
}

// logContentLineAt converts a terminal Y coordinate to a content-absolute
// line index for the given log panel. Returns -1 if outside content area.
func (m Model) logContentLineAt(y int, target panel) int {
	stdoutH, _ := m.logPanelHeights()

	var panelTopY int
	var yOffset, viewHeight int

	if target == panelStdout {
		panelTopY = 0
		yOffset = m.stdoutView.YOffset
		viewHeight = m.stdoutView.Height
	} else {
		panelTopY = stdoutH
		yOffset = m.stderrView.YOffset
		viewHeight = m.stderrView.Height
	}

	visibleRow := y - panelTopY - 1 // -1 for top border/title
	if visibleRow < 0 || visibleRow >= viewHeight {
		return -1
	}
	return yOffset + visibleRow
}

// extractSelectedText returns the selected lines as plain text (ANSI stripped).
func (m Model) extractSelectedText() string {
	var content string
	if m.selection.panel == panelStdout {
		content = m.formatStdout()
	} else {
		content = m.formatStderr()
	}

	lines := strings.Split(content, "\n")
	startLine, endLine := m.selectionRange()

	if startLine >= len(lines) {
		return ""
	}
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	selected := lines[startLine : endLine+1]
	result := strings.Join(selected, "\n")
	return ansi.Strip(result)
}

// selectionRange returns the start and end lines in order (start <= end).
func (m Model) selectionRange() (startLine, endLine int) {
	s, e := m.selection.startLine, m.selection.endLine
	if s > e {
		s, e = e, s
	}
	return s, e
}

// selectionLineCount returns the number of selected lines.
func (m Model) selectionLineCount() int {
	s, e := m.selectionRange()
	return e - s + 1
}

// highlightedLogView returns the viewport view with selected lines highlighted.
func (m Model) highlightedLogView(target panel) string {
	var view string
	var yOffset int

	if target == panelStdout {
		view = m.stdoutView.View()
		yOffset = m.stdoutView.YOffset
	} else {
		view = m.stderrView.View()
		yOffset = m.stderrView.YOffset
	}

	if !m.selection.active || m.selection.panel != target {
		return view
	}

	startLine, endLine := m.selectionRange()
	lines := strings.Split(view, "\n")

	for i := range lines {
		contentLine := yOffset + i
		if contentLine >= startLine && contentLine <= endLine {
			// Strip existing ANSI and re-render with selection style
			plain := ansi.Strip(lines[i])
			lines[i] = textSelectionStyle.Render(plain)
		}
	}
	return strings.Join(lines, "\n")
}

// selectedJobPortCount returns the number of ports for the currently selected job.
func (m Model) selectedJobPortCount() int {
	if len(m.jobs) > 0 && m.jobScroll.Cursor < len(m.jobs) && m.jobs[m.jobScroll.Cursor].Running {
		return len(m.jobs[m.jobScroll.Cursor].Ports)
	}
	return 0
}

// runsHeaderRows returns the number of non-item header rows inside the runs panel content.
// This accounts for the stats line, optional progress bar, and empty separator.
func (m Model) runsHeaderRows() int {
	headerRows := 2 // stats(1) + empty(1)
	if len(m.jobs) > 0 && m.jobScroll.Cursor < len(m.jobs) &&
		m.jobs[m.jobScroll.Cursor].Running &&
		m.stats != nil && m.stats.AvgDurationMs > 0 {
		headerRows = 3 // stats(1) + progress(1) + empty(1)
	}
	return headerRows
}


