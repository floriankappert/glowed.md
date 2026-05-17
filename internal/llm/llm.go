package llm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	Command         string
	TerminalCommand string
	TerminalApp     string
	OpenMode        string
}

type Message struct {
	Role    string
	Content string
}

type Request struct {
	Messages []Message
	Context  FileContext
}

type FileContext struct {
	Root             string
	AbsPath          string
	RelPath          string
	Mode             string
	SelectedMarkdown string
	RawMarkdown      string
	Truncated        bool
}

type Provider interface {
	Send(ctx context.Context, req Request) (Message, error)
}

type LaunchResult struct {
	ContextFile string
	ScriptFile  string
}

func NewProvider(cfg Config) Provider {
	kind := commandKind(cfg.Command)
	switch kind {
	case "", "mock":
		return MockProvider{}
	case "claude", "codex":
		return CLIProvider{Kind: kind, Command: cfg.Command}
	default:
		return MockProvider{Name: cfg.Command}
	}
}

type MockProvider struct {
	Name string
}

func (p MockProvider) Send(_ context.Context, req Request) (Message, error) {
	name := p.Name
	if name == "" {
		name = "mock"
	}
	last := lastUserMessage(req.Messages)
	if last == "" {
		return Message{}, fmt.Errorf("empty user message")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[%s] 현재 파일 경로를 인지했습니다.\n", name)
	if req.Context.AbsPath != "" {
		fmt.Fprintf(&b, "- abs: %s\n", req.Context.AbsPath)
	}
	if req.Context.RelPath != "" {
		fmt.Fprintf(&b, "- rel: %s\n", req.Context.RelPath)
	}
	if req.Context.Mode != "" {
		fmt.Fprintf(&b, "- mode: %s\n", req.Context.Mode)
	}
	if req.Context.SelectedMarkdown != "" {
		fmt.Fprintf(&b, "- selected markdown: %d bytes\n", len([]byte(req.Context.SelectedMarkdown)))
	}
	if req.Context.RawMarkdown != "" {
		fmt.Fprintf(&b, "- current file context: %d bytes", len([]byte(req.Context.RawMarkdown)))
		if req.Context.Truncated {
			b.WriteString(" (truncated)")
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\n질문: %s", last)
	return Message{Role: "assistant", Content: b.String()}, nil
}

type CLIProvider struct {
	Kind    string
	Command string
}

func (p CLIProvider) Send(ctx context.Context, req Request) (Message, error) {
	prompt := BuildPrompt(req)
	if strings.TrimSpace(prompt) == "" {
		return Message{}, fmt.Errorf("empty prompt")
	}

	cmd, stdin := p.command(ctx, prompt)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Message{}, fmt.Errorf("%s cli failed: %s", p.Kind, msg)
	}
	content := strings.TrimSpace(string(out))
	if content == "" {
		return Message{}, fmt.Errorf("%s cli returned empty response", p.Kind)
	}
	return Message{Role: "assistant", Content: content}, nil
}

func (p CLIProvider) command(ctx context.Context, prompt string) (*exec.Cmd, string) {
	command := p.Command
	if command == "" {
		command = p.Kind
	}
	switch p.Kind {
	case "claude":
		return exec.CommandContext(ctx, command,
			"--print",
			"--output-format", "text",
			"--no-session-persistence",
			"--tools", "",
			prompt,
		), ""
	case "codex":
		args := []string{
			"exec",
			"--skip-git-repo-check",
			"--sandbox", "read-only",
			"--ask-for-approval", "never",
			"--color", "never",
		}
		if strings.TrimSpace(extractRootFromPrompt(prompt)) != "" {
			args = append(args, "--cd", extractRootFromPrompt(prompt))
		}
		args = append(args, "-")
		return exec.CommandContext(ctx, command, args...), prompt
	default:
		return exec.CommandContext(ctx, command, prompt), ""
	}
}

func LaunchExternal(ctx context.Context, cfg Config, req Request) (LaunchResult, error) {
	command := cfg.Command
	if command == "" {
		return LaunchResult{}, fmt.Errorf("llm.command is required, e.g. claude, codex, aider, or another CLI command")
	}

	dir, err := os.MkdirTemp("", "glowed-llm-*")
	if err != nil {
		return LaunchResult{}, err
	}
	command = resolveCommand(command)
	contextFile := filepath.Join(dir, "context.md")
	scriptFile := filepath.Join(dir, "launch.sh")
	if err := os.WriteFile(contextFile, []byte(BuildPrompt(req)), 0600); err != nil {
		return LaunchResult{}, err
	}
	if err := os.WriteFile(scriptFile, []byte(launchScript(command, req.Context.Root, contextFile)), 0700); err != nil {
		return LaunchResult{}, err
	}

	if err := launchTerminal(ctx, cfg, req.Context.Root, scriptFile); err != nil {
		return LaunchResult{}, err
	}
	return LaunchResult{ContextFile: contextFile, ScriptFile: scriptFile}, nil
}

func launchTerminal(ctx context.Context, cfg Config, root, script string) error {
	if root == "" {
		root = "."
	}
	terminal := cfg.TerminalCommand
	if terminal == "" {
		terminal = "ghostty"
	}
	openMode := normalizeOpenMode(cfg.OpenMode)
	if openMode == "" {
		openMode = "split-right"
	}
	if runtime.GOOS == "darwin" && isGhosttyCommand(terminal) && strings.HasPrefix(openMode, "split") {
		cmd := ghosttySplitCommand(ctx, cfg, root, script, openMode)
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("ghostty split failed: %s", msg)
		}
		return nil
	}
	cmd := terminalCommand(ctx, cfg, root, script)
	return cmd.Start()
}

func terminalCommand(ctx context.Context, cfg Config, root, script string) *exec.Cmd {
	terminal := cfg.TerminalCommand
	if terminal == "" {
		terminal = "ghostty"
	}
	if root == "" {
		root = "."
	}
	if runtime.GOOS == "darwin" && isGhosttyCommand(terminal) {
		app := cfg.TerminalApp
		if app == "" {
			app = "Ghostty"
		}
		return exec.CommandContext(ctx, "open", "-na", app, "--args", "--working-directory="+root, "-e", "/bin/sh", script)
	}
	return exec.CommandContext(ctx, terminal, "--working-directory="+root, "-e", "/bin/sh", script)
}

func ghosttySplitCommand(ctx context.Context, cfg Config, root, script, openMode string) *exec.Cmd {
	app := cfg.TerminalApp
	if app == "" {
		app = "Ghostty"
	}
	direction := strings.TrimPrefix(openMode, "split-")
	if direction == "" || direction == "split" {
		direction = "right"
	}
	command := "/bin/sh " + shellQuote(script)
	lines := []string{
		"tell application " + appleScriptQuote(app),
		"activate",
		"set cfg to new surface configuration from {initial working directory:" + appleScriptQuote(root) + ", command:" + appleScriptQuote(command) + ", wait after command:true}",
		"split (focused terminal of selected tab of front window) direction " + direction + " with configuration cfg",
		"end tell",
	}
	args := []string{}
	for _, line := range lines {
		args = append(args, "-e", line)
	}
	return exec.CommandContext(ctx, "osascript", args...)
}

func isGhosttyCommand(command string) bool {
	base := strings.ToLower(filepath.Base(command))
	return base == "ghostty" || base == "ghostty.app"
}

func normalizeOpenMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	mode = strings.ReplaceAll(mode, "_", "-")
	return mode
}

func launchScript(command, root, contextFile string) string {
	if root == "" {
		root = "."
	}
	kind := commandKind(command)
	var b strings.Builder
	b.WriteString("#!/bin/sh\nset -eu\n")
	fmt.Fprintf(&b, "ROOT=%s\n", shellQuote(root))
	fmt.Fprintf(&b, "CTX=%s\n", shellQuote(contextFile))
	fmt.Fprintf(&b, "COMMAND=%s\n", shellQuote(command))
	fmt.Fprintf(&b, "KIND=%s\n", shellQuote(kind))
	writeEnvExports(&b)
	b.WriteString("cd \"$ROOT\"\n")
	b.WriteString("printf '\\033[33m%s\\033[0m\\n' 'glowed: context was copied to your clipboard.'\n")
	b.WriteString("printf '\\033[33m%s\\033[0m\\n' 'Paste it into this LLM session, add your question, then press Enter.'\n")
	b.WriteString("printf '\\033[33m%s\\033[0m\\n' \"context file: $CTX\"\n")
	b.WriteString("case \"$KIND\" in\n")
	b.WriteString("  claude) exec $COMMAND ;;\n")
	b.WriteString("  codex) exec $COMMAND --cd \"$ROOT\" ;;\n")
	b.WriteString("  *) exec $COMMAND ;;\n")
	b.WriteString("esac\n")
	return b.String()
}

func commandKind(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	base := strings.ToLower(filepath.Base(strings.Fields(command)[0]))
	base = strings.TrimSuffix(base, ".exe")
	switch {
	case strings.Contains(base, "claude"):
		return "claude"
	case strings.Contains(base, "codex"):
		return "codex"
	default:
		return base
	}
}

func resolveCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return command
	}
	if path, err := exec.LookPath(fields[0]); err == nil {
		fields[0] = path
	}
	return strings.Join(fields, " ")
}

func writeEnvExports(b *strings.Builder) {
	keys := []string{
		"PATH",
		"HOME",
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
		"FNM_DIR",
		"FNM_MULTISHELL_PATH",
		"NODE_PATH",
		"NVM_DIR",
		"VOLTA_HOME",
		"PNPM_HOME",
		"BUN_INSTALL",
		"CODEX_HOME",
	}
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			fmt.Fprintf(b, "export %s=%s\n", key, shellQuote(value))
		}
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func appleScriptQuote(s string) string {
	return "\"" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\"", "\\\"") + "\""
}

func BuildPrompt(req Request) string {
	var b strings.Builder
	b.WriteString("You are opened from glowed, a terminal Markdown browser/editor.\n")
	b.WriteString("Initialize this session with the current Markdown file path context below.\n")
	b.WriteString("Use the current file path as the primary context for follow-up requests.\n\n")
	b.WriteString("<glowed_context>\n")
	if req.Context.Root != "" {
		fmt.Fprintf(&b, "root: %s\n", req.Context.Root)
	}
	if req.Context.AbsPath != "" {
		fmt.Fprintf(&b, "current_abs_path: %s\n", req.Context.AbsPath)
	}
	if req.Context.RelPath != "" {
		fmt.Fprintf(&b, "current_rel_path: %s\n", req.Context.RelPath)
	}
	if req.Context.Mode != "" {
		fmt.Fprintf(&b, "mode: %s\n", req.Context.Mode)
	}
	if req.Context.Truncated {
		b.WriteString("raw_markdown_truncated: true\n")
	}
	b.WriteString("</glowed_context>\n")
	if req.Context.SelectedMarkdown != "" {
		b.WriteString("\n<selected_original_markdown>\n")
		b.WriteString(req.Context.SelectedMarkdown)
		b.WriteString("\n</selected_original_markdown>\n")
	}
	if req.Context.RawMarkdown != "" {
		b.WriteString("\n<current_file_raw_markdown>\n")
		b.WriteString(req.Context.RawMarkdown)
		b.WriteString("\n</current_file_raw_markdown>\n")
	}
	return b.String()
}

func lastUserMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func extractRootFromPrompt(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		if strings.HasPrefix(line, "root: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "root: "))
		}
	}
	return ""
}
