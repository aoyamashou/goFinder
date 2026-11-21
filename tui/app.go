package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gofinder/internal/fsops"
	"gofinder/internal/keymap"
	"gofinder/internal/pane"
	"gofinder/internal/vfs"
)

// Options configures a gf TUI session.
type Options struct {
	StartDir     string
	ConfigDir    string
	KeymapPath   string
	VimOverride  *bool
	Hidden       *bool
	NerdOverride *bool
	System       System
	Now          func() time.Time
}

type inputMode int

const (
	modeNormal inputMode = iota
	modeFilter
	modeGoto
	modeRename
	modeNewFolder
	modeNewFile
	modeSort
	modeConfirmTrash
	modeHelp
)

type (
	dirLoadedMsg struct {
		seq     int
		dir     string
		entries []vfs.Entry
		err     error
		keep    string
	}
	parentLoadedMsg struct {
		seq     int
		dir     string
		entries []vfs.Entry
		err     error
	}
	previewLoadedMsg struct {
		seq int
		pv  previewState
	}
	fileOpDoneMsg struct {
		status string
		focus  string
		err    error
		reload bool
	}
	editorDoneMsg struct {
		err error
	}
	tapTimeoutMsg struct {
		seq int
	}
	dirChangedMsg struct{}
)

type sortChoice struct {
	label string
	key   pane.SortKey
	desc  bool
}

var sortChoices = []sortChoice{
	{"名称（升序 A→Z）", pane.ByName, false},
	{"名称（降序 Z→A）", pane.ByName, true},
	{"修改时间（最新在前）", pane.ByDate, true},
	{"修改时间（最旧在前）", pane.ByDate, false},
	{"大小（从大到小）", pane.BySize, true},
	{"大小（从小到大）", pane.BySize, false},
	{"类型（升序）", pane.ByKind, false},
	{"类型（降序）", pane.ByKind, true},
}

// Model is the Bubble Tea model for gf.
type Model struct {
	sys  System
	cfg  Config
	km   *keymap.Keymap
	th   theme
	nerd bool
	now  func() time.Time

	startDir string
	tab      *pane.Tab
	cursor   int
	scroll   int
	selected map[string]bool
	loadSeq  int
	loadErr  string

	parentDir     string
	parentEntries []vfs.Entry
	parentSeq     int

	pv previewState

	width  int
	height int

	clip    []string
	clipCut bool
	status  string
	warns   []string

	taps   keymap.Taps
	tapSeq int

	mode         inputMode
	input        textinput.Model
	renameTarget string
	trashTargets []string
	completions  []string
	compIdx      int
	sortCursor   int
	helpScroll   int

	watchDir  string
	watchStop func()
	watchCh   chan struct{}

	writeCwd  bool
	lastClick struct {
		at  time.Time
		row int
	}
}

// New creates a new gf TUI Model.
func New(opts Options) *Model {
	cfgDir := opts.ConfigDir
	if cfgDir == "" {
		cfgDir = DefaultConfigDir()
	}
	cfg := LoadConfig(cfgDir)
	if opts.VimOverride != nil {
		cfg.VimKeys = *opts.VimOverride
	}
	if opts.Hidden != nil {
		cfg.ShowHidden = *opts.Hidden
	}
	nerd := cfg.WantNerdFont()
	if opts.NerdOverride != nil {
		nerd = *opts.NerdOverride
	}
	kmPath := opts.KeymapPath
	if kmPath == "" && cfgDir != "" {
		kmPath = filepath.Join(cfgDir, "keymap.json")
	}
	km, warns := buildKeymap(cfg.VimKeys, kmPath)

	startDir := opts.StartDir
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(startDir); err == nil {
		startDir = abs
	}
	startDir = filepath.Clean(startDir)

	sys := opts.System
	if sys == nil {
		sys = realSystem{}
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	tab := pane.New(startDir)
	tab.ShowHidden = cfg.ShowHidden

	ti := textinput.New()
	ti.CharLimit = 512

	m := &Model{
		sys:      sys,
		cfg:      cfg,
		km:       km,
		th:       newTheme(lipgloss.HasDarkBackground()),
		nerd:     nerd,
		now:      nowFn,
		startDir: startDir,
		tab:      tab,
		cursor:   -1,
		selected: map[string]bool{},
		width:    100,
		height:   30,
		warns:    warns,
		input:    ti,
		watchCh:  make(chan struct{}, 1),
	}
	if len(warns) > 0 {
		m.status = warns[0]
	}
	return m
}

// Close stops background filesystem watchers cleanly.
func (m *Model) Close() {
	if m.watchStop != nil {
		m.watchStop()
		m.watchStop = nil
		m.watchDir = ""
	}
}

// FinalDir returns the directory to cd into on exit, or "" when keeping original cwd.
func (m *Model) FinalDir() string {
	if !m.writeCwd {
		return ""
	}
	return m.tab.Dir
}

// Init kicks off initial directory, parent, and watcher commands.
func (m *Model) Init() tea.Cmd {
	m.syncWatch()
	return tea.Batch(
		m.loadCurrentCmd(""),
		m.loadParentCmd(),
		waitWatchCmd(m.watchCh),
	)
}

func waitWatchCmd(ch <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-ch
		return dirChangedMsg{}
	}
}

func (m *Model) syncWatch() {
	dir := m.tab.Dir
	if m.watchDir == dir && m.watchStop != nil {
		return
	}
	if m.watchStop != nil {
		m.watchStop()
		m.watchStop = nil
	}
	m.watchDir = dir
	ch := m.watchCh
	m.watchStop = m.sys.Watch(dir, func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	})
}

// Update handles Bubble Tea messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(msg.Width, 20)
		m.height = max(msg.Height, 6)
		m.ensureCursorVisible()
		if m.pv.kind == pvImage {
			return m, m.reloadPreviewCmd()
		}
		return m, nil

	case dirLoadedMsg:
		if msg.seq != m.loadSeq || msg.dir != m.tab.Dir {
			return m, nil
		}
		if msg.err != nil {
			m.loadErr = msg.err.Error()
			m.tab.SetEntries(nil)
			m.cursor = -1
			m.pv = previewState{kind: pvNone}
			return m, nil
		}
		m.loadErr = ""
		keep := msg.keep
		if m.tab.Focus != "" {
			keep = m.tab.Focus
			m.tab.Focus = ""
		}
		m.tab.SetEntries(msg.entries)
		m.pruneSelection()
		m.restoreCursor(keep)
		m.ensureCursorVisible()
		m.syncWatch()
		return m, m.ensurePreviewCmd()

	case parentLoadedMsg:
		if msg.seq != m.parentSeq || msg.dir != m.parentDir {
			return m, nil
		}
		if msg.err != nil {
			m.parentEntries = nil
			return m, nil
		}
		out := make([]vfs.Entry, 0, len(msg.entries))
		for _, e := range msg.entries {
			if !m.tab.ShowHidden && e.Hidden() {
				continue
			}
			out = append(out, e)
		}
		pane.SortEntries(out, m.tab.Sort, m.tab.Desc)
		m.parentEntries = out
		return m, nil

	case previewLoadedMsg:
		if msg.seq != m.pv.seq {
			return m, nil
		}
		cur, ok := m.cursorEntry()
		if !ok || cur.Path != msg.pv.path {
			return m, nil
		}
		m.pv = msg.pv
		return m, nil

	case fileOpDoneMsg:
		if msg.err != nil {
			m.status = "错误：" + msg.err.Error()
		} else if msg.status != "" {
			m.status = msg.status
		}
		if msg.focus != "" {
			m.tab.Focus = msg.focus
		}
		if msg.reload {
			return m, tea.Batch(m.loadCurrentCmd(m.cursorName()), m.loadParentCmd())
		}
		return m, nil

	case editorDoneMsg:
		if msg.err != nil {
			m.status = "编辑器退出失败：" + msg.err.Error()
		}
		return m, tea.Batch(m.loadCurrentCmd(m.cursorName()), m.reloadPreviewCmd())

	case dirChangedMsg:
		return m, tea.Batch(
			m.loadCurrentCmd(m.cursorName()),
			m.loadParentCmd(),
			waitWatchCmd(m.watchCh),
		)

	case tapTimeoutMsg:
		if msg.seq != m.tapSeq {
			return m, nil
		}
		if id, due := m.km.Due(&m.taps, m.now()); due && id != "" {
			return m, m.runAction(id)
		}
		return m, nil

	case tea.MouseMsg:
		return m, m.handleMouse(msg)

	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyCtrlC {
		m.Close()
		m.writeCwd = false
		return tea.Quit
	}
	switch m.mode {
	case modeNormal:
		return m.handleNormalKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeGoto:
		return m.handleGotoKey(msg)
	case modeRename:
		return m.handleRenameKey(msg)
	case modeNewFolder:
		return m.handleNewItemKey(msg, true)
	case modeNewFile:
		return m.handleNewItemKey(msg, false)
	case modeSort:
		return m.handleSortKey(msg)
	case modeConfirmTrash:
		return m.handleConfirmTrashKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	}
	return nil
}

func (m *Model) handleNormalKey(msg tea.KeyMsg) tea.Cmd {
	ch, ok := keyMsgToChord(msg)
	if !ok {
		return nil
	}
	acts := m.km.Press(&m.taps, ch, m.now())
	var cmds []tea.Cmd
	for _, id := range acts {
		if cmd := m.runAction(id); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if wait, pending := m.taps.Waiting(m.now()); pending {
		m.tapSeq++
		seq := m.tapSeq
		cmds = append(cmds, tea.Tick(wait, func(time.Time) tea.Msg {
			return tapTimeoutMsg{seq: seq}
		}))
	}
	return tea.Batch(cmds...)
}

func (m *Model) runAction(id string) tea.Cmd {
	switch id {
	case "cursor.up":
		return m.moveCursor(-1)
	case "cursor.down":
		return m.moveCursor(1)
	case "cursor.top":
		return m.setCursor(0)
	case "cursor.bottom":
		return m.setCursor(len(m.tab.Entries()) - 1)
	case "open":
		return m.openCurrent()
	case "up":
		if m.tab.Up() {
			return m.afterNav()
		}
	case "back":
		if m.tab.Back() {
			return m.afterNav()
		}
	case "forward":
		if m.tab.Forward() {
			return m.afterNav()
		}
	case "refresh":
		m.status = "已刷新"
		return tea.Batch(m.loadCurrentCmd(m.cursorName()), m.loadParentCmd(), m.reloadPreviewCmd())
	case "hidden":
		keep := m.cursorName()
		m.tab.ToggleHidden()
		m.restoreCursor(keep)
		m.ensureCursorVisible()
		if m.tab.ShowHidden {
			m.status = "已显示隐藏文件"
		} else {
			m.status = "已隐藏点文件"
		}
		return tea.Batch(m.loadParentCmd(), m.reloadPreviewCmd())
	case "sortmenu":
		m.mode = modeSort
		m.sortCursor = 0
		for i, c := range sortChoices {
			if c.key == m.tab.Sort && c.desc == m.tab.Desc {
				m.sortCursor = i
				break
			}
		}
	case "addr":
		m.startInput(modeGoto, "前往: ", m.tab.Dir+"/")
		m.completions = nil
		m.compIdx = 0
	case "go.start":
		return m.navigate(m.startDir)
	case "go.home":
		if home, err := os.UserHomeDir(); err == nil {
			return m.navigate(home)
		}
	case "filter":
		m.startInput(modeFilter, "过滤: ", m.tab.Filter)
	case "select.toggle":
		if e, ok := m.cursorEntry(); ok {
			if m.selected[e.Path] {
				delete(m.selected, e.Path)
			} else {
				m.selected[e.Path] = true
			}
			return m.moveCursor(1)
		}
	case "select.all":
		for _, e := range m.tab.Entries() {
			m.selected[e.Path] = true
		}
		m.status = fmt.Sprintf("已选 %d 项", len(m.selected))
	case "select.none":
		if len(m.selected) > 0 {
			clear(m.selected)
			m.status = "已取消选择"
		} else if m.tab.Filter != "" {
			keep := m.cursorName()
			m.tab.SetFilter("")
			m.restoreCursor(keep)
			m.ensureCursorVisible()
			m.status = "已清除过滤"
			return m.ensurePreviewCmd()
		}
	case "copy":
		ps := entryPaths(m.chosen())
		if len(ps) > 0 {
			m.clip = ps
			m.clipCut = false
			m.status = fmt.Sprintf("已复制 %d 项", len(ps))
		}
	case "copypath":
		ps := entryPaths(m.chosen())
		if len(ps) == 0 {
			ps = []string{m.tab.Dir}
		}
		text := strings.Join(ps, "\n")
		if err := m.sys.CopyText(text); err != nil {
			m.status = "复制路径失败：" + err.Error()
		} else {
			m.status = fmt.Sprintf("已复制 %d 条路径", len(ps))
		}
	case "cut":
		ps := entryPaths(m.chosen())
		if len(ps) > 0 {
			m.clip = ps
			m.clipCut = true
			m.status = fmt.Sprintf("已剪切 %d 项", len(ps))
		}
	case "paste":
		return m.pasteCmd()
	case "rename":
		if e, ok := m.cursorEntry(); ok {
			m.renameTarget = e.Path
			m.startInput(modeRename, "重命名: ", e.Name)
		}
	case "newfolder":
		m.startInput(modeNewFolder, "新建文件夹: ", "")
	case "newfile":
		m.startInput(modeNewFile, "新建文件: ", "")
	case "trash":
		ps := entryPaths(m.chosen())
		if len(ps) == 0 {
			return nil
		}
		if m.cfg.WantConfirmTrash() {
			m.trashTargets = ps
			m.mode = modeConfirmTrash
			return nil
		}
		return m.trashCmd(ps)
	case "editor":
		return m.openEditorCmd()
	case "sysopen":
		if e, ok := m.cursorEntry(); ok {
			if err := m.sys.Open(e.Path); err != nil {
				m.status = "打开失败：" + err.Error()
			} else {
				m.status = "已打开 " + e.Name
			}
		}
	case "preview.up":
		m.scrollPreview(-max(m.contentHeight()/2, 3))
	case "preview.down":
		m.scrollPreview(max(m.contentHeight()/2, 3))
	case "help":
		m.startInput(modeHelp, "搜索快捷键: ", "")
		m.helpScroll = 0
	case "quit":
		m.Close()
		m.writeCwd = true
		return tea.Quit
	case "quit.keep":
		m.Close()
		m.writeCwd = false
		return tea.Quit
	}
	return nil
}

func (m *Model) startInput(mode inputMode, prompt, val string) {
	m.mode = mode
	m.input.Prompt = prompt
	m.input.SetValue(val)
	m.input.CursorEnd()
	m.input.Focus()
}

func (m *Model) handleFilterKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		keep := m.cursorName()
		m.tab.SetFilter("")
		m.restoreCursor(keep)
		m.ensureCursorVisible()
		m.mode = modeNormal
		m.input.Blur()
		return m.ensurePreviewCmd()
	case tea.KeyEnter:
		m.mode = modeNormal
		m.input.Blur()
		return nil
	case tea.KeyUp:
		return m.moveCursor(-1)
	case tea.KeyDown:
		return m.moveCursor(1)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	keep := m.cursorName()
	m.tab.SetFilter(m.input.Value())
	m.restoreCursor(keep)
	m.ensureCursorVisible()
	return tea.Batch(cmd, m.ensurePreviewCmd())
}

func (m *Model) handleGotoKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input.Blur()
		m.completions = nil
		return nil
	case tea.KeyTab:
		val := m.input.Value()
		if len(m.completions) == 0 {
			m.completions = vfs.Complete(val, m.tab.ShowHidden)
			m.compIdx = 0
		}
		if len(m.completions) > 0 {
			m.input.SetValue(m.completions[m.compIdx])
			m.input.CursorEnd()
			m.compIdx = (m.compIdx + 1) % len(m.completions)
		}
		return nil
	case tea.KeyEnter:
		typed := strings.TrimSpace(m.input.Value())
		m.mode = modeNormal
		m.input.Blur()
		m.completions = nil
		return m.goTyped(typed)
	}
	m.completions = nil
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) handleRenameKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input.Blur()
		return nil
	case tea.KeyEnter:
		newName := strings.TrimSpace(m.input.Value())
		target := m.renameTarget
		m.mode = modeNormal
		m.input.Blur()
		if newName == "" || target == "" {
			return nil
		}
		return func() tea.Msg {
			dst, err := fsops.Rename(target, newName)
			if err != nil {
				return fileOpDoneMsg{err: err}
			}
			return fileOpDoneMsg{status: "已重命名为 " + filepath.Base(dst), focus: filepath.Base(dst), reload: true}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) handleNewItemKey(msg tea.KeyMsg, isDir bool) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input.Blur()
		return nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.input.Value())
		dir := m.tab.Dir
		m.mode = modeNormal
		m.input.Blur()
		if name == "" {
			return nil
		}
		if strings.ContainsRune(name, '/') {
			m.status = "名称不能包含 /"
			return nil
		}
		return func() tea.Msg {
			var dst string
			var err error
			if isDir {
				dst, err = fsops.NewFolder(dir, name)
			} else {
				dst, err = fsops.NewFile(dir, name, nil)
			}
			if err != nil {
				return fileOpDoneMsg{err: err}
			}
			return fileOpDoneMsg{status: "已新建 " + filepath.Base(dst), focus: filepath.Base(dst), reload: true}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) handleSortKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		return nil
	case tea.KeyUp:
		if m.sortCursor > 0 {
			m.sortCursor--
		}
		return nil
	case tea.KeyDown:
		if m.sortCursor < len(sortChoices)-1 {
			m.sortCursor++
		}
		return nil
	case tea.KeyEnter:
		c := sortChoices[m.sortCursor]
		keep := m.cursorName()
		m.tab.SetSort(c.key, c.desc)
		m.restoreCursor(keep)
		m.ensureCursorVisible()
		m.mode = modeNormal
		m.status = "排序：" + c.label
		return m.loadParentCmd()
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'k':
				if m.sortCursor > 0 {
					m.sortCursor--
				}
			case 'j':
				if m.sortCursor < len(sortChoices)-1 {
					m.sortCursor++
				}
			case 'q', ',':
				m.mode = modeNormal
			}
		}
	}
	return nil
}

func (m *Model) handleConfirmTrashKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.trashTargets = nil
		m.status = "已取消移到废纸篓"
		return nil
	case tea.KeyEnter:
		ps := slices.Clone(m.trashTargets)
		m.trashTargets = nil
		m.mode = modeNormal
		return m.trashCmd(ps)
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'y', 'Y':
				ps := slices.Clone(m.trashTargets)
				m.trashTargets = nil
				m.mode = modeNormal
				return m.trashCmd(ps)
			case 'n', 'N', 'q':
				m.trashTargets = nil
				m.mode = modeNormal
				m.status = "已取消移到废纸篓"
				return nil
			}
		}
	}
	return nil
}

func (m *Model) handleHelpKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input.Blur()
		return nil
	case tea.KeyUp:
		if m.helpScroll > 0 {
			m.helpScroll--
		}
		return nil
	case tea.KeyDown:
		m.helpScroll++
		return nil
	case tea.KeyPgUp:
		m.helpScroll = max(m.helpScroll-10, 0)
		return nil
	case tea.KeyPgDown:
		m.helpScroll += 10
		return nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && msg.Runes[0] == '?' && m.input.Value() == "" {
			m.mode = modeNormal
			m.input.Blur()
			return nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.helpScroll = 0
	return cmd
}

func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.mode != modeNormal {
		return nil
	}
	parentW, curW, _ := m.layoutWidths()
	previewStart := curW + 1
	if parentW > 0 {
		previewStart = parentW + 1 + curW + 1
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if msg.X >= previewStart {
			m.scrollPreview(-3)
			return nil
		}
		return m.moveCursor(-1)
	case tea.MouseButtonWheelDown:
		if msg.X >= previewStart {
			m.scrollPreview(3)
			return nil
		}
		return m.moveCursor(1)
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return nil
		}
		curStart := 0
		if parentW > 0 {
			curStart = parentW + 1
		}
		if msg.Y >= 1 && msg.Y <= m.contentHeight() && msg.X >= curStart && msg.X < curStart+curW {
			idx := m.scroll + (msg.Y - 1)
			if idx >= 0 && idx < len(m.tab.Entries()) {
				now := m.now()
				dbl := m.lastClick.row == idx && now.Sub(m.lastClick.at) < 400*time.Millisecond
				m.lastClick.row = idx
				m.lastClick.at = now
				cmd := m.setCursor(idx)
				if dbl {
					return tea.Batch(cmd, m.openCurrent())
				}
				return cmd
			}
		}
	}
	return nil
}

func (m *Model) moveCursor(delta int) tea.Cmd {
	n := len(m.tab.Entries())
	if n == 0 {
		m.cursor = -1
		return nil
	}
	next := m.cursor + delta
	if next < 0 {
		next = 0
	}
	if next >= n {
		next = n - 1
	}
	return m.setCursor(next)
}

func (m *Model) setCursor(idx int) tea.Cmd {
	n := len(m.tab.Entries())
	if n == 0 {
		m.cursor = -1
		m.pv = previewState{kind: pvNone}
		return nil
	}
	idx = max(0, min(idx, n-1))
	if idx == m.cursor && m.pv.path != "" {
		return nil
	}
	m.cursor = idx
	m.ensureCursorVisible()
	return m.ensurePreviewCmd()
}

func (m *Model) ensureCursorVisible() {
	h := m.contentHeight()
	if m.cursor < 0 || h <= 0 {
		m.scroll = 0
		return
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+h {
		m.scroll = m.cursor - h + 1
	}
	maxScroll := max(len(m.tab.Entries())-h, 0)
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *Model) scrollPreview(delta int) {
	if m.pv.kind != pvText && m.pv.kind != pvMeta {
		return
	}
	viewH := max(m.contentHeight()-1, 1)
	maxScroll := max(len(m.pv.lines)-viewH, 0)
	m.pv.scroll = max(0, min(m.pv.scroll+delta, maxScroll))
}

func (m *Model) openCurrent() tea.Cmd {
	e, ok := m.cursorEntry()
	if !ok {
		return nil
	}
	if e.IsDir {
		return m.navigate(e.Path)
	}
	return m.openEditorCmd()
}

func (m *Model) openEditorCmd() tea.Cmd {
	e, ok := m.cursorEntry()
	if !ok || e.IsDir {
		return nil
	}
	editor := strings.TrimSpace(m.cfg.Editor)
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("VISUAL"))
	}
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	cmd := exec.Command(parts[0], append(parts[1:], e.Path)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

func (m *Model) navigate(dir string) tea.Cmd {
	if m.tab.Go(dir) {
		return m.afterNav()
	}
	return nil
}

func (m *Model) goTyped(typed string) tea.Cmd {
	if typed == "" {
		return nil
	}
	p := filepath.Clean(vfs.ExpandHome(typed))
	if !filepath.IsAbs(p) {
		p = filepath.Join(m.tab.Dir, p)
	}
	st, err := os.Stat(p)
	if err != nil {
		m.status = "找不到：" + p
		return nil
	}
	if st.IsDir() {
		return m.navigate(p)
	}
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	if m.tab.Go(dir) {
		m.tab.Focus = base
		return m.afterNav()
	}
	if idx := m.tab.Index(base); idx >= 0 {
		return m.setCursor(idx)
	}
	return nil
}

func (m *Model) afterNav() tea.Cmd {
	m.cursor = -1
	m.scroll = 0
	clear(m.selected)
	m.tab.SetEntries(nil)
	m.pv = previewState{kind: pvNone}
	m.syncWatch()
	return tea.Batch(m.loadCurrentCmd(""), m.loadParentCmd())
}

func (m *Model) loadCurrentCmd(keep string) tea.Cmd {
	m.loadSeq++
	seq := m.loadSeq
	dir := m.tab.Dir
	return func() tea.Msg {
		es, err := vfs.ReadDir(dir)
		return dirLoadedMsg{seq: seq, dir: dir, entries: es, err: err, keep: keep}
	}
}

func (m *Model) loadParentCmd() tea.Cmd {
	parent := filepath.Dir(m.tab.Dir)
	if parent == m.tab.Dir {
		m.parentDir = ""
		m.parentEntries = nil
		return nil
	}
	m.parentSeq++
	seq := m.parentSeq
	m.parentDir = parent
	return func() tea.Msg {
		es, err := vfs.ReadDir(parent)
		return parentLoadedMsg{seq: seq, dir: parent, entries: es, err: err}
	}
}

func (m *Model) ensurePreviewCmd() tea.Cmd {
	e, ok := m.cursorEntry()
	if !ok {
		m.pv = previewState{kind: pvNone}
		return nil
	}
	_, _, prevW := m.layoutWidths()
	h := m.contentHeight()
	if m.pv.path == e.Path && m.pv.mod.Equal(e.ModTime) && m.pv.hidden == m.tab.ShowHidden &&
		(m.pv.kind != pvImage || (m.pv.width == prevW && m.pv.height == h)) {
		return nil
	}
	return m.reloadPreviewCmd()
}

func (m *Model) reloadPreviewCmd() tea.Cmd {
	e, ok := m.cursorEntry()
	if !ok {
		m.pv = previewState{kind: pvNone}
		return nil
	}
	_, _, prevW := m.layoutWidths()
	h := m.contentHeight()
	m.pv.seq++
	seq := m.pv.seq
	m.pv = previewState{
		path:    e.Path,
		mod:     e.ModTime,
		seq:     seq,
		width:   prevW,
		height:  h,
		hidden:  m.tab.ShowHidden,
		loading: true,
	}
	hidden := m.tab.ShowHidden
	th := m.th
	return func() tea.Msg {
		pv := buildPreview(e, hidden, prevW, h, th)
		pv.path = e.Path
		pv.mod = e.ModTime
		pv.seq = seq
		pv.width = prevW
		pv.height = h
		pv.hidden = hidden
		return previewLoadedMsg{seq: seq, pv: pv}
	}
}

func (m *Model) pasteCmd() tea.Cmd {
	if len(m.clip) == 0 {
		m.status = "没有复制或剪切的项目"
		return nil
	}
	srcs := slices.Clone(m.clip)
	cut := m.clipCut
	dst := m.tab.Dir
	if cut {
		m.clip = nil
		m.clipCut = false
	}
	return func() tea.Msg {
		var out []string
		var err error
		verb := "复制"
		if cut {
			verb = "移动"
			out, err = fsops.Move(srcs, dst, nil)
		} else {
			out, err = fsops.Copy(srcs, dst, nil)
		}
		if err != nil {
			return fileOpDoneMsg{err: fmt.Errorf("%s失败：%w", verb, err), reload: len(out) > 0}
		}
		focus := ""
		if len(out) > 0 {
			focus = filepath.Base(out[len(out)-1])
		}
		return fileOpDoneMsg{status: fmt.Sprintf("已%s %d 项", verb, len(out)), focus: focus, reload: true}
	}
}

func (m *Model) trashCmd(paths []string) tea.Cmd {
	clear(m.selected)
	sys := m.sys
	return func() tea.Msg {
		okCount := 0
		var lastErr error
		for _, p := range paths {
			if err := sys.Trash(p); err != nil {
				lastErr = err
			} else {
				okCount++
			}
		}
		if lastErr != nil {
			if okCount > 0 {
				return fileOpDoneMsg{
					status: fmt.Sprintf("已移到废纸篓 %d 项，部分失败：%v", okCount, lastErr),
					reload: true,
				}
			}
			return fileOpDoneMsg{err: lastErr}
		}
		return fileOpDoneMsg{status: fmt.Sprintf("已移到废纸篓 %d 项", okCount), reload: true}
	}
}

func (m *Model) cursorEntry() (vfs.Entry, bool) {
	es := m.tab.Entries()
	if m.cursor < 0 || m.cursor >= len(es) {
		return vfs.Entry{}, false
	}
	return es[m.cursor], true
}

func (m *Model) cursorName() string {
	if e, ok := m.cursorEntry(); ok {
		return e.Name
	}
	return ""
}

func (m *Model) chosen() []vfs.Entry {
	var out []vfs.Entry
	if len(m.selected) > 0 {
		for _, e := range m.tab.Entries() {
			if m.selected[e.Path] {
				out = append(out, e)
			}
		}
	}
	if len(out) == 0 {
		if e, ok := m.cursorEntry(); ok {
			out = append(out, e)
		}
	}
	return out
}

func (m *Model) pruneSelection() {
	if len(m.selected) == 0 {
		return
	}
	present := make(map[string]bool, len(m.tab.Entries()))
	for _, e := range m.tab.Entries() {
		present[e.Path] = true
	}
	for p := range m.selected {
		if !present[p] {
			delete(m.selected, p)
		}
	}
}

func (m *Model) restoreCursor(keep string) {
	es := m.tab.Entries()
	if len(es) == 0 {
		m.cursor = -1
		return
	}
	if keep != "" {
		if idx := m.tab.Index(keep); idx >= 0 {
			m.cursor = idx
			return
		}
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(es) {
		m.cursor = len(es) - 1
	}
}

func entryPaths(es []vfs.Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Path
	}
	return out
}
