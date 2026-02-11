package tui

import (
	"testing"
)

func TestLogPanelHeights_DefaultPanel(t *testing.T) {
	// When stderr is not focused, it should get 20% and stdout 80%
	m := Model{
		height:      101, // totalH = 100
		width:       200,
		activePanel: panelJobs,
	}

	stdoutH, stderrH := m.logPanelHeights()

	if stderrH != 20 {
		t.Errorf("stderrH = %d, want 20 (20%% of 100)", stderrH)
	}
	if stdoutH != 80 {
		t.Errorf("stdoutH = %d, want 80 (100 - 20)", stdoutH)
	}
}

func TestLogPanelHeights_StderrFocused(t *testing.T) {
	// When stderr is focused, it should get 80% and stdout 20%
	m := Model{
		height:      101, // totalH = 100
		width:       200,
		activePanel: panelStderr,
	}

	stdoutH, stderrH := m.logPanelHeights()

	if stderrH != 80 {
		t.Errorf("stderrH = %d, want 80 (80%% of 100)", stderrH)
	}
	if stdoutH != 20 {
		t.Errorf("stdoutH = %d, want 20 (100 - 80)", stdoutH)
	}
}

func TestLogPanelHeights_StdoutFocused(t *testing.T) {
	// Stdout focused should behave like default (stderr stays small)
	m := Model{
		height:      101,
		width:       200,
		activePanel: panelStdout,
	}

	stdoutH, stderrH := m.logPanelHeights()

	if stderrH != 20 {
		t.Errorf("stderrH = %d, want 20", stderrH)
	}
	if stdoutH != 80 {
		t.Errorf("stdoutH = %d, want 80", stdoutH)
	}
}

func TestLogPanelHeights_MinimumStderrHeight(t *testing.T) {
	// With very small terminal, stderr should be at least 4
	m := Model{
		height:      10, // totalH = 9
		width:       200,
		activePanel: panelJobs,
	}

	stdoutH, stderrH := m.logPanelHeights()

	// 9 * 20 / 100 = 1, which is less than 4
	if stderrH < 4 {
		t.Errorf("stderrH = %d, want >= 4 (minimum)", stderrH)
	}
	if stdoutH+stderrH != 9 {
		t.Errorf("stdoutH(%d) + stderrH(%d) = %d, want 9 (totalH)", stdoutH, stderrH, stdoutH+stderrH)
	}
}

func TestLogPanelHeights_MinimumTotalHeight(t *testing.T) {
	// With tiny terminal, totalH should be clamped to 8
	m := Model{
		height:      3, // totalH = 2, clamped to 8
		width:       200,
		activePanel: panelJobs,
	}

	stdoutH, stderrH := m.logPanelHeights()

	// totalH clamped to 8, stderrH = 8 * 20 / 100 = 1, clamped to 4
	if stderrH < 4 {
		t.Errorf("stderrH = %d, want >= 4 (minimum)", stderrH)
	}
	if stdoutH+stderrH != 8 {
		t.Errorf("stdoutH(%d) + stderrH(%d) = %d, want 8 (clamped totalH)", stdoutH, stderrH, stdoutH+stderrH)
	}
}

func TestLogPanelHeights_SumEqualsTotal(t *testing.T) {
	// Heights should always sum to totalH for all panels
	panels := []panel{panelJobs, panelPorts, panelRuns, panelStdout, panelStderr}

	for _, p := range panels {
		m := Model{
			height:      51, // totalH = 50
			width:       200,
			activePanel: p,
		}

		stdoutH, stderrH := m.logPanelHeights()
		totalH := 50

		if stdoutH+stderrH != totalH {
			t.Errorf("panel %d: stdoutH(%d) + stderrH(%d) = %d, want %d",
				p, stdoutH, stderrH, stdoutH+stderrH, totalH)
		}
	}
}

func TestUpdateLogViewportSizes_SetsCorrectDimensions(t *testing.T) {
	m := New()
	m.width = 200
	m.height = 101 // totalH = 100
	m.activePanel = panelJobs

	m.updateLogViewportSizes()

	// jobPanelWidth for width=200: 200*40/100 = 80, capped at 60
	// rightPanelW = 200 - 60 = 140
	// viewportWidth = 140 - 4 = 136
	expectedWidth := 136

	// stderrH = 100 * 20 / 100 = 20, viewportHeight = 20 - 3 = 17
	// stdoutH = 100 - 20 = 80, viewportHeight = 80 - 3 = 77
	expectedStderrHeight := 17
	expectedStdoutHeight := 77

	if m.stderrView.Width != expectedWidth {
		t.Errorf("stderrView.Width = %d, want %d", m.stderrView.Width, expectedWidth)
	}
	if m.stderrView.Height != expectedStderrHeight {
		t.Errorf("stderrView.Height = %d, want %d", m.stderrView.Height, expectedStderrHeight)
	}
	if m.stdoutView.Width != expectedWidth {
		t.Errorf("stdoutView.Width = %d, want %d", m.stdoutView.Width, expectedWidth)
	}
	if m.stdoutView.Height != expectedStdoutHeight {
		t.Errorf("stdoutView.Height = %d, want %d", m.stdoutView.Height, expectedStdoutHeight)
	}
}

func TestUpdateLogViewportSizes_StderrExpands(t *testing.T) {
	m := New()
	m.width = 200
	m.height = 101 // totalH = 100

	// First set to jobs panel (stderr = 20%)
	m.activePanel = panelJobs
	m.updateLogViewportSizes()
	smallStderrHeight := m.stderrView.Height
	smallStdoutHeight := m.stdoutView.Height

	// Switch to stderr panel (stderr = 80%)
	m.activePanel = panelStderr
	m.updateLogViewportSizes()
	bigStderrHeight := m.stderrView.Height
	bigStdoutHeight := m.stdoutView.Height

	if bigStderrHeight <= smallStderrHeight {
		t.Errorf("stderr focused: Height %d should be > unfocused Height %d",
			bigStderrHeight, smallStderrHeight)
	}
	if bigStdoutHeight >= smallStdoutHeight {
		t.Errorf("stderr focused: stdout Height %d should be < unfocused stdout Height %d",
			bigStdoutHeight, smallStdoutHeight)
	}
}
