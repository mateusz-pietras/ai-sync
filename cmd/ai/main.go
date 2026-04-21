package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	beginGeneratedRules   = "<!-- AI_SYNC:BEGIN_GENERATED_RULES -->"
	endGeneratedRules     = "<!-- AI_SYNC:END_GENERATED_RULES -->"
	launcherCommandPrefix = "ai-sync:launcher:"
)

type syncOptions struct {
	source string
	target string
	ide    string
	force  bool
	dryRun bool
}

type counters struct {
	created int
	updated int
	skipped int
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "sync":
		opts, err := parseSyncFlags(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if err := runSync(opts); err != nil {
			fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("ai-sync")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  ai sync --source <repo> --target <repo> --ide cursor|copilot|claude|all [--force] [--dry-run]")
	fmt.Println("  ai sync -s <repo> -t <repo> -i cursor|copilot|claude|all [-f] [-d]")
}

func parseSyncFlags(args []string) (syncOptions, error) {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	opts := syncOptions{}
	flags.StringVar(&opts.source, "source", ".", "Path to source repo containing .ai/")
	flags.StringVar(&opts.source, "s", ".", "Shorthand for --source")
	flags.StringVar(&opts.target, "target", ".", "Path to destination repo")
	flags.StringVar(&opts.target, "t", ".", "Shorthand for --target")
	flags.StringVar(&opts.ide, "ide", "all", "Target setup to generate: cursor|copilot|claude|all")
	flags.StringVar(&opts.ide, "i", "all", "Shorthand for --ide")
	flags.BoolVar(&opts.force, "force", false, "Overwrite managed files")
	flags.BoolVar(&opts.force, "f", false, "Shorthand for --force")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Preview changes only")
	flags.BoolVar(&opts.dryRun, "d", false, "Shorthand for --dry-run")
	if err := flags.Parse(args); err != nil {
		return opts, err
	}

	opts.ide = strings.ToLower(strings.TrimSpace(opts.ide))
	if opts.ide != "cursor" && opts.ide != "copilot" && opts.ide != "claude" && opts.ide != "all" {
		return opts, fmt.Errorf("invalid --ide value %q", opts.ide)
	}

	var err error
	opts.source, err = filepath.Abs(opts.source)
	if err != nil {
		return opts, err
	}
	opts.target, err = filepath.Abs(opts.target)
	if err != nil {
		return opts, err
	}
	return opts, nil
}

func runSync(opts syncOptions) error {
	if !pathExists(opts.source) {
		return fmt.Errorf("source path not found: %s", opts.source)
	}
	if !pathExists(opts.target) {
		return fmt.Errorf("target path not found: %s", opts.target)
	}

	sourceAI := filepath.Join(opts.source, ".ai")
	if !pathExists(sourceAI) {
		return fmt.Errorf("missing canonical source directory: %s", sourceAI)
	}

	count := &counters{}
	if opts.ide == "cursor" || opts.ide == "all" {
		if err := syncCursor(sourceAI, opts.target, opts.force, opts.dryRun, count); err != nil {
			return err
		}
	}
	if opts.ide == "copilot" || opts.ide == "all" {
		if err := syncCopilot(sourceAI, opts.target, opts.force, opts.dryRun, count); err != nil {
			return err
		}
	}
	if opts.ide == "claude" || opts.ide == "all" {
		if err := syncClaude(sourceAI, opts.target, opts.force, opts.dryRun, count); err != nil {
			return err
		}
	}
	if err := syncGitignore(sourceAI, opts.target, opts.force, opts.dryRun, count); err != nil {
		return err
	}

	fmt.Printf("sync summary: created=%d updated=%d skipped=%d\n", count.created, count.updated, count.skipped)
	return nil
}

func syncCursor(sourceAI, target string, force, dryRun bool, count *counters) error {
	mappings := [][2]string{
		{filepath.Join(sourceAI, "rules"), filepath.Join(target, ".cursor", "rules")},
		{filepath.Join(sourceAI, "skills"), filepath.Join(target, ".cursor", "skills")},
		{filepath.Join(sourceAI, "agents"), filepath.Join(target, ".cursor", "agents")},
		{filepath.Join(sourceAI, "commands"), filepath.Join(target, ".cursor", "commands")},
	}
	for _, mapping := range mappings {
		if pathExists(mapping[0]) {
			if err := copyTree(mapping[0], mapping[1], force, dryRun, count); err != nil {
				return err
			}
		}
	}

	mcpPath := filepath.Join(sourceAI, "mcp", "mcp.json")
	if pathExists(mcpPath) {
		baseContent, err := os.ReadFile(mcpPath)
		if err != nil {
			return err
		}
		cursorContent, launcherTools, err := buildProviderMCP(baseContent, "cursor")
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(target, ".cursor", "mcp.json"), cursorContent, force, dryRun, count); err != nil {
			return err
		}
		if err := writeLauncherTools(sourceAI, target, "cursor", launcherTools, force, dryRun, count); err != nil {
			return err
		}
	}
	return nil
}

func syncCopilot(sourceAI, target string, force, dryRun bool, count *counters) error {
	mappings := [][2]string{
		{filepath.Join(sourceAI, "agents"), filepath.Join(target, ".github", "agents")},
		{filepath.Join(sourceAI, "skills"), filepath.Join(target, ".agents", "skills")},
		{filepath.Join(sourceAI, "rules"), filepath.Join(target, ".github", "ai", "rules")},
		{filepath.Join(sourceAI, "commands"), filepath.Join(target, ".github", "ai", "commands")},
		{filepath.Join(sourceAI, "prompts"), filepath.Join(target, ".github", "ai", "prompts")},
	}
	for _, mapping := range mappings {
		if pathExists(mapping[0]) {
			if err := copyTree(mapping[0], mapping[1], force, dryRun, count); err != nil {
				return err
			}
		}
	}

	mcpPath := filepath.Join(sourceAI, "mcp", "mcp.json")
	if pathExists(mcpPath) {
		baseContent, err := os.ReadFile(mcpPath)
		if err != nil {
			return err
		}
		copilotContent, copilotLaunchers, err := buildProviderMCP(baseContent, "copilot")
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(target, ".vscode", "mcp.json"), copilotContent, force, dryRun, count); err != nil {
			return err
		}

		if err := writeLauncherTools(sourceAI, target, "copilot", copilotLaunchers, force, dryRun, count); err != nil {
			return err
		}
	}

	return generateCopilotInstructions(sourceAI, target, force, dryRun, count)
}

func syncClaude(sourceAI, target string, force, dryRun bool, count *counters) error {
	mappings := [][2]string{
		{filepath.Join(sourceAI, "agents"), filepath.Join(target, ".claude", "agents")},
		{filepath.Join(sourceAI, "skills"), filepath.Join(target, ".claude", "ai", "skills")},
		{filepath.Join(sourceAI, "rules"), filepath.Join(target, ".claude", "ai", "rules")},
		{filepath.Join(sourceAI, "commands"), filepath.Join(target, ".claude", "commands")},
		{filepath.Join(sourceAI, "prompts"), filepath.Join(target, ".claude", "ai", "prompts")},
	}
	for _, mapping := range mappings {
		if pathExists(mapping[0]) {
			if err := copyTree(mapping[0], mapping[1], force, dryRun, count); err != nil {
				return err
			}
		}
	}

	mcpPath := filepath.Join(sourceAI, "mcp", "mcp.json")
	if pathExists(mcpPath) {
		baseContent, err := os.ReadFile(mcpPath)
		if err != nil {
			return err
		}
		claudeContent, launcherTools, err := buildProviderMCP(baseContent, "claude")
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(target, ".mcp.json"), claudeContent, force, dryRun, count); err != nil {
			return err
		}
		if err := writeLauncherTools(sourceAI, target, "claude", launcherTools, force, dryRun, count); err != nil {
			return err
		}
	}

	return generateClaudeInstructions(sourceAI, target, force, dryRun, count)
}

func generateCopilotInstructions(sourceAI, target string, force, dryRun bool, count *counters) error {
	templatePath := filepath.Join(sourceAI, "templates", "copilot", "copilot-instructions.md")
	if !pathExists(templatePath) {
		return nil
	}
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	generated, err := buildGeneratedDictionary(sourceAI)
	if err != nil {
		return err
	}

	output := injectGeneratedBlock(string(templateContent), generated)
	return writeFile(filepath.Join(target, ".github", "copilot-instructions.md"), []byte(output), force, dryRun, count)
}

func generateClaudeInstructions(sourceAI, target string, force, dryRun bool, count *counters) error {
	templatePath := filepath.Join(sourceAI, "templates", "claude", "CLAUDE.md")
	if !pathExists(templatePath) {
		return nil
	}
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	generated, err := buildGeneratedDictionary(sourceAI)
	if err != nil {
		return err
	}

	output := injectGeneratedBlock(string(templateContent), generated)
	return writeFile(filepath.Join(target, "CLAUDE.md"), []byte(output), force, dryRun, count)
}

func buildGeneratedDictionary(sourceAI string) (string, error) {
	lines := []string{
		"_Generated by ai-sync. Update canonical content in `.ai/*` and re-run `ai sync`._",
		"",
		"### Rules",
	}

	rules, err := listFiles(filepath.Join(sourceAI, "rules"), ".mdc")
	if err != nil {
		return "", err
	}
	if len(rules) == 0 {
		lines = append(lines, "- No rules found in `.ai/rules`.")
	} else {
		for _, rule := range rules {
			rel, _ := filepath.Rel(sourceAI, rule)
			lines = append(lines, fmt.Sprintf("- `%s` — %s", filepath.ToSlash(rel), extractRuleDescription(rule)))
		}
	}

	lines = append(lines, "", "### Skills")
	if err := appendIndex(&lines, sourceAI, "skills", "SKILL.md"); err != nil {
		return "", err
	}

	lines = append(lines, "", "### Agents")
	if err := appendIndex(&lines, sourceAI, "agents", ".agent.md"); err != nil {
		return "", err
	}

	lines = append(lines, "", "### Commands")
	if err := appendIndex(&lines, sourceAI, "commands", ".md"); err != nil {
		return "", err
	}

	lines = append(lines, "", "### MCP")
	mcpPath := filepath.Join(sourceAI, "mcp", "mcp.json")
	if pathExists(mcpPath) {
		lines = append(lines, "- Cursor MCP file: `.cursor/mcp.json` (`mcpServers`)")
		lines = append(lines, "- Copilot MCP file: `.vscode/mcp.json` (`servers`)")
		lines = append(lines, "- Claude Code MCP file: `.mcp.json` (`mcpServers`)")
		lines = append(lines, "- Tool launcher scripts are generated when server command uses `ai-sync:launcher:<tool>` and template exists in `.ai/templates/mcp-tools/<tool>.sh`.")
		names, err := listMCPServerNames(mcpPath)
		if err == nil && len(names) > 0 {
			lines = append(lines, "- Configured servers:")
			for _, name := range names {
				lines = append(lines, fmt.Sprintf("  - `%s`", name))
			}
		}
	} else {
		lines = append(lines, "- No MCP template found in `.ai/mcp/mcp.json`.")
	}

	return strings.Join(lines, "\n"), nil
}

func appendIndex(lines *[]string, sourceAI, section, extension string) error {
	files, err := listFiles(filepath.Join(sourceAI, section), extension)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		*lines = append(*lines, fmt.Sprintf("- No entries found in `.ai/%s`.", section))
		return nil
	}
	for _, file := range files {
		rel, _ := filepath.Rel(sourceAI, file)
		label := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		*lines = append(*lines, fmt.Sprintf("- `%s` — %s", filepath.ToSlash(rel), label))
	}
	return nil
}

func injectGeneratedBlock(template, generated string) string {
	start := strings.Index(template, beginGeneratedRules)
	end := strings.Index(template, endGeneratedRules)
	if start == -1 || end == -1 || end < start {
		return template + "\n\n## Generated Rules Dictionary\n" + beginGeneratedRules + "\n" + generated + "\n" + endGeneratedRules + "\n"
	}
	before := template[:start+len(beginGeneratedRules)]
	after := template[end:]
	return before + "\n" + generated + "\n" + after
}

func syncGitignore(sourceAI, target string, force, dryRun bool, count *counters) error {
	templatePath := filepath.Join(sourceAI, "templates", "gitignore", ".gitignore")
	if !pathExists(templatePath) {
		return nil
	}
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	destPath := filepath.Join(target, ".gitignore")
	existing := []byte{}
	if pathExists(destPath) {
		existing, err = os.ReadFile(destPath)
		if err != nil {
			return err
		}
	}

	merged := mergeGitignore(string(existing), string(content))
	return writeFile(destPath, []byte(merged), force, dryRun, count)
}

func mergeGitignore(existing, additions string) string {
	existingLines := strings.Split(strings.ReplaceAll(existing, "\r\n", "\n"), "\n")
	addLines := strings.Split(strings.ReplaceAll(additions, "\r\n", "\n"), "\n")

	seen := map[string]struct{}{}
	for _, line := range existingLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			seen[trimmed] = struct{}{}
		}
	}

	var missing []string
	for _, line := range addLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		missing = append(missing, line)
	}

	if len(missing) == 0 {
		if existing == "" {
			return ""
		}
		if strings.HasSuffix(existing, "\n") {
			return existing
		}
		return existing + "\n"
	}

	out := strings.TrimRight(existing, "\n")
	if out != "" {
		out += "\n\n"
	}
	out += "# Added by ai-sync\n"
	out += strings.Join(missing, "\n")
	out += "\n"
	return out
}

func copyTree(src, dst string, force, dryRun bool, count *counters) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)

		if d.IsDir() {
			if dryRun {
				return nil
			}
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				if count != nil {
					count.skipped++
				}
				return nil
			}
			return nil
		}
		if shouldSkipFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFile(targetPath, content, force, dryRun, count)
	})
}

func shouldSkipFile(path string) bool {
	name := filepath.Base(path)
	if name == ".gitkeep" || name == ".DS_Store" {
		return true
	}
	return strings.HasPrefix(name, "._")
}

func writeFile(path string, content []byte, force, dryRun bool, count *counters) error {
	exists := pathExists(path)
	if exists {
		info, err := os.Stat(path)
		if err != nil {
			if count != nil {
				count.skipped++
			}
			return nil
		}
		if info.IsDir() {
			if count != nil {
				count.skipped++
			}
			return nil
		}

		existing, err := os.ReadFile(path)
		if err != nil {
			if count != nil {
				count.skipped++
			}
			return nil
		}
		if bytes.Equal(existing, content) {
			if count != nil {
				count.skipped++
			}
			return nil
		}
		if !force {
			if count != nil {
				count.skipped++
			}
			return nil
		}
	}

	if dryRun {
		if count != nil {
			if exists {
				count.updated++
			} else {
				count.created++
			}
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		if count != nil {
			count.skipped++
		}
		return nil
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		if count != nil {
			count.skipped++
		}
		return nil
	}
	if count != nil {
		if exists {
			count.updated++
		} else {
			count.created++
		}
	}
	return nil
}

func listFiles(root, extension string) ([]string, error) {
	if !pathExists(root) {
		return nil, nil
	}
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || shouldSkipFile(path) {
			return nil
		}
		name := filepath.Base(path)
		switch extension {
		case "SKILL.md":
			if strings.EqualFold(name, "SKILL.md") {
				out = append(out, path)
			}
		case ".agent.md":
			if strings.HasSuffix(name, ".agent.md") {
				out = append(out, path)
			}
		default:
			if strings.HasSuffix(strings.ToLower(name), strings.ToLower(extension)) {
				out = append(out, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func extractRuleDescription(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return "No description available."
	}
	lines := strings.Split(string(content), "\n")

	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "---" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "description:") {
				raw := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				raw = strings.Trim(raw, "\"")
				if raw != "" {
					return raw
				}
			}
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title
			}
		}
	}
	return "No description available."
}

func buildProviderMCP(baseRaw []byte, provider string) ([]byte, []string, error) {
	var doc map[string]any
	if err := json.Unmarshal(baseRaw, &doc); err != nil {
		return nil, nil, err
	}

	baseServers := map[string]any{}
	if raw, ok := doc["mcpServers"]; ok {
		if cast, ok := raw.(map[string]any); ok {
			baseServers = cast
		}
	} else if raw, ok := doc["servers"]; ok {
		if cast, ok := raw.(map[string]any); ok {
			baseServers = cast
		}
	}

	outServers := map[string]any{}
	launcherTools := make([]string, 0)
	launcherSeen := map[string]struct{}{}
	for name, raw := range baseServers {
		server, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		serverCopy := cloneMap(server)
		if toolName, ok := parseLauncherToolName(serverCopy); ok {
			serverCopy["command"] = toolLauncherCommand(provider, toolName)
			if _, exists := launcherSeen[toolName]; !exists {
				launcherSeen[toolName] = struct{}{}
				launcherTools = append(launcherTools, toolName)
			}
		}
		if provider == "copilot" {
			normalizeServerForCopilot(serverCopy)
		}
		outServers[name] = serverCopy
	}

	delete(doc, "mcpServers")
	delete(doc, "servers")
	if provider == "copilot" {
		doc["servers"] = outServers
	} else {
		doc["mcpServers"] = outServers
	}

	formatted, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(launcherTools)
	return append(formatted, '\n'), launcherTools, nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func parseLauncherToolName(server map[string]any) (string, bool) {
	command, _ := server["command"].(string)
	command = strings.TrimSpace(command)
	if !strings.HasPrefix(command, launcherCommandPrefix) {
		return "", false
	}

	rawTool := strings.TrimSpace(strings.TrimPrefix(command, launcherCommandPrefix))
	if rawTool == "" {
		return "", false
	}
	return sanitizeToolName(rawTool), true
}

func normalizeServerForCopilot(server map[string]any) {
	headers, hasHeaders := server["headers"]
	if !hasHeaders {
		return
	}

	requestInit, ok := server["requestInit"].(map[string]any)
	if !ok {
		requestInit = map[string]any{}
	}
	if _, ok := requestInit["headers"]; !ok {
		requestInit["headers"] = headers
	}
	server["requestInit"] = requestInit
	delete(server, "headers")
}

func toolLauncherCommand(provider, toolName string) string {
	switch provider {
	case "cursor":
		return fmt.Sprintf("./.cursor/run-%s", toolName)
	case "copilot":
		return fmt.Sprintf("./.vscode/run-%s", toolName)
	case "claude":
		return fmt.Sprintf("./.claude/run-%s", toolName)
	default:
		return fmt.Sprintf("./run-%s", toolName)
	}
}

func toolLauncherOutputPath(target, provider, toolName string) string {
	switch provider {
	case "cursor":
		return filepath.Join(target, ".cursor", fmt.Sprintf("run-%s", toolName))
	case "copilot":
		return filepath.Join(target, ".vscode", fmt.Sprintf("run-%s", toolName))
	case "claude":
		return filepath.Join(target, ".claude", fmt.Sprintf("run-%s", toolName))
	default:
		return filepath.Join(target, fmt.Sprintf("run-%s", toolName))
	}
}

func writeLauncherTools(sourceAI, target, provider string, toolNames []string, force, dryRun bool, count *counters) error {
	for _, toolName := range toolNames {
		templatePath := filepath.Join(sourceAI, "templates", "mcp-tools", toolName+".sh")
		if !pathExists(templatePath) {
			if count != nil {
				count.skipped++
			}
			continue
		}

		scriptContent, err := os.ReadFile(templatePath)
		if err != nil {
			if count != nil {
				count.skipped++
			}
			continue
		}

		outputPath := toolLauncherOutputPath(target, provider, toolName)
		if err := writeFile(outputPath, scriptContent, force, dryRun, count); err != nil {
			return err
		}
		if dryRun {
			continue
		}
		if err := os.Chmod(outputPath, 0o755); err != nil {
			if count != nil {
				count.skipped++
			}
		}
	}
	return nil
}

func sanitizeToolName(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "tool"
	}

	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	clean := strings.Trim(b.String(), "-_")
	if clean == "" {
		return "tool"
	}
	return clean
}

func listMCPServerNames(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	var servers any
	if v, ok := data["servers"]; ok {
		servers = v
	} else if v, ok := data["mcpServers"]; ok {
		servers = v
	}
	serverMap, ok := servers.(map[string]any)
	if !ok {
		return nil, nil
	}
	names := make([]string, 0, len(serverMap))
	for name := range serverMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}
