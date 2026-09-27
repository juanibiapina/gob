package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/juanibiapina/gob/internal/daemon"
)

func TestJobShowsStoppingUntilCompletion(t *testing.T) {
	m := Model{jobs: []Job{{ID: "abc", Command: "sleep 30", Running: true}}, width: 70, activePanel: panelJobs, jobScroll: ScrollState{VisibleRows: 1}}
	updated, cmd := m.updateMain(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil || !updated.(Model).jobs[0].Stopping {
		t.Fatal("stop key did not render pending state immediately")
	}
	m = updated.(Model)
	m.handleDaemonEvent(daemon.Event{Type: daemon.EventTypeJobStopping, JobID: "abc", Job: daemon.JobResponse{ID: "abc", Status: "stopping"}})
	if !m.jobs[0].Stopping || !strings.Contains(m.renderJobList(70), "stopping:") {
		t.Fatal("pending stop was not visible")
	}
	if command, _ := m.jobLifecycleCmd("s"); command != nil {
		t.Fatal("duplicate graceful stop remained available")
	}
	m.handleDaemonEvent(daemon.Event{Type: daemon.EventTypeJobStopped, JobID: "abc", Job: daemon.JobResponse{ID: "abc", Status: "stopped"}})
	if m.jobs[0].Stopping || m.jobs[0].Running {
		t.Fatal("verified completion did not clear stopping")
	}
}
