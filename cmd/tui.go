package cmd

import (
	"github.com/juanibiapina/gob/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI",
	Long: `Launch an interactive terminal user interface for managing gob jobs.

The TUI provides a full-screen split-panel interface with:
  - Left panel: Job list with status indicators
  - Right panel: Live log viewer for selected job
  - Real-time updates every 500ms

LAYOUT:
  ┌─ Jobs ─────────────┬─ Logs ──────────────────┐
  │ ● V3x0QqI [1234]   │ [V3x0QqI] output line 1 │
  │ ○ V3x0PrH [1235]   │ [V3x0QqI] output line 2 │
  └────────────────────┴─────────────────────────┘

KEYBINDINGS:

  Navigation:
    tab       Switch between Jobs/Logs panels
    ↑/k ↓/j   Move cursor (Jobs) or scroll (Logs)
    g/G       First/last item or top/bottom of logs

  Job Actions (in Jobs panel):
    s         Stop job (SIGTERM)
    S         Force kill job (SIGKILL)
    r         Restart job
    d         Delete stopped job
    n         Start new job

  Log Viewer (in Logs panel):
    pgup/pgdn Page scroll
    f         Toggle follow mode (auto-scroll)

  Global:
    a         Toggle all directories / current directory
    ?         Show help overlay
    q         Quit

Example:
  gob tui`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

func init() {
	RootCmd.AddCommand(tuiCmd)
}
