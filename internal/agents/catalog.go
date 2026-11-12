// Package agents knows the coding agents (Claude, Codex, …) a folder can
// be opened with: which are installed, and the command that starts each.
//
// The catalog is taken from Orca (MIT, https://github.com/stablyai/orca):
// src/shared/agent-launch/catalog/tui-agent-config.ts (binary names,
// aliases, required commands, launch commands) and
// src/shared/agent-launch/permissions/tui-agent-permissions.ts (the YOLO
// arguments). Only the CLI launch is kept, no chat mode or permission tiers.
package agents

// Entry is one agent.
type Entry struct {
	ID, Name string
	// Detect is the binary's name, then its aliases; any one found counts.
	Detect []string
	// Requires are commands that must also be installed.
	Requires []string
	// Launch is the base command, Yolo the arguments that skip every
	// permission prompt, "" for none.
	Launch, Yolo string
}

// Catalog lists the agents in Orca's order.
var Catalog = []Entry{
	{ID: "claude", Name: "Claude", Detect: []string{"claude"}, Launch: "claude", Yolo: "--dangerously-skip-permissions"},
	{ID: "claude-agent-teams", Name: "Claude Agent Teams", Detect: []string{"orca", "orca-dev", "orca-ide"}, Requires: []string{"claude"}, Launch: "orca claude-teams", Yolo: "--dangerously-skip-permissions"},
	{ID: "codebuddy", Name: "CodeBuddy", Detect: []string{"codebuddy", "cbc"}, Launch: "codebuddy", Yolo: "--dangerously-skip-permissions"},
	{ID: "openclaude", Name: "OpenClaude", Detect: []string{"openclaude"}, Launch: "openclaude", Yolo: "--dangerously-skip-permissions"},
	{ID: "codex", Name: "Codex", Detect: []string{"codex"}, Launch: "codex", Yolo: "--dangerously-bypass-approvals-and-sandbox"},
	{ID: "autohand", Name: "Autohand", Detect: []string{"autohand"}, Launch: "autohand", Yolo: "--unrestricted"},
	{ID: "ante", Name: "Ante", Detect: []string{"ante"}, Launch: "ante", Yolo: "--yolo"},
	{ID: "trae", Name: "Trae", Detect: []string{"traecli"}, Launch: "traecli", Yolo: "--yolo"},
	{ID: "opencode", Name: "OpenCode", Detect: []string{"opencode"}, Launch: "opencode"},
	{ID: "opencode2", Name: "OpenCode 2", Detect: []string{"opencode2"}, Launch: "opencode2 --standalone"},
	{ID: "mimo-code", Name: "MiMo Code", Detect: []string{"mimo"}, Launch: "mimo"},
	{ID: "pi", Name: "Pi", Detect: []string{"pi"}, Launch: "pi"},
	{ID: "omp", Name: "OMP", Detect: []string{"omp"}, Launch: "omp"},
	{ID: "prime-agent", Name: "Prime Agent", Detect: []string{"prime-agent"}, Launch: "prime-agent"},
	{ID: "qoder", Name: "Qoder", Detect: []string{"qodercli", "qoder"}, Launch: "qodercli", Yolo: "--dangerously-skip-permissions"},
	{ID: "qoder-cn", Name: "Qoder CN", Detect: []string{"qoderclicn", "qodercn"}, Launch: "qoderclicn", Yolo: "--dangerously-skip-permissions"},
	{ID: "gemini", Name: "Gemini", Detect: []string{"gemini"}, Launch: "gemini", Yolo: "--yolo"},
	{ID: "antigravity", Name: "Antigravity", Detect: []string{"agy"}, Launch: "agy", Yolo: "--dangerously-skip-permissions"},
	{ID: "aider", Name: "Aider", Detect: []string{"aider"}, Launch: "aider", Yolo: "--yes-always"},
	{ID: "goose", Name: "Goose", Detect: []string{"goose"}, Launch: "goose"},
	{ID: "amp", Name: "Amp", Detect: []string{"amp"}, Launch: "amp", Yolo: "--dangerously-allow-all"},
	{ID: "kilo", Name: "Kilo", Detect: []string{"kilo"}, Launch: "kilo"},
	{ID: "kiro", Name: "Kiro", Detect: []string{"kiro-cli"}, Launch: "kiro-cli chat --tui", Yolo: "--trust-all-tools"},
	{ID: "crush", Name: "Crush", Detect: []string{"crush"}, Launch: "crush", Yolo: "--yolo"},
	{ID: "aug", Name: "Auggie", Detect: []string{"auggie"}, Launch: "auggie"},
	{ID: "cline", Name: "Cline", Detect: []string{"cline"}, Launch: "cline", Yolo: "--auto-approve true"},
	{ID: "freebuff", Name: "Freebuff", Detect: []string{"freebuff"}, Launch: "freebuff"},
	{ID: "codebuff", Name: "Codebuff", Detect: []string{"codebuff"}, Launch: "codebuff"},
	{ID: "command-code", Name: "Command Code", Detect: []string{"command-code"}, Launch: "command-code --trust", Yolo: "--yolo"},
	{ID: "continue", Name: "Continue", Detect: []string{"cn"}, Launch: "cn", Yolo: `--allow "*"`},
	{ID: "cursor", Name: "Cursor Agent", Detect: []string{"cursor-agent"}, Launch: "cursor-agent", Yolo: "--yolo"},
	{ID: "droid", Name: "Droid", Detect: []string{"droid"}, Launch: "droid", Yolo: "--auto high"},
	{ID: "kimi", Name: "Kimi", Detect: []string{"kimi", "kimi-code"}, Launch: "kimi", Yolo: "--yolo"},
	{ID: "mistral-vibe", Name: "Mistral Vibe", Detect: []string{"vibe", "mistral-vibe"}, Launch: "vibe", Yolo: "--agent auto-approve"},
	{ID: "qwen-code", Name: "Qwen Code", Detect: []string{"qwen"}, Launch: "qwen", Yolo: "--approval-mode yolo"},
	{ID: "rovo", Name: "Rovo Dev", Detect: []string{"acli"}, Launch: "acli rovodev run", Yolo: "--yolo"},
	{ID: "hermes", Name: "Hermes", Detect: []string{"hermes"}, Launch: "hermes --tui", Yolo: "--yolo"},
	{ID: "openclaw", Name: "OpenClaw", Detect: []string{"openclaw"}, Launch: "openclaw"},
	{ID: "copilot", Name: "Copilot", Detect: []string{"copilot"}, Launch: "copilot", Yolo: "--yolo"},
	{ID: "grok", Name: "Grok", Detect: []string{"grok"}, Launch: "grok", Yolo: "--permission-mode bypassPermissions"},
	{ID: "muse", Name: "Muse", Detect: []string{"muse"}, Launch: "muse --trust-workspace", Yolo: "--yolo"},
	{ID: "dsh", Name: "DSH", Detect: []string{"dsh-tui", "dst"}, Requires: []string{"dsh"}, Launch: "dsh-tui ."},
	{ID: "zcode", Name: "ZCode", Detect: []string{"zcode"}, Launch: "zcode", Yolo: "--mode yolo"},
	{ID: "devin", Name: "Devin", Detect: []string{"devin"}, Launch: "devin", Yolo: "--permission-mode bypass --respect-workspace-trust false"},
	{ID: "jcode", Name: "JCode", Detect: []string{"jcode"}, Launch: "jcode"},
}
