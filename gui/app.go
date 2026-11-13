package gui

import (
	"fmt"
	"hash/fnv"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/fsops"
	"gofinder/internal/git"
	"gofinder/internal/keymap"
	"gofinder/internal/pane"
	"gofinder/internal/projectinit"
	"gofinder/internal/shellcmd"
	"gofinder/internal/templates"
	"gofinder/internal/vfs"
)

// Every pane shows three columns (the parent folder, the current one and
// the preview); the layouts differ in how many panes there are.
const (
	layoutColumns = iota // one pane
	layoutDual           // two panes side by side
	layoutRows           // two panes, one above the other
)

// tabState is a tab and what the table needs to show it.
type tabState struct {
	t      *pane.Tab
	list   ui.ListState
	sel    ui.Selection[string] // chosen paths
	cursor int                  // row with the keyboard, -1 for none
	sort   ui.SortOrder
	seq    int // load generation, to drop stale results
	loaded bool
	err    string

	watched string
	stop    func()

	gitRoot string // the repository the folder is in, "" for none

	width widthCache // the widest name, for the column's width

	// box and hits are where the list and its rows were in the last
	// frame, in the window, to tell a press on the blank space.
	box  ui.Rect
	hits []rowHit
}

func newTabState(dir string, hidden bool) *tabState {
	ts := &tabState{t: pane.New(dir), cursor: -1, sort: ui.SortOrder{Column: "name"}}
	ts.t.ShowHidden = hidden
	ts.list.Selected = &ts.cursor
	ts.list.Selection = &ts.sel
	ts.list.Sort = &ts.sort
	ts.list.Key = func(i int) any {
		if es := ts.t.Entries(); i >= 0 && i < len(es) {
			return es[i].Path
		}
		return i
	}
	return ts
}

func (ts *tabState) cursorEntry() (vfs.Entry, bool) {
	es := ts.t.Entries()
	if ts.cursor < 0 || ts.cursor >= len(es) {
		return vfs.Entry{}, false
	}
	return es[ts.cursor], true
}

// chosen is what commands act on: the selection, else the cursor row.
func (ts *tabState) chosen() []vfs.Entry {
	var out []vfs.Entry
	if ts.sel.Len() > 0 {
		for _, e := range ts.t.Entries() {
			if ts.sel.Has(e.Path) {
				out = append(out, e)
			}
		}
	}
	if len(out) == 0 {
		if e, ok := ts.cursorEntry(); ok {
			out = append(out, e)
		}
	}
	return out
}

func paths(es []vfs.Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Path
	}
	return out
}

type paneState struct {
	ts *tabState // a pane has one folder: workspaces take the place of tabs

	addrEditing bool
	addrText    string
	addrSugg    []string
	addrSuggFor string

	// The pane's other columns: its parent folder, and the preview of
	// what its cursor is on, with the preview's list of a folder's items;
	// and the inspector's item.
	parent  parentState
	pv      previewState
	pvList  ui.ListState
	pvLast  int       // the row last clicked there
	pvSel   int       // the third column's clicked row
	pvSeq   int       // ends a pending click once a double click comes
	pvSubAt time.Time // when a double click came: its Changed that follows is not a click
	insp    inspState
}

func (p *paneState) tab() *tabState { return p.ts }

const (
	dlgNone = iota
	dlgRename
	dlgNewFile
	dlgNewFolder
	dlgSettings
	dlgWorkspace
	dlgTrash
	dlgWsRename
)

// blankTemplate is the new-file dialog's choice of no template.
const blankTemplate = "空白文件"

type app struct {
	// vibrant is the sidebar material showing: the window has it and the
	// app draws in macOS's own appearance (set each frame by view).
	vibrant bool
	km      *keymap.Keymap
	kmFile  map[string][]string // keymap.json as last read or written
	kmPath  string              // keymap.json; "" saves nothing
	warns   []string            // what the keymap and config.json could not apply
	sys     system
	cfg     config
	async   func(func()) // runs work off the main thread
	post    func(func()) // runs on the main thread and redraws
	svc     ui.Services  // clipboard, from the last frame
	hasSvc  bool

	panes       [2]*paneState
	active      int
	layout      int
	showPreview bool
	// inspWide is whether the window was widened for the inspector;
	// growWindow widens it by dw DIPs to the right (nil in tests).
	inspWide   bool
	growWindow func(dw int)
	splitDual  float32
	splitRows  float32
	splitPrev  float32
	wantFocus  bool

	// The keyboard: wantSidebar asks for the sidebar to take it, and
	// focusZone is the column that has it, outlined ("sidebar", "pane0",
	// "pane1").
	wantSidebar bool
	focusZone   string

	// nerd says a Nerd Font is installed, for the lists' glyphs; charW
	// is the width of a column of the lists' font (charWFont). font is
	// the lists' font, JetBrainsMono Nerd Font Mono, and its download.
	nerd      bool
	charW     float32
	charWFont string
	font      fontState

	// qlFailed are the thumbnails qlmanage could not draw, not asked
	// for again while the app runs.
	qlFailed map[string]bool

	clip    []string
	clipCut bool
	status  string

	copied     string // the last text copyPathText put on the clipboard
	filterEat  bool   // the "f" typed with the key that opened the box is no text
	filterOn   bool   // the filter box is open and typing
	filterText string // what the filter box holds

	// taps waits on a key bound alone and as a double tap ("n", "n n").
	taps keymap.Taps
	// keyNow and plainBlocked are dispatch's clock and whether plain keys
	// step aside, as of the last frame, for keys only input events see.
	keyNow       time.Time
	plainBlocked bool
	// enterKey says the list's last input was ↩, not the pointer: its
	// Submitted runs ↩'s action (重命名) instead of opening.
	enterKey bool

	// pop is the menu the keyboard opens at the cursor row: "sort" (,)
	// or "new" (n n, ⌘N: a blank file or a template); popTpls are the
	// templates it offers, listed off the main thread.
	pop     string
	popOpen bool
	// menuHot is the side-column row a right-click menu shows for (rightclick.go).
	menuHot           string
	hotSeen, hotNudge bool
	popTpls           []string

	helpOpen  bool
	helpQuery string
	recording string // the action whose new key the settings' 快捷键 page waits for
	recEat    bool   // the text the recorded key types is no search
	setPage   int    // the settings dialog's page: 0 通用, 1 快捷键, 2 Agent, 3 工具栏, 4 关于
	keyQ      ui.Handle
	focusKeyQ bool // the 快捷键 page's search takes the keyboard
	toKeys    bool // the cheat sheet closed for the 快捷键 page, which opens next frame

	dlg       int
	dlgOpen   bool
	dlgText   string
	dlgTarget string
	dlgTab    *tabState
	dlgDir    string // the folder a new item goes in, "" for dlgTab's
	// The trash confirmation: what it moves, and its 不再提醒.
	trashPaths []string
	trashNoAsk bool
	setFolder  projectinit.Options // settings draft, committed only on Save
	dlgFolder  projectinit.Options // new-folder draft, applied only after creation
	initGit    func(string) error  // nil uses projectinit.InitGit
	setConfirm bool                // the settings' 删除前确认
	tplNames   []string            // the templates, for the new-file menus
	ag         agentState

	// The new-file dialog's templates, and the one chosen.
	tplDir     string
	dlgTpls    []string
	dlgTpl     string
	dlgTplSeen string

	// The settings dialog: installed apps to choose from.
	setApps    []string
	setLoading bool
	setTerm    string
	setEditor  string
	setViewer  string // NSFileViewer as read, "" for Finder
	bundleID   string
	debugOn    bool   // Debug 编号 (settings › 关于), this run only
	qlStop     func() // ends the open Quick Look
	qlSeq      int    // drops a Quick Look started before a newer press
	fullDisk   bool   // Full Disk Access granted (settings)

	configSeq     uint64     // assigned on the main thread, before queuing a save
	configMu      sync.Mutex // serializes config writes
	configWritten uint64     // protected by configMu
	dataDir       string     // config.json is saved here; "" saves nothing
	updates       bool       // the updater plugin is used (not in tests): menu item and About button
	cacheDir      string     // files opened from archives are extracted here

	sb  sidebarState
	sh  shelfState
	dnd dndState

	// The workspace dialog: saved names, and the one chosen.
	wsPick string
	// The workspaces switched with 1–9, [ and ]: the saved ones by name,
	// the one on screen ("" for none: goFinder starts in none), its index
	// for the strip, and a switch on its way (one at a time).
	wsList []string
	wsCur  string
	wsSel  int
	wsBusy bool

	git      map[string]*gitRepo // by repository root
	gitDelay time.Duration       // waits for changes to settle before git runs
	gitOff   bool                // no git installed

	menu    menuState
	setMenu func([]*mygo.MenuItem) // installs the menu bar; nil in tests
}

func newApp(km *keymap.Keymap, sys system, cfg config, start string) *app {
	a := &app{
		km: km, sys: sys, cfg: cfg,
		async:     func(f func()) { go f() },
		post:      func(f func()) { f() },
		layout:    layoutColumns,
		sb:        sidebarState{show: cfg.ShowSidebar, open: [3]bool{true, true, true}, volumesDir: "/Volumes"},
		splitDual: 50, splitRows: 50, splitPrev: 68,
		kmFile:    map[string][]string{},
		wantFocus: true,
		git:       map[string]*gitRepo{},
		gitDelay:  300 * time.Millisecond,
		qlFailed:  map[string]bool{},
		font:      newFontState(),
		ag:        newAgentState(),
	}
	for i := range a.panes {
		a.panes[i] = &paneState{ts: newTabState(start, cfg.ShowHidden)}
	}
	return a
}

// start loads the first listings.
func (a *app) start() {
	a.checkFonts()
	if a.cacheDir != "" {
		dir := a.qlDir()
		a.async(func() { pruneThumbs(dir, time.Now()) })
	}
	a.loadAll()
	a.loadTemplates()
	a.loadFavorites()
	a.loadShelf()
	a.listWorkspaces()
}

// setWarnings keeps what the keymap and config.json could not apply, for
// the cheat sheet, and says so in the status bar.
func (a *app) setWarnings(w []string) {
	a.warns = w
	if len(w) > 0 {
		a.status = fmt.Sprintf("快捷键或 config.json 有 %d 处未按原样生效，按 ? 查看", len(w))
	}
}

// configSave captures an independent snapshot on the main thread. Its returned
// worker shares the write lock with every config save, including folder init.
func (a *app) configSave() func() error {
	if a.dataDir == "" {
		return func() error { return nil }
	}
	a.configSeq++
	seq, dir, cfg := a.configSeq, a.dataDir, a.cfg
	cfg.Agents = maps.Clone(cfg.Agents)
	cfg.ContextMenu = slices.Clone(cfg.ContextMenu)
	cfg.BlankMenu = slices.Clone(cfg.BlankMenu)
	cfg.Workspaces = slices.Clone(cfg.Workspaces)
	cfg.Commands = slices.Clone(cfg.Commands)
	for i := range cfg.Commands {
		cfg.Commands[i].Keys = slices.Clone(cfg.Commands[i].Keys)
	}
	return func() error {
		a.configMu.Lock()
		defer a.configMu.Unlock()
		if seq <= a.configWritten {
			return nil
		}
		if err := saveConfig(dir, cfg); err != nil {
			return err
		}
		a.configWritten = seq
		return nil
	}
}

// saveConfig writes config.json off the main thread.
func (a *app) saveConfig() {
	if a.dataDir == "" {
		return
	}
	save, post := a.configSave(), a.post
	a.async(func() {
		if err := save(); err != nil {
			post(func() { a.status = "无法保存 config.json：" + err.Error() })
		}
	})
}

// readOnly refuses a change inside an archive, saying so.
func (a *app) readOnly(ts *tabState) bool {
	if vfs.InZip(ts.t.Dir) {
		a.status = "压缩包内只读：可以复制（" + a.firstKey("copy") + "）后到别处粘贴来解压"
		return true
	}
	return false
}

func (a *app) pane() *paneState { return a.panes[a.active] }
func (a *app) tab() *tabState   { return a.pane().tab() }

// visiblePanes is how many panes the layout shows.
func (a *app) visiblePanes() int {
	if a.layout == layoutDual || a.layout == layoutRows {
		return 2
	}
	return 1
}

// load reads the tab's folder off the main thread. The cursor stays on the
// item it was on, or goes to the one the tab asked to focus.
func (a *app) load(ts *tabState) {
	ts.seq++
	seq, dir := ts.seq, ts.t.Dir
	keep := ts.t.Focus
	if e, ok := ts.cursorEntry(); ok && keep == "" {
		keep = e.Name
	}
	findGit := !vfs.InZip(dir) && !a.gitOff
	a.async(func() {
		es, err := vfs.ReadDir(dir)
		root := ""
		if findGit && err == nil {
			root, _ = git.FindRoot(dir)
		}
		a.post(func() {
			if ts.seq != seq || ts.t.Dir != dir {
				return
			}
			// Each reading, including one a change set off, refreshes git.
			ts.gitRoot = root
			a.refreshGit(root)
			ts.loaded = true
			ts.t.Focus = ""
			if err != nil {
				ts.err = err.Error()
				ts.t.SetEntries(nil)
				ts.cursor = -1
				return
			}
			ts.err = ""
			ts.t.SetEntries(es)
			ts.cursor = -1
			if keep != "" {
				ts.cursor = ts.t.Index(keep)
			}
			if ts.cursor < 0 && len(ts.t.Entries()) > 0 {
				ts.cursor = 0
			}
			if ts.cursor >= 0 {
				ts.list.ScrollIntoView(ts.cursor)
			}
		})
	})
}

// syncWatches watches the folders on screen and nothing else.
func (a *app) syncWatches() {
	for pi, p := range a.panes {
		ts, visible := p.ts, pi < a.visiblePanes()
		if !visible || ts.watched != ts.t.Dir {
			if ts.stop != nil {
				ts.stop()
				ts.stop, ts.watched = nil, ""
			}
		}
		if visible && ts.stop == nil && ts.loaded && ts.err == "" && !vfs.InZip(ts.t.Dir) {
			ts.watched = ts.t.Dir
			ts.stop = a.sys.Watch(ts.t.Dir, func() { a.post(func() { a.load(ts) }) })
		}
	}
}

func (a *app) afterNav(ts *tabState) {
	ts.cursor = -1
	ts.sel.Clear()
	ts.loaded = false
	ts.t.SetEntries(nil)
	a.filterOn, a.filterText = false, ""
	a.load(ts)
	a.wantFocus = true
}

func (a *app) navigate(ts *tabState, dir string) {
	if ts.t.Go(dir) {
		a.afterNav(ts)
	}
}

// openEntry opens e as Finder does: a folder is entered, an app (a .app
// bundle, a folder on disk) runs, an archive is browsed, a file opens
// with its default app.
func (a *app) openEntry(ts *tabState, e vfs.Entry) {
	switch {
	case e.IsDir && isApp(e.Name) && !vfs.InZip(e.Path):
		if err := a.sys.Open(e.Path); err != nil {
			a.status = "无法打开：" + err.Error()
		}
	case e.IsDir:
		a.navigate(ts, e.Path)
	case vfs.IsZipName(e.Name) && !vfs.InZip(e.Path):
		a.navigate(ts, vfs.ZipRoot(e.Path)) // browse the archive as a folder
	case vfs.InZip(e.Path):
		a.openFromZip(e)
	default:
		if err := a.sys.Open(e.Path); err != nil {
			a.status = "无法打开：" + err.Error()
		}
	}
}

// isApp tells an app bundle by its name, as Finder runs it on a double
// click instead of entering it.
func isApp(name string) bool { return strings.EqualFold(filepath.Ext(name), ".app") }

// openFromZip extracts an archive's file to the cache, then opens it with
// its default app. Each archive has its own cache folder; opening again
// extracts afresh.
func (a *app) openFromZip(e vfs.Entry) {
	archive, inner, _ := vfs.SplitZip(e.Path)
	base := a.cacheDir
	if base == "" {
		base = os.TempDir()
	}
	h := fnv.New64a()
	h.Write([]byte(archive))
	dst := filepath.Join(base, "zip", fmt.Sprintf("%x", h.Sum64()), filepath.FromSlash(inner))
	a.status = "正在解压 " + e.Name + "…"
	a.async(func() {
		os.RemoveAll(dst)
		err := os.MkdirAll(filepath.Dir(dst), 0o755)
		if err == nil {
			err = vfs.Extract(e.Path, dst)
		}
		if err == nil {
			err = a.sys.Open(dst)
		}
		a.post(func() {
			if err != nil {
				a.status = "无法打开：" + err.Error()
			} else {
				a.status = "已解压并打开 " + e.Name
			}
		})
	})
}

// goTyped goes where the address bar says: a folder, or a file's folder
// with the file chosen.
func (a *app) goTyped(ts *tabState, typed string) {
	p := filepath.Clean(vfs.ExpandHome(strings.TrimSpace(typed)))
	if !filepath.IsAbs(p) {
		p = filepath.Join(ts.t.Dir, p)
	}
	st, err := vfs.Stat(p)
	if err != nil {
		a.status = "找不到：" + p
		return
	}
	if st.IsDir {
		a.navigate(ts, p)
		return
	}
	if ts.t.Go(filepath.Dir(p)) {
		ts.t.Focus = filepath.Base(p)
		a.afterNav(ts)
	} else if i := ts.t.Index(filepath.Base(p)); i >= 0 {
		ts.cursor = i
	}
}

func (a *app) reloadVisible() {
	for pi := 0; pi < a.visiblePanes(); pi++ {
		a.load(a.panes[pi].tab())
	}
}

// paste pastes into dst, ts's folder or one a side column shows.
func (a *app) paste(ts *tabState, dst string) {
	if len(a.clip) == 0 {
		a.status = "没有复制或剪切的项目"
		return
	}
	srcs, cut := a.clip, a.clipCut
	verb := "复制"
	if cut {
		verb = "移动"
	}
	a.status = fmt.Sprintf("正在%s %d 项…", verb, len(srcs))
	a.async(func() {
		prog := func(done, total int, _ string) {
			a.post(func() { a.status = fmt.Sprintf("正在%s %d/%d", verb, done, total) })
		}
		var out []string
		var err error
		if cut {
			out, err = fsops.Move(srcs, dst, prog)
		} else {
			out, err = fsops.Copy(srcs, dst, prog)
		}
		a.post(func() {
			if err != nil {
				a.status = verb + "失败：" + err.Error()
			} else {
				a.status = fmt.Sprintf("已%s %d 项", verb, len(out))
			}
			if cut {
				a.clip = nil
			}
			if len(out) > 0 && ts.t.Dir == dst {
				ts.t.Focus = filepath.Base(out[len(out)-1])
			}
			a.refreshLists()
		})
	})
}

func (a *app) trash(ts *tabState) {
	ps := paths(ts.chosen())
	if len(ps) == 0 {
		return
	}
	// Keep the cursor near where it was: on the next row not trashed.
	if ts.cursor >= 0 {
		es := ts.t.Entries()
		gone := map[string]bool{}
		for _, p := range ps {
			gone[p] = true
		}
		for i := ts.cursor; i < len(es); i++ {
			if !gone[es[i].Path] {
				ts.t.Focus = es[i].Name
				break
			}
		}
	}
	a.async(func() {
		var failed error
		n := 0
		for _, p := range ps {
			if err := a.sys.Trash(p); err != nil {
				failed = err
				continue
			}
			n++
		}
		a.post(func() {
			a.status = fmt.Sprintf("已移到废纸篓 %d 项", n)
			if failed != nil {
				a.status += "；失败：" + failed.Error()
			}
			ts.sel.Clear()
			a.load(ts)
		})
	})
}

func (a *app) openDialog(kind int, ts *tabState, text, target string) {
	a.dlg, a.dlgOpen, a.dlgText, a.dlgTarget, a.dlgTab, a.dlgDir = kind, true, text, target, ts, ""
	if kind == dlgNewFolder {
		a.dlgFolder = a.folderOptions()
	}
}

func (a *app) submitDialog() {
	ts, name := a.dlgTab, strings.TrimSpace(a.dlgText)
	a.dlgOpen = false
	a.wantFocus = true
	if a.dlg == dlgSettings {
		a.cfg.Terminal, a.cfg.Editor = a.setTerm, a.setEditor
		a.cfg.ConfirmTrash = a.setConfirm
		a.cfg.NewFolderGit, a.cfg.NewFolderAgentFiles, a.cfg.NewFolderSkills = a.setFolder.Git, a.setFolder.AgentFiles, a.setFolder.Skills
		a.status = fmt.Sprintf("终端：%s，编辑器：%s", a.cfg.Terminal, a.cfg.Editor)
		a.saveConfig()
		return
	}
	if a.dlg == dlgWsRename {
		a.renameWorkspace(a.dlgTarget, name)
		return
	}
	if ts == nil || name == "" {
		return
	}
	dir := ts.t.Dir
	if a.dlgDir != "" {
		dir = a.dlgDir
	}
	opts := a.dlgFolder
	opts.AgentID = a.folderOptions().AgentID
	var p string
	var err error
	switch a.dlg {
	case dlgRename:
		p, err = fsops.Rename(a.dlgTarget, name)
	case dlgNewFile:
		var body []byte
		if a.dlgTpl != blankTemplate && a.dlgTpl != "" {
			if body, err = templates.Read(a.tplDir, a.dlgTpl); err != nil {
				break
			}
		}
		p, err = fsops.NewFile(dir, name, body)
	case dlgNewFolder:
		p, err = fsops.NewFolder(dir, name)
	}
	if err != nil {
		a.status = "失败：" + err.Error()
		return
	}
	if dir != ts.t.Dir { // in a side column: read those again
		a.refreshLists()
		if a.dlg == dlgNewFolder {
			a.initializeFolder(p, opts)
		}
		return
	}
	ts.t.Focus = filepath.Base(p)
	ts.sel.Clear()
	a.load(ts)
	if a.dlg == dlgNewFolder {
		a.initializeFolder(p, opts)
	}
}

// newFileDialog asks for the new file's name and template. The templates
// are listed off the main thread; until they arrive, it offers a blank file.
func (a *app) newFileDialog(ts *tabState) {
	a.openDialog(dlgNewFile, ts, "未命名.txt", "")
	a.dlgTpls, a.dlgTpl, a.dlgTplSeen = nil, blankTemplate, blankTemplate
	dir := a.tplDir
	if dir == "" {
		return
	}
	a.async(func() {
		names := templates.List(dir)
		a.post(func() { a.dlgTpls = append([]string{blankTemplate}, names...) })
	})
}

// templateChosen names the new file after the template just chosen.
func (a *app) templateChosen() {
	if a.dlgTpl == a.dlgTplSeen {
		return
	}
	a.dlgTplSeen = a.dlgTpl
	if a.dlgTpl == blankTemplate {
		a.dlgText = "未命名.txt"
	} else {
		a.dlgText = a.dlgTpl
	}
}

// runCommand runs the user's shell command id in the tab's folder, off
// the main thread; a failure shows in the status bar.
func (a *app) runCommand(ts *tabState, id string) {
	var cm command
	for _, x := range a.cfg.Commands {
		if cmdPrefix+x.ID == id {
			cm = x
		}
	}
	if cm.Command == "" {
		return
	}
	title := a.actionTitle(id)
	sel := paths(ts.chosen())
	if vfs.InZip(ts.t.Dir) && strings.Contains(cm.Command, "{selection}") {
		a.status = "压缩包内的项目不能交给命令「" + title + "」"
		return
	}
	dir := vfs.RealDir(ts.t.Dir)
	line := shellcmd.Expand(cm.Command, dir, sel)
	a.status = "正在运行「" + title + "」…"
	a.async(func() {
		out, err := a.sys.RunShell(dir, line)
		a.post(func() {
			if err != nil {
				msg := err.Error()
				if out != "" {
					lines := strings.Split(out, "\n")
					msg += "：" + lines[len(lines)-1]
				}
				a.status = "命令「" + title + "」失败（" + msg + "）"
				return
			}
			a.status = "已运行「" + title + "」"
			a.reloadVisible()
		})
	})
}

// openPath shows a folder the system asked the app to open (Dock, "open
// -a", the default file viewer), or a file in its folder, in the active
// pane: goFinder has one window and no tabs.
func (a *app) openPath(path string) {
	st, err := os.Stat(path)
	if err != nil {
		a.status = "找不到：" + path
		return
	}
	dir := path
	if !st.IsDir() {
		dir = filepath.Dir(path)
	}
	ts := a.tab()
	moved := ts.t.Go(dir)
	if !st.IsDir() {
		ts.t.Focus = filepath.Base(path)
	}
	if moved || !st.IsDir() {
		a.afterNav(ts)
	}
}

// quickLook shows the chosen items (else the one under the cursor) with
// the system's Quick Look, ending the one already open first. Files inside
// an archive are extracted to the cache first.
func (a *app) quickLook(ts *tabState) {
	a.stopQuickLook()
	ps := paths(ts.chosen())
	if len(ps) == 0 {
		e, ok := ts.cursorEntry()
		if !ok {
			return
		}
		ps = []string{e.Path}
	}
	a.qlSeq++
	seq := a.qlSeq
	a.async(func() {
		out := make([]string, 0, len(ps))
		for _, p := range ps {
			if vfs.InZip(p) {
				dst := a.zipCacheFile(p)
				os.RemoveAll(dst)
				if err := os.MkdirAll(filepath.Dir(dst), 0o755); err == nil {
					err = vfs.Extract(p, dst)
					if err != nil {
						continue
					}
				}
				p = dst
			}
			out = append(out, p)
		}
		a.post(func() {
			if seq != a.qlSeq || len(out) == 0 {
				return // pressed again meanwhile
			}
			a.qlStop = a.sys.QuickLook(out)
		})
	})
}

// stopQuickLook ends the open Quick Look, if any, and any still starting.
func (a *app) stopQuickLook() {
	a.qlSeq++
	if a.qlStop != nil {
		a.qlStop()
		a.qlStop = nil
	}
}

// setFileViewer makes this app, or Finder when off, the folder viewer of
// macOS. It takes effect after logging out or restarting Finder.
func (a *app) setFileViewer(on bool) {
	id := ""
	if on {
		id = a.bundleID
	}
	a.async(func() {
		err := a.sys.SetFileViewer(id)
		cur, _ := a.sys.FileViewer()
		a.post(func() {
			a.setViewer = cur
			switch {
			case err != nil:
				a.status = "无法更改默认文件浏览器：" + err.Error()
			case on:
				a.status = "已设为默认文件浏览器（注销重新登录或重启 Finder 后生效）"
			default:
				a.status = "已恢复 Finder 为默认文件浏览器（注销重新登录或重启 Finder 后生效）"
			}
		})
	})
}

// settingsDialog chooses the terminal and editor among the installed
// apps, which are listed off the main thread.
func (a *app) settingsDialog() {
	a.openDialog(dlgSettings, nil, "", "")
	a.setPage, a.recording = 0, ""
	a.setTerm, a.setEditor = a.cfg.Terminal, a.cfg.Editor
	a.syncAgentRows()
	a.setConfirm = a.cfg.ConfirmTrash
	a.setFolder = a.folderOptions()
	a.setApps, a.setLoading = nil, true
	a.async(func() {
		apps, err := a.sys.InstalledApps()
		id := a.sys.BundleID()
		viewer, _ := a.sys.FileViewer()
		disk := a.sys.FullDiskAccess()
		a.post(func() {
			a.fullDisk = disk
			a.setLoading = false
			a.setApps = apps
			a.bundleID, a.setViewer = id, viewer
			if err != nil {
				a.status = "无法列出已安装的应用：" + err.Error()
			}
		})
	})
}

func (a *app) setLayout(l int) {
	a.layout = l
	if a.active >= a.visiblePanes() {
		a.active = 0
	}
	a.wantFocus = true
}

func (a *app) goHomeSub(sub string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	a.navigate(a.tab(), filepath.Join(home, sub))
}

// moveCursor matches a list's plain Up/Down choice. ListState's private
// Shift anchor cannot be reset here; native Shift+arrows keep that anchor.
func (a *app) moveCursor(ts *tabState, delta int) {
	if a.modalOpen() || a.focusZone == "sidebar" {
		return
	}
	es := ts.t.Entries()
	if len(es) == 0 {
		return
	}
	to := 0
	if ts.cursor >= 0 && ts.cursor < len(es) {
		to = ts.cursor + delta
		if to < 0 || to >= len(es) {
			ts.list.ScrollIntoView(ts.cursor)
			return
		}
	}
	ts.cursor = to
	ts.sel.Clear()
	ts.sel.Add(es[to].Path)
	ts.list.ScrollIntoView(to)
}

// run runs the action id on the active tab.
func (a *app) run(id string) {
	p, ts := a.pane(), a.tab()
	if a.focusZone == "sidebar" {
		switch id {
		case "open": // → or ↩ in the sidebar: on to the files
			a.wantFocus = true
			return
		case "up":
			return
		}
	}
	switch id {
	case "cursor.up":
		a.moveCursor(ts, -1)
	case "cursor.down":
		a.moveCursor(ts, 1)
	case "open":
		if e, ok := ts.cursorEntry(); ok {
			a.openEntry(ts, e)
		}
	case "up":
		if ts.t.Up() {
			a.afterNav(ts)
		}
	case "back":
		if ts.t.Back() {
			a.afterNav(ts)
		}
	case "forward":
		if ts.t.Forward() {
			a.afterNav(ts)
		}
	case "refresh":
		a.load(ts)
	case "hidden":
		ts.t.ToggleHidden()
		if ts.cursor >= len(ts.t.Entries()) {
			ts.cursor = len(ts.t.Entries()) - 1
		}
	case "addr":
		p.addrEditing = true
		p.addrText = ts.t.Dir
		if !strings.HasSuffix(p.addrText, "/") {
			p.addrText += "/"
		}
	case "go.start":
		// The folder 启动时从这里开始 chose for this pane, while it is
		// there; else ~. The two panes keep their own.
		if d := *a.startDirOf(a.active); d != "" {
			if fi, err := os.Stat(d); err == nil && fi.IsDir() {
				a.navigate(ts, d)
				break
			}
		}
		a.goHomeSub("")
	case "go.home":
		a.goHomeSub("")
	case "go.desktop":
		a.goHomeSub("Desktop")
	case "go.downloads":
		a.goHomeSub("Downloads")
	case "go.documents":
		a.goHomeSub("Documents")
	case "go.apps":
		a.navigate(ts, "/Applications")

	case "quicklook":
		a.quickLook(ts)
	case "select.all":
		for _, e := range ts.t.Entries() {
			ts.sel.Add(e.Path)
		}
	case "select.none":
		// Esc: first clears a filter that is on, then the selection.
		if ts.t.Filter != "" {
			a.filterOn, a.filterText = false, ""
			a.setFilter(ts, "")
		} else {
			ts.sel.Clear()
		}

	case "copy", "cut":
		if id == "cut" && a.readOnly(ts) {
			return
		}
		a.clip = paths(ts.chosen())
		a.clipCut = id == "cut"
		verb := map[bool]string{false: "已复制", true: "已剪切"}[a.clipCut]
		a.status = fmt.Sprintf("%s %d 项，到目标文件夹按 %s 粘贴", verb, len(a.clip), a.firstKey("paste"))
	case "paste":
		if !a.readOnly(ts) {
			a.paste(ts, ts.t.Dir)
		}
	case "trash":
		if !a.readOnly(ts) {
			a.askTrash(ts)
		}
	case "rename":
		if a.readOnly(ts) {
			return
		}
		if e, ok := ts.cursorEntry(); ok {
			a.openDialog(dlgRename, ts, e.Name, e.Path)
		}
	case "newfile":
		if !a.readOnly(ts) {
			a.popup("new")
		}
	case "sortmenu":
		a.popup("sort")
	case "filter":
		a.filterOn, a.filterText, a.filterEat = true, ts.t.Filter, true
	case "newfolder":
		if !a.readOnly(ts) {
			a.openDialog(dlgNewFolder, ts, "未命名文件夹", "")
		}
	case "copypath":
		ps := paths(ts.chosen())
		if len(ps) == 0 {
			ps = []string{ts.t.Dir}
		}
		a.copyPathText(strings.Join(ps, "\n"))
	case "reveal":
		if e, ok := ts.cursorEntry(); ok {
			p := e.Path
			if archive, _, in := vfs.SplitZip(p); in {
				p = archive // Finder cannot look inside: show the archive
			}
			a.sys.Reveal(p)
		}
	case "terminal", "editor":
		a.openFolderWith(id, ts.t.Dir)
	case "agent":
		if it, ok := a.defaultAgentItem(); ok {
			a.openAgent(it, a.agentDir(ts))
		} else {
			a.status = noAgentHint
		}

	case "ws.next":
		a.stepWorkspace(1)
	case "ws.prev":
		a.stepWorkspace(-1)
	case "pane.switch":
		// Tab: between the two panes, else between the sidebar and the
		// files; from the sidebar, back to the files.
		switch {
		case a.focusZone == "sidebar":
			a.wantFocus = true
		case a.visiblePanes() == 2:
			a.active ^= 1
			a.filterOn = false
			a.wantFocus = true
		case a.sb.show:
			a.filterOn = false
			a.wantSidebar, a.focusZone = true, "sidebar"
		}

	case "layout.dual":
		a.setLayout(layoutDual)
	case "layout.columns":
		a.setLayout(layoutColumns)
	case "layout.rows":
		a.setLayout(layoutRows)
	case "preview":
		a.showPreview = !a.showPreview
		a.wantFocus = true // the table moves into a split: keep its focus

	case "settings":
		a.settingsDialog()

	case "sidebar":
		a.sb.show = !a.sb.show
		a.cfg.ShowSidebar = a.sb.show
		a.saveConfig()
		a.wantFocus = true
	case "fav.add":
		a.setFavorite(ts.t.Dir, true)
	case "fav.remove":
		a.setFavorite(ts.t.Dir, false)

	case "shelf":
		a.sh.show = !a.sh.show
		a.wantFocus = true
	case "shelf.add":
		a.shelve(paths(ts.chosen()))
	case "shelf.copy", "shelf.move":
		a.shelfTo(ts, id == "shelf.move")
	case "shelf.clear":
		a.sh.items = nil
		a.saveShelf()
	case "workspace":
		a.workspaceDialog()

	case "help":
		a.helpOpen, a.helpQuery = true, ""

	default:
		if strings.HasPrefix(id, cmdPrefix) {
			a.runCommand(ts, id)
			return
		}
		var n int
		if _, err := fmt.Sscanf(id, "ws.%d", &n); err == nil && n >= 1 {
			a.workspaceAt(n)
		}
	}
}

// popup opens the menu kind ("sort", "new") at the cursor row. The new
// menu's templates are listed off the main thread; until they come, it
// offers a blank file.
func (a *app) popup(kind string) {
	a.pop, a.popOpen = kind, true
	if kind != "new" {
		return
	}
	a.popTpls = nil
	if dir := a.tplDir; dir != "" {
		a.async(func() {
			names := templates.List(dir)
			a.post(func() { a.popTpls = names })
		})
	}
}

// popMenu is the menu popup opened: the sort columns, or a blank file and
// the templates, each going on to the name.
func (a *app) popMenu(m *ui.Menu) {
	ts := a.tab()
	switch a.pop {
	case "sort":
		for _, s := range sortOrder {
			if m.Item(s.title).Checked(ts.sort.Column == s.col).Chosen() {
				ts.sort.Column = s.col
				a.wantFocus = true
			}
		}
		m.Separator()
		if m.Item("降序").Checked(ts.sort.Descending).Chosen() {
			ts.sort.Descending = !ts.sort.Descending
			a.wantFocus = true
		}
	case "new":
		for _, name := range append([]string{blankTemplate}, a.popTpls...) {
			if m.Item(name).Chosen() {
				a.newFileDialog(ts)
				if name != blankTemplate {
					a.dlgTpl, a.dlgTplSeen, a.dlgText = name, name, name
				}
			}
		}
	}
}

// setFilter filters ts's listing by q. The cursor stays on the item it
// was on; when that one is filtered out, it goes to the first.
func (a *app) setFilter(ts *tabState, q string) {
	name := ""
	if e, ok := ts.cursorEntry(); ok {
		name = e.Name
	}
	ts.t.SetFilter(q)
	if q != "" {
		ts.sel.Clear() // selected items may have been filtered out of sight
	}
	i := ts.t.Index(name)
	if i < 0 {
		i = 0
	}
	if len(ts.t.Entries()) == 0 {
		i = -1
	}
	ts.cursor = i
	if i >= 0 {
		ts.list.ScrollIntoView(i)
	}
}

// filterBox is the filter's input under the address bar: it filters as
// it is typed; ↩ keeps the filter and hands the keyboard back to the
// list, Esc clears it.
func (a *app) filterBox(c *ui.Context, ts *tabState) {
	if a.filterEat && a.filterText != ts.t.Filter {
		// The "f" that opened the box may arrive as its first text.
		a.filterEat = false
		if a.filterText == ts.t.Filter+"f" {
			a.filterText = ts.t.Filter
		}
	}
	if a.filterText != ts.t.Filter {
		a.setFilter(ts, a.filterText)
	}
	fb := ui.Row(c.Key("filterbox")).Margin(0, 6, 4, 6).Gap(px("space-1.5")).Padding(0, px("space-2")).Height(capsuleH).AlignItems(ui.Center).
		Background(tok(c, "surface-100")).Radius(px("radius-pill")).Font(a.listFont()).FontSize(float32(a.listSize()))
	fb.Children(func() {
		ui.Text(c, "过滤").Font("").FontSize(12).TextColor(tok(c, "muted")).Shrink(0)
		in := ui.TextInputBase(c.Key("filter"), &a.filterText).Label("过滤").Placeholder("名称含…（有大写字母时区分大小写）").AutoFocus().Grow(1)
		ui.Text(c, fmt.Sprintf("%d 项", len(ts.t.Entries()))).Font("").FontSize(11).TextColor(tok(c, "muted")).SingleLine().Shrink(0)
		switch {
		case in.Submitted():
			a.filterOn = false
			a.wantFocus = true
		case in.Shortcut(0, ui.KeyEscape):
			a.filterOn, a.filterText = false, ""
			a.setFilter(ts, "")
			a.wantFocus = true
		}
	})
	a.dbg(c, fb, "addr.filter", a.active)
}

// copyPathText puts paths (one per line) on the clipboard and says so.
// copied keeps the last text, for the tests: the Tester has no clipboard.
func (a *app) copyPathText(s string) {
	if a.hasSvc {
		a.svc.WriteClipboard(s)
	}
	a.copied = s
	a.status = "已复制路径"
}
