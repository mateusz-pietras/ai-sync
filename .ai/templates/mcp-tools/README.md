# MCP Tool Launcher Templates

Define reusable launcher scripts for MCP servers that use:

`"command": "ai-sync:launcher:<tool>"`

For each `<tool>`, create:

- `.ai/templates/mcp-tools/<tool>.sh`

During sync, the CLI will generate provider-specific launchers:

- Cursor: `.cursor/run-<tool>`
- Copilot: `.vscode/run-<tool>`
- Claude Code: `.claude/run-<tool>`

Template files are copied as-is and marked executable.
