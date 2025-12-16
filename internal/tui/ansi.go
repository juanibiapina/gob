// ansi.go - ANSI escape sequence handling for safe TUI rendering

package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// cursorSequenceRegex matches ANSI cursor movement and screen control sequences.
// These are used by progress bars and spinners to update in place, but they
// break TUI rendering when displayed in a viewport.
var cursorSequenceRegex = regexp.MustCompile(
	`\x1b\[` + // CSI (Control Sequence Introducer)
		`(?:` +
		`\d*[ABCDEFGH]` + // Cursor movement (up/down/forward/back/next line/prev line/column/position)
		`|\d*;\d*[Hf]` + // Cursor position (row;col)
		`|[suKJ]` + // Save/restore cursor, erase line/screen
		`|\d*[KJ]` + // Erase with count
		`|\?(?:25[hl]|\d+[hl])` + // Show/hide cursor, other private modes
		`)`,
)

// StripCursorSequences removes ANSI cursor movement and line erasing sequences
// while preserving color codes. Progress bars and spinners use these sequences
// to update in place, which breaks TUI rendering.
func StripCursorSequences(s string) string {
	return cursorSequenceRegex.ReplaceAllString(s, "")
}

// FitToWidth ensures a string is exactly the specified visual width.
// If the string is too long, it truncates using ANSI-aware truncation.
// If the string is too short, it pads with spaces.
// Color codes are preserved in both cases.
func FitToWidth(s string, width int) string {
	currentWidth := lipgloss.Width(s)

	if currentWidth > width {
		// Truncate using ANSI-aware function (preserves escape sequences)
		return ansi.Truncate(s, width, "")
	}

	if currentWidth < width {
		// Pad with spaces to reach exact width
		return s + strings.Repeat(" ", width-currentWidth)
	}

	return s
}

// FitCellContent ensures a string fits within the specified width for table cells.
// If the string is too long, it truncates with an ellipsis (…).
// If the string is too short, it pads with spaces.
// Color codes are preserved in both cases.
func FitCellContent(s string, width int) string {
	if width <= 0 {
		return ""
	}

	currentWidth := lipgloss.Width(s)

	if currentWidth > width {
		// Truncate with ellipsis using ANSI-aware function
		if width <= 1 {
			return "…"
		}
		return ansi.Truncate(s, width-1, "") + "…"
	}

	if currentWidth < width {
		// Pad with spaces to reach exact width
		return s + strings.Repeat(" ", width-currentWidth)
	}

	return s
}
