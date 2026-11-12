# Debug 编号表

> 来源：goFinder 开发会话（2026-11-12），阶段 15（见 `docs/plan.md`）。

**用法**：设置 › 关于 › 勾选「Debug 编号」（只在本次运行有效）。界面各元素左上角出现小胶囊编号；状态栏左侧出现「Debug」标记，点一下关闭。沟通时说「30 号」「41·2 号」即可。

**规则**
- 编号固定：同一个元素永远是同一个号，新元素只往所在分段的空号里加，已有编号不改、不复用。
- 列表里的行不逐行编号，只给整列一个号；分成两个窗格时，窗格内的元素在号后加窗格序号（`41·2` 是第 2 个窗格的当前栏）。
- 徽章画在元素之上（`ui.Overlay` + `AttachTo`），不占布局、不拦截点击（`IgnoreInput` + `HitTest`）。
- 代码入口：`debug.go` 的 `debugIDs`（编号表）与 `a.dbg(c, e, name, pane...)`；本文件与 `debugIDs` 由 `TestDebugIDsDoc` 保证一致，改一处必须同步另一处。
- 工具栏按钮的号按动作 ID 固定，与它在工具栏里的位置无关；`config.json` 里自定义的其他按钮统一用 17。
- 右键菜单由 MyGo 弹层绘制，没有可挂徽章的元素，所以 81 只标在键盘打开的新建/排序弹出菜单所依附的光标行上；鼠标右键的菜单没有编号。

| 编号 | 名称 | 元素 | 位置 |
|---|---|---|---|
| 1 | `titlebar` | 标题栏（Agent 按钮、工作区按钮、文件夹名） | `chrome.go` `titleBar` |
| 11 | `tb.agent` | Agent（左半段，打开默认 Agent） | `chrome.go` `agentSplit` |
| 12 | `tb.workspace` | 工作区对话框按钮（标题栏） | `workflow.go` `workspaceButtons` |
| 20 | `sidebar` | 侧边栏整体 | `sidebar.go` `sidebar` |
| 21 | `sidebar.favs` | 「个人收藏」分组 | `sidebar.go` `sidebar` |
| 22 | `sidebar.locs` | 「位置」分组 | `sidebar.go` `sidebar` |
| 23 | `sidebar.mine` | 「收藏夹」分组 | `sidebar.go` `sidebar` |
| 30 | `addr.capsule` | 地址栏胶囊 | `view.go` `addressBar` |
| 31 | `addr.crumbs` | 面包屑 | `view.go` `addressBar` |
| 32 | `addr.git` | Git 分支标记 | `view.go` `addressBar` |
| 34 | `addr.filter` | 过滤框 | `app.go` `filterBox` |
| 40 | `col.parent` | 上级栏 | `columns.go` `parentView` |
| 41 | `col.current` | 当前栏（文件列表） | `columns.go` `columnsView` |
| 42 | `col.preview` | 第 3 栏（预览/子项） | `columns.go` `columnsView` |
| 43 | `inspector` | 检查器 | `inspector.go` `inspectorView` |
| 44 | `inspector.info` | 检查器「通用」组 | `inspector.go` `inspGroup` |
| 45 | `inspector.perm` | 检查器「权限」组 | `inspector.go` `inspGroup` |
| 46 | `shelf` | 暂存架面板 | `workflow.go` `shelfView` |
| 60 | `status.msg` | 状态栏结果句 | `chrome.go` `statusBar` |
| 61 | `status.hints` | 状态栏键帽提示行 | `chrome.go` `statusBar` |
| 62 | `status.layout` | 视图切换按钮 | `chrome.go` `statusBar` |
| 63 | `status.insp` | 状态栏右端的检查器/侧栏按钮 | `chrome.go` `statusBar` |
| 64 | `status.debug` | 状态栏「Debug」标记 | `chrome.go` `statusBar` |
| 70 | `dlg.help` | 快捷键速查表 | `view.go` `helpSheet` |
| 71 | `dlg.set.general` | 设置 › 通用 | `view.go` `settingsView` |
| 72 | `dlg.set.keys` | 设置 › 快捷键 | `view.go` `settingsView` |
| 73 | `dlg.set.agent` | 设置 › Agent | `view.go` `settingsView` |
| 75 | `dlg.set.about` | 设置 › 关于 | `view.go` `settingsView`、`debug.go` `aboutPage` |
| 76 | `dlg.rename` | 重命名对话框 | `view.go` `dialog` |
| 77 | `dlg.newfile` | 新建文件对话框 | `view.go` `dialog` |
| 78 | `dlg.newfolder` | 新建文件夹对话框 | `view.go` `dialog` |
| 79 | `dlg.trash` | 废纸篓确认 | `trash.go` `trashView` |
| 80 | `dlg.workspace` | Workspace 对话框 | `workflow.go` `workspaceView` |
| 81 | `menu.context` | 弹出菜单（键盘打开的新建/排序菜单，标在光标行上） | `view.go` `table` |
