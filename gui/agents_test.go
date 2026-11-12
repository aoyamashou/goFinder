package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

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

// testAgents is the agents the next setup installs, for one test: the
// detection's result (catalog entries in this order), their commands and
// the default, as config.json would give them.
var testAgents *struct {
	def   string
	items []agentItem
}

// withAgents sets the agents (and the default) setup gives the app.
func withAgents(t *testing.T, def string, items ...agentItem) {
	t.Helper()
	testAgents = &struct {
		def   string
		items []agentItem
	}{def, items}
	t.Cleanup(func() { testAgents, testApp = nil, nil })
	if testApp != nil {
		applyTestAgents(testApp) // a change while the test runs
	}
}

// testApp is the app the running test set up.
var testApp *app

// applyTestAgents puts withAgents' agents into a, as a detection and the
// settings would.
func applyTestAgents(a *app) {
	testApp = a
	ta := testAgents
	if ta == nil {
		return // the app's own detection
	}
	a.ag.found = nil
	if a.cfg.Agents == nil {
		a.cfg.Agents = map[string]agentCfg{}
	}
	for _, it := range ta.items {
		e, ok := agents.ByID(it.ID)
		if !ok {
			panic("no catalog entry " + it.ID)
		}
		a.ag.found = append(a.ag.found, agents.Found{Entry: e, Path: "/bin/" + it.ID})
		a.cfg.Agents[it.ID] = agentCfg{Enabled: true, Command: it.Command}
	}
	a.cfg.DefaultAgent = ta.def
}

var (
	agClaude = agentItem{ID: "claude", Name: "Claude", Symbol: "sparkles", Command: "claude --dangerously-skip-permissions"}
	agCodex  = agentItem{ID: "codex", Name: "Codex", Symbol: "terminal", Command: "codex --yolo"}
)

func menuLabels(tt *ui.Tester) []string {
	return slices.DeleteFunc(tt.Menu(), func(s string) bool { return s == "" || s == "-" })
}

// `a` runs the default agent in the login shell through a .command
// script, the folder quoted for sh and the command as is; a terminal that
// cannot run scripts falls back to Terminal.
func TestAgentKey(t *testing.T) {
	withAgents(t, "claude", agClaude, agCodex)
	t.Setenv("SHELL", "/bin/zsh")
	a, tt, fs, root := setup(t)
	dir := filepath.Join(root, "it's $(here)")
	os.Mkdir(dir, 0o755)
	a.cacheDir = t.TempDir()
	a.navigate(a.tab(), dir)
	tt.Frame()
	os.WriteFile(filepath.Join(dir, "f.txt"), nil, 0o644)
	a.refreshLists()
	tt.Frame()
	tt.TypeKey(0, ui.KeyA, "a")
	tt.Frame()
	if len(fs.openedWith) != 1 || fs.openedWith[0][0] != "Terminal" || !strings.HasSuffix(fs.openedWith[0][1], ".command") {
		t.Fatalf("open -a %v", fs.openedWith)
	}
	script := fs.openedWith[0][1]
	b, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	want := "#!/bin/zsh -l\ncd '" + root + "/it'\\''s $(here)' && exec claude --dangerously-skip-permissions\n"
	if string(b) != want {
		t.Fatalf("script %q, want %q", b, want)
	}
	if st, _ := os.Stat(script); st.Mode()&0o111 == 0 {
		t.Fatalf("not executable: %v", st.Mode())
	}
	if !strings.Contains(a.status, "已在 Terminal 中运行 claude --dangerously-skip-permissions") {
		t.Fatalf("status %q", a.status)
	}

	// ⌘⌥A is the second key; another shell and terminal.
	t.Setenv("SHELL", "")
	a.cfg.Terminal = "Kaku"
	tt.Key(ui.Cmd|ui.Alt, ui.KeyA)
	tt.Frame()
	if got := fs.openedWith[1]; got != [2]string{"Kaku", script} {
		t.Fatalf("Kaku: %v", got)
	}
	if b, _ := os.ReadFile(script); !strings.HasPrefix(string(b), "#!/bin/zsh -l\n") {
		t.Fatalf("fallback shell: %q", b)
	}
	a.cfg.Terminal = "NoScripts"
	a.run("agent")
	if got := fs.openedWith[2]; got[0] != "Terminal" || !strings.Contains(a.status, "NoScripts 不能运行 .command") {
		t.Fatalf("fallback: %v %q", got, a.status)
	}

	// `a` and ` are default hints of the status bar.
	hints := map[string]bool{"agent": true, "go.start": true}
	for _, h := range a.km.Hints(0) {
		delete(hints, h.ID)
	}
	if len(hints) > 0 {
		t.Fatalf("missing from the default hints: %v", hints)
	}
}

// On a folder the agent opens that folder, on a file the list's folder.
func TestAgentDir(t *testing.T) {
	withAgents(t, "claude", agClaude)
	a, tt, fs, root := setup(t)
	a.cacheDir = t.TempDir()
	scriptDir := func() string {
		b, _ := os.ReadFile(fs.openedWith[len(fs.openedWith)-1][1])
		return string(b)
	}
	if cursorName(a) != "alpha" {
		t.Fatalf("cursor on %q", cursorName(a))
	}
	a.run("agent")
	if s := scriptDir(); !strings.Contains(s, "cd '"+filepath.Join(root, "alpha")+"' &&") {
		t.Fatalf("on a folder: %q", s)
	}
	for cursorName(a) != "notes.md" {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	a.run("agent")
	if s := scriptDir(); !strings.Contains(s, "cd '"+root+"' &&") {
		t.Fatalf("on a file: %q", s)
	}
}

// Without an enabled agent, a says so.
func TestAgentNone(t *testing.T) {
	withAgents(t, "")
	a, tt, fs, _ := setup(t)
	tt.TypeKey(0, ui.KeyA, "a")
	tt.Frame()
	if len(fs.openedWith) != 0 || a.status != "没有启用的 Agent，去设置 › Agent 启用" {
		t.Fatalf("%v %q", fs.openedWith, a.status)
	}
}

// Both context menus list the terminal, then the enabled agents, at the
// top level, once.
func TestAgentsInMenus(t *testing.T) {
	withAgents(t, "codex", agClaude, agCodex)
	a, tt, fs, root := setup(t)
	a.cacheDir = t.TempDir()
	agentsOf := func(m []string) []string {
		return slices.DeleteFunc(slices.Clone(m), func(s string) bool { return s != "在终端中打开" && !strings.HasPrefix(s, "在这里打开") })
	}
	want := []string{"在终端中打开", "在这里打开 Claude", "在这里打开 Codex"}

	if err := tt.RightClick("notes.md"); err != nil {
		t.Fatal(err)
	}
	if got := agentsOf(menuLabels(tt)); !slices.Equal(got, want) {
		t.Fatalf("row menu: %q", menuLabels(tt))
	}
	if err := tt.ChooseMenuItem("在这里打开 Claude"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if b, _ := os.ReadFile(fs.openedWith[0][1]); !strings.HasSuffix(string(b), "exec claude --dangerously-skip-permissions\n") {
		t.Fatalf("script %q", b)
	}

	// A disabled agent is gone; a user's list may have its own terminal item.
	withAgents(t, "claude", agClaude)
	a.cfg.ContextMenu = []string{"open", "terminal", "@agents", "-", "trash"}
	if err := tt.RightClick("notes.md"); err != nil {
		t.Fatal(err)
	}
	if got := agentsOf(menuLabels(tt)); !slices.Equal(got, want[:2]) {
		t.Fatalf("one agent: %q", menuLabels(tt))
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	// The blank space's menu, on the folder it belongs to.
	withAgents(t, "codex", agClaude, agCodex)
	r, _ := tt.Find("文件")
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H-8)
	if got := agentsOf(menuLabels(tt)); !slices.Equal(got, want) {
		t.Fatalf("blank menu: %q", menuLabels(tt))
	}
	if err := tt.ChooseMenuItem("在这里打开 Codex"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if b, _ := os.ReadFile(fs.openedWith[len(fs.openedWith)-1][1]); !strings.Contains(string(b), "cd '"+root+"' && exec codex --yolo") {
		t.Fatalf("blank script %q", b)
	}
	// config.json's blankMenu may place @agents, with the terminal once.
	a.cfg.BlankMenu = []string{"refresh", "-", "terminal", "@agents"}
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H-8)
	if got := menuLabels(tt); !slices.Equal(got, append([]string{"刷新"}, want...)) {
		t.Fatalf("custom blankMenu: %q", got)
	}
}

// The title bar's agent button opens the default agent; its dropdown lists
// the enabled agents.
func TestAgentTitleBar(t *testing.T) {
	withAgents(t, "codex", agClaude, agCodex)
	a, tt, fs, _ := setup(t)
	a.cacheDir = t.TempDir()
	if err := tt.Click("在这里打开默认 Agent"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if b, _ := os.ReadFile(fs.openedWith[0][1]); !strings.Contains(string(b), "exec codex --yolo") {
		t.Fatalf("default: %q", b)
	}
	if err := tt.Click("▾"); err != nil {
		t.Fatal(err)
	}
	if got := menuLabels(tt); !slices.Equal(got, []string{"在这里打开 Claude", "在这里打开 Codex"}) {
		t.Fatalf("dropdown: %q", got)
	}
	if err := tt.ChooseMenuItem("在这里打开 Claude"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if b, _ := os.ReadFile(fs.openedWith[1][1]); !strings.Contains(string(b), "exec claude ") {
		t.Fatalf("chosen: %q", b)
	}
}

// The File menu has an agent submenu; the bar is rebuilt when the agents
// change.
func TestAgentMenuBar(t *testing.T) {
	withAgents(t, "claude", agClaude, agCodex)
	a, tt, fs, _ := setup(t)
	a.cacheDir = t.TempDir()
	built := 0
	a.setMenu = func([]*mygo.MenuItem) { built++ }
	var sub *mygo.MenuItem
	walk(a.buildMenu(), func(it *mygo.MenuItem) {
		if it.Label == "在这里打开 Agent" {
			sub = it
		}
	})
	if sub == nil || len(sub.Submenu) != 2 || sub.Submenu[0].Label != "Claude" || sub.Submenu[1].Label != "Codex" {
		t.Fatalf("submenu %+v", sub)
	}
	sub.Submenu[1].Click(sub.Submenu[1], nil)
	tt.Frame()
	if b, _ := os.ReadFile(fs.openedWith[0][1]); !strings.Contains(string(b), "exec codex --yolo") {
		t.Fatalf("clicked: %q", b)
	}
	a.syncMenu()
	if built != 0 {
		t.Fatalf("rebuilt without a change")
	}
	withAgents(t, "claude", agClaude)
	a.syncMenu()
	if built != 1 {
		t.Fatalf("not rebuilt after a change: %d", built)
	}
	a.syncMenu()
	if built != 1 {
		t.Fatalf("rebuilt twice: %d", built)
	}
}
