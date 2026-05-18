package tui

// leftLayout holds the computed Y boundaries and heights for left-side panels.
// All values are in terminal rows. The panels stack top-to-bottom:
// info → jobs → [description] → ports → runs
type leftLayout struct {
	infoH  int // info panel height (fixed at 3)
	descH  int // description panel height (0 or 3)
	jobsH  int // jobs panel height
	portsH int // ports panel height
	runsH  int // runs panel height

	// Y start positions (0-indexed from top of terminal)
	jobsStart  int
	portsStart int
	runsStart  int
}

// leftPanelLayout computes the Y boundaries for left-side panels.
// This is the single source of truth — used by renderPanels, WindowSizeMsg,
// and mouse hit-testing.
func (m Model) leftPanelLayout() leftLayout {
	totalH := m.height - 1 // height - status bar
	if totalH < 8 {
		totalH = 8
	}

	infoH := 3

	hasDesc := m.selectedJobHasDescription()
	descH := 0
	if hasDesc {
		descH = 3
	}

	leftH := totalH - infoH - descH

	portsH := leftH * 20 / 100
	if portsH < 4 {
		portsH = 4
	}
	runsH := leftH * 30 / 100
	if runsH < 5 {
		runsH = 5
	}
	jobsH := leftH - portsH - runsH

	jobsStart := infoH
	portsStart := jobsStart + jobsH + descH
	runsStart := portsStart + portsH

	return leftLayout{
		infoH:      infoH,
		descH:      descH,
		jobsH:      jobsH,
		portsH:     portsH,
		runsH:      runsH,
		jobsStart:  jobsStart,
		portsStart: portsStart,
		runsStart:  runsStart,
	}
}
