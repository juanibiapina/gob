package telemetry

import "time"

// CLI

func CLICommandRun(commandName string) {
	send("cli:command_run", "command_name", commandName)
}

// MCP

func MCPToolCall(toolName string) {
	send("mcp:tool_call", "tool_name", toolName)
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
