package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
)

// actions lists every command, in the order the cheat sheet shows them.
// Actions with Hint show in the status bar when no modifier is held.
func actions() []keymap.Action {
	as := []keymap.Action{
		{ID: "open", Title: "打开 / 进入", Group: "浏览"},
		{ID: "up", Title: "上级目录", Group: "浏览"},
		{ID: "back", Title: "后退", Group: "浏览"},
		{ID: "forward", Title: "前进", Group: "浏览"},
		{ID: "refresh", Title: "刷新", Group: "浏览"},
		{ID: "hidden", Title: "显示/隐藏隐藏文件", Group: "浏览"},
		{ID: "sortmenu", Title: "排序方式…", Group: "浏览"},
		{ID: "addr", Title: "前往（输入路径）", Group: "浏览"},

		{ID: "go.start", Title: "启动位置", Group: "跳转", Hint: true},
		{ID: "go.home", Title: "主目录", Group: "跳转"},
		{ID: "go.desktop", Title: "桌面", Group: "跳转"},
		{ID: "go.downloads", Title: "下载", Group: "跳转"},
		{ID: "go.documents", Title: "文稿", Group: "跳转"},
		{ID: "go.apps", Title: "应用程序", Group: "跳转"},
		{ID: "fav.add", Title: "收藏当前文件夹", Group: "跳转"},
		{ID: "fav.remove", Title: "从收藏夹移除当前文件夹", Group: "跳转"},

		{ID: "filter", Title: "过滤当前文件夹", Group: "选择", Hint: true},
		{ID: "quicklook", Title: "快速查看", Group: "选择", Hint: true},
		{ID: "select.all", Title: "全选", Group: "选择"},
		{ID: "select.none", Title: "取消选择", Group: "选择"},

		{ID: "copy", Title: "拷贝", Group: "文件", Hint: true},
		{ID: "copypath", Title: "拷贝路径", Group: "文件", Hint: true},
		{ID: "cut", Title: "剪切", Group: "文件", Hint: true},
		{ID: "paste", Title: "粘贴", Group: "文件", Hint: true},
		{ID: "newfolder", Title: "新建文件夹", Group: "文件", Hint: true},
		{ID: "newfile", Title: "新建文件", Group: "文件", Hint: true},
		{ID: "rename", Title: "重命名", Group: "文件", Hint: true},
		{ID: "trash", Title: "移到废纸篓", Group: "文件", Hint: true},
		{ID: "reveal", Title: "在 Finder 中显示", Group: "文件"},
		{ID: "terminal", Title: "在终端中打开", Group: "文件"},
		{ID: "editor", Title: "在编辑器中打开", Group: "文件"},
		{ID: "agent", Title: "在这里打开默认 Agent", Group: "文件", Hint: true},
		{ID: "settings", Title: "设置（终端、编辑器、默认文件浏览器）", Group: "文件"},

		{ID: "ws.next", Title: "下一个工作区", Group: "工作区与窗格"},
		{ID: "ws.prev", Title: "上一个工作区", Group: "工作区与窗格"},
		{ID: "pane.switch", Title: "切换左右窗格", Group: "工作区与窗格"},

		{ID: "layout.dual", Title: "双栏布局", Group: "布局"},
		{ID: "layout.columns", Title: "三列布局（上级/当前/预览）", Group: "布局"},
		{ID: "layout.rows", Title: "上下分栏布局", Group: "布局"},
		{ID: "preview", Title: "检查器", Group: "布局"},
		{ID: "sidebar", Title: "显示/隐藏侧边栏", Group: "布局"},
		{ID: "toolbar", Title: "显示/隐藏工具栏", Group: "布局"},
		{ID: "workspace", Title: "保存/恢复工作区", Group: "工作区与窗格"},

		{ID: "shelf.add", Title: "把选中项加入暂存架", Group: "暂存架"},
		{ID: "shelf", Title: "显示/隐藏暂存架", Group: "暂存架"},
		{ID: "shelf.copy", Title: "把暂存架拷贝到当前文件夹", Group: "暂存架"},
		{ID: "shelf.move", Title: "把暂存架移动到当前文件夹", Group: "暂存架"},
		{ID: "shelf.clear", Title: "清空暂存架", Group: "暂存架"},

		{ID: "help", Title: "快捷键速查表", Group: "帮助", Hint: true},
	}
	for i := 1; i <= 9; i++ {
		as = append(as, keymap.Action{ID: fmt.Sprintf("ws.%d", i), Title: fmt.Sprintf("第 %d 个工作区", i), Group: "工作区与窗格"})
	}
	return as
}

// defaultKeys are the built-in bindings. keymap.json in the data folder
// overrides them per action. The first key is Yazi's single key (no hjkl,
// no sequences but the double taps "c c" and "n n"), the one the hints
// show; the ⌘ key after it is the one the menu bar can take.
func defaultKeys() map[string][]string {
	m := map[string][]string{
		"open":     {"Right", "Cmd+Down"},
		"up":       {"Left", "Cmd+Up"},
		"back":     {"Cmd+[", "Back"},
		"forward":  {"Cmd+]", "Forward"},
		"refresh":  {"Cmd+R"},
		"hidden":   {".", "Cmd+Shift+."},
		"sortmenu": {","},
		"addr":     {"g", "Cmd+L"},

		"go.start":     {"`"},
		"go.home":      {"Cmd+Shift+H"},
		"go.desktop":   {"Cmd+Shift+D"},
		"go.downloads": {"Cmd+Alt+L"},
		"go.documents": {"Cmd+Shift+O"},
		"go.apps":      {"Cmd+Shift+A"},
		"fav.add":      {"Cmd+Ctrl+T"},
		"fav.remove":   {"Cmd+Ctrl+Backspace"},

		"filter":      {"f"},
		"quicklook":   {"Space"},
		"select.all":  {"Cmd+A"},
		"select.none": {"Escape"},

		"copy":      {"c", "Cmd+C"},
		"cut":       {"x", "Cmd+X"},
		"paste":     {"v", "Cmd+V"},
		"trash":     {"d", "Cmd+Backspace"},
		"rename":    {"Enter", "r"},
		"newfile":   {"n n", "Cmd+N"},
		"newfolder": {"n", "Cmd+Shift+N"},
		"copypath":  {"c c", "Cmd+Alt+C"},
		"reveal":    {"Cmd+Alt+R"},
		"terminal":  {";", "Cmd+Alt+T"},
		"editor":    {"Cmd+Alt+E"},
		"agent":     {"a", "Cmd+Alt+A"},
		"settings":  {"Cmd+,"},

		"ws.next":     {"]", "Cmd+Shift+]", "Ctrl+Tab"},
		"ws.prev":     {"[", "Cmd+Shift+[", "Ctrl+Shift+Tab"},
		"pane.switch": {"Tab"},

		"layout.dual":    {"Cmd+Alt+2"},
		"layout.columns": {"Cmd+Alt+3"},
		"layout.rows":    {"Cmd+Alt+4"},
		"preview":        {"Cmd+Alt+P"},
		"sidebar":        {"Cmd+Ctrl+S"},
		"toolbar":        {"Cmd+Ctrl+B"},
		"workspace":      {"Cmd+Alt+W"},

		"shelf.add":   {"Cmd+Alt+S"},
		"shelf":       {"Cmd+Alt+B"},
		"shelf.copy":  {"Cmd+Ctrl+V"},
		"shelf.move":  {"Cmd+Ctrl+M"},
		"shelf.clear": {},

		"help": {"Shift+/"},
	}
	for i := 1; i <= 9; i++ {
		m[fmt.Sprintf("ws.%d", i)] = []string{fmt.Sprint(i), fmt.Sprintf("Cmd+%d", i)}
	}
	return m
}

// defaultMenu is the context menu when config.json does not list one;
// "-" is a separator.
var defaultMenu = []string{
	"open", "-",
	"copy", "cut", "paste", "-",
	"rename", "copypath", "newfile", "newfolder", "-",
	"@agents", "editor", "-",
	"fav.add", "shelf.add", "workspace", "-",
	"trash",
}

// menuTitles are shorter titles for menus and the toolbar; other actions
// show their own title.
var menuTitles = map[string]string{
	"open": "打开", "copypath": "复制路径", "reveal": "在 Finder 中显示", "shelf.add": "加入暂存架",
	"workspace": "工作区…", "settings": "设置…", "back": "‹ 后退", "forward": "前进 ›",
	"up": "↑ 上级", "layout.dual": "双栏",
	"layout.columns": "三列", "layout.rows": "上下", "preview": "检查器",
}

func (a *app) actionTitle(id string) string {
	if t, ok := menuTitles[id]; ok {
		return t
	}
	act, _ := a.km.Action(id)
	return act.Title
}

// cmdPrefix starts the action IDs of the user's shell commands.
const cmdPrefix = "cmd."

// buildKeymap makes the keymap from the built-in actions, the user's
// shell commands and their keys, and keymap.json at kmPath ("" for none).
// It returns keymap.json's contents, which recording keys updates, and
// what could not be applied, for the status bar and the cheat sheet.
func buildKeymap(cfg config, kmPath string) (*keymap.Keymap, map[string][]string, []string) {
	var warns []string
	acts := actions()
	cmdKeys := map[string][]string{}
	for _, cm := range cfg.Commands {
		id := cmdPrefix + cm.ID
		switch {
		case cm.ID == "" || cm.Command == "":
			warns = append(warns, fmt.Sprintf("config.json 命令「%s」：缺少 id 或 command，已忽略", cm.Title))
			continue
		case slices.ContainsFunc(acts, func(x keymap.Action) bool { return x.ID == id }):
			warns = append(warns, fmt.Sprintf("config.json 命令 id %q 重复，已忽略", cm.ID))
			continue
		}
		title := cm.Title
		if title == "" {
			title = cm.ID
		}
		acts = append(acts, keymap.Action{ID: id, Title: title, Group: "自定义命令"})
		if len(cm.Keys) > 0 {
			cmdKeys[id] = cm.Keys
		}
	}
	km, err := keymap.New(acts, defaultKeys())
	if err != nil {
		panic(err) // the built-in table is wrong
	}
	for _, w := range km.Override(cmdKeys) {
		warns = append(warns, "config.json 命令快捷键："+w)
	}
	file := map[string][]string{}
	if kmPath != "" {
		var err error
		if file, err = keymap.ReadOverrides(kmPath); err != nil {
			warns = append(warns, "keymap.json 读取失败，未使用："+err.Error())
		}
		for _, w := range km.Override(file) {
			warns = append(warns, "keymap.json："+w)
		}
	}
	for _, list := range []struct {
		name string
		ids  []string
	}{{"contextMenu", cfg.ContextMenu}} {
		for _, id := range list.ids {
			if _, ok := km.Action(id); !ok && id != "-" && id != agentsItem {
				warns = append(warns, fmt.Sprintf("config.json %s：未知动作 %q，已忽略", list.name, id))
			}
		}
	}
	warns = append(warns, chromeWarnings(cfg, func(id string) bool { _, ok := km.Action(id); return ok })...)
	return km, file, warns
}

// keymapPath is keymap.json in the data folder.
func keymapPath(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "keymap.json")
}

// chordOf is the chord of a key pressed, false for keys the keymap has no
// name for (modifiers alone among them).
func chordOf(mods ui.Modifiers, k ui.Key) (keymap.Chord, bool) {
	for name, uk := range uiKeys {
		if uk == k {
			return keymap.Chord{Mods: fromUIMods(mods), Key: name}, true
		}
	}
	return keymap.Chord{}, false
}

// setKeys gives action id the keys (none to unbind), or its built-in
// keys when reset, and saves keymap.json. Actions that lose a key to it
// are saved too, so the file says what the app does.
func (a *app) setKeys(id string, keys []string, reset bool) {
	before := map[string][]string{}
	for _, act := range a.km.Actions() {
		before[act.ID] = a.specs(act.ID)
	}
	if reset {
		keys = a.km.Defaults(id)
		for _, cm := range a.cfg.Commands {
			if cmdPrefix+cm.ID == id {
				keys = cm.Keys
			}
		}
	}
	if keys == nil {
		keys = []string{}
	}
	warns := a.km.Override(map[string][]string{id: keys})
	for _, act := range a.km.Actions() {
		if now := a.specs(act.ID); act.ID != id && !slices.Equal(now, before[act.ID]) {
			a.kmFile[act.ID] = now
		}
	}
	if reset {
		delete(a.kmFile, id)
	} else {
		a.kmFile[id] = keys
	}
	a.applyMenu() // the menu shows the new keys
	a.status = "已更新快捷键"
	if len(warns) > 0 {
		a.status += "：" + strings.Join(warns, "；")
	}
	if a.kmPath == "" {
		return
	}
	p, m := a.kmPath, map[string][]string{}
	for k, v := range a.kmFile {
		m[k] = slices.Clone(v)
	}
	a.async(func() {
		if err := keymap.WriteOverrides(p, m); err != nil {
			a.post(func() { a.status = "无法保存 keymap.json：" + err.Error() })
		}
	})
}

// specs is an action's keys as keymap.json writes them.
func (a *app) specs(id string) []string {
	out := []string{}
	for _, ch := range a.km.KeysFor(id) {
		out = append(out, ch.Spec())
	}
	return out
}

// uiKeys maps keymap key names to MyGo keys.
var uiKeys = func() map[string]ui.Key {
	m := map[string]ui.Key{
		"Up": ui.KeyUp, "Down": ui.KeyDown, "Left": ui.KeyLeft, "Right": ui.KeyRight,
		"Home": ui.KeyHome, "End": ui.KeyEnd, "PageUp": ui.KeyPageUp, "PageDown": ui.KeyPageDown,
		"Enter": ui.KeyEnter, "Escape": ui.KeyEscape, "Tab": ui.KeyTab, "Space": ui.KeySpace,
		"Backspace": ui.KeyBackspace, "ForwardDelete": ui.KeyDelete,
		".": ui.KeyPeriod, ",": ui.KeyComma, "/": ui.KeySlash, "[": ui.KeyBracketLeft,
		"]": ui.KeyBracketRight, "-": ui.KeyMinus, "=": ui.KeyEqual, ";": ui.KeySemicolon,
		"'": ui.KeyQuote, "`": ui.KeyBackquote, "\\": ui.KeyBackslash,
		"F1": ui.KeyF1, "F2": ui.KeyF2, "F3": ui.KeyF3, "F4": ui.KeyF4, "F5": ui.KeyF5,
		"F6": ui.KeyF6, "F7": ui.KeyF7, "F8": ui.KeyF8, "F9": ui.KeyF9, "F10": ui.KeyF10,
		"F11": ui.KeyF11, "F12": ui.KeyF12,
		// The side buttons of a mouse and the swipes of a trackpad.
		"Back": ui.KeyBack, "Forward": ui.KeyForward,
	}
	letters := []ui.Key{ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG, ui.KeyH,
		ui.KeyI, ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM, ui.KeyN, ui.KeyO, ui.KeyP, ui.KeyQ, ui.KeyR,
		ui.KeyS, ui.KeyT, ui.KeyU, ui.KeyV, ui.KeyW, ui.KeyX, ui.KeyY, ui.KeyZ}
	for i, k := range letters {
		m[string(rune('A'+i))] = k
	}
	digits := []ui.Key{ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}
	for i, k := range digits {
		m[string(rune('0'+i))] = k
	}
	return m
}()

func toUIMods(m keymap.Mods) ui.Modifiers {
	var out ui.Modifiers
	if m&keymap.Shift != 0 {
		out |= ui.Shift
	}
	if m&keymap.Ctrl != 0 {
		out |= ui.Ctrl
	}
	if m&keymap.Alt != 0 {
		out |= ui.Alt
	}
	if m&keymap.Cmd != 0 {
		out |= ui.Cmd
	}
	return out
}

func fromUIMods(m ui.Modifiers) keymap.Mods {
	var out keymap.Mods
	if m&ui.Shift != 0 {
		out |= keymap.Shift
	}
	if m&ui.Ctrl != 0 {
		out |= keymap.Ctrl
	}
	if m&ui.Alt != 0 {
		out |= keymap.Alt
	}
	if m&ui.Cmd != 0 {
		out |= keymap.Cmd
	}
	return out
}

// menuItem puts an action's first key on a context-menu item; a double
// tap cannot be one, so the next key shows instead.
func (a *app) menuItem(m *ui.Menu, id, title string) *ui.MenuItem {
	it := m.Item(title)
	// The action's first key, as the status bar shows it: a single key
	// before its ⌘ one. A double tap ("c c") has no Shortcut form, so it
	// is the item's trailing text, at the end as the shortcuts are.
	if keys := a.km.KeysFor(id); len(keys) > 0 {
		ch := keys[0]
		if k, ok := uiKeys[ch.Key]; ok && !ch.Double {
			it.Shortcut(toUIMods(ch.Mods), k)
		} else {
			it.Trailing(ch.String())
		}
	}
	return it
}
