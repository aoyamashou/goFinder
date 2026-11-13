package gui

import (
	"errors"
	"gofinder/internal/keymap"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"gofinder/internal/agents"
	"gofinder/internal/projectinit"
)

func folderSettings(t *testing.T, a *app, tt *ui.Tester) {
	t.Helper()
	a.run("settings")
	tt.Frame()
	tt.Scroll(550, 400, 0, 2000)
	tt.Frame()
}

func TestFolderInitSettings(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.dataDir = t.TempDir()
	if a.folderOptions().Enabled() {
		t.Fatal("enabled by default")
	}
	os.WriteFile(filepath.Join(a.dataDir, "config.json"), []byte(`{"version":7}`), 0o644)
	old := loadConfig(a.dataDir)
	if old.NewFolderGit || old.NewFolderAgentFiles || old.NewFolderSkills {
		t.Fatal("old config enabled options")
	}
	for _, close := range []string{"取消", "Escape", "保存"} {
		folderSettings(t, a, tt)
		for _, label := range []string{"初始化 Git 仓库", "创建 Agent 说明文件", "创建项目级 skills 目录"} {
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
		}
		if a.folderOptions().Enabled() {
			t.Fatal("draft applied before save")
		}
		// Moving between tabs retains the draft and updates the path caption.
		a.setPage = 1
		tt.Frame()
		a.setPage = 0
		tt.Frame()
		if !a.setFolder.Git || !a.setFolder.AgentFiles || !a.setFolder.Skills {
			t.Fatal("draft lost")
		}
		if close == "Escape" {
			tt.Key(0, ui.KeyEscape)
		} else if err := tt.Click(close); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.dlgOpen {
			t.Fatal("dialog remained open")
		}
		want := close == "保存"
		cfg := loadConfig(a.dataDir)
		if a.cfg.NewFolderGit != want || a.cfg.NewFolderAgentFiles != want || a.cfg.NewFolderSkills != want || cfg.NewFolderGit != want || cfg.NewFolderAgentFiles != want || cfg.NewFolderSkills != want {
			t.Fatalf("%s: %+v", close, cfg)
		}
	}
	// A queued save failure must remain visible after the ordinary settings status.
	folderSettings(t, a, tt)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	os.WriteFile(blocker, nil, 0o644)
	a.dataDir = blocker
	var jobs []func()
	a.async = func(f func()) { jobs = append(jobs, f) }
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for len(jobs) > 0 {
		f := jobs[0]
		jobs = jobs[1:]
		f()
	}
	if !strings.Contains(a.status, "无法保存 config.json") {
		t.Fatal(a.status)
	}
}

func TestFolderSkillsDefaultAgent(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.ag.found = nil
	for _, id := range []string{"claude", "claude-agent-teams", "codex", "openclaude"} {
		e, _ := agents.ByID(id)
		a.ag.found = append(a.ag.found, agents.Found{Entry: e})
	}
	a.cfg.Agents = map[string]agentCfg{}
	for _, tc := range []struct{ id, path, name string }{
		{"claude", ".claude/skills/", "Claude"}, {"claude-agent-teams", ".claude/skills/", "Claude Agent Teams"}, {"codex", ".agents/skills/", "Codex"}, {"openclaude", ".agents/skills/", "OpenClaude"},
	} {
		a.cfg.DefaultAgent = tc.id
		folderSettings(t, a, tt)
		want := "按默认 Agent（" + tc.name + "）创建 " + tc.path
		if !tt.HasText(want) {
			t.Fatalf("missing %q", want)
		}
		if got := projectinit.SkillsDir(a.folderOptions().AgentID) + "/"; got != tc.path {
			t.Fatal(got)
		}
		a.dlgOpen = false
		tt.Frame()
	}
	a.cfg.DefaultAgent = "codex"
	a.cfg.Agents["codex"] = agentCfg{Enabled: false}
	if a.folderOptions().AgentID != "claude" {
		t.Fatal("did not use first enabled agent")
	}
	a.ag.found = nil
	if a.folderSkillsCaption() != "创建 .agents/skills/" {
		t.Fatal(a.folderSkillsCaption())
	}
}

func TestFolderInitWorkflow(t *testing.T) {
	a, tt, _, root := setup(t)
	a.cfg.NewFolderAgentFiles = true
	a.cfg.NewFolderSkills = true
	a.cfg.NewFolderGit = true
	a.initGit = func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0o755) }
	// Exercise the actual key binding, including its single-key/double-tap delay.
	tt.Key(0, ui.KeyN)
	tt.Frame()
	tt.Advance(keymap.DoubleTap + 10*time.Millisecond)
	if !a.dlgOpen || a.dlg != dlgNewFolder {
		t.Fatal("new-folder key failed")
	}
	a.dlgText = "项目 ' $(touch nope)"
	a.submitDialog()
	tt.Frame()
	path := filepath.Join(root, "项目 ' $(touch nope)")
	if cursorName(a) != filepath.Base(path) {
		t.Fatal(cursorName(a))
	}
	if b, err := os.ReadFile(filepath.Join(path, "CLAUDE.md")); err != nil || string(b) != "@AGENTS.md\n" {
		t.Fatal(string(b), err)
	}
	// A repeated name initializes the unique returned path, preserving the old tree.
	os.WriteFile(filepath.Join(path, "AGENTS.md"), []byte("keep"), 0o644)
	a.run("newfolder")
	a.dlgText = filepath.Base(path)
	a.submitDialog()
	tt.Frame()
	second := filepath.Join(root, cursorName(a))
	if second == path {
		t.Fatal("not uniquified")
	}
	if _, err := os.Stat(filepath.Join(second, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(path, "AGENTS.md"))
	if string(b) != "keep" {
		t.Fatal("overwritten")
	}
	// A side-column target must not initialize the active directory.
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	tt.Frame()
	a.openDialog(dlgNewFolder, a.tab(), "side-project", "")
	a.dlgDir = root
	a.submitDialog()
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "side-project", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "side-project")); !os.IsNotExist(err) {
		t.Fatal("wrong target")
	}
	// Ordinary files and failed folder creation must never run the initializer.
	calls := 0
	a.initGit = func(string) error { calls++; return nil }
	a.openDialog(dlgNewFile, a.tab(), "ordinary.txt", "")
	a.dlgTpl = blankTemplate
	a.submitDialog()
	a.openDialog(dlgNewFolder, a.tab(), "bad/path", "")
	a.submitDialog()
	if calls != 0 {
		t.Fatal("initialized a file or failed directory")
	}
	a.run("newfolder")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if calls != 0 {
		t.Fatal("initialized on cancel")
	}
}

func TestFolderInitAsyncSnapshot(t *testing.T) {
	a, tt, _, root := setup(t)
	a.cfg.NewFolderAgentFiles = true
	a.cfg.NewFolderSkills = true
	a.cfg.NewFolderGit = true
	e, _ := agents.ByID("claude")
	a.ag.found = []agents.Found{{Entry: e}}
	a.cfg.DefaultAgent = "claude"
	calls := 0
	a.initGit = func(string) error { calls++; return errors.New("test unavailable") }
	var jobs, posts []func()
	a.async = func(f func()) { jobs = append(jobs, f) }
	a.post = func(f func()) { posts = append(posts, f) }
	a.run("newfolder")
	a.dlgText = "slow"
	a.submitDialog()
	// Creation is synchronous, initialization has not run. Navigation remains usable.
	path := filepath.Join(root, "slow")
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("initialization was synchronous")
	}
	a.cfg.NewFolderSkills = false
	a.cfg.NewFolderAgentFiles = false
	a.cfg.DefaultAgent = "codex"
	a.ag.found = nil
	a.layout = layoutDual
	a.active = 1
	a.navigate(a.tab(), filepath.Join(root, "beta"))
	a.wantFocus = false
	a.focusZone = "pane1"
	// Drain controlled worker and main queues, including listing reads.
	for len(jobs) > 0 || len(posts) > 0 {
		for len(jobs) > 0 {
			f := jobs[0]
			jobs = jobs[1:]
			f()
		}
		for len(posts) > 0 {
			f := posts[0]
			posts = posts[1:]
			f()
		}
	}
	if calls != 1 || a.active != 1 || a.tab().t.Dir != filepath.Join(root, "beta") || a.wantFocus || a.focusZone != "pane1" {
		t.Fatal("late callback changed focus or repeated", calls)
	}
	if _, err := os.Stat(filepath.Join(path, ".claude", "skills")); err != nil {
		t.Fatal("snapshot lost", err)
	}
	if _, err := os.Stat(filepath.Join(path, ".agents")); !os.IsNotExist(err) {
		t.Fatal("used changed default Agent")
	}
	if !strings.Contains(a.status, "初始化部分完成") || !strings.Contains(a.status, "test unavailable") {
		t.Fatal(a.status)
	}
	for range 3 {
		tt.Frame()
	}
	for len(jobs) > 0 || len(posts) > 0 {
		for len(jobs) > 0 {
			f := jobs[0]
			jobs = jobs[1:]
			f()
		}
		for len(posts) > 0 {
			f := posts[0]
			posts = posts[1:]
			f()
		}
	}
	if calls != 1 {
		t.Fatal("extra frames repeated initialization")
	}
}

func TestFolderInitStatus(t *testing.T) {
	for _, tc := range []struct {
		r    []projectinit.Result
		want string
	}{
		{[]projectinit.Result{{Name: "Git"}}, "初始化完成"},
		{[]projectinit.Result{{Name: "Git", Err: errors.New("missing")}}, "初始化未完成"},
		{[]projectinit.Result{{Name: "AGENTS.md", Skipped: true}}, "已存在，已跳过"},
	} {
		if got := folderInitStatus("/tmp/project", tc.r); !strings.Contains(got, tc.want) {
			t.Fatal(got)
		}
	}
}

func TestFolderInitRefreshPreservesCursor(t *testing.T) {
	a, tt, _, root := setup(t)
	// Complete a listing read only after the user has moved to another row.
	var jobs, posts []func()
	a.async = func(f func()) { jobs = append(jobs, f) }
	a.post = func(f func()) { posts = append(posts, f) }
	a.refreshInitializedFolder(filepath.Join(root, "alpha"))
	a.tab().cursor = a.tab().t.Index("notes.md")
	a.tab().sel.Add(filepath.Join(root, "notes.md"))
	a.wantFocus = false
	for len(jobs) > 0 {
		f := jobs[0]
		jobs = jobs[1:]
		f()
	}
	for len(posts) > 0 {
		f := posts[0]
		posts = posts[1:]
		f()
	}
	if cursorName(a) != "notes.md" || !a.tab().sel.Has(filepath.Join(root, "notes.md")) || a.wantFocus {
		t.Fatal("refresh moved selection")
	}
	tt.Frame()
}

func TestFolderInitShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("MYGO_SHOTS not set")
	}
	os.MkdirAll(dir, 0o755)
	for _, mode := range []string{"light", "dark"} {
		a, tt, _, _ := setup(t)
		a.cfg.Appearance = mode
		e, _ := agents.ByID("claude")
		a.ag.found = []agents.Found{{Entry: e}}
		a.cfg.DefaultAgent = "claude"
		folderSettings(t, a, tt)
		for _, label := range []string{"初始化 Git 仓库", "创建 Agent 说明文件", "创建项目级 skills 目录"} {
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
		}
		writePNG(t, filepath.Join(dir, "folder-init-"+mode+"-settings.png"), tt.Image())
	}
}

func TestFolderInitMenuEntries(t *testing.T) {
	a, tt, _, root := setup(t)
	a.cfg.NewFolderAgentFiles = true
	// Native menu dispatch and a current-column context menu share submitDialog.
	a.buildMenu()
	a.menu.items["newfolder"].Click(nil, nil)
	tt.Frame()
	a.dlgText = "menu-project"
	a.submitDialog()
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "menu-project", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	r, ok := tt.Find("文件")
	if !ok {
		t.Fatal("no current list")
	}
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H-8)
	if err := tt.ChooseMenuItem("新建文件夹"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.dlgText = "context-project"
	a.submitDialog()
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "context-project", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
}

func TestFolderInitPendingCreationRefresh(t *testing.T) {
	a, _, _, root := setup(t)
	a.cfg.NewFolderGit = true
	a.initGit = func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0o755) }
	var jobs, posts []func()
	a.async = func(f func()) { jobs = append(jobs, f) }
	a.post = func(f func()) { posts = append(posts, f) }
	a.run("newfolder")
	a.dlgText = "pending-project"
	a.submitDialog()
	if len(jobs) != 2 {
		t.Fatalf("jobs %d", len(jobs))
	}
	// The ordinary listing can read before initialization, yet post after it.
	jobs[0]()
	jobs[1]()
	jobs = nil
	posts[1]()
	posts[0]()
	posts = nil
	for len(jobs) > 0 || len(posts) > 0 {
		for len(jobs) > 0 {
			f := jobs[0]
			jobs = jobs[1:]
			f()
		}
		for len(posts) > 0 {
			f := posts[0]
			posts = posts[1:]
			f()
		}
	}
	if cursorName(a) != "pending-project" || a.tab().t.Focus != "" || !a.tab().loaded {
		t.Fatal("creation focus lost", cursorName(a))
	}
	if a.git[filepath.Join(root, "pending-project")] == nil {
		t.Fatal("repository icon cache not refreshed")
	}
}
