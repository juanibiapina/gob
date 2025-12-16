package tui

import "testing"

func TestScrollState_Up(t *testing.T) {
	tests := []struct {
		name        string
		initial     ScrollState
		wantChanged bool
		wantCursor  int
		wantOffset  int
	}{
		{
			name:        "move up from middle",
			initial:     ScrollState{Cursor: 5, Offset: 0, VisibleRows: 10},
			wantChanged: true,
			wantCursor:  4,
			wantOffset:  0,
		},
		{
			name:        "at top, cannot move up",
			initial:     ScrollState{Cursor: 0, Offset: 0, VisibleRows: 10},
			wantChanged: false,
			wantCursor:  0,
			wantOffset:  0,
		},
		{
			name:        "move up scrolls offset",
			initial:     ScrollState{Cursor: 5, Offset: 5, VisibleRows: 10},
			wantChanged: true,
			wantCursor:  4,
			wantOffset:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			changed := s.Up()
			if changed != tt.wantChanged {
				t.Errorf("Up() changed = %v, want %v", changed, tt.wantChanged)
			}
			if s.Cursor != tt.wantCursor {
				t.Errorf("Up() Cursor = %v, want %v", s.Cursor, tt.wantCursor)
			}
			if s.Offset != tt.wantOffset {
				t.Errorf("Up() Offset = %v, want %v", s.Offset, tt.wantOffset)
			}
		})
	}
}

func TestScrollState_Down(t *testing.T) {
	tests := []struct {
		name        string
		initial     ScrollState
		itemCount   int
		wantChanged bool
		wantCursor  int
		wantOffset  int
	}{
		{
			name:        "move down from middle",
			initial:     ScrollState{Cursor: 5, Offset: 0, VisibleRows: 10},
			itemCount:   20,
			wantChanged: true,
			wantCursor:  6,
			wantOffset:  0,
		},
		{
			name:        "at bottom, cannot move down",
			initial:     ScrollState{Cursor: 9, Offset: 0, VisibleRows: 10},
			itemCount:   10,
			wantChanged: false,
			wantCursor:  9,
			wantOffset:  0,
		},
		{
			name:        "move down scrolls offset",
			initial:     ScrollState{Cursor: 9, Offset: 0, VisibleRows: 10},
			itemCount:   20,
			wantChanged: true,
			wantCursor:  10,
			wantOffset:  1,
		},
		{
			name:        "empty list",
			initial:     ScrollState{Cursor: 0, Offset: 0, VisibleRows: 10},
			itemCount:   0,
			wantChanged: false,
			wantCursor:  0,
			wantOffset:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			changed := s.Down(tt.itemCount)
			if changed != tt.wantChanged {
				t.Errorf("Down() changed = %v, want %v", changed, tt.wantChanged)
			}
			if s.Cursor != tt.wantCursor {
				t.Errorf("Down() Cursor = %v, want %v", s.Cursor, tt.wantCursor)
			}
			if s.Offset != tt.wantOffset {
				t.Errorf("Down() Offset = %v, want %v", s.Offset, tt.wantOffset)
			}
		})
	}
}

func TestScrollState_First(t *testing.T) {
	s := ScrollState{Cursor: 15, Offset: 10, VisibleRows: 10}
	s.First()
	if s.Cursor != 0 {
		t.Errorf("First() Cursor = %v, want 0", s.Cursor)
	}
	if s.Offset != 0 {
		t.Errorf("First() Offset = %v, want 0", s.Offset)
	}
}

func TestScrollState_Last(t *testing.T) {
	tests := []struct {
		name       string
		initial    ScrollState
		itemCount  int
		wantCursor int
		wantOffset int
	}{
		{
			name:       "jump to last with scrolling",
			initial:    ScrollState{Cursor: 0, Offset: 0, VisibleRows: 10},
			itemCount:  20,
			wantCursor: 19,
			wantOffset: 10,
		},
		{
			name:       "list smaller than visible rows",
			initial:    ScrollState{Cursor: 0, Offset: 0, VisibleRows: 10},
			itemCount:  5,
			wantCursor: 4,
			wantOffset: 0,
		},
		{
			name:       "empty list",
			initial:    ScrollState{Cursor: 5, Offset: 3, VisibleRows: 10},
			itemCount:  0,
			wantCursor: 5, // unchanged
			wantOffset: 3, // unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			s.Last(tt.itemCount)
			if s.Cursor != tt.wantCursor {
				t.Errorf("Last() Cursor = %v, want %v", s.Cursor, tt.wantCursor)
			}
			if s.Offset != tt.wantOffset {
				t.Errorf("Last() Offset = %v, want %v", s.Offset, tt.wantOffset)
			}
		})
	}
}

func TestScrollState_VisibleRange(t *testing.T) {
	tests := []struct {
		name      string
		state     ScrollState
		itemCount int
		wantStart int
		wantEnd   int
	}{
		{
			name:      "normal range",
			state:     ScrollState{Cursor: 5, Offset: 0, VisibleRows: 10},
			itemCount: 20,
			wantStart: 0,
			wantEnd:   10,
		},
		{
			name:      "scrolled down",
			state:     ScrollState{Cursor: 15, Offset: 10, VisibleRows: 10},
			itemCount: 20,
			wantStart: 10,
			wantEnd:   20,
		},
		{
			name:      "fewer items than visible rows",
			state:     ScrollState{Cursor: 0, Offset: 0, VisibleRows: 10},
			itemCount: 5,
			wantStart: 0,
			wantEnd:   5,
		},
		{
			name:      "empty list",
			state:     ScrollState{Cursor: 0, Offset: 0, VisibleRows: 10},
			itemCount: 0,
			wantStart: 0,
			wantEnd:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := tt.state.VisibleRange(tt.itemCount)
			if start != tt.wantStart {
				t.Errorf("VisibleRange() start = %v, want %v", start, tt.wantStart)
			}
			if end != tt.wantEnd {
				t.Errorf("VisibleRange() end = %v, want %v", end, tt.wantEnd)
			}
		})
	}
}

func TestScrollState_ClampToCount(t *testing.T) {
	tests := []struct {
		name       string
		initial    ScrollState
		itemCount  int
		wantCursor int
		wantOffset int
	}{
		{
			name:       "cursor beyond count",
			initial:    ScrollState{Cursor: 15, Offset: 10, VisibleRows: 10},
			itemCount:  10,
			wantCursor: 9,
			wantOffset: 9,
		},
		{
			name:       "empty list",
			initial:    ScrollState{Cursor: 5, Offset: 3, VisibleRows: 10},
			itemCount:  0,
			wantCursor: 0,
			wantOffset: 0,
		},
		{
			name:       "offset beyond cursor",
			initial:    ScrollState{Cursor: 2, Offset: 5, VisibleRows: 10},
			itemCount:  10,
			wantCursor: 2,
			wantOffset: 2,
		},
		{
			name:       "already valid",
			initial:    ScrollState{Cursor: 5, Offset: 3, VisibleRows: 10},
			itemCount:  20,
			wantCursor: 5,
			wantOffset: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			s.ClampToCount(tt.itemCount)
			if s.Cursor != tt.wantCursor {
				t.Errorf("ClampToCount() Cursor = %v, want %v", s.Cursor, tt.wantCursor)
			}
			if s.Offset != tt.wantOffset {
				t.Errorf("ClampToCount() Offset = %v, want %v", s.Offset, tt.wantOffset)
			}
		})
	}
}

func TestScrollState_Reset(t *testing.T) {
	s := ScrollState{Cursor: 15, Offset: 10, VisibleRows: 10}
	s.Reset()
	if s.Cursor != 0 {
		t.Errorf("Reset() Cursor = %v, want 0", s.Cursor)
	}
	if s.Offset != 0 {
		t.Errorf("Reset() Offset = %v, want 0", s.Offset)
	}
	// VisibleRows should be preserved
	if s.VisibleRows != 10 {
		t.Errorf("Reset() VisibleRows = %v, want 10", s.VisibleRows)
	}
}

func TestScrollState_ShiftForInsertAt(t *testing.T) {
	tests := []struct {
		name        string
		initial     ScrollState
		insertIndex int
		wantCursor  int
		wantOffset  int
	}{
		{
			name:        "insert before cursor",
			initial:     ScrollState{Cursor: 5, Offset: 3, VisibleRows: 10},
			insertIndex: 2,
			wantCursor:  6,
			wantOffset:  4,
		},
		{
			name:        "insert at cursor",
			initial:     ScrollState{Cursor: 5, Offset: 3, VisibleRows: 10},
			insertIndex: 5,
			wantCursor:  6,
			wantOffset:  4,
		},
		{
			name:        "insert after cursor",
			initial:     ScrollState{Cursor: 5, Offset: 3, VisibleRows: 10},
			insertIndex: 7,
			wantCursor:  5,
			wantOffset:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			s.ShiftForInsertAt(tt.insertIndex)
			if s.Cursor != tt.wantCursor {
				t.Errorf("ShiftForInsertAt() Cursor = %v, want %v", s.Cursor, tt.wantCursor)
			}
			if s.Offset != tt.wantOffset {
				t.Errorf("ShiftForInsertAt() Offset = %v, want %v", s.Offset, tt.wantOffset)
			}
		})
	}
}

func TestScrollState_SetCursorTo(t *testing.T) {
	tests := []struct {
		name       string
		initial    ScrollState
		index      int
		wantCursor int
		wantOffset int
	}{
		{
			name:       "set within visible range",
			initial:    ScrollState{Cursor: 5, Offset: 0, VisibleRows: 10},
			index:      7,
			wantCursor: 7,
			wantOffset: 0,
		},
		{
			name:       "set below visible range, scrolls down",
			initial:    ScrollState{Cursor: 5, Offset: 0, VisibleRows: 10},
			index:      15,
			wantCursor: 15,
			wantOffset: 6,
		},
		{
			name:       "set above visible range, scrolls up",
			initial:    ScrollState{Cursor: 15, Offset: 10, VisibleRows: 10},
			index:      5,
			wantCursor: 5,
			wantOffset: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			s.SetCursorTo(tt.index)
			if s.Cursor != tt.wantCursor {
				t.Errorf("SetCursorTo() Cursor = %v, want %v", s.Cursor, tt.wantCursor)
			}
			if s.Offset != tt.wantOffset {
				t.Errorf("SetCursorTo() Offset = %v, want %v", s.Offset, tt.wantOffset)
			}
		})
	}
}
