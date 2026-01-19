package telemetry

import "time"

// CLI

var cliCommandName string
var cliStartTime time.Time

func CLICommandStart(commandName string) {
	cliCommandName = commandName
	cliStartTime = time.Now()
}

func CLICommandEnd() {
	if cliCommandName == "" {
		return
	}
	durationMs := time.Since(cliStartTime).Milliseconds()
	send("cli:command_run", "command_name", cliCommandName, "duration_ms", durationMs)
}

// TUI

var tuiStartTime time.Time

func TUISessionStart() {
	tuiStartTime = time.Now()
	send("tui:session_start")
}

func TUISessionEnd() {
	durationMs := time.Since(tuiStartTime).Milliseconds()
	send("tui:session_end", "duration_ms", durationMs)
}

func TUIActionExecute(actionName string) {
	send("tui:action_execute", "action_name", actionName)
}
