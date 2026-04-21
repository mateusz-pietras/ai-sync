# Canonical AI Source

This directory is the source of truth for `ai-sync`.

Expected layout:

- `rules/`
- `skills/`
- `agents/`
- `commands/`
- `prompts/`
- `mcp/mcp.json`
- `templates/copilot/copilot-instructions.md`
- `templates/claude/CLAUDE.md`
- `templates/gitignore/.gitignore`
- `templates/mcp-tools/<tool>.sh`

Put your own project-specific content here.

MCP notes:

- `mcp/mcp.json` is the single base source.
- `ai sync` generates provider-specific files (`.cursor/mcp.json`, `.vscode/mcp.json`, `.mcp.json`).
- For reusable launcher scripts, set server `command` to `ai-sync:launcher:<tool>`.
- Provide matching template at `.ai/templates/mcp-tools/<tool>.sh`.
