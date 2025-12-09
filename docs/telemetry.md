# Telemetry

gob collects anonymous usage telemetry to help understand how the tool is used and prioritize improvements. All data is anonymized using a hashed machine identifier.

## What's Collected

Every event includes basic context:
- gob version
- Operating system and architecture
- Terminal type and shell name

### Events

| Event | When | Additional Data |
|-------|------|-----------------|
| `cli:command_run` | CLI command executes | Command name |
| `mcp:tool_call` | MCP tool is invoked | Tool name |
| `tui:session_start` | TUI opens | - |
| `tui:session_end` | TUI exits | Session duration |
| `tui:action_execute` | TUI action performed | Action name |

## What's NOT Collected

- Command arguments or output
- File paths or project names
- Personal information
- IP addresses

## Opting Out

Set either environment variable:

```bash
export GOB_TELEMETRY_DISABLED=1
```

Or use the standard `DO_NOT_TRACK` convention (note that this may affect other software):

```bash
export DO_NOT_TRACK=1
```
