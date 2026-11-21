package tui

import "gofinder/internal/keymap"

// actions lists all commands available in gf, in cheat-sheet display order.
// Actions marked Hint appear in the compact bottom hint bar.
func actions() []keymap.Action {
	return []keymap.Action{
		{ID: "cursor.up", Title: "光标上移", Group: "浏览"},
		{ID: "cursor.down", Title: "光标下移", Group: "浏览"},
		{ID: "cursor.top", Title: "跳到顶部", Group: "浏览"},
		{ID: "cursor.bottom", Title: "跳到底部", Group: "浏览"},
		{ID: "open", Title: "进入 / 打开", Group: "浏览", Hint: true},
		{ID: "up", Title: "上级目录", Group: "浏览"},
		{ID: "back", Title: "后退", Group: "浏览"},
		{ID: "forward", Title: "前进", Group: "浏览"},
		{ID: "refresh", Title: "刷新", Group: "浏览"},
		{ID: "hidden", Title: "显示隐藏文件", Group: "浏览"},
		{ID: "sortmenu", Title: "排序方式", Group: "浏览"},
		{ID: "addr", Title: "前往路径", Group: "浏览", Hint: true},

		{ID: "go.start", Title: "回到起点", Group: "跳转"},
		{ID: "go.home", Title: "主目录", Group: "跳转"},

		{ID: "filter", Title: "过滤", Group: "选择", Hint: true},
		{ID: "select.toggle", Title: "选中 / 取消", Group: "选择"},
		{ID: "select.all", Title: "全选", Group: "选择"},
		{ID: "select.none", Title: "取消选择", Group: "选择"},

		{ID: "copy", Title: "复制", Group: "文件", Hint: true},
		{ID: "copypath", Title: "复制路径", Group: "文件"},
		{ID: "cut", Title: "剪切", Group: "文件", Hint: true},
		{ID: "paste", Title: "粘贴", Group: "文件", Hint: true},
		{ID: "rename", Title: "重命名", Group: "文件", Hint: true},
		{ID: "newfolder", Title: "新建文件夹", Group: "文件", Hint: true},
		{ID: "newfile", Title: "新建文件", Group: "文件"},
		{ID: "trash", Title: "移到废纸篓", Group: "文件", Hint: true},
		{ID: "editor", Title: "编辑器打开", Group: "文件"},
		{ID: "sysopen", Title: "系统默认打开", Group: "文件"},

		{ID: "preview.up", Title: "预览向上滚动", Group: "预览"},
		{ID: "preview.down", Title: "预览向下滚动", Group: "预览"},

		{ID: "help", Title: "速查表", Group: "帮助", Hint: true},
		{ID: "quit", Title: "退出", Group: "帮助", Hint: true},
		{ID: "quit.keep", Title: "退出(不切目录)", Group: "帮助"},
	}
}

// defaultKeys defines gf's standalone terminal keybindings:
// arrow keys by default, optional Vim h/j/k/l, Enter exclusively for rename,
// and double-taps "c c" (copy path) and "n n" (new blank file).
func defaultKeys(vim bool) map[string][]string {
	m := map[string][]string{
		"cursor.up":     {"Up"},
		"cursor.down":   {"Down"},
		"cursor.top":    {"Home"},
		"cursor.bottom": {"End"},
		"open":          {"Right"},
		"up":            {"Left"},
		"back":          {"Alt+Left"},
		"forward":       {"Alt+Right"},
		"refresh":       {"Ctrl+R"},
		"hidden":        {"."},
		"sortmenu":      {","},
		"addr":          {"g"},

		"go.start": {"`"},
		"go.home":  {"Alt+H"},

		"filter":        {"f"},
		"select.toggle": {"Space"},
		"select.all":    {"Ctrl+A"},
		"select.none":   {"Escape"},

		"copy":      {"c"},
		"copypath":  {"c c"},
		"cut":       {"x"},
		"paste":     {"v"},
		"rename":    {"Enter"},
		"newfolder": {"n"},
		"newfile":   {"n n"},
		"trash":     {"d"},
		"editor":    {"e"},
		"sysopen":   {"o"},

		"preview.up":   {"PageUp", "Ctrl+U"},
		"preview.down": {"PageDown", "Ctrl+D"},

		"help":      {"Shift+/"},
		"quit":      {"q"},
		"quit.keep": {"Shift+Q"},
	}
	if vim {
		m["cursor.up"] = append(m["cursor.up"], "k")
		m["cursor.down"] = append(m["cursor.down"], "j")
		m["up"] = append(m["up"], "h")
		m["open"] = append(m["open"], "l")
	}
	return m
}

// buildKeymap constructs gf's keymap and applies overrides from kmPath if given.
func buildKeymap(vim bool, kmPath string) (*keymap.Keymap, []string) {
	km, err := keymap.New(actions(), defaultKeys(vim))
	if err != nil {
		panic(err)
	}
	var warns []string
	if kmPath != "" {
		overrides, err := keymap.ReadOverrides(kmPath)
		if err != nil {
			warns = append(warns, "keymap.json 读取失败："+err.Error())
		} else if len(overrides) > 0 {
			warns = append(warns, km.Override(overrides)...)
		}
	}
	return km, warns
}
