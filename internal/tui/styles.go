package tui

import "github.com/charmbracelet/lipgloss"

// CharmTone-inspired color palette
var (
	// Backgrounds (dark to light)
	bgPepper   = lipgloss.Color("#201F26") // Darkest
	bgBBQ      = lipgloss.Color("#2d2c35") // Dark
	bgCharcoal = lipgloss.Color("#3A3943") // Base
	bgIron     = lipgloss.Color("#4D4C57") // Lighter

	// Text (muted to bright)
	fgOyster = lipgloss.Color("#605F6B") // Muted
	fgSquid  = lipgloss.Color("#858392") // Subtle
	fgSmoke  = lipgloss.Color("#BFBCC8") // Base
	fgAsh    = lipgloss.Color("#DFDBDD") // Light
	fgSalt   = lipgloss.Color("#F1EFEF") // Bright

	// Accents
	accentMalibu = lipgloss.Color("#00A4FF") // Blue (primary)
	accentGuac   = lipgloss.Color("#12C78F") // Green (success)
	accentCoral  = lipgloss.Color("#FF577D") // Pink (error)
	accentViolet = lipgloss.Color("#C259FF") // Purple (highlight)
	accentAmber  = lipgloss.Color("#FFAA33") // Orange (warning)

	// Semantic aliases
	primaryColor   = accentMalibu
	successColor   = accentGuac
	dangerColor    = accentCoral
	warningColor   = accentAmber
	highlightColor = accentViolet
	mutedColor     = fgOyster
	bgColor        = bgBBQ
	fgColor        = fgSmoke

	// Header bar
	headerStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(bgPepper).
			Bold(true).
			Padding(0, 1)

	// Status bar (no background - uses terminal default)
	statusBarStyle = lipgloss.NewStyle().
			Foreground(fgSmoke)

	statusKeyStyle = lipgloss.NewStyle().
			Background(bgIron).
			Foreground(primaryColor).
			Padding(0, 1)

	statusTextStyle = lipgloss.NewStyle().
			Background(bgCharcoal).
			Foreground(fgSquid).
			Padding(0, 1)

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(fgOyster)

	panelTitleStyle = lipgloss.NewStyle().
			Background(bgIron).
			Foreground(fgAsh).
			Padding(0, 1).
			Bold(true)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor)

	activePanelTitleStyle = lipgloss.NewStyle().
				Background(primaryColor).
				Foreground(bgPepper).
				Padding(0, 1).
				Bold(true)

	// Job list styles - selection background color
	selectionBg = lipgloss.Color("#2d3f76")

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
			Foreground(fgOyster) // Gray for manually stopped/killed

	jobStoppedSelectedStyle = lipgloss.NewStyle().
				Foreground(fgOyster).
				Background(selectionBg)

	jobIDStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	jobIDSelectedStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Background(selectionBg)

	jobPIDStyle = lipgloss.NewStyle().
			Foreground(fgSquid)

	jobPIDSelectedStyle = lipgloss.NewStyle().
				Foreground(fgSquid).
				Background(selectionBg)

	jobCommandStyle = lipgloss.NewStyle().
			Foreground(fgSmoke)

	jobCommandSelectedStyle = lipgloss.NewStyle().
				Foreground(fgSmoke).
				Background(selectionBg)

	// Log styles
	logPrefixStyle = lipgloss.NewStyle().
			Foreground(fgSmoke)

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
			Foreground(fgSquid)

	// Workdir style
	workdirStyle = lipgloss.NewStyle().
			Foreground(fgSquid).
			Italic(true)

	workdirSelectedStyle = lipgloss.NewStyle().
				Foreground(fgSquid).
				Background(selectionBg).
				Italic(true)

	// Help key style
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(fgSquid)
)
