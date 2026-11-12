package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gofinder/internal/agents"
)

// installed makes the app find only these binaries.
func installed(a *app, bins ...string) {
	a.ag.exists = func(p string) bool { return slices.Contains(bins, filepath.Base(p)) }
	a.ag.home = "/h"
	a.detectAgents()
}

func agentIDs(a *app) []string {
	var s []string
	for _, it := range a.enabledAgents() {
		s = append(s, it.ID)
	}
	return s
}

func savedConfig(t *testing.T, a *app) config {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(a.dataDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c config
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDetectAgentsEnablesNewOnes(t *testing.T) {
	a, _, fs, _ := setup(t)
	installed(a, "claude", "orca", "codex", "pi", "agy")
	want := []string{"claude", "claude-agent-teams", "codex", "pi", "antigravity"}
	if got := agentIDs(a); !slices.Equal(got, want) {
		t.Fatalf("enabled %v, want %v", got, want)
	}
	if len(fs.shells) != 1 || fs.shells[0][1] != `$SHELL -lc 'printf %s "$PATH"'` {
		t.Fatalf("login PATH read: %v", fs.shells)
	}
	installed(a, "claude", "orca", "codex", "pi", "agy")
	if len(fs.shells) != 1 {
		t.Fatalf("PATH read again: %v", fs.shells)
	}
	it, ok := a.defaultAgentItem()
	if !ok || it.ID != "claude" || it.Command != "claude --dangerously-skip-permissions" || it.Symbol != "sparkle" {
		t.Fatalf("default %+v", it)
	}
	// only orca: not installed
	installed(a, "orca")
	if len(a.enabledAgents()) != 0 {
		t.Fatalf("orca alone: %v", agentIDs(a))
	}
	if _, ok := a.defaultAgentItem(); ok {
		t.Fatal("default without agents")
	}
}

func TestDefaultAgentFallback(t *testing.T) {
	a, _, _, _ := setup(t)
	installed(a, "claude", "codex", "pi")
	a.cfg.DefaultAgent = "codex"
	if it, _ := a.defaultAgentItem(); it.ID != "codex" {
		t.Fatalf("default %v", it)
	}
	a.cfg.Agents = map[string]agentCfg{"codex": {Enabled: false}} // disabled
	if it, _ := a.defaultAgentItem(); it.ID != "claude" {
		t.Fatalf("disabled default: %v", it)
	}
	a.cfg.DefaultAgent = "gemini" // not installed
	if it, _ := a.defaultAgentItem(); it.ID != "claude" {
		t.Fatalf("missing default: %v", it)
	}
	a.cfg.Agents["claude"] = agentCfg{Enabled: true, Command: "claude --continue"}
	if it, _ := a.defaultAgentItem(); it.Command != "claude --continue" {
		t.Fatalf("custom command: %v", it)
	}
}

func TestAgentConfigMigration(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) config {
		os.WriteFile(filepath.Join(dir, "config.json"), []byte(s), 0o644)
		return loadConfig(dir)
	}
	c := write(`{"version": 7, "agentCommand": "codex resume --yolo"}`)
	if c.DefaultAgent != "codex" || c.Agents["codex"] != (agentCfg{Enabled: true, Command: "codex resume --yolo"}) {
		t.Fatalf("migrated: %+v", c)
	}
	if c := write(`{"version": 7, "agentCommand": "claude"}`); c.DefaultAgent != "" || len(c.Agents) != 0 {
		t.Fatalf("default claude: %+v", c)
	}
	if c := write(`{"version": 7, "agentCommand": "my-tool"}`); c.DefaultAgent != "" || len(c.Agents) != 0 {
		t.Fatalf("unknown binary: %+v", c)
	}
	// new settings win over the old field
	if c := write(`{"version": 7, "agentCommand": "codex", "defaultAgent": "pi"}`); c.DefaultAgent != "pi" || len(c.Agents) != 0 {
		t.Fatalf("not overridden: %+v", c)
	}
}

// Each of the settings page's four changes is written to config.json.
func TestAgentSettingsWrite(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.dataDir = t.TempDir()
	installed(a, "claude", "codex")
	a.run("settings")
	a.setPage = 2
	tt.Frame()
	if !tt.HasText("已安装 · 2 个") || !tt.HasText("Codex") || !tt.HasText("刷新") {
		t.Fatalf("page: %q", tt.Texts())
	}
	a.cfg.Agents = nil

	a.setAgentEnabled("codex", false)
	if c := savedConfig(t, a); c.Agents["codex"].Enabled {
		t.Fatalf("disable: %+v", c.Agents)
	}
	a.setAgentEnabled("codex", true)
	if c := savedConfig(t, a); !c.Agents["codex"].Enabled {
		t.Fatalf("enable: %+v", c.Agents)
	}
	a.setAgentCommand("codex", "codex --full-auto")
	if c := savedConfig(t, a); c.Agents["codex"].Command != "codex --full-auto" || !c.Agents["codex"].Enabled {
		t.Fatalf("command: %+v", c.Agents)
	}
	a.resetAgentCommand("codex")
	if c := savedConfig(t, a); c.Agents["codex"].Command != "" {
		t.Fatalf("reset: %+v", c.Agents)
	}
	if a.ag.rows["codex"].cmd != "codex --dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("box %q", a.ag.rows["codex"].cmd)
	}
	a.setDefaultAgent("codex")
	if c := savedConfig(t, a); c.DefaultAgent != "codex" {
		t.Fatalf("default: %+v", c)
	}
	// typing the default command stores nothing
	a.setAgentCommand("codex", agents.DefaultCommand(agents.Catalog[4]))
	if c := savedConfig(t, a); c.Agents["codex"].Command != "" {
		t.Fatalf("default typed: %+v", c.Agents)
	}
}

// The page's controls do the same: 禁用 and 恢复默认 are clicked.
func TestAgentSettingsClicks(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.dataDir = t.TempDir()
	installed(a, "pi")
	a.run("settings")
	a.setPage = 2
	tt.Frame()
	if err := tt.Click("禁用"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if c := savedConfig(t, a); c.Agents["pi"].Enabled {
		t.Fatalf("click 禁用: %+v", c.Agents)
	}
	if len(a.enabledAgents()) != 0 {
		t.Fatal("still enabled")
	}
	if err := tt.Click("启用"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !savedConfig(t, a).Agents["pi"].Enabled {
		t.Fatal("click 启用")
	}
}

func TestAgentDetecting(t *testing.T) {
	a, tt, _, _ := setup(t)
	var later []func()
	a.async = func(f func()) { later = append(later, f) }
	installed(a, "pi")
	a.run("settings")
	a.setPage = 2
	tt.Frame()
	if !tt.HasText("正在检测…") {
		t.Fatalf("detecting: %q", tt.Texts())
	}
	for _, f := range later {
		f()
	}
	tt.Frame()
	if tt.HasText("正在检测…") || !tt.HasText("已安装 · 1 个") {
		t.Fatalf("done: %q", tt.Texts())
	}
}
