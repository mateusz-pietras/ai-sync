# ai-sync

Minimal Go CLI to sync a canonical `.ai` source into IDE-specific outputs for Cursor, GitHub Copilot, and Claude Code.

## Purpose

- Keep one source of truth (`.ai/`).
- Generate hard-copy IDE config directories (no symlinks).
- Auto-build a generated rules dictionary section in `copilot-instructions.md`.

## Canonical source schema

```text
.ai/
  rules/
  skills/
  agents/
  commands/
  prompts/
  mcp/mcp.json
  templates/
    copilot/copilot-instructions.md
    claude/CLAUDE.md
    gitignore/.gitignore
```

## Package structure

```text
ai-sync/
  cmd/ai/main.go
  go.mod
  .gitignore
  .ai/
    README.md
    rules/
    skills/
    agents/
    commands/
    prompts/
    mcp/mcp.json
    templates/
      copilot/copilot-instructions.md
      claude/CLAUDE.md
      gitignore/.gitignore
```

## Output mapping

### Cursor

- `.ai/rules` -> `.cursor/rules`
- `.ai/skills` -> `.cursor/skills`
- `.ai/agents` -> `.cursor/agents`
- `.ai/commands` -> `.cursor/commands`
- `.ai/mcp/mcp.json` -> `.cursor/mcp.json`
- tool launchers generated from `.ai/templates/mcp-tools/*.sh` when command is `ai-sync:launcher:<tool>`

### Copilot

- `.ai/agents` -> `.github/agents`
- `.ai/skills` -> `.agents/skills`
- `.ai/rules` -> `.github/ai/rules`
- `.ai/commands` -> `.github/ai/commands`
- `.ai/prompts` -> `.github/ai/prompts`
- `.ai/mcp/mcp.json` -> `.vscode/mcp.json` (normalized to `servers`)
- tool launchers generated from `.ai/templates/mcp-tools/*.sh` when command is `ai-sync:launcher:<tool>`
- generated output -> `.github/copilot-instructions.md`

### Claude Code

- `.ai/agents` -> `.claude/agents`
- `.ai/skills` -> `.claude/ai/skills`
- `.ai/rules` -> `.claude/ai/rules`
- `.ai/commands` -> `.claude/commands`
- `.ai/prompts` -> `.claude/ai/prompts`
- `.ai/mcp/mcp.json` -> `.mcp.json` (project root, `mcpServers`)
- tool launchers generated from `.ai/templates/mcp-tools/*.sh` when command is `ai-sync:launcher:<tool>`
- generated output -> `CLAUDE.md`

### Shared

- `.ai/templates/gitignore/.gitignore` entries are merged into target `.gitignore`.

## Copilot template markers

Use marker comments in `.ai/templates/copilot/copilot-instructions.md`:

```md
<!-- AI_SYNC:BEGIN_GENERATED_RULES -->
<!-- AI_SYNC:END_GENERATED_RULES -->
```

Only the content between markers is regenerated.
The same markers are used in both Copilot and Claude templates.

## MCP launcher catalog (generic)

To use reusable MCP launcher scripts:

1. In `.ai/mcp/mcp.json`, set server command to:
   - `ai-sync:launcher:<tool>`
2. Add template script:
   - `.ai/templates/mcp-tools/<tool>.sh`

`ai sync` generates executable provider-specific launchers:
- `.cursor/run-<tool>`
- `.vscode/run-<tool>`
- `.claude/run-<tool>`

## CLI usage

```bash
ai sync --source <repo-with-ai> --target <destination-repo> --ide cursor|copilot|claude|all [--force] [--dry-run]
```

Default behavior is safe sync: existing conflicting files are skipped.  
Pass `-f` / `--force` to overwrite conflicting files.

## Build/install

```bash
go build -o ai ./cmd/ai
go install ./cmd/ai
```

## Notes

- This folder intentionally contains **no project-specific rules, prompts, or private instructions**.
- This folder intentionally contains **no project-specific MCP servers or local credentials**.
- It contains only generic sync logic and public schema documentation.
