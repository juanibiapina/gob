package tui

import "github.com/charmbracelet/lipgloss"

// Terminal theme colors (ANSI 0-15)
// These adapt to the user's terminal color scheme
var (
	// Basic colors (0-7)
	colorBlack   = lipgloss.Color("0")
	colorRed     = lipgloss.Color("1")
	colorGreen   = lipgloss.Color("2")
	colorYellow  = lipgloss.Color("3")
	colorBlue    = lipgloss.Color("4")
	colorMagenta = lipgloss.Color("5")
	colorCyan    = lipgloss.Color("6")
	colorWhite   = lipgloss.Color("7")

	// Bright colors (8-15)
	colorBrightBlack   = lipgloss.Color("8")
	colorBrightRed     = lipgloss.Color("9")
	colorBrightGreen   = lipgloss.Color("10")
	colorBrightYellow  = lipgloss.Color("11")
	colorBrightBlue    = lipgloss.Color("12")
	colorBrightMagenta = lipgloss.Color("13")
	colorBrightCyan    = lipgloss.Color("14")
	colorBrightWhite   = lipgloss.Color("15")

	// Semantic aliases
	primaryColor   = colorYellow
	successColor   = colorGreen
	dangerColor    = colorRed
	warningColor   = colorYellow
	highlightColor = colorMagenta
	mutedColor     = colorBrightBlack
	fgColor        = colorWhite

	// Header bar
	headerStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(colorBlack).
			Bold(true).
			Padding(0, 1)

	// Status bar (no background - uses terminal default)
	statusBarStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	statusKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1)

	statusTextStyle = lipgloss.NewStyle().
			Foreground(colorBrightBlack).
			Padding(0, 1)

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBrightBlack)

	panelTitleStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Padding(0, 1).
			Bold(true)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor)

	activePanelTitleStyle = lipgloss.NewStyle().
				Background(primaryColor).
				Foreground(colorBlack).
				Padding(0, 1).
				Bold(true)

	// Job list styles - selection uses bright black background
	selectionBg = colorBrightBlack

	jobSelectedBgStyle = lipgloss.NewStyle().
				Background(selectionBg)

	jobRunningStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	jobRunningSelectedStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Background(selectionBg).
				Bold(true)

	jobSuccessStyle = lipgloss.NewStyle().
			Foreground(successColor)

	jobSuccessSelectedStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Background(selectionBg)

	jobFailedStyle = lipgloss.NewStyle().
			Foreground(dangerColor)

	jobFailedSelectedStyle = lipgloss.NewStyle().
				Foreground(dangerColor).
				Background(selectionBg)

	jobStoppedStyle = lipgloss.NewStyle().
			Foreground(colorBrightBlack)

	jobStoppedSelectedStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(selectionBg)

	jobIDStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	jobIDSelectedStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Background(selectionBg)

	jobPIDStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	jobPIDSelectedStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Background(selectionBg)

	jobCommandStyle = lipgloss.NewStyle()

	jobCommandSelectedStyle = lipgloss.NewStyle().
				Background(selectionBg)

	// Log styles
	logPrefixStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	logStderrPrefixStyle = lipgloss.NewStyle().
				Foreground(warningColor)

	// Modal/dialog styles
	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	dialogTitleStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	// Input styles
	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	// Error/success messages
	errorStyle = lipgloss.NewStyle().
			Foreground(dangerColor).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorBrightBlack)

	// Workdir style
	workdirStyle = lipgloss.NewStyle().
			Foreground(colorBrightBlack).
			Italic(true)

	workdirSelectedStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(selectionBg).
				Italic(true)

	// Help key style
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	// Job detail line styles (for expanded view)
	jobDetailStyle = lipgloss.NewStyle().
			Foreground(colorBrightBlack)

	jobDetailSelectedStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(selectionBg)

	jobTimeStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	jobTimeSelectedStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Background(selectionBg)
)
