package tui

// ScrollState manages cursor position and scroll offset for a scrollable list.
// It handles navigation (up/down/first/last) and ensures the cursor stays visible.
type ScrollState struct {
	Cursor      int // Selected item index
	Offset      int // First visible item index
	VisibleRows int // Number of visible rows (set on window resize)
}

// Up moves the cursor up by one, adjusting offset if needed.
// Returns true if cursor changed.
func (s *ScrollState) Up() bool {
	if s.Cursor <= 0 {
		return false
	}
	s.Cursor--
	if s.Cursor < s.Offset {
		s.Offset = s.Cursor
	}
	return true
}

// Down moves the cursor down by one, adjusting offset if needed.
// Returns true if cursor changed.
func (s *ScrollState) Down(itemCount int) bool {
	if s.Cursor >= itemCount-1 {
		return false
	}
	s.Cursor++
	if s.VisibleRows > 0 && s.Cursor >= s.Offset+s.VisibleRows {
		s.Offset = s.Cursor - s.VisibleRows + 1
	}
	return true
}

// First moves cursor to the first item.
func (s *ScrollState) First() {
	s.Cursor = 0
	s.Offset = 0
}

// Last moves cursor to the last item, adjusting offset to show it.
func (s *ScrollState) Last(itemCount int) {
	if itemCount <= 0 {
		return
	}
	s.Cursor = itemCount - 1
	if s.VisibleRows > 0 && s.Cursor >= s.VisibleRows {
		s.Offset = s.Cursor - s.VisibleRows + 1
	} else {
		s.Offset = 0
	}
}

// VisibleRange returns the start (inclusive) and end (exclusive) indices
// of items that should be rendered.
func (s *ScrollState) VisibleRange(itemCount int) (start, end int) {
	start = s.Offset
	end = s.Offset + s.VisibleRows
	if end > itemCount {
		end = itemCount
	}
	if start > end {
		start = end
	}
	return start, end
}

// ClampToCount ensures cursor and offset are valid for the given item count.
// Call this after items are added/removed externally.
func (s *ScrollState) ClampToCount(itemCount int) {
	if s.Cursor >= itemCount {
		s.Cursor = itemCount - 1
	}
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	if s.Offset > s.Cursor {
		s.Offset = s.Cursor
	}
	if s.Offset < 0 {
		s.Offset = 0
	}
}

// Reset resets cursor and offset to zero.
func (s *ScrollState) Reset() {
	s.Cursor = 0
	s.Offset = 0
}

// SetCursorTo moves cursor to the specified index, adjusting offset to keep it visible.
func (s *ScrollState) SetCursorTo(index int) {
	s.Cursor = index
	// Ensure cursor is visible
	if s.Cursor < s.Offset {
		s.Offset = s.Cursor
	}
	if s.VisibleRows > 0 && s.Cursor >= s.Offset+s.VisibleRows {
		s.Offset = s.Cursor - s.VisibleRows + 1
	}
}
