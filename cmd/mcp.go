package cmd

import (
	"github.com/juanibiapina/gob/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server on stdio",
	Long: `Start an MCP (Model Context Protocol) server on stdio.

This allows AI agents like Claude Code to manage gob jobs through the MCP protocol.

Example configuration for .mcp.json:
  {
    "mcpServers": {
      "gob": {
        "command": "gob",
        "args": ["mcp"]
      }
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server := mcp.NewServer(version)
		return server.Serve()
	},
}

func init() {
	RootCmd.AddCommand(mcpCmd)
}
