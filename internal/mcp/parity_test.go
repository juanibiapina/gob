package mcp_test

import (
	"strings"
	"testing"

	"github.com/juanibiapina/gob/cmd"
	"github.com/juanibiapina/gob/internal/mcp"
)

// This test ensures CLI commands and MCP tools stay in sync.
// When adding a new CLI command, you must either:
// 1. Add a corresponding MCP tool, OR
// 2. Add the command to cliOnlyCommands with a reason
//
// When adding a new MCP tool, you must either:
// 1. Add a corresponding CLI command, OR
// 2. Add the tool to mcpOnlyTools with a reason

// Naming convention:
// - CLI commands use verbs: "add", "list", "stop", "await-any"
// - MCP tools use "gob_" prefix: gob_add, gob_list, gob_await_any
// - Hyphens in CLI commands become underscores in MCP tools

// cliOnlyCommands lists CLI commands that intentionally have no MCP equivalent.
// Each entry must include a reason explaining why.
var cliOnlyCommands = map[string]string{
	"completion": "Shell completion - not applicable to MCP",
	"daemon":     "Internal command to start the daemon process",
	"events":     "Internal debugging command",
	"follow":     "Internal command used by logs",
	"help":       "Built-in Cobra command",
	"logs":       "Follows all logs interactively - MCP has gob_stdout/gob_stderr per job",
	"mcp":        "Starts the MCP server itself",
	"overview":   "Shows CLI help/overview",
	"ping":       "Daemon health check - not needed for MCP (connection errors are reported)",
	"run":        "Convenience command (add + await) - MCP clients can call gob_add then gob_await",
	"tui":        "Interactive terminal UI - not applicable to MCP",
}

// mcpOnlyTools lists MCP tools that intentionally have no CLI equivalent.
// Each entry must include a reason explaining why.
var mcpOnlyTools = map[string]string{
	// Currently empty - all MCP tools have CLI equivalents
}

// getCLICommands returns all CLI command names from the root command.
func getCLICommands() []string {
	var commands []string
	for _, c := range cmd.RootCmd.Commands() {
		commands = append(commands, c.Name())
	}
	return commands
}

// cliToMCPName converts a CLI command name to its MCP tool name.
// Examples: "add" -> "gob_add", "await-any" -> "gob_await_any"
func cliToMCPName(cliCmd string) string {
	return "gob_" + strings.ReplaceAll(cliCmd, "-", "_")
}

// mcpToCLIName converts an MCP tool name to its CLI command name.
// Examples: "gob_add" -> "add", "gob_await_any" -> "await-any"
func mcpToCLIName(mcpTool string) string {
	name := strings.TrimPrefix(mcpTool, "gob_")
	return strings.ReplaceAll(name, "_", "-")
}

func TestCLIMCPParity(t *testing.T) {
	// Get MCP tools from server
	server := mcp.NewServer("test")
	mcpTools := make(map[string]bool)
	for _, toolName := range server.ListToolNames() {
		mcpTools[toolName] = true
	}

	// Get CLI commands dynamically
	cliCommands := getCLICommands()

	// Check that all CLI commands either have MCP tools or are in cliOnlyCommands
	var missingMCPTools []string
	for _, cliCmd := range cliCommands {
		if _, isException := cliOnlyCommands[cliCmd]; isException {
			continue
		}
		mcpTool := cliToMCPName(cliCmd)
		if !mcpTools[mcpTool] {
			missingMCPTools = append(missingMCPTools, mcpTool+" (for CLI command '"+cliCmd+"')")
		}
	}

	if len(missingMCPTools) > 0 {
		t.Errorf(`
MCP tools missing for CLI commands:
  %v

To fix this, either:
1. Implement the missing MCP tool(s) in internal/mcp/server.go
2. If the CLI command should NOT have an MCP equivalent, add it to cliOnlyCommands with a reason
`, missingMCPTools)
	}

	// Build set of CLI commands that should have MCP tools
	cliCommandSet := make(map[string]bool)
	for _, cliCmd := range cliCommands {
		if _, isException := cliOnlyCommands[cliCmd]; !isException {
			cliCommandSet[cliCmd] = true
		}
	}

	// Check that all MCP tools have corresponding CLI commands (or are in mcpOnlyTools)
	var unexpectedMCPTools []string
	for mcpTool := range mcpTools {
		cliCmd := mcpToCLIName(mcpTool)
		if !cliCommandSet[cliCmd] {
			if _, isException := mcpOnlyTools[mcpTool]; !isException {
				unexpectedMCPTools = append(unexpectedMCPTools, mcpTool+" (would be CLI command '"+cliCmd+"')")
			}
		}
	}

	if len(unexpectedMCPTools) > 0 {
		t.Errorf(`
MCP tools without corresponding CLI command:
  %v

To fix this, either:
1. Add the corresponding CLI command
2. If the MCP tool should NOT have a CLI equivalent, add it to mcpOnlyTools with a reason
`, unexpectedMCPTools)
	}
}

func TestCLIOnlyCommandsHaveReasons(t *testing.T) {
	for cmd, reason := range cliOnlyCommands {
		if reason == "" {
			t.Errorf("CLI-only command %q has no reason - explain why it has no MCP equivalent", cmd)
		}
	}
}

func TestMCPOnlyToolsHaveReasons(t *testing.T) {
	for tool, reason := range mcpOnlyTools {
		if reason == "" {
			t.Errorf("MCP-only tool %q has no reason - explain why it has no CLI equivalent", tool)
		}
	}
}

func TestNameConversion(t *testing.T) {
	tests := []struct {
		cli string
		mcp string
	}{
		{"add", "gob_add"},
		{"list", "gob_list"},
		{"await-any", "gob_await_any"},
		{"await-all", "gob_await_all"},
		{"cleanup", "gob_cleanup"},
		{"nuke", "gob_nuke"},
	}

	for _, tt := range tests {
		if got := cliToMCPName(tt.cli); got != tt.mcp {
			t.Errorf("cliToMCPName(%q) = %q, want %q", tt.cli, got, tt.mcp)
		}
		if got := mcpToCLIName(tt.mcp); got != tt.cli {
			t.Errorf("mcpToCLIName(%q) = %q, want %q", tt.mcp, got, tt.cli)
		}
	}
}
