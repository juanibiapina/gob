package tui

import (
	"testing"
)

func TestLeftPanelLayout_BoundariesSumToTotal(t *testing.T) {
	m := Model{
		height: 51, // totalH = 50
		width:  200,
		jobs:   []Job{}, // no description
	}

	l := m.leftPanelLayout()
	totalH := 50

	got := l.infoH + l.descH + l.jobsH + l.portsH + l.runsH
	if got != totalH {
		t.Errorf("panel heights sum = %d, want %d (infoH=%d descH=%d jobsH=%d portsH=%d runsH=%d)",
			got, totalH, l.infoH, l.descH, l.jobsH, l.portsH, l.runsH)
	}
}

func TestLeftPanelLayout_BoundariesSumWithDescription(t *testing.T) {
	m := Model{
		height: 51, // totalH = 50
		width:  200,
		jobs: []Job{
			{ID: "test", Description: "has description"},
		},
	}

	l := m.leftPanelLayout()
	totalH := 50

	got := l.infoH + l.descH + l.jobsH + l.portsH + l.runsH
	if got != totalH {
		t.Errorf("panel heights sum = %d, want %d", got, totalH)
	}
	if l.descH != 3 {
		t.Errorf("descH = %d, want 3 when job has description", l.descH)
	}
}

func TestLeftPanelLayout_StartPositions(t *testing.T) {
	m := Model{
		height: 51,
		width:  200,
		jobs:   []Job{},
	}

	l := m.leftPanelLayout()

	if l.jobsStart != l.infoH+l.descH {
		t.Errorf("jobsStart = %d, want infoH(%d) + descH(%d) = %d",
			l.jobsStart, l.infoH, l.descH, l.infoH+l.descH)
	}
	if l.portsStart != l.jobsStart+l.jobsH {
		t.Errorf("portsStart = %d, want jobsStart(%d) + jobsH(%d) = %d",
			l.portsStart, l.jobsStart, l.jobsH, l.jobsStart+l.jobsH)
	}
	if l.runsStart != l.portsStart+l.portsH {
		t.Errorf("runsStart = %d, want portsStart(%d) + portsH(%d) = %d",
			l.runsStart, l.portsStart, l.portsH, l.portsStart+l.portsH)
	}
}

func TestLeftPanelLayout_MinimumHeights(t *testing.T) {
	// Tiny terminal — minimums should be enforced
	m := Model{
		height: 5, // totalH = 4, clamped to 8
		width:  200,
		jobs:   []Job{},
	}

	l := m.leftPanelLayout()

	if l.portsH < 4 {
		t.Errorf("portsH = %d, want >= 4 (minimum)", l.portsH)
	}
	if l.runsH < 5 {
		t.Errorf("runsH = %d, want >= 5 (minimum)", l.runsH)
	}
}

func TestLeftPanelLayout_DescriptionChangesLayout(t *testing.T) {
	// Without description
	m1 := Model{height: 51, width: 200, jobs: []Job{}}
	l1 := m1.leftPanelLayout()

	// With description
	m2 := Model{height: 51, width: 200, jobs: []Job{{ID: "test", Description: "desc"}}}
	l2 := m2.leftPanelLayout()

	if l2.descH != 3 {
		t.Errorf("with description: descH = %d, want 3", l2.descH)
	}
	if l1.descH != 0 {
		t.Errorf("without description: descH = %d, want 0", l1.descH)
	}
	// Jobs panel should be smaller when description takes space
	if l2.jobsH >= l1.jobsH {
		t.Errorf("jobsH with desc (%d) should be smaller than without (%d)", l2.jobsH, l1.jobsH)
	}
}
