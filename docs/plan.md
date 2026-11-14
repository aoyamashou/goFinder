> 来源：goFinder 规划会话（2026-11-10），由 Claude Code plan mode 产出并经用户批准；2026-11-11 迁入仓库。
> 状态：阶段 0、阶段 1 已实现（commit 25d2691）；阶段 2 已实现（commit a3c3a74）；阶段 3 已实现（commit d50d1a6）；阶段 4 已实现（commit 1b7134f）；原生菜单栏（commit e13d455）；阶段 5 已实现（commit cd134dc，中间截断补充 ab2cdb5，失焦高亮修复 31b121d）；阶段 6 已实现（6A d242d43、6B 81ea582，合并与侧边栏拖放 e5f6d2d）；阶段 7 已实现（7A e61a248、7B e35615b，合并 45dddc6）；阶段 8 已实现（8A 178cd6e、8B 6fa6bc1 + 菜单栏一键 fb3fa09，合并 8e2e180）；阶段 9 已实现（9A 488bffe、9B 7314f1a，合并 a70c170）；阶段 10 已实现（10A 7394426、10B cf86cb9，合并 9ecda39）；阶段 11 已实现（设计系统 fd1c9f6、theme.go 1cf6e0b、11A a672d82、11B 6b59c44、11C 图标规范 a43ab4e，图标 418e03a）；阶段 12 已实现（commit 509c297；工具栏开关与默认深色 4ce6b2f）；阶段 13 已实现（commit afade25）；阶段 14 已实现（14A 730df41、14B 1101343，合并 2af75b8）；阶段 16 已实现（commit ac22859）；阶段 17 已实现（commit 173cb62）；阶段 18 已实现（本地 commit 11ce8a5；v0.18.0 已于 2026-11-12 发布，tag 指向 f5aac9a；v0.18.1 同日发布，tag 指向 0fa62da；v0.18.2 同日发布，tag 指向 0b7fef2；v0.18.3 同日发布，tag 指向 b63009e；v0.18.4 同日发布，tag 指向 2c1511f，DMG 起不再签名）。

# goFinder：用 MyGo 写的 Finder 替代品

## Context
用户想做一个自用的 Finder 替代品，把三款软件的长处合在一起：
- Yazi 的简洁界面和键盘操作，再补上快捷键提醒（经常记不住）；
- QSpace 的多布局、可交互地址栏、按模板新建文件、暂存架、Zip 浏览、一键打开终端或编辑器，以及各项自定义；
- ForkLift 的双栏、标签页、Workspace、iCloud 同步收藏夹、Quick Select、Git 状态、预览面板，以及设为默认文件浏览器。

技术前提（查 https://mygo.egoist.dev/docs 得到）：MyGo **不是一门新语言**，而是一个 Go 桌面框架（v0.8.3，MIT）。项目就是普通的 Go module，`import "github.com/egoist/mygo"` 和 `.../mygo/ui` 即可。框架不用 cgo，原生 UI 在 macOS 上用 Metal 绘制。

用户已确定的选择：
- 纯 Go 原生 UI；
- 快捷键提醒做两种：`?` 全量速查表 + 底部状态栏常驻提示；
- Git 只显示状态和分支；
- Touch Bar 不做（MyGo 不支持）；
- 分阶段交付，先出核心浏览器。

环境：Go 1.27.3（Homebrew）+ MyGo v0.8.3，native 模板。

## 可直接复用的 MyGo 能力
- **UI 组件**：
  - `ui.Table`：支持列宽调整和排序、方向键、`Submitted`（回车/双击），回车或 F2 可就地重命名；
  - `ui.List`：首字母 type-ahead；
  - `ui.Sidebar` / `SidebarSection` / `SidebarItem`：Finder 式源列表；
  - `ui.Split`（可拖动分栏）、`ui.Tabs`、`ui.Toolbar`、`row.ContextMenu(func(m *ui.Menu))`。
- **按键**：`c.Shortcut(mod, key)` 是窗口级快捷键，`Element.Shortcut` 只在焦点所在元素生效。菜单加速键写成 `Accelerator: "CmdOrCtrl+N"`。
- **状态更新**：视图函数是 `func (a *App) view(c *ui.Context)`；后台 goroutine 通过 `win.Update(func(){...})` 改状态。
- **系统能力（`mygo.Shell`）**：
  - `OpenPath`（用默认应用打开）、`ShowItemInFolder`、`TrashItem`（只进废纸篓）；
  - `FileIcon(path,size)` / `TypeIcon(ext,size)` 取图标；
  - `InstalledApps()` 列出已装应用，用来选择终端和编辑器。
- **文件监听**：`mygo.WatchFiles(ctx, dir, FileWatchOptions{...})`，自带 50ms 防抖。
- **应用级接口**：
  - `mygo.App.Path(mygo.PathUserData)` → `~/Library/Application Support/goFinder`；
  - `mygo.App.OnOpenFile`：接收"打开文件夹"请求，设为默认浏览器时要用；
  - 配置文件里的 `macos.infoPlist` 用于声明文档类型；
  - `mygo.App.SetMenu` 设置菜单栏。
- **插件**：
  - Terminal：内嵌终端视图，后续可选；
  - SF Symbols：提供图标；
  - Automation：外部进程可以驱动运行中的 app，用于端到端测试。
- **Go 标准库**：`archive/zip`、`os`、`io/fs`、`path/filepath`、`os/exec`（调用 `git` 和 `open -a`）。

## 架构
```
goFinder/
  main.go                 App 初始化、菜单、窗口、OnOpenFile
  mygo.json               MyGo 配置（名称、identifier、infoPlist）
  internal/
    keymap/   按键注册表 + 按上下文分发（核心）
    vfs/      统一文件系统接口：本地 OSFS + ZipFS（路径形如 /x/a.zip!/dir）
    fsops/    复制/移动/重命名/新建/废纸篓，任务队列 + 进度
    pane/     Pane{Tabs, Active}、Tab{Path, History, Cursor, Selection, Sort, Filter}
    layout/   布局：单栏 / 双栏 / 双栏+预览 / Yazi 三列（父/当前/预览）…
    preview/  文本、图片、目录、Zip 列表、文件信息
    git/      执行 `git status --porcelain=v2 --branch -z`，按仓库根目录缓存
    config/   config.json、keymap.json、templates/、workspaces/，收藏夹放 iCloud
    shelf/    暂存架
    view/     各视图：地址栏、文件表、状态栏、速查表、侧边栏、对话框
```

**关键设计 1：按键注册表（`internal/keymap`）是唯一数据源。**
- 每个动作是一条 `Action{ID, Title, Group, Contexts, Run}`；
- 默认键位是 `Binding{Keys: "Cmd+Shift+H", Action: "go.home"}`，用户的 `keymap.json` 可以覆盖；
- 按键分发、`?` 速查表、状态栏提示、右键菜单、工具栏全部从这张表读数据，所以提醒内容永远和真实键位一致，用户改了键也会自动跟着变。

**关键设计 2：按键分发流程（不做 Vim 风格，不做多键序列）。**
- 只用单键或"修饰键 + 单键"，按当前上下文（浏览 / 地址栏输入 / Quick Select / 对话框）查表分发；
- 没有前缀状态机，逻辑简单，也省掉了等待超时。

**默认键位（方向键 + Finder 习惯）：**

| 类别 | 键位 |
|---|---|
| 移动 | `↑/↓`，`Home/End`，`PageUp/PageDown` |
| 进出目录 | `→` 或 `Enter` 进入/打开，`←` 返回上级 |
| 选择 | `Shift+↑/↓` 连选，`Space` 单个勾选，`Cmd+A` 全选 |
| 复制/剪切/粘贴 | `Cmd+C` / `Cmd+X` / `Cmd+V` |
| 删除到废纸篓 | `Cmd+Delete` |
| 重命名 / 新建 | `F2` / `Cmd+N` 新建文件，`Cmd+Shift+N` 新建文件夹 |
| 显示隐藏文件 | `Cmd+Shift+.` |
| Quick Select | 直接打字即可选中匹配项，`Esc` 退出 |
| 标签页 | `Cmd+T` 新建，`Cmd+W` 关闭，`Cmd+1-9` 切换 |
| 双栏 | `Tab` 切换窗格 |
| 地址栏 | `Cmd+L` |
| 速查表 | `?` 或 `F1` |
| 跳转 | `Cmd+Shift+H` 主目录，`Cmd+Alt+L` Downloads，`Cmd+[` / `Cmd+]` 后退/前进 |
| 终端 / 编辑器 | `Cmd+Alt+T` 打开终端，`Cmd+Alt+E` 打开编辑器，`Cmd+,` 选择用哪个应用 |
| 侧边栏 / 收藏夹 | `Cmd+Ctrl+S` 开关侧边栏，`Cmd+Ctrl+T` 收藏当前文件夹，`Cmd+Ctrl+Backspace` 移除 |
| 暂存架 | `Cmd+Alt+S` 加入选中项，`Cmd+Alt+B` 开关面板，`Cmd+Ctrl+V` 全部拷贝到此，`Cmd+Ctrl+M` 全部移动到此 |
| Workspace | `Cmd+Alt+W` 保存/恢复 |
| 布局 | `Cmd+Alt+1/2/3/4` 单栏/双栏/三列/上下分栏，`Cmd+Alt+P` 预览 |
| 改键 | `?` 速查表每行"改键"录制 |

## 资源与性能原则（贯穿所有阶段）
用户要求：短小精悍，不在乎好不好看，只追求实用、低资源、响应快。
- **只用原生 UI**：不用 WebView；不引入 Terminal、Glass、SQLite、Loading、SF Symbols 等插件；不做动画，界面用默认样式。
- **单个二进制**：依赖只有 MyGo 和 Go 标准库，不引入第三方库。
- **按需加载**：
  - 图标按扩展名用 `TypeIcon` 取，并缓存（每种类型只取一次），不逐个文件调用 `FileIcon`；
  - 预览面板默认关闭，打开后只预览当前选中项；文本最多读 64KB，图片按面板大小缩小；
  - 只监听当前可见的目录，切走就取消；
  - Git 只在确认处于仓库里时才运行，并做缓存和防抖。
- **少重绘**：状态真的变了才调用 `win.Update` 或 `Invalidate`；大目录只在后台读一次，排序和过滤只在内存里做。
- **性能目标**（阶段 0 实测，之后作为回归检查）：
  - 冷启动 < 300ms；
  - 空闲内存 < 80MB，空闲时 CPU ≈ 0；
  - 打开 1 万个文件的目录 < 100ms。

## 分阶段计划

### 阶段 0：环境和技术验证（先做，降低风险）
1. 安装 Go：`brew install go`（≥ 1.27）；然后在项目目录运行 `go run github.com/egoist/mygo/cmd/mygo@latest init -template native .`，再跑 `go tool mygo doctor`。
2. 写一个小 spike，验证以下四点，结论记进 README：
   - `ui.Context` 能否拿到原始字符输入，Quick Select 需要；
   - `ui.Table` 列出 1 万个文件时是否流畅，并实测上面的性能目标；
   - 原生 UI 能否显示 PNG 或图片，供 `FileIcon` 和图片预览使用；
   - `ui.Split` 能否嵌套，三列布局需要。
3. 如果某项不支持，在这里调整方案。例如拿不到原始字符输入，Quick Select 就改用一个文本输入框来接收。

### 阶段 1：核心浏览器（第一个可用版本）
- `vfs` 只实现本地 OSFS；目录读取放在 goroutine 里，结果用 `win.Update` 回填；接入 `WatchFiles` 自动刷新。
- `pane` 和 `tab`：
  - 后退/前进历史；
  - 按名称、大小、修改时间、类型排序；
  - 隐藏文件开关。
- 布局先做三种：单栏、双栏（`ui.Split`）、Yazi 三列。可以从菜单切换，也可以用 `Cmd+Alt+1/2/3`。
- `keymap` 注册表 + 分发 + 默认键位。
- **快捷键提醒**：
  - 底部状态栏常驻显示当前上下文最常用的 6–8 个键；按住 Cmd 或 Alt 时，改为列出带这个修饰键的全部快捷键；
  - `?` 打开速查表浮层，按 Group 分组，支持输入过滤，Esc 关闭。
- **地址栏**：
  - 默认是面包屑，每一级都能点开，弹出同级子目录菜单直接跳转；
  - `Cmd+L` 或点击空白处进入编辑模式，按前缀匹配 `ReadDir` 结果自动补全，Tab 接受补全；
  - 支持 `~`。
- **Quick Select**：在列表里直接打字，实时选中名称包含这些文字的文件（大小写不敏感）；`Enter` 确认选择，`Esc` 取消。
- 基础文件操作：重命名（Table 自带）、新建文件夹、废纸篓、复制/剪切/粘贴（走 `fsops` 任务队列，状态栏显示进度）。
- 预览面板：文本文件读前 64KB、图片、目录摘要（子项数和总大小）、文件信息。
- 回车用 `mygo.Shell.OpenPath` 打开文件。

### 阶段 2：效率功能
- **按模板新建文件**：模板放在数据目录的 `templates/`（首次启动写入 `Untitled.txt`、`Untitled.md`、`main.go`；目录已存在就不再补）。`Cmd+N` 对话框里从下拉框选模板或"空白文件"，文件名默认取模板名，可改。逻辑在 `internal/templates`。
- **一键打开终端/编辑器**：
  - 执行 `open -a <App> <dir>`（在 zip 里时用压缩包所在的文件夹）；
  - `Cmd+,` 打开设置对话框，从 `InstalledApps()`（后台 goroutine 调用）列出的应用里选，存 `.app` 的包名（不是本地化名称，`open -a` 才找得到），写回 `config.json`；
  - 默认终端是 Terminal，默认编辑器是 Visual Studio Code。
- **Zip 浏览**（`internal/vfs/zip.go`，基于 `archive/zip`）：
  - 路径形如 `/x/a.zip!/sub/dir`，压缩包根是 `/x/a.zip!`；对 `.zip` 按 `→`/`↩` 进入，`←`、后退、面包屑能正确进出，离开时光标落回压缩包；
  - 每个压缩包的目录索引缓存一份（按修改时间和大小失效）；跳过 `__MACOSX` 和含 `..`/绝对路径的条目；
  - 预览包内文本/图片，预览 `.zip` 文件本身时列出包内顶层；
  - 包内回车：解压到 `PathCache/zip/<压缩包路径哈希>/` 再 `OpenPath`；
  - `Cmd+C` 后到包外粘贴 = 解压（`fsops.Copy` 调 `vfs.Extract`）。包内只读：剪切/粘贴/重命名/新建/废纸篓被拦下并在状态栏提示；包内不监听文件变化；不支持嵌套压缩包（包内的 .zip 当普通文件解压打开）。
- **侧边栏**（`ui.Sidebar`，默认隐藏，`Cmd+Ctrl+S` 开关并记入 `config.json` 的 `showSidebar`）：
  - 常用位置（主目录、桌面、下载、文稿、应用程序）、收藏夹、外接卷（`/Volumes` 下除启动盘外的目录，侧边栏显示时监听 `/Volumes`）；
  - 收藏夹由 `internal/favorites` 的 `Store` 接口读写，现在是数据目录的 `favorites.json`，阶段 3 换成 iCloud 实现即可；`Cmd+Ctrl+T` 收藏当前文件夹，`Cmd+Ctrl+Backspace` 或右键移除；
  - 原计划暂存架用的 `Cmd+Alt+S` 留给阶段 3，侧边栏用 Finder 的 `Cmd+Ctrl+S`。

### 阶段 3：工作流功能
- **暂存架**（`internal/shelf` + `workflow.go`）：
  - 右侧可开关的面板，`Cmd+Alt+B` 开关（显示状态随 `session.json` 保存）；
  - `Cmd+Alt+S` 加入选中项（加入后自动显示面板）；可从 Finder 拖入（`DroppedFiles`）。MyGo v0.8.3 没有表格行拖出（`DragRows`），所以应用内只能用快捷键加入；
  - `Cmd+Ctrl+V` 全部拷贝到当前目录（暂存架保留），`Cmd+Ctrl+M` 全部移动到当前目录（移走的离开暂存架）；"清空"按钮；每项 `×` 移除，右键"在 Finder 中显示"；
  - 压缩包内的项目不能加入；内容存数据目录 `shelf.json`，启动时去掉已不存在的项。
- **Workspace**（`internal/workspace` + `workflow.go`）：
  - 把布局、预览/暂存架开关、两个窗格所有标签页的路径和排序保存成 `workspaces/<name>.json`；
  - `Cmd+Alt+W`、右键菜单或菜单栏"Workspace"打开对话框选择并一键恢复；
  - 关窗/退出时把当前状态写到 `session.json`（快照在主线程，写盘在后台 goroutine，最多等 500ms），启动时自动恢复；命令行指定了文件夹时不恢复；已不存在的文件夹退到最近的上级。
- **收藏夹 iCloud 同步**：有 iCloud Drive 时写到 `~/Library/Mobile Documents/com~apple~CloudDocs/goFinder/favorites.json`（不需要额外签名权限），否则仍用数据目录；首次启动把阶段 2 的本地 `favorites.json` 合并进去并改名为 `.migrated`。侧边栏显示时监听该文件夹，外部改动按 `favorites.Merge` 三方合并（基准 = 上次读到或写入的列表）：顺序以磁盘为准，本机自基准后新增的追加在后，本机删除的去掉，对方删除的保持删除，按清理后的路径去重；合并结果与磁盘不同时写回。
- **Git**（`internal/git` + `gitstatus.go`）：
  - 读目录时在同一个后台 goroutine 里向上查找 `.git`；在仓库里才执行 `git --no-optional-locks status --porcelain=v2 --branch -z`，zip 内不运行，找不到 git 就本次启动不再尝试；
  - 按仓库根缓存；同一仓库同时只跑一个，跑的过程中有变化就结束后再跑一次；文件变化触发的刷新等 300ms 防抖（首次立即）；
  - 名称列标出 M/A/D/R/U/??（取暂存区优先的一个字母），含改动的文件夹标 `•`，未跟踪文件夹里的项标 `??`；地址栏在路径前显示 `⎇ 分支 · 改动数`；
  - 只显示状态和分支，不做 add/commit/push。

### 阶段 4：自定义和系统集成
- **自定义快捷键**（`internal/keymap` 的 `Override` + `actions.go` 的 `buildKeymap`/`setKeys`）：
  - 启动时依次叠加：内置默认键 → `config.json` 自定义命令的 `keys` → `keymap.json`。`Override` 不再整体失败，能应用的都应用：覆盖的键优先于默认键（原动作失去该键），两个覆盖抢同一键时动作顺序靠前的得到，未知动作和写错的键忽略；每条都生成一句说明；
  - 有说明时状态栏提示"有 N 处未按原样生效，按 ? 查看"，速查表顶部逐条列出；
  - 录制按键放在 `?` 速查表里（而不是单独的设置页，速查表本来就是全部动作的可搜索列表）：每行"改键"→ 按下组合键立即生效并写回 `keymap.json`（被抢走键的动作也写入，文件与实际一致），同一行可"清除"或恢复"默认"；不带 ⌘/⌃/⌥ 的字母、数字、标点不接受（会和 Quick Select 打字冲突）；录制时不分发快捷键。
- **自定义右键菜单和工具栏**：`config.json` 的 `contextMenu`（动作 ID 列表，`"-"` 分隔线；缺省用内置菜单）和 `toolbar`（默认空，不显示）；未知 ID 跳过并在速查表提示。
- **自定义 shell 命令**（`internal/shellcmd`）：`config.json` 的 `commands: [{id, title, command, keys}]`，注册为动作 `cmd.<id>`（分组"自定义命令"），因此出现在速查表、状态栏提示，可放进菜单和工具栏。`{path}`=当前文件夹，`{selection}`=选中项，逐个用单引号转义（用户自己给占位符加了引号也整体替换，防止被文件名闭合）；`/bin/sh -c` 在当前文件夹后台运行，失败时把错误和输出最后一行显示在状态栏；压缩包内不把选中项交给命令。
- **更多布局**：加上下分栏（`ui.SplitVerticalPercent`，`⌘⌥4`，Workspace 中为 `"rows"`）；任一布局都可叠加预览（双栏+预览即 `⌘⌥2` + `⌘⌥P`）。四宫格需要四个窗格，本阶段不做。
- **设为默认文件浏览器**：
  - `mygo.json` 的 `macos.infoPlist` 声明 `CFBundleDocumentTypes`：`public.folder`，角色 Viewer，`LSHandlerRank: Alternate`；
  - `OnOpenFile`：文件夹开新标签，文件进入其所在文件夹并选中；窗口出现前收到的先排队，窗口出现后打开，并替代上次会话的恢复；
  - `⌘,` 设置里显示当前 `NSFileViewer`，"设为默认"执行 `defaults write -g NSFileViewer -string <bundle id>`（bundle id 从运行中 app 的 Info.plist 读取），"恢复 Finder"执行 `defaults delete -g NSFileViewer`；界面说明需注销或 `killall Finder` 后生效。测试只用假 `system` 验证参数，不在本机实际执行。
- **打包签名**：`mygo.json` 的 `macos.signingIdentity` 为 `Apple_self_dev`；`go tool mygo build -skip-installer` 生成签名的 `.app` 和 `.dmg`。

### 阶段 5：Yazi 式界面（视觉与 UX 改版）
> 来源：goFinder 主会话，2026-11-11；用户给了两张 Yazi 截图作参考。

**要改什么，为什么**：现在的界面像表格式的 Finder，信息多、行距大，键盘焦点也不明显。改成 Yazi 那样紧凑的 Miller 列，用户能一眼看清自己在哪、键盘在哪。

视觉：
1. **图标用 Nerd Font 字形**：Yazi 本身就是这样做的，用字体里的私有区字符。
   - 字体用系统已装的 `JetBrainsMono Nerd Font`（`Font("JetBrainsMono Nerd Font, monospace")`），**不打包进 app**，保持体积小。
   - 新建纯逻辑包 `internal/icons`，按扩展名或文件名映射到字形和颜色，参照 Yazi 默认图标主题；目录分闭合和打开两种。
   - 没装字体时，回退到现在的文字符号。
2. **布局和 Yazi 一致**：
   - 最左边是 Finder 式侧边栏，始终开着；
   - 右边三栏依次是父目录、当前目录、子目录或预览；
   - 第 4 栏：光标在文件夹上时显示它的子项；在可预览的文件上时显示预览（文本或图片）；其他文件留空，或只显示文件信息；
   - 单独的预览面板默认关闭。
   - 这套布局作为默认的 `columns` 布局。单栏、双栏、上下分栏仍然能从「显示」菜单切换，不删。
3. **列表只显示名称**：
   - 去掉大小、修改日期、类型这几列，文件类型靠图标看；
   - 每栏宽度等于这一栏最长文件名的宽度，加上图标和少量内边距，并设一个上限，超出部分用 `…` 截断；
   - 不显示表头。排序仍然用快捷键或菜单。
4. **紧凑行距**：等宽字体 13–14pt，行高约 1.2–1.3 倍，接近 Yazi 在终端里的密度。
5. **光标停在文件夹上时**，这个文件夹的图标换成"打开的文件夹"字形；父目录栏里当前路径所在的那一项也一样。

UX：
1. **按住修饰键时的状态栏提示按 Group 分组排列**（新建、跳转、文件操作、视图……），每组前面加组名，顺序固定。`?` 速查表使用同样的分组顺序。
2. **键盘焦点栏位高亮**：当前接收键盘输入的栏位（侧边栏、当前列表、双栏中的哪一栏）加一圈强调色边框，其他栏位不加。
3. **启动位置是 `~`**：不再自动恢复上次会话的路径和标签页；Workspace 仍然可以手动恢复。命令行或 `OnOpenFile` 指定的文件夹优先。

**边界（明确不改）**：
- 不打包字体，不引入第三方库或插件，不加动画；
- 快捷键和动作不变（`actions.go` 只按需调整 Group）；
- 不删已有的布局和功能（标签页、暂存架、Git 标记、Zip、Workspace 等）。Git 状态改为在文件名后面加一个字母标记，不再单独占一列。

**验证**：
- `go test ./...`、`go vet`、`mygo vet`、`-race` 全部通过；`Open10k` 基准的结果不比现在差；
- 新增测试：图标映射、列宽计算、第 4 栏在文件夹、文本文件、二进制文件三种情况下的切换、焦点边框跟随 Tab 或侧边栏移动、启动位置是 `~`；
- 打包后实际运行，对照 Yazi 截图检查；空闲内存不超过阶段 4 的水平（约 44MB）。
- CLAUDE.md 里"图标用文字符号"这一句同步改成 Nerd Font 的说法；README 补上说明。

**已实现（commit cd134dc，分支 claude/phase5-474e9c）**：
- 与上面的差异：
  - 没有表头后，排序放在菜单"显示 › 排序方式"（生成的子菜单，不新增动作）；
  - 单栏和三列布局里 `Tab` 在侧边栏和文件列表之间切换焦点（双栏仍切换窗格）；
  - 不再读写 `session.json`；
  - 侧边栏默认显示；
  - 长文件名从中间截断并保留扩展名（`midTrunc`，按列表上一帧的宽度和等宽字符宽度计算列数）。
- 内存：同一时间、同样方式测得阶段 4 的包 75–79 MB、本阶段 73–82 MB（top MEM），没有退化；当天环境整体高于阶段 4 记录的 44 MB。

### 阶段 6：QSpace 式路径栏、Finder 式侧边栏、读取 Finder 设置、文件拖放
> 来源：goFinder 主会话，2026-11-11（plan mode，用户已批准）。

#### 背景
阶段 5 合并后用户试用，提出三点：
1. 路径栏现在是一排独立按钮（‹ › ↑ ✎ / ▸ Users ▸ me ▸），想改成 QSpace 那样的一整条胶囊。中间的 ▸ 仍可点开，快速跳到子目录（这个功能已有）。
2. 侧边栏想做得和 Finder 原生的一样（图 2）：有图标、分组和半透明材质。用户要求评估原生组件是否可行，以及 Go 和 Swift 哪个开销小。
3. 直接读取用户在 Finder 里的设置。用户选定读两项：**侧边栏条目与顺序**、**新窗口起始位置**。另外两项决定：自有收藏夹与 Finder 条目**两者并存**；图标用 **SF Symbols 插件**。

#### 评估结论：原生侧边栏用 Go 还是 Swift
- **MyGo 的 `ui.Sidebar` 本来就是照 AppKit source list 实现的**：分组可折叠，选中项在有焦点时是强调色、失焦时是灰色，支持键盘和无障碍。现在看起来"不原生"，原因只有三个：没有图标、没有侧边栏材质、条目内容不对。
- **嵌入真正的 AppKit NSOutlineView 不可行**：
  - MyGo 的原生 UI 是一整块 Metal 画布（一个 layer-backed NSView），框架没有提供把 AppKit 控件嵌进画布的接口。
  - 要嵌入只能自己写 cgo/ObjC 或 Swift 桥接，这违反"纯 Go、无 cgo"。
  - 键盘焦点、焦点边框、快捷键分发也会被拆到两套 UI 系统里，阶段 5 的焦点模型要重做。
- **性能对比**：
  - AppKit 本来就已经加载（窗口和菜单都靠它），一个二十来行的 NSOutlineView 和 MyGo 自绘的侧边栏，开销都在 1MB、1ms 以内，差别可以忽略。
  - 真正的成本是架构：多一个 Swift 运行时和桥接层，再加上两套事件循环之间的协调。
  - **结论：继续用 Go。** 改为 `ui.Sidebar` + 窗口材质 `VibrancySidebar` + SF Symbols 图标 + Finder 的条目，就能达到图 2 的效果。

#### 方案

#### 1. 新包 `internal/finder`（纯 Go，不依赖 MyGo，可单独测试）
- **`Sidebar(home) ([]Section, error)`**：读取 `~/Library/Application Support/com.apple.sharedfilelist/` 下的文件：
  - `com.apple.LSSharedFileList.FavoriteItems.sfl4` → 「个人收藏」；
  - `…FavoriteVolumes.sfl4` → 「位置」；
  - `…iCloudItems.sfl4` → 如有内容，并入相应分组；
  - 兼容旧系统的 `.sfl3` 后缀。
- **自写最小解析器**，只用标准库：
  - binary plist（bplist00）解析；
  - NSKeyedArchiver 解档（`$objects` / UID 引用）；
  - bookmark 数据（"book" 头，TOC 里的路径分量 0x1004 和卷路径），从中还原文件路径。
- **条目处理**：
  - 保留 Finder 中的顺序和名称（如有自定义名）；
  - 跳过被隐藏的条目；
  - 跳过没有文件路径的虚拟条目（AirDrop、最近使用、网络、iPhone），这一阶段不显示。
- **`NewWindowDir(home) string`**：执行 `defaults read com.apple.finder NewWindowTarget` 和 `NewWindowTargetPath`，或解析 `~/Library/Preferences/com.apple.finder.plist`（复用上面的 bplist 解析器，不再起子进程）。映射关系：

  | NewWindowTarget | 起始位置 |
  |---|---|
  | `PfHm` | 主目录 |
  | `PfDe` | 桌面 |
  | `PfDo` | 文稿 |
  | `PfID` | iCloud Drive |
  | `PfCm` / `PfVo` | `/` |
  | `PfLo` | `NewWindowTargetPath`（file:// URL） |

  读取失败或路径不存在时，回退到 `~`。
- 测试用的 fixture：本机 sfl4 文件里含个人路径，不放进仓库。改为在测试里用解析器的逆过程构造最小 bplist + bookmark，或者用脱敏后的样本。

#### 2. 侧边栏（`sidebar.go`）
- **分组从上到下**：
  1. Finder 的「个人收藏」；
  2. 「位置」：iCloud 云盘、CloudStorage 下的云盘（Google Drive 等），以及外接卷（只显示真正挂载、用户可见的卷）。这一组替换现在的「外接卷」，顺带去掉 UUID 卷和 Recovery 这类系统卷。
  3. 现有的自有「收藏夹」（iCloud `favorites.json`），保留在最下面。
  - 「常用位置」由 Finder 的个人收藏代替；读取失败时回退到现有的 `places()`。
- **跟随 Finder 变化**：用 `mygo.WatchFiles`（HintsOnly）监听 sharedfilelist 目录，变化后重新读取。读取在 `a.async` 里做，结果经 `a.post` 回到主线程。
- **SF Symbols 图标**：引入 `github.com/egoist/mygo/plugins/sfsymbols`。
  - 按路径映射符号：主目录 `house`、桌面 `menubar.dock.rectangle`、下载 `arrow.down.circle`、文稿 `doc`、应用程序 `a.square`（如无此符号就用 `square.grid.2x2`）、图片 `photo`、音乐 `music.note`、影片 `film`、iCloud `icloud`、云盘 `cloud`、外接卷 `externaldrive`、其他文件夹 `folder`。
  - 注意：`ui.SidebarItem` 的图标参数只接受 `*ui.SVG`，而 `sfsymbols.Symbol` 是一个 Element。先查 `ui/sidebar.go` 的 `coreSidebarItem`，看能否用 `nil` 图标并在前面插入 Symbol。不行就把条目改成自绘的行：`ui.Sidebar` 仍是外层容器，用 `Children` 或同等方式实现，选中、键盘和无障碍的行为保持 Sidebar 的原样。由实现会话按源码选择可行路径。
- **窗口材质**：`WindowOptions.Vibrancy = mygo.VibrancySidebar`。按 `docs/ui/windows.md` 的 Vibrancy 一节：
  - 根节点透明；
  - 侧边栏不画背景；
  - 文件列表区域画不透明的主题背景。

#### 3. 路径栏（`view.go` 的 `addressBar`）
- 整体是一条圆角胶囊：浅灰底、细边框、圆角。布局为 `‹ ›`（去掉按钮边框的图标）+ 胶囊。
- **胶囊内部**：
  - 开头是一个 `/` 的 ▸ 菜单；
  - 后面依次是各级文件夹名（无边框的文本按钮，点击跳转），每级之间是可点击的 `▸`（无边框，点开就是现有的子目录菜单）；
  - 结尾显示 Git 分支（灰色小字）。
- **去掉独立的 ↑ 和 ✎ 按钮**：
  - 上级仍可通过 `←` 键和菜单「前往 › 上级」进入；
  - 进入路径输入：`⌘L`，或点击胶囊里的空白处（这是阶段 1 计划中原本就有的设计）。
- 深层路径的 `…` 折叠逻辑保留。编辑模式的 Autocomplete 也放进同样的胶囊外观里。
- 复用：`vfs.Crumbs`、`vfs.CrumbPath`、`vfs.Subdirs`、`vfs.SplitZip`。

#### 4. 启动位置（`main.go` 的 `startDir`）
- 优先级：命令行或 `OnOpenFile` 指定的文件夹 > `finder.NewWindowDir(home)` > `~`。

#### 5. 文件拖放（参照 Finder 的规则）
- **背景**：用户提出，文件列表只能用键盘，不支持鼠标拖放。目标是保持 Yazi 的外观和键盘导航不变，同时支持 Finder 的标准拖放。现在只有暂存架接受从外部拖入的文件（`workflow.go` 的 `DroppedFiles`）。
- **MyGo 能力**（见 `docs/ui/drag-and-drop.md`、`docs/data-transfer.md`）：
  - `DragData` / `DragDataFrom` + `transfer.FileData(paths...)` 会发起系统级的拖动，拖进 Finder、其他应用或本应用的其他窗口都能被识别；
  - `DropData` / `DataDragOver` 配合 `transfer.Files` 接收拖入；
  - `Operations: transfer.Copy | transfer.Move` 由系统按修饰键决定：⌥ 复制，⌘ 移动；
  - `Done` 回调会给出最终操作结果。
- **拖出**：文件列表的行（三列都可以拖）用 `DragDataFrom` 在开始拖动的瞬间取当前选中项；按住的那一行不在选中项里时，只拖这一行。
  - 拖到 Finder 或其他应用时，由对方执行复制或移动，本应用不删除任何文件，`Done` 之后只刷新列表。
  - zip 内的条目只允许复制：先解压到缓存目录，再提供这些路径。
- **拖入目标**（内部拖动和从外部拖入走同一条路径，即统一用 `DropData`）：

  | 拖到哪里 | 放到哪里 |
  |---|---|
  | 文件夹那一行 | 该文件夹 |
  | 某一栏的空白处 | 这一栏显示的目录 |
  | 侧边栏的条目 | 该位置 |
  | 路径栏的某一级 | 该目录 |
  | 暂存架 | 加入暂存架（保留现有行为） |

  悬停在目标上时，该行或该区域用强调色描边，复用阶段 5 焦点边框的样式。
- **决定复制还是移动**：与 Finder 一致。
  - 默认：同一卷内移动，跨卷复制；⌥ 强制复制，⌘ 强制移动。
  - 判断"是否同卷"放进纯函数 `fsops.DropOp(srcs, dst, mods)`，比较 `syscall.Stat_t.Dev`，可以单独测试。
  - 真正执行时调用 `fsops.Copy` / `fsops.Move`，放在 `a.async` 里，状态栏显示进度。拖到自身或自身的子目录、拖进 zip 时拒绝，并在状态栏提示。
- **与键盘模型共存**：
  - 单击仍然只是移动光标并选中，拖动有移动阈值，所以不会误触；
  - 拖放完成后，键盘焦点和光标保持原样，只刷新受影响的栏；
  - 不新增快捷键。
- 不做弹簧文件夹（悬停自动展开）、不做替身（⌘⌥）、不支持拖动重排序；这些留到以后。

#### 6. 文档与规则
- `docs/plan.md` 新增「阶段 6」小节。按文档规范写明：来源（主会话，2026-11-11）、要改什么及为什么、边界、验证方式，并附上上面的 Go/Swift 评估结论。
- 更新 `CLAUDE.md`：
  - 总规则里的"不引入 MyGo 插件"改为"仅允许 SF Symbols 插件（用于侧边栏图标）"；
  - 架构部分补上 `internal/finder`。
- `README.md` 补充说明：侧边栏跟随 Finder、启动位置跟随 Finder。

#### 边界
- **只读 Finder**：不写 sfl4，也不写 `com.apple.finder` 偏好。⌘D 收藏仍写入自有的 `favorites.json`。
- 文件列表、快捷键、三列布局都不改。
- 不引入第三方库；除 sfsymbols 外不加其他插件。

#### 验证
- `go test ./...`、`go test -race ./...`、`go vet ./... && go tool mygo vet .` 全部通过。
- **新增测试**：
  - bplist、NSKeyedArchiver、bookmark 解析（用构造出的样本）；
  - NewWindowTarget 各取值的映射；
  - 侧边栏分组和读取失败时的回退；
  - 启动位置的优先级；
  - 路径栏点击 ▸ 打开菜单、点击空白处进入编辑。
  - 拖放：`DropOp` 的同卷/跨卷判断和修饰键规则；拖到文件夹行、栏的空白处、侧边栏条目时目标目录是否正确；拖到自身子目录、拖进 zip 时是否拒绝。`ui.Tester` 支持的部分写成界面测试。
- 在本机实际运行，检查拖放：
  - 在 Finder 和 goFinder 之间来回拖；
  - 在三列之间、双栏之间拖；
  - 拖到侧边栏条目上；
  - 按住 ⌥ 和 ⌘ 时光标的提示；
  - 拖放之后键盘焦点是否保持不变。
- 在本机实际运行：侧边栏的条目和顺序与 Finder 一致；在 Finder 里拖入或移除一个收藏后，goFinder 自动更新；路径栏的样子对照 QSpace 截图检查；有焦点和失焦时选中颜色正确。
- 空闲内存与阶段 5 相比不明显上升（sfsymbols 只在每个符号首次显示时绘制一次，之后缓存）。
- `go tool mygo build -skip-installer` 打包成功，并用 Apple_self_dev 签名。

### 阶段 7：工具栏、空白处右键菜单、状态栏、多栏视图、Yazi 单键键位、提示重做
> 来源：goFinder 主会话，2026-11-11（plan mode，用户已批准并逐条批注键位）。

#### 背景
阶段 6 合并后，用户提出 5 项改进：
1. 增加参照 QSpace 的工具栏；
2. 在文件列表空白处点右键时弹出菜单，可以新建文件、在这里打开终端、在这里打开 Agent；
3. 增加状态栏，可以点状态栏上的按钮切换视图；
4. 调整视图：默认就是现在的三列视图；切到双栏（左右或上下）时，每个窗格都是一个完整的三列视图；
5. 快捷键：
   - 换成 Yazi 的单键风格；
   - 设置里提供功能列表，并能改快捷键；
   - 快捷键提示要保留，但现在的样子太难看，需要重做。

用户的选择（AskUserQuestion）：
- 键位用**单键，不要 hjkl，不要多键序列**；
- Quick Select 改为**按 `/` 触发**；
- Agent = **在设定的终端里以当前文件夹为工作目录运行可配置的命令，默认 `claude`**；
- 提示做成**状态栏键帽化**，不弹浮层。

#### 一、键位：Yazi 单键风格（`actions.go`）
原则：
- 第一个键用 Yazi 的单字母键或符号键，状态栏、速查表、菜单标题里显示的都是这个键；
- **原有的 ⌘ 组合键保留为第二键位**，因为菜单的快捷键只能用带 ⌘ 或 ⌃ 的组合（`menu.go` 的 `menuAccel` 不用改）；
- 移动只用方向键，不用 hjkl；
- 不做多键序列，唯一的例外是用户点名要的同键双击 `c c` 和 `n n`。Yazi 中用其他序列触发的功能，改成单键或走菜单。

| 功能 | 新的第一键 | 说明（Yazi 原键） |
|---|---|---|
| 移动 | `↑ ↓` `Home/End` `PageUp/PageDown` | 不变 |
| 上级 / 进入、打开 | `←` / `→` `↩` `o` | Yazi 的 `h/l` 不用 |
| 后退 / 前进 | 无单键 | 只保留 `⌘[` `⌘]`（用户：不需要 H/L） |
| 勾选并下移 / 全选 / 取消选择 | `Space` / `⌘A` / `Esc` | 全选按用户要求用 `⌘A` |
| 拷贝 / 剪切 / 粘贴 | `c` / `x` / `p` | 按用户要求，拷贝用 `c`（不是 Yazi 的 `y`）；Edit 菜单的 ⌘C、⌘X、⌘V 角色照旧 |
| 拷贝路径 | `c c` | 同键双击，见下文 |
| 删除到废纸篓 | `d` | `D`（永久删除）**不绑定**，项目规则是不做永久删除 |
| 新建文件夹 | `n` | 直接进入新建文件夹的命名 |
| 新建文件 | `n n` | 同键双击。在光标位置弹出"新建"菜单（空白文件，以及各个模板），用方向键选、`↩` 确认，再输入文件名 |
| 重命名 | `r` | 同 Yazi |
| 显示隐藏文件 | `.` | 同 Yazi |
| Quick Select | `/` | 原来"直接打字"触发的方式取消 |
| 排序 | `,` | 弹出排序菜单；Yazi 是 `, m` 等序列 |
| 前往（输入路径） | `g` | 相当于 Yazi 的 `g Space` 交互跳转，即打开地址输入；主目录等跳转保留 ⌘ 组合键 |
| 新标签 / 关闭标签 | 无单键 | 只保留 `⌘T` `⌘W`（用户：不需要设快捷键） |
| 切换标签 | `1`–`9`、`[` `]` | 同 Yazi |
| 在终端打开 | `;` | Yazi 的 `;` 是运行 shell 命令 |
| 帮助 / 速查表 | `?` | 只留 `?`（去掉 F1） |
| 切换窗格 | `Tab` | 保持现状 |

**同键双击**（只给 `c c` 和 `n n` 用，不做通用的多键序列）：
- keymap 支持一种 chord，写成 `"c c"`，表示同一个键在 400ms 内按两次。
- 某个键如果同时绑定了单击和双击动作，按第一下时不立即执行，而是等 400ms：
  - 期间又按了同一个键，执行双击动作；
  - 超时没有第二下，执行单击动作；
  - 期间按了别的键，先执行单击动作，再处理这个键。
- 计时用 `time.AfterFunc` + `a.post` 回到主线程，测试里用可注入的时钟。
- `keymap.Parse` / `String` 要支持这种写法；`menuAccel` 遇到它时不设为菜单快捷键，只写进菜单标题。

- **分发规则不变**：`view.go` 的 `dispatch` 在文本框获得焦点或 Quick Select 正在输入时，屏蔽不带修饰键的按键。
- **取消"在列表里直接打字触发 Quick Select"**：`ObserveInput` 只在 Quick Select 已经打开后才收字符。还要确认 `ui.Table` / `ui.List` 自带的首字母跳转（type-ahead）不会吃掉单字母命令；必要时关掉它。
- **用户已有的 `keymap.json` 覆盖照常生效**：`buildKeymap` 不改。
- **规则文档同步**：`CLAUDE.md` 总规则里"只用方向键 + Finder 习惯的快捷键，不做 Vim 风格和多键序列"改为"Yazi 单键风格（不含 hjkl 和多键序列，只有 `c c`、`n n` 两个同键双击），方向键移动，⌘ 组合键作为第二键位供菜单使用"。同时更新记忆 `mygofinder-minimal-arrow-keys`。

#### 二、设置：功能列表与快捷键
- `settingsView`（`view.go`，即现在的终端/编辑器/默认浏览器对话框）改为分页（`ui.Tabs`），分为「通用」和「快捷键」两页。
- 「快捷键」页：按 Group 分组列出全部动作，每行是"功能名 + 当前键帽 + 改键 / 清除 / 默认"，带搜索。
  - 录制逻辑沿用 `helpSheet` / `recordKey` / `setKeys`，抽成公共函数，避免两处各写一份。
  - 录制时要能录单键，例如直接按 `y`。
- `?` 速查表改为只读。表底放一个"在设置中修改"按钮，点击后跳到快捷键页。

#### 三、快捷键提示：状态栏键帽化（新文件 `hints.go`）
- 状态栏中间显示提示：每条是一个**键帽**加功能名，键帽是圆角浅底、等宽字体的小块。按 Group 排列，组之间用细分隔或间距隔开。
- 平时只显示最常用的 6–8 条：`/` 选择、`c` 拷贝、`p` 粘贴、`d` 删除、`n` 新文件夹、`n n` 新文件、`r` 重命名、`?` 帮助。
- 按住修饰键时，换成用这个修饰键的全部快捷键，同样分组显示。放不下就截断，最后一个是"… ~ 全部"。
- Quick Select 进行中时，显示"选择「xx」：n 项"和 `↩` `⌫` `Esc` 的键帽。
- 数据仍取自 `keymap.Hints` / `Sheet`，不改 keymap 包。`?` 速查表也换成同样的键帽样式。

#### 四、视图：每个窗格都是三列（`view.go`、`columns.go`、`app.go`、`workflow.go`）
- **布局只剩三种**：`layout.columns`（默认，单个三列）、`layout.dual`（左右两个三列）、`layout.rows`（上下两个三列）。
  - 删去旧的单列表布局 `layout.single`，从 `actions.go`、`menu.go` 中去掉。
  - 读取旧的 Workspace 和会话文件时，`single` 一律映射为 `columns`（见 `workflow.go` 的 `layoutNames`）。
- **`columnsView` 按窗格参数化**：从 `a.tab()` 改为传入窗格索引 `pi`。每个窗格有自己的上级栏、当前栏、第 4 栏和路径栏。侧边栏全局只有一个，作用于当前活动窗格。
- **焦点**：焦点边框只出现在活动窗格的当前栏。`Tab` 在两个窗格之间切换，侧边栏的焦点逻辑不变。
- **单独的预览面板**（`showPreview`）：三列视图本身就有第 4 栏预览。保留开关，但只在单窗格时显示，避免双栏时太挤。

#### 五、工具栏（参照 QSpace，`view.go` 的 `toolbar`）
- 默认开启。`config.json` 的 `toolbar` 列表仍可以覆盖：设成空列表就隐藏。
- 默认按钮用 SF Symbols 图标，悬停显示"名称 + 键帽"：
  1. 后退、前进：从路径栏移到工具栏，路径栏只留那一整条胶囊；
  2. 新建：下拉，含新建文件、新建文件夹和模板列表；
  3. 排序：下拉；
  4. 显示隐藏文件；
  5. 暂存架；
  6. 在终端打开；
  7. 在这里打开 Agent；
  8. 最右边是 Quick Select 按钮（`/`）。
- 视图切换不放在工具栏，放在状态栏，避免重复。

#### 六、状态栏（`view.go` 的 `statusBar`）
- 三段布局：
  - 左边：项目数、已选数，以及 `a.status` 的提示；
  - 中间：键帽提示（第三节）；
  - 右边：视图切换的分段按钮（三列 / 左右 / 上下），后面是预览开关和隐藏文件开关两个小图标。
- 点击按钮直接调用 `a.run("layout.dual")` 等已有的动作，不重复写逻辑。

#### 七、空白处右键菜单 + Agent 动作
- 在文件列表空白处右键（每一栏都适用，作用于这一栏显示的目录）弹出菜单：
  - 新建文件（子菜单：空白文件，以及各个模板）
  - 新建文件夹
  - 粘贴
  - 在终端中打开
  - 在这里打开 Agent
  - 排序方式
  - 显示隐藏文件
  - 刷新
  - 在 Finder 中显示
- 菜单项可以通过 `config.json` 新增的 `blankMenu` 列表覆盖，做法和现有的 `contextMenu` 一样。
- 新增动作 `agent`（在这里打开 Agent）：
  - 注册在 `actions.go`，放进 `menu.go` 的「文件」菜单；
  - 默认键 `⌘⌥A`，它没有对应的 Yazi 键；
  - `config.json` 新增 `agentCommand`，默认 `claude`，设置的「通用」页可以修改。
- 实现方式：在临时目录写一个 `.command` 脚本，内容是 `cd <dir> && exec <agentCommand>`，路径用 `shellcmd` 的单引号转义；然后执行 `open -a <终端应用> <script>`。
  - 先用 Terminal.app 和 iTerm2 验证；用户实际用的终端（`config.json` 里的 terminal，例如 Kaku 或 Ghostty）如果不支持 `.command`，就退回到 Terminal.app，并在状态栏提示。
  - 临时脚本运行后删除，或放在缓存目录里覆盖使用。

#### 边界
- 不实现 Yazi 中项目没有的功能：visual 模式、反选、过滤、`n`/`N` 查找、行模式、spot、任务管理器、符号链接、永久删除。
- 不引入第三方库；插件仍然只用 sfsymbols。
- 拖放、侧边栏、Finder 读取等阶段 6 的行为不变。

#### 验证
- `go test ./...`、`go test -race ./...`、`go vet ./... && go tool mygo vet .` 全部通过；`TestMenuCoversActions` 通过。
- 新增或调整的测试：
  - **键位**：`c` `x` `p` `d` `n` `r` `.` `/` `,` `g` `;` `[` `]` `?` `⌘A` 都分发到正确的动作；在文本框和 Quick Select 中不分发；在列表里打字不再触发 Quick Select。
  - **同键双击**：`c c` 拷贝路径、`n n` 弹出新建文件菜单；单按 `n` 超时后新建文件夹；`n` 之后按别的键时，先执行新建文件夹再处理那个键。
  - **设置**：在快捷键页录制一个单键，结果写入 `keymap.json`。
  - **布局**：双栏和上下布局中，每个窗格都有三列；`Tab` 切换窗格；旧的 `single` 布局映射为 `columns`。
  - **状态栏**：点视图按钮切换布局。
  - **右键菜单**：空白处右键的菜单项正确，新建文件落在这一栏显示的目录。
  - **Agent**：用 `fakeSys` 验证生成的脚本内容和转义，以及 `open -a` 的参数，不在本机真的打开终端。
- `go tool mygo build -skip-installer` 打包成功，并用 Apple_self_dev 签名，然后由用户试用。

### 阶段 8：预览与检查器、删除确认、回车重命名、两行状态栏、菜单风格、列表字体与图标
> 来源：goFinder 主会话，2026-11-11（plan mode，用户已批准，栏位定义按用户截图说明）。

#### 背景
阶段 7 合并后用户试用，提出以下改进：
1. **预览**：现在只能预览图片和纯文本，希望增加 PDF 和 HTML。
2. **检查器**：预览面板重新定位，改名为"检查器"，只显示所选对象的属性。
3. **`?` 速查表**：用键盘按 `?` 打不开，只能从菜单打开。
4. **状态栏右下角**：去掉"显示隐藏文件"按钮，工具栏上已经有了。
5. **按 `d` 删除**：先弹确认框，`↩` 确认、`Esc` 取消，带"不再提醒"选项；设置里可以重新打开或关闭这个提示。
6. **回车**：默认动作改为重命名。
7. **状态栏**：改成两行。上一行显示当前状态和上一个操作；下一行显示快捷键提示。
8. **菜单风格**：菜单栏和右键菜单都改成参考截图中 Claude Desktop 的样子：大圆角，右侧用灰色单字母标出快捷键，"删除"用红色。
9. **列表字体**：文件列表改为 13 号字，默认字体为 JetBrains Mono Nerd Font（Mono 变体）；本机没有这个字体时，提供一键下载。
10. **图标**：充分利用 Nerd Font 里的符号，让列表中各类条目的图标更明显、更好认。

用户在这一轮的决定：
- PDF/HTML 预览用 **QuickLook 缩略图**（`qlmanage`）。
- **栏位**（不含侧边栏，从左往右，用户附了截图说明）：
  - 第 1 栏：上级目录；
  - 第 2 栏：当前目录；
  - 第 3 栏：光标在文件夹上时列出它的子项；在文件上时自动变成内容预览。这一点现在已经实现，这次加上系统 QuickLook，让能预览的文件更多。
  - 第 4 栏：**检查器**，只在用户打开它时出现，默认关闭。现在打开后会再显示一遍图片预览和信息，与第 3 栏重复。改成**只显示基本信息、不显示内容预览**，栏宽尽量窄。光标在文件上时显示大小、种类、创建和修改时间、权限等；在文件夹上时只显示权限和统计（子项数、总大小）。
- **打开**：
  - `→` 进入文件夹；光标在文件上时 `→` 打开文件；
  - 去掉 `o`；
  - `⌘↓` 保留；
  - 鼠标双击仍然是打开。

#### 一、第 3 栏预览 + QuickLook（`preview.go`、`columns.go`）
- 第 3 栏现在的行为保持不变：光标在文件夹上时列出子项，在文件上时显示预览。这次只扩充第 3 栏能预览的类型。
- 预览类型：
  - 图片、文本：沿用现有实现。
  - PDF、HTML，以及系统能生成缩略图的其他类型（docx、key、pages、视频封面等）：执行 `qlmanage -t -s <像素> -o <缓存目录> <file>` 生成首页 PNG，缓存键为"路径 + 修改时间 + 尺寸"，放在 `~/Library/Caches/<id>/ql/`。
    - 在 `a.async` 里跑，`previewState.seq` 丢弃过期结果，超时 3s。
    - 失败时不显示缩略图，只显示图标。
  - zip 内的文件先解压到缓存再生成，复用现有逻辑。
- 通过 `system` 接口调用 `qlmanage`，测试里用 `fakeSys`。
- 资源：只在光标停在文件上约 150ms 后才发起生成，避免方向键连续移动时每一项都起一个进程。

#### 二、第 4 栏检查器（替代预览面板）
- 现有的 `showPreview`、`previewView` 改成检查器。动作 ID 仍用 `preview`，避免用户已有的 `keymap.json` 失效；标题改为"检查器"，菜单、工具栏、状态栏中对应的标签一并改名。
- **去掉检查器里的内容预览**（图片缩略图、文本内容都去掉，因为和第 3 栏重复），只留信息。
- 检查器默认关闭，与现在一致。打开后作为第 4 栏，宽度尽量窄：固定约 200pt，长名称换行，值用小字号；不能拖宽。
- **文件**：名称、种类（UTType 的描述或扩展名）、大小（字节数和易读的大小）、创建时间、修改时间、上次打开时间、权限（`rwxr-xr-x` + 八进制 + 所有者/组）、路径。是链接时显示目标；有 Git 状态时显示状态。
- **文件夹**：权限 + 统计（直接子项数、文件数/文件夹数、递归总大小）。总大小在 `a.async` 里计算，按 `seq` 丢弃过期结果，最多统计 10 万项，超出时显示"≥"。
- 数据来自 `os.Lstat` + `syscall.Stat_t`（`Birthtimespec`、`Atimespec`），放进纯函数 `internal/fsops` 或新建 `internal/finfo`，可以单独测试。
- 多选时显示"已选 N 项，共 X"。

**一、二、六已实现（会话 8A，分支 claude/phase8a-preview-5892e6，commit 见合并记录）**，与上文的差异：
- 检查器在所有布局下都可打开（作用于活动窗格），不再只限单窗格；多选时显示"已选 N 项"和文件总大小（文件夹不计）。
- QuickLook 只对白名单扩展名（PDF、HTML、Office/iWork、RTF、视频、非 PNG/JPEG 图片、字体等）调用；本机实测 `qlmanage` 对无法生成缩略图的二进制文件会一直挂起直到被杀，所以不对任意二进制调用。失败的文件本次运行内不再重试；缓存 30 天未生成的启动时清理。
- `fontNotice(c)` 在 `font.go` 实现，由 8B 的状态栏调用；状态栏里"预览"按钮的文字在 `chrome.go`（8B 负责），合并时应改为读 `actionTitle("preview")`（"检查器"），并去掉多窗格时的禁用。

#### 三、键位与交互（`actions.go`、`view.go`、`app.go`）
- **`?` 用键盘打不开**：先写一个失败的测试复现问题，再找根因。怀疑 `/`（Quick Select）的 `c.Shortcut(0, KeySlash)` 也匹配到 Shift+/，把事件先吃掉了；也可能和按键布局有关。只修根因。
  - 根因（8B 查明）：不是 `/` 抢键。MyGo 在 macOS 上用 `charactersIgnoringModifiers` 给按键命名，它保留 Shift，所以 `⇧/` 报成 `?`，MyGo 没有对应的 `Key`（`KeyUnknown`，`/` 只在 `PhysicalKey` 里）；而 MyGo 只在按着 ⌃/⌥/⌘ 时按位置匹配快捷键，`c.Shortcut(Shift, KeySlash)` 因此永远收不到。`ui.Tester` 的 `Key(Shift, KeySlash)` 直接给出 `KeySlash`，所以旧测试没发现。修法：`view.go` 的 `shiftedKey` 在根容器的 `ObserveInput` 里，把"只按 Shift + 位置键"的按键按 keymap 的写法（`Shift+/`）匹配执行；回归测试用 `tt.KeyAt(Shift, KeySlash)` 模拟真机事件。
- **`↩` = 重命名**：`rename` 的第一键改为 `Enter`，`F2` 保留为第二键位。`dispatch` 中原来"Enter 交给表格的 Submitted"的特殊处理相应调整：表格的 Submitted 只保留给双击，用来打开。
- **`→`**：光标在文件夹上时进入，在文件上时打开（同 `open` 动作）。`open` 去掉 `o`，`⌘↓` 保留。
- **删除确认**：
  - `trash`（`d`、`⌘⌫`）先弹出模态框："将 N 个项目移到废纸篓？"，列出最多 5 个名称；`↩` 确认、`Esc` 取消；带"不再提醒"复选框。
  - `config.json` 新增 `confirmTrash`（默认 true）。勾选"不再提醒"后写入 false；设置的「通用」页可以重新打开或关闭。
  - 拖到废纸篓等其他删除途径不受影响。

#### 四、两行状态栏（`chrome.go` 的 `statusBar`、`hints.go`）
- **上一行**：
  - 左侧：项目数、已选数，以及 `a.status`（上一个操作，例如"已拷贝 3 项"）；
  - 右侧：视图分段按钮（三列 / 左右 / 上下）和检查器开关。**去掉隐藏文件开关**。
- **下一行**：整行显示 `statusHints` 的键帽提示（阶段 7 的实现），平时显示的常用键里加上 `↩ 重命名`。

#### 五、菜单风格（参照 Claude Desktop）
- **右键菜单和工具栏下拉**：在根视图调用 `c.SetMenuPresentation(ui.PopoverMenus)`，改由 MyGo 自己绘制，风格就能统一。
  - 每个菜单项用 `Shortcut(...)` 在右侧显示单字母键，从 keymap 读取，与动作的第一键一致。
  - "移到废纸篓"用 `IconColor(t.Danger)`，文字为红色。如果 `Item` 不支持改文字颜色，退而只把图标改成红色。
  - 菜单的圆角、留白交给 MyGo 绘制菜单的主题；有需要时在 theme 里加大圆角和行高，向截图靠拢。
- **原生菜单栏**：它是系统的 NSMenu，样式由 macOS 绘制，无法自定义（macOS 26 的原生菜单本身已经是大圆角）。只做到一致：在菜单项标题里显示单字母键（`menuAccel` 已有这个机制），不另外自绘菜单栏。这个限制写进文档。

#### 六、列表字体与 Nerd Font 图标（`columns.go`、`internal/icons`、新文件 `font.go`）
- 文件列表（上级、当前、第 3 栏子项）用 13pt 等宽字体，默认 `JetBrainsMono Nerd Font Mono`，回退顺序为 `JetBrainsMono Nerd Font` → `monospace`。行高按阶段 5 的紧凑比例重新计算，列宽的测量随字体一起更新。
- **一键下载**：用 `ui.FontFamilies()` 检测，没有 NFM 时：
  - 状态栏上一行显示"未安装 JetBrains Mono Nerd Font · 下载"，设置「通用」页也放一个下载按钮。
  - 点击后用 `net/http` 从 nerd-fonts 官方 GitHub 仓库下载 `JetBrainsMonoNerdFontMono-Regular.ttf` 和 `-Bold.ttf`（只下这两个文件，每个约 2MB），写入 `~/Library/Fonts`，然后 `ui.RegisterFont` 立即生效，不用重启。
  - 下载在 `a.async` 里进行，状态栏显示进度或失败原因。不会自动下载，必须用户点击。
- **图标增强**（`internal/icons`）：参照 Yazi 和 nerd-fonts 的 `nf-seti`、`nf-dev`、`nf-md` 字形，把映射扩充到常见类型：
  - 语言和配置文件、文档、表格、演示文稿、图片、视频、音频、压缩包、磁盘映像、字体、证书和密钥、日志、数据库；
  - `.app`；
  - 可执行文件（有 x 权限的普通文件）；
  - 符号链接（用链接字形，或在名称后加 `→`）；
  - 隐藏文件（颜色变暗）；
  - 特殊文件夹：`.git`、`node_modules`、`.config`、Desktop、Downloads、Documents、Pictures、Music、Movies、Applications、iCloud；
  - Git 仓库的根目录（用带分支的文件夹字形）。
  - 颜色按类别区分，并且在深色和浅色模式下都要清晰可读。

#### 边界
- 不引入 WebView，不引入第三方库；插件仍然只用 sfsymbols。
- 不做永久删除；确认框只针对移到废纸篓。
- 原生菜单栏不自绘。
- 字体只在用户点击后才下载，只下 2 个 ttf 文件，不下整个 zip 包。
- 阶段 6 和 7 的拖放、侧边栏、键位（除了上面改到的 `↩`、`→`、`o`）保持不变。

#### 验证
- `go test ./...`、`go test -race ./...`、`go vet ./... && go tool mygo vet .` 全部通过，`TestMenuCoversActions` 也通过。
- 新测试：
  - **键位**：`?` 能打开速查表；`↩` 进入重命名；`→` 在文件上时打开；`o` 不再有动作。
  - **删除确认**：`d` 弹框，`↩` 确认后文件进入废纸篓（用 `fakeSys`），`Esc` 取消后文件不变；勾选"不再提醒"后 `confirmTrash` 写入 false，此后 `d` 直接删除。
  - **状态栏**：两行都在；隐藏文件开关已去掉；视图按钮仍然可用。
  - **检查器**：文件和文件夹各自的属性字段；检查器里没有图片和文本预览；默认关闭；统计结果正确；过期结果被丢弃。
  - **QuickLook**：缓存键正确、命中缓存时不再调用 `qlmanage`、失败时回退。
  - **字体**：没有字体时显示提示；下载流程用假的 HTTP 服务端验证，并检查写入的路径。
  - **图标**：新增类型的映射，以及特殊文件夹、可执行文件、符号链接的字形。
  - **菜单**：绘制出的菜单里，各项都带快捷键字母，删除项是红色。
- `go tool mygo build -skip-installer` 打包成功，并用 Apple_self_dev 签名。

### 阶段 9：去掉快速选择、f 过滤器、提示精简、检查器点击复制、暂存架修整、右键复制路径、状态栏去统计
> 来源：goFinder 主会话，2026-11-11（plan mode，用户已批准）。

#### 背景
阶段 8 合并后用户继续试用，提出以下改进：
1. 去掉 `/` 快速选择（用户多选习惯用鼠标），工具栏右上角的对应按钮也去掉。
2. 新增 `f`，复刻 Yazi 的过滤器。
3. 底部快捷键提示默认只显示最常用的几个；粘贴改为 `v`，不用 `p`，`⌘V` 保留。
4. 检查器里的信息点一下就复制到剪贴板，权限除外。
5. 检查器加两个暂存架按钮：加入暂存架、打开暂存架。
6. 暂存架的界面右侧溢出了窗口（用户附了截图：三个按钮那一行超出了面板宽度）。
7. 右键菜单去掉"在 Finder 中显示"，改为"复制路径"：在空白处右键复制当前文件夹的路径，在文件上右键复制文件的路径。
8. 状态栏最左边的项目统计去掉，例如"5 项"。

#### 一、键位与过滤器（`actions.go`、`app.go`、`view.go`、`internal/pane/tab.go`、`hints.go`、`chrome.go`）
- **去掉快速选择**：
  - 删除 `quickselect` 动作、`/` 键、`qs`/`qsOn`/`qsEat` 相关代码及其输入处理；
  - 删除 `menu.go` 编辑菜单中的这一项、工具栏最右边的按钮（`startQuickSelect`），以及 README 和速查表里的说明；
  - Space 勾选、⌘A 全选、鼠标多选（⌘点击、⇧点击）照旧。
- **`f` 过滤器**（复刻 Yazi 的 `filter --smart`）：
  - 按 `f` 后，当前栏底部（或路径栏位置）出现一个过滤输入框。边输入边过滤当前栏，只显示名称包含该文字的项。大小写规则与 Yazi 的 smart case 相同：输入全为小写时不区分大小写，含大写字母时区分。
  - `↩`：保留过滤并让焦点回到列表，此时状态栏显示"过滤：xx"，可以继续用方向键和其他单键操作。
  - `Esc`：在输入框里按时清空过滤并关闭输入框；过滤生效、焦点在列表时按，也是清除过滤。
  - 过滤只作用于当前栏和这个标签页；进入其他文件夹后自动清除（与 Yazi 一致）。光标尽量停在原来那一项上，如果它被滤掉了，就停在第一项。
  - 过滤逻辑写成纯函数，放进 `internal/pane` 的 `Tab`（加 `Filter` 字段，`Entries()` 返回过滤后的结果），可以单独测试。排序和隐藏文件的逻辑不变。
- **粘贴改为 `v`**：`paste` 的第一键改为 `v`，`⌘V` 保留为第二键位，`p` 不再绑定。
- **默认提示**（`hints.go`），按以下顺序显示：
  `f` 过滤、`c` 拷贝、`c c` 拷贝路径、`x` 剪切、`v` 粘贴、`n` 新文件夹、`n n` 新文件（弹出新建菜单，可以用键盘或鼠标选择）、`↩` 重命名、`d` 删除、`?` 帮助。
  - 按住修饰键时的提示和过滤进行中时的提示照旧。过滤中显示"过滤「xx」：n 项"，以及 `↩`、`Esc` 的键帽。

#### 二、状态栏（`chrome.go` 的 `statusBar`）
- 上一行最左边的项目数和已选数统计去掉，只留 `a.status`（上一个操作）。过滤生效时显示"过滤：xx"。右边的视图按钮和检查器开关保留。

#### 三、右键菜单：复制路径替代"在 Finder 中显示"（`actions.go` 的 `defaultMenu`、`chrome.go` 的 `blankMenu`）
- 文件行的右键菜单：去掉 `reveal`，换成 `copypath`，复制所点文件的路径；如果所点的文件在多选范围内，就把所有选中项的路径用换行连接后一起复制。
- 空白处的右键菜单：去掉 `reveal`，换成"复制路径"，复制这一栏所显示文件夹的路径。
- `reveal` 动作本身保留，原生菜单栏里仍然有。用户在 `config.json` 里自定义的 `contextMenu` / `blankMenu` 不受影响。
- 复制后状态栏提示"已复制路径"。

#### 四、检查器：点击复制 + 暂存架按钮（`inspector.go`）
- 除了权限以外，每个值都可以点击：文件名、种类、大小、各项时间、路径、链接目标、Git 状态、文件夹统计。
  - 点击后立即用 `system` 接口写入剪贴板（复用 `copypath` 的写剪贴板方法），状态栏提示"已复制 <字段名>"。
  - 悬停时显示手形光标、浅色底，tooltip 为"点击复制"。复制的是显示出来的文字；大小同时复制字节数，时间用显示的格式。
- 底部放两个按钮：
  - **加入暂存架**：执行 `shelf.add`，作用于当前选中项或光标所在项；
  - **打开暂存架**：执行 `shelf`，切换暂存架面板的显示；面板已打开时，按钮文字为"关闭暂存架"。
- 检查器保持约 200pt 的窄宽，按钮可以纵向排列或用图标按钮，不能溢出。

#### 五、暂存架界面修整（`workflow.go` 的 `shelfView`）
- **问题**：面板 `Width(220)`，加上内边距和边框后，"拷贝到此 / 移动到此 / 清空"三个文字按钮那一行超出了面板宽度，右边被窗口截断（见截图）。
- **修复**：按钮行改为在面板内撑满并允许收缩：三个按钮平分宽度；或改成 SF Symbols 图标按钮加 tooltip；或在宽度不够时折行。条目行的 `×` 靠右对齐，长文件名中间截断，不撑宽面板。整个面板不能超出窗口右边缘，在窄窗口下也要正确显示。
- 用离屏渲染截图核对 220pt 宽度下没有溢出，并写一个测试，断言按钮行的 bounds 都在面板内。

#### 边界
- 不做 Yazi 的 `find`（`/`、`n`、`N`）和 visual 模式。
- 不做永久删除；不引入第三方库；插件只用 sfsymbols。
- 阶段 8 的检查器字段、QuickLook、删除确认、两行状态栏的结构保持不变。

#### 验证
- `go test ./...`、`go test -race ./...`、`go vet ./... && go tool mygo vet .` 全部通过，`TestMenuCoversActions` 也通过。
- 新测试：
  - **键位**：`/` 不再有动作，工具栏没有快速选择按钮；`v` 粘贴，`⌘V` 也能粘贴，`p` 不再有动作。
  - **过滤器**：`f` 打开输入框后边输入边过滤；smart case；`↩` 保留过滤，焦点回到列表；`Esc` 清除；进入其他文件夹后过滤自动清除；光标位置正确。
  - **提示**：默认提示的顺序和内容与第一节一致。
  - **状态栏**：不显示项目统计。
  - **右键菜单**：行和空白处的菜单都没有"在 Finder 中显示"；复制路径时，单个文件、多选、空白处三种情况复制的内容都正确（用 `fakeSys` 检查剪贴板）。
  - **检查器**：点击各个值写入剪贴板，点权限不复制；两个暂存架按钮能正常工作。
  - **暂存架**：220pt 宽度下按钮行不溢出。
- `go tool mygo build -skip-installer` 打包成功，并用 Apple_self_dev 签名。

### 阶段 10：Agent 增强（检测已安装 Agent、启用/禁用与 YOLO 参数、默认 Agent 与 a 键、右键菜单直接列出）
> 来源：goFinder 主会话，2026-11-11（plan mode，用户已批准；参考 Orca，MIT）。

#### 背景
现在只有一个 `agent` 动作：在设定的终端里运行 `config.json` 的 `agentCommand`（默认是 `claude`）。用户希望参考 Orca（https://github.com/stablyai/orca，MIT 许可）大幅增强这部分功能：
1. **设置里列出本机已安装的 Agent**（参照图 1，Orca 的 Installed 列表）：
   - 每个 Agent 可以启用或禁用；
   - 默认带上 YOLO 参数，用户可以改成自己想要的参数。
2. **默认 Agent**：可以设置默认 Agent，在文件列表里按 `a`，就在当前目录打开默认 Agent。
3. **右键菜单**：直接列出所有已启用的 Agent，放在一级菜单，不折叠进子菜单。顺序是"在终端中打开"在最上面，下面依次是各个已启用的 Agent。

调研结果：
- Orca 的目录在 `src/shared/agent-launch/catalog/tui-agent-config.ts`，YOLO 参数在 `permissions/tui-agent-permissions.ts`。
- Orca 的检测方式：自己扫描 PATH，PATH 里找不到时，再查常见的安装目录（`~/.local/bin`、`~/.bun/bin`、`/opt/homebrew/bin`、`/usr/local/bin`、nvm、volta、asdf、mise、pnpm 等）。
- 本机实测装了 5 个：`claude`、`codex`、`pi`、`agy`（都在 `~/.local/bin`），以及 `orca`（`/usr/local/bin`，所以 Claude Agent Teams 也算已安装）。这与图 1 的 5 个一致。
- `codex` 和 `agy` 在用户的 zsh 里是带 YOLO 参数的 alias。我们直接拼出完整参数，不依赖 alias。

#### 一、新的纯逻辑包 `internal/agents`
- **`Catalog`**：按 Orca 的目录搬过来，带注释注明来源和 MIT 许可。每一项包含 `ID`、`Name`、`Detect`（检测用的二进制名及别名）、`Requires`（额外依赖的命令，例如 claude-agent-teams 需要 `claude`）、`Launch`（基础启动命令）、`Yolo`（YOLO 参数）。
  - 常用的几项：
    - claude：`--dangerously-skip-permissions`
    - codex：`--dangerously-bypass-approvals-and-sandbox`
    - agy：`--dangerously-skip-permissions`
    - gemini、crush、cursor-agent、copilot、kimi：`--yolo`
    - qwen：`--approval-mode yolo`
    - amp：`--dangerously-allow-all`
    - aider：`--yes-always`
    - droid：`--auto high`
    - pi、opencode：没有 YOLO 参数
  - 其余条目按 Orca 的表照搬，共 47 项。
  - 只做 CLI，**不做 Orca 的 Chat 模式**。
- **`Detect(dirs []string, exists func(string) bool) []Found`**：在给定目录里逐个找可执行文件，返回已安装的条目及其路径，顺序与目录顺序一致。
  - 目录 = 登录 shell 的 PATH（用 `$SHELL -lc 'printf %s "$PATH"'` 读取，通过 `system` 接口调用，只读一次并缓存），再加上 Orca 的那串安装目录兜底。
  - 原因：从 Dock 启动的 app，PATH 很短。
- **`DefaultCommand(e) string`**：即 `Launch + " " + Yolo`，例如 `claude --dangerously-skip-permissions`。

#### 二、配置（`sys.go` 的 `config`）
- 新增字段：
  - `agents`：`map[id]{ "enabled": bool, "command": string }`。`command` 为空时使用 `DefaultCommand`。
  - `defaultAgent`：Agent 的 ID。
- **旧配置迁移**：旧的 `agentCommand` 如果不是默认的 `claude`，就写进对应条目（按命令里第一个词匹配目录），并设为默认 Agent。
- **新检测到的 Agent 默认启用**。默认 Agent 如果没设或已不可用，就取第一个已启用的。

#### 三、设置：新增「Agent」页（`view.go` 的 `settingsView`）
- 页面标题为"已安装 · N 个"，右上角有「刷新」按钮，用于重新检测。
- 每一行包含：
  - SF Symbols 图标、名称；
  - 一个可编辑的命令输入框（默认是 YOLO 命令）和"恢复默认"按钮；
  - 启用/禁用分段按钮；
  - 默认 Agent 单选（设为默认）。
- 修改即时写回 `config.json`。
- 不显示没安装的 Agent。检测中显示"正在检测…"。
- 「通用」页原来的 `agentCommand` 输入框去掉，改到这一页。

#### 四、`a` 键与 `agent` 动作（`actions.go`、`chrome.go`）
- `agent` 的标题改为"在这里打开默认 Agent"，第一键 `a`，`⌘⌥A` 保留为第二键。
- 打开的目录为：光标在文件夹上时打开这个文件夹，在文件上时打开它所在的文件夹，与现在 `openFolderWith` 的规则一致。
- 状态栏的默认提示**不加** `a`，因为这一列表是用户逐项定过的；速查表和设置的快捷键页里自然会出现它。
- **执行**：沿用现有的 `.command` 脚本方案（`openAgent`）。
  - 脚本首行用用户的登录 shell（`#!/bin/zsh -l` 或 `$SHELL -l`），保证 `~/.local/bin` 在 PATH 里。
  - 正文为 `cd <转义后的 dir> && exec <command>`。`command` 是用户写的 shell 命令行，原样执行，不转义；目录路径仍用 `shellcmd` 做单引号转义。
  - 终端不支持 `.command` 时，退回 Terminal.app，这一点不变。
- 没有任何已启用的 Agent 时，按 `a` 在状态栏提示"没有启用的 Agent，去设置 › Agent 启用"。

#### 五、右键菜单与工具栏
- **文件行右键菜单**（`defaultMenu`）和**空白处右键菜单**（`blankMenu`）中，原来的 `agent` 一项展开为一组一级菜单项：
  - 在终端中打开
  - 在这里打开 Claude
  - 在这里打开 Codex
  - ……（所有已启用的 Agent，按设置页的顺序）
  - 默认 Agent 那一项右侧显示 `a`。
  - 实现方式：在菜单 ID 列表里用占位符 `@agents`，绘制菜单时展开；用户自定义的 `contextMenu`/`blankMenu` 也可以使用 `@agents`。
- **工具栏的 Agent 按钮**：点击打开默认 Agent，按钮旁边加一个下拉，列出已启用的 Agent。
- **原生菜单栏**：「文件」菜单里在"在这里打开默认 Agent"之外，加一个"在这里打开 Agent"子菜单，列出已启用的 Agent，供不习惯右键的情况使用。保证 `TestMenuCoversActions` 通过。

#### 边界
- 不做 Chat 模式，也不做 Orca 的权限档位细分（ask、accept-edits、auto）；只有"默认 YOLO 命令 + 用户自定义命令"。
- 不内嵌终端；不引入第三方库；插件只用 sfsymbols。
- 检测只在启动时和点刷新时运行，后台执行，不在每帧都跑。

#### 分工接口
用 Nested Session 开 **2 个** worktree 会话，都用 Sonnet（`claude-sonnet-5-5`，`use_worktree: true`）。两边先约定好接口再分工：
   - **约定的接口**（新文件 `agents.go`，由 10A 实现）：
     - `type agentItem struct{ ID, Name, Symbol, Command string }`
     - `func (a *app) enabledAgents() []agentItem`：按设置页顺序返回已启用的 Agent
     - `func (a *app) defaultAgentItem() (agentItem, bool)`
     - `func (a *app) detectAgents()`：后台检测，结果经 `a.post` 写回
   - **10A 目录、检测、配置、设置页**：负责第一、二、三节。改动 `internal/agents`、`agents.go`、`sys.go` 的 config、`settingsView` 的「Agent」页，并去掉「通用」页的 `agentCommand`。
   - **10B 调用入口**：负责第四、五节。改动 `actions.go`（`agent` 动作的标题和 `a` 键）、`chrome.go`（`openAgent` 改为接收 `agentItem`、脚本首行用登录 shell、`blankMenu`、工具栏下拉）、`defaultMenu`、`@agents` 的展开、`menu.go` 的子菜单。
     - 10B 先在 `agents_stub.go` 里放一个按约定接口写的假实现（固定返回一个 claude），用来编译和测试；合并时删掉这个文件，换成 10A 的实现。
   - 两边都不碰对方负责的文件。

#### 验证
- `go test ./...`、`go test -race ./...`、`go vet ./... && go tool mygo vet .` 全部通过，`TestMenuCoversActions` 通过。
- 新增测试：
  - `agents`：用临时目录里的假可执行文件验证 `Detect`；Requires 依赖关系（只有 orca、没有 claude 时不算已安装）；`DefaultCommand`；
  - 配置：旧 `agentCommand` 的迁移；新检测到的 Agent 默认启用；默认 Agent 不可用时的回退；
  - 设置页：启用/禁用、修改命令、恢复默认、设为默认之后都写回 `config.json`；
  - `a` 键：用默认 Agent 的命令生成脚本（用 `fakeSys` 检查脚本内容和 `open -a` 参数）；没有启用的 Agent 时给出提示；
  - 右键菜单：行菜单和空白处菜单都在一级列出"在终端中打开 + 各个已启用的 Agent"，禁用的 Agent 不出现，顺序正确。
- 本机实测：设置页检测出 5 个 Agent（Claude、Claude Agent Teams、Codex、Pi、Antigravity），命令带 YOLO 参数；按 `a` 后终端在当前目录启动 claude。
- `go tool mygo build -skip-installer` 打包成功，用 Apple_self_dev 签名。

### 阶段 11：UI 打磨（以 Unlocknote 设计系统为底）
> 来源：goFinder 主会话，2026-11-11（plan mode，用户已批准）。

- **第一步（已产出）**：产出「goFinder 设计系统」，内容见 `docs/design-system.md` 与 `docs/design-tokens.json`；Artifact 只作展示。
  - 用户的决定：
    - 选中行、焦点和主控件跟随 macOS 系统强调色；
    - 品牌青 `#00DAFC` 只做点缀，用于字标句点、Agent 三道光、“已完成”圆点、过滤标记；
    - 整体感受为简洁、高速、实用。
- **第二步（用户 2026-11-11 确认设计系统后开始）**：
  - 主会话先写 `theme.go`：embed `docs/design-tokens.json`，提供 `tok(c, name) ui.Color`（selection/focus/selection-ink 取系统强调色）、`px(name) float32`、`applyTheme(c)`；`theme_test.go` 校验全部 token。界面代码只通过 `tok`/`px` 取色取尺寸，不写 hex。
  - 用两个 Sonnet worktree 子会话套用，组件规范见 `docs/design-system.md` §10：
    - **11A 列表区**：在 `view()` 开头调用 `applyTheme`；文件行（FileRow）、三列视图与焦点边框（ColumnView）、侧边栏（SidebarItem）、路径胶囊（PathBar）、过滤输入框（FilterField）；`internal/icons` 的类别色改用 `type-*` token（icons 包只返回类别，颜色在界面层映射）；Git 字母颜色。负责文件：`columns.go`、`sidebar.go`、`internal/icons`、`view.go`（addressBar、过滤框、`view()` 开头）、`gitstatus.go`。
    - **11B 外框**：工具栏（Toolbar）、分段控件（Segmented）、两行状态栏与键帽（StatusBar，`hints.go`）、绘制菜单（Menu）、检查器（Inspector）、删除确认框（ConfirmDialog）、设置页页眉字标 “goFinder.”（青色句点）、Agent 三道光用 `signal`、“已完成”圆点与过滤标记。负责文件：`chrome.go`、`hints.go`、`inspector.go`、`trash.go`、`workflow.go`（暂存架）、`view.go` 中的 settingsView/helpSheet、`agents.go` 的设置页。
  - 两边都不改 `theme.go`；缺 token 时在汇报里提出，由主会话补进 `docs/design-tokens.json` 和设计系统。
- **边界**：第一步只改文档；不打包字体，不新增插件。

### 阶段 12：工具栏优化（默认按钮重排 + 设置里的「工具栏」页）
> 来源：goFinder 主会话，2026-11-11（plan mode，用户已批准）。

#### 背景
用户想按现有功能重新考虑工具栏上放什么。现在的默认工具栏（`chrome.go` 的 `defaultToolbar`）是：
`‹ › ｜ ＋新建▾ ⇅排序▾ ◉隐藏文件 ｜ 暂存架 终端 ✴Agent▾`；视图切换和检查器开关在状态栏。

讨论后用户的决定：
- 右端加「过滤」按钮，点击等同于按 `f`；
- 暂存架开关移到右端，与右侧的暂存架面板对齐；
- 设置里加一个「工具栏」页，可以勾选要显示的按钮、调整顺序，结果写回 `config.json` 的 `toolbar`；
- 保留 **上级 ↑**、**移到废纸篓**、**Workspace** 三个按钮；
- 不加"在编辑器中打开"。
- 状态栏已有的视图切换、检查器开关不再在工具栏重复放；拷贝、剪切、粘贴用单键和右键菜单，不上工具栏；侧边栏常驻，不需要开关。

#### 一、新的默认工具栏（`chrome.go`）
```
‹ 后退  › 前进  ↑ 上级 ｜ ＋新建▾  ⇅排序▾  ◉隐藏文件 ｜ 🗑移到废纸篓 ｜ ▭终端  ✴Agent▾ ｜ ⟶(@flex) ｜ ⧉Workspace  ☰暂存架  ⌕过滤
```
- `defaultToolbar` 改为：
  `back, forward, up, -, @new, @sort, hidden, -, trash, -, terminal, agent, @flex, workspace, shelf, filter`
- 按钮说明：
  - **过滤**：动作 `filter`，SF Symbol 用 `line.3.horizontal.decrease.circle`。过滤生效时为开启态（`surface-200` 常驻）。点击 = `a.run("filter")`，即在活动窗格当前栏打开过滤框。
  - **上级**：动作 `up`，符号沿用 `arrow.up`；在根目录时禁用。
  - **移到废纸篓**：动作 `trash`，符号 `trash`，颜色 `ink`。只有悬停时图标用 `danger`；没有选中项、或在 zip 里时禁用。仍然走删除确认框。
  - **Workspace**：动作 `workspace`，符号 `square.on.square`。
- `actionSymbols` 补上 `filter` 的符号。
- **旧配置**：`config.json` 里已经写了 `toolbar` 的用户，照用自己的配置，不受影响。没写的用户，自动用新的默认工具栏。

#### 二、设置的「工具栏」页（`view.go` 的 `settingsView`，新增第 4 个分页）
- **左列"当前工具栏"**：按顺序列出当前的条目，每行显示图标、名称和快捷键。
  - 每行有：
    - 上移、下移按钮；
    - 移除按钮；
  - 分隔条目 `-` 显示为"分组间隔"，`@flex` 显示为"推到右侧"，可以调整位置，也可以移除。
- **右列"可添加"**：列出可以放进工具栏的条目，即 `actionSymbols` 里有符号的动作，加上 `@new`、`@sort`、"分组间隔"、"推到右侧"。已经在工具栏上的条目不再列出（"分组间隔"除外，它可以有多个）。点"添加"放到末尾。
- 底部有"恢复默认"按钮，把 `toolbar` 设为 nil（即使用 `defaultToolbar`）。
- 每次修改都立即写回 `config.json` 的 `toolbar`，工具栏实时刷新。
- 交互用键盘也能完成：方向键选行，再用按钮完成上移、下移和移除。不做拖拽排序，以免增加复杂度。
- 样式按设计系统：行高 `row-height`，列表底色 `bg`，选中行用 `selection`，按钮底色 `surface-200`，圆角 `radius-control`。

#### 三、文档
- `README.md` 中的工具栏说明改成新的默认工具栏，并写上设置里的「工具栏」页。
- `docs/design-system.md` §10 Toolbar 组件规范里的按钮清单同步更新，并在 Artifact 的 `components/Toolbar` 中同步说明和预览。
- `docs/plan.md` 新增阶段 12 小节并更新状态行。

#### 边界
- 不加"在编辑器中打开"；不加拷贝、剪切、粘贴，不加视图切换、检查器、侧边栏开关。
- 不改键位，不改状态栏和右键菜单。
- 不做拖拽排序。

#### 验证
- `go test ./...`、`-race`、`go vet ./... && go tool mygo vet .` 全部通过，`TestMenuCoversActions` 通过。
- 新测试：
  - 默认工具栏的按钮顺序；
  - 过滤按钮能打开过滤框，过滤生效时为开启态；
  - 上级按钮在根目录时禁用；
  - 废纸篓按钮在没有选中项时禁用，点击后弹出确认框；
  - 设置页的上移、下移、移除、添加、恢复默认，都会写回 `config.json`，工具栏同步变化；
  - 写了 `toolbar` 的旧配置不受影响。
- 打包并签名，然后请用户试用。

### 阶段 12 补充：工具栏开关（默认隐藏）与默认深色主题
> 来源：goFinder 主会话，2026-11-11；用户要求"给工具栏加一个开关，默认隐藏工具栏，默认为暗色主题"。

- **工具栏开关**：新增动作 `toolbar`，快捷键 ⌃⌘B，放在「显示」菜单的勾选项和设置「通用」页的开关里，保存为 `config.json` 的 `showToolbar`，默认隐藏。
- **外观**：`config.json` 的 `appearance` 可选 `dark`（默认）、`light`、`system`，在设置「通用」页的分段控件中切换。
- **整窗深色的做法**：MyGo 没有强制窗口外观的接口，用户选择隐藏系统标题栏，改由应用自己绘制。
  - 窗口使用 `TitleBarHidden`，标题栏由 `titleBar` 绘制，可拖动窗口：工具栏隐藏时是一条显示文件夹名的标题条，工具栏显示时工具栏本身就是标题栏。
  - 外观与 macOS 不一致时，侧边栏不用毛玻璃，改用实底。
  - 强调色仍然取系统设置。
- **边界**：键位（除新增的 ⌃⌘B）、布局、其他功能都不变。

### 阶段 13：列表字号设置、速查表收窄、工具栏即时提示与 Agent 合并按钮
> 来源：goFinder 主会话，2026-11-11（plan mode，用户已批准）。

#### 背景
工具栏开关和深色主题做完后，用户试用并提出三点：
1. 列表字体偏小，图标也太小、不好认，希望设置里能调字号。
2. 按 `?` 弹出的快捷键速查表太宽，而且几组快捷键挤在一起分不清。
3. 鼠标移到工具栏按钮上时，提示要**立即**出现（0 等待），说明这个按钮是做什么的；"启动默认 Agent"和"选择 Agent"两个按钮要合成一个，体现它们是一组功能。

现状（已查代码）：
- 列表字号是常量：`columns.go` 里 `listFontSize = 13`，`listLineHeight = 1.25`；字形列宽 `iconW = 18`。字形和文件名同字号，看起来偏小。
- 速查表：`view.go` 的 `helpSheet` 宽 600。`keyList` 的每一行是"标题 + 各组键帽"。不同组合键的键帽（`keycaps`）直接并排，和同一组合里的多个键（如 `n n`）看不出区别。
- 工具栏提示用的是主题默认的 `TooltipDelay`，即 0.6 秒。MyGo 文档：设成负数就立即显示；只想作用于工具栏，可以用 `ui.Themed` 给局部单独设主题。
- Agent 按钮：`chrome.go` 里分成 `tb:agent`（三道光）和 `tb:agent.more`（▾）两个独立按钮。

#### 一、列表字号设置
- `config.json` 新增 `listFontSize`：可选 13 / 14 / 15 / 16 / 18，**默认 14**（用户觉得 13 偏小）。0 或缺省时按默认值处理。
- `columns.go`：
  - 把常量 `listFontSize` 改成 `a.listSize()`；行高按 `listSize × 1.25` 取整，并且不低于 token `row-height` 22。
  - **字形放大**：字形单独用 `listSize + 3`；`iconW` 改为按字号计算（约 `listSize × 1.45`），垂直方向与文件名居中对齐。
  - 列宽测量的缓存键（`charWFont`）带上字号，改了字号会重新测量。路径胶囊、过滤框同样用这个字号。
- **设置「通用」页**加一行"列表字号"，用分段控件（复用 `chrome_style.go` 的 `segmented`）给出 13 / 14 / 15 / 16 / 18 五档。改动立即生效并保存，做法同"外观"那一行（`appearance.go` 的 `appearanceSettings`）。
- 设计系统同步：
  - `docs/design-system.md` 里 `list` 样式的说明改成"默认 14，可在设置中调整为 13–18；字形比文字大 3pt"；
  - `docs/design-tokens.json` 的 `list` 样式 fontSize 改为 14px；
  - 设计系统 Artifact 的 tokens 和 README 同步修改。

#### 二、速查表收窄、快捷键分开
- `helpSheet` 宽度从 600 改为 **440**。
- `keyList` 的每一行：
  - 左边是功能名，单行，超长用 `…` 截断，占满剩余宽度；
  - 右边是快捷键区，靠右对齐。
- **同一个组合内**的多个键帽（`n n`）间隔 `space-0.5`（2px），看起来是一组。
- **不同组合之间**（例如 `c` 和 `⌘C`）间隔 `space-3`（12px），中间放一个 `muted` 色的"/"，一眼能看出是两种按法。
- 新增一个公共函数 `keyGroups(c, chords)` 做这种排列，速查表和设置的快捷键页都用它。
- 设置里的快捷键页保持原宽 600，只用新的排列方式。

#### 三、工具栏：即时提示 + Agent 合并按钮
- **即时提示**：工具栏这一行用 `ui.Themed` 包起来，主题设为当前主题的副本，只把 `TooltipDelay` 改成 `-1`（立即显示）。窗口其他地方的提示仍是默认延迟。
- **提示内容**：写成"名称 · 一句话说明 · 快捷键"，例如：
  - "过滤 · 只显示名称包含输入文字的项 · f"
  - "移到废纸篓 · 删除选中项（可在废纸篓恢复） · d"
  - "在这里打开 Agent · 在当前文件夹启动 Claude · a"
  新增 `toolbarTips` 映射，给默认工具栏和"可添加"列表里所有按钮都写上说明；没有说明的按钮回退为动作标题加快捷键（即现有的 `tip`）。
- **Agent 合并按钮**：把 `tb:agent` 和 `tb:agent.more` 合成一个**分体按钮**：外层是一个 `surface-200` 底、`radius-control` 圆角的容器，左段是三道光（`signal` 色）加上默认 Agent 的名字（如 "Claude"），中间一条 1px `separator` 竖线，右段是 ▾。
  - 左段：在这里打开默认 Agent，提示为"在这里打开 Claude · a"。
  - 右段：弹出已启用 Agent 的菜单（`agentsMenu`），提示为"选择其他 Agent"。
  - 悬停时只高亮鼠标所在的那一段。没有启用任何 Agent 时，左段禁用，提示"去设置 › Agent 启用"。
  - 设置「工具栏」页里，这个合并按钮仍然作为一项（`agent`）显示和排序。

#### 边界
- 键位不变；工具栏按钮的种类和顺序不变；状态栏和右键菜单不变。
- 不改设计系统里除 `list` 字号以外的 token。
- 不引入第三方库，插件只用 sfsymbols。

#### 验证
- `go test ./...`、`go test -race ./...`、`go vet ./... && go tool mygo vet .` 全部通过。
- 新测试：
  - 字号：默认 14；设置页切换后行高、字形大小、列宽都变；写回 `config.json`；旧配置没有该字段时为 14。
  - 速查表：宽度 440；不同组合之间有"/"分隔；同一组合内的键帽间距是 2px。
  - 工具栏：提示延迟为负值（立即显示），只作用于工具栏；提示文字里有说明和快捷键；Agent 合并按钮左段打开默认 Agent，右段弹出菜单，没有启用 Agent 时左段禁用。
- 打包并用 Apple_self_dev 签名；离屏截图核对速查表和工具栏；真机由用户试用。

### 阶段 13 补充：启动文件夹、键帽字体

> 来源：goFinder 开发会话（2026-11-11），用户提出。
- 右键文件夹（文件列表中的行）在菜单末尾加「启动时从这里开始」：选中后写入 `config.json` 的 `startDir`，以后每次启动打开它；当前已选的文件夹打勾，再选一次取消。启动优先级：命令行/打开文件夹 > `startDir`（文件夹仍存在时）> Finder 新窗口位置 > `~`。zip 内的文件夹不显示此项。
- 速查表等处的键帽改用系统字体加 1px 字距：等宽 Nerd Font 把 ⇧⌘⌥⌃ 各压进一个字符格，`⇧⌘H` 挤成一团。
- 边界：不改键位、不改菜单其他项；上级栏和预览栏里的文件夹右键菜单不加此项。

### 改名：MygoFinder → goFinder

> 来源：goFinder 开发会话（2026-11-11），用户提出。
- 应用名、窗口与菜单、文档、Go 模块名（`gofinder`）统一改为 goFinder。
- 数据目录按应用名命名，启动时把旧的 `~/Library/Application Support/MygoFinder/` 和 iCloud 云盘的 `MygoFinder/` 文件夹移到新名字下（`rename.go` 的 `moveOld`：只在新文件夹不存在或为空时移动）。
- 边界：Bundle ID 保持 `dev.mygofinder.app`，以免丢失窗口位置和"默认文件浏览器"设置；仓库所在文件夹名不改。
- **已完成（2026-11-12，仓库公开为 github.com/aoyamashou/goFinder）**：Bundle ID 改为 `io.github.aoyamashou.gofinder`（`mygo.json`、`sys.go` 的 `fallbackBundleID`、测试）。旧 ID 下设过"默认文件浏览器"的需要重新设一次；数据目录按应用名存放，不受影响。

### 阶段 14：空格快速查看、第 3 栏鼠标、检查器重做、默认浏览器修复、完全磁盘访问

> 来源：goFinder 开发会话（2026-11-12），用户提出并批准计划。
#### Context
用户试用 goFinder 后提出 6 点（2026-11-12）：
1. 设为默认文件浏览器后，提示要执行 `killall Finder` 才生效 → 提供「复制命令」按钮。
2. 空格现在是 Yazi 式多选（`select.toggle`）→ 去掉，改成 Finder 式：空格直接快速查看。
3. 经常遇到权限问题 → 设置里加「获取完全磁盘访问权限」按钮。
4. 第 3 栏（预览栏里的子项列表）鼠标点不动 → 像对鼠标友好的 Yazi 那样可点。
5. 设成默认浏览器后，其他应用「在 Finder 中显示」调不起 goFinder。
6. 检查器按参考图（Finder 风格属性面板）重做，值仍可点击复制。

已确认：空格用系统快速查看（`qlmanage -p`）；检查器勾选框**可以修改**文件（所有者、组只读）。

#### 一、默认浏览器（`view.go` 设置「通用」页约 660–690 行，`app.go` `setFileViewer`）
- 说明文字旁加按钮「复制命令」：复制 `killall Finder` 到剪贴板（用现有 `clip.go` 的写剪贴板方法），状态栏提示「已复制，粘贴到终端运行」。不在本机执行。
- **「在 Finder 中显示」调不起的问题，先诊断再修**：
  - 用一个 Swift 小脚本调用 `NSWorkspace.activateFileViewerSelectingURLs` / `selectFile:inFileViewerRootedAtPath:`，记录 goFinder 收到什么（`OnOpenFile` 的路径、`OnOpenURL`、还是什么都没收到）。
  - 检查 LaunchServices：改名后 `.app` 路径由 `MygoFinder.app` 变成 `goFinder.app`，`lsregister -dump` 看 `dev.mygofinder.app` 是否仍指向已删除的旧路径；必要时在设为默认时对当前 bundle 调 `lsregister -f`。
  - `mygo.json` 的 `CFBundleDocumentTypes` 只声明了 `public.folder`；reveal 一个文件时可能需要再声明 `public.item`（`LSHandlerRank: None`，不抢默认打开方式）才能收到。
  - 收到文件路径时 `openPath` 已能打开其上级并选中它；确认应用已在运行和冷启动两种情况都能把窗口带到前台。
  - 说明里写清限制：直接用 AppleScript 指名 Finder 的应用（如 `tell application "Finder"`）无法接管。
  - **边界**：测试里用 `fakeSys`，不在本机实际改 NSFileViewer；诊断脚本只发 reveal 请求。

#### 二、空格 = 快速查看（`actions.go`、`app.go`、`sys.go`）
- 删除动作 `select.toggle` 及其键位、菜单项、提示；多选保留 ⌘ 点击、⇧ 方向键、⌘A。
- 新动作 `quicklook`「快速查看」，键 `Space`（第二键位 `Cmd+Y`），放进 `menuBar`「显示」菜单，状态栏提示 `␣ 预览`。
- `system` 接口加 `QuickLook(paths []string) (stop func())`：起 `qlmanage -p <选中项>`（stdout/stderr 丢弃）；再按空格，或光标移动后再按，先结束上一个进程再开新的；goFinder 退出时结束它。zip 内的文件先用现有的解压到缓存逻辑。
- `fakeSys` 记录调用，测试验证。

#### 三、完全磁盘访问（设置「通用」页）
- 一行「完全磁盘访问」：显示状态（能读 `~/Library/Application Support/com.apple.TCC/TCC.db` 即已授权）＋ 按钮「打开系统设置」→ `open "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"`。说明：「在列表里打开 goFinder 的开关，然后重启 goFinder」。
- 不用 permissionflow 插件（插件只允许 sfsymbols）。

#### 四、第 3 栏鼠标友好（`columns.go` `previewBody` 的 `pvDir` 列表）
- 单击子项：进入该文件夹（当前栏变成它），光标落在点中的项上——等同 Yazi 点击右栏。
- 双击子项：文件夹则再进入，文件则打开（同 → 键）。
- 拖放、右键菜单保持现有行为；右键子项给出文件行的菜单（`contextMenu`）而非空白处菜单。
- 第 1 栏（上级）点击已有行为，核对一致即可。

#### 五、检查器重做（`inspector.go`、`internal/finfo`）
按参考图：
- 顶部：文件名（标题字号，粗）＋ 第二行「种类 - 大小」（muted）。
- **通用**（可折叠，`ui.Collapsible` 或现有折叠写法，标题行右侧 chevron）：路径、大小（`1.5 MB（1 469 302 字节）`）、修改时间、创建时间、添加时间、扩展名、隐藏扩展名 ☐、隐藏 ☐。
- **权限**（可折叠）：R W X 三列 × 所有者/组/其他 的勾选框网格；「八进制」行显示 `rw-r--r--` 和 `644`；所有者、组（只读文本）；锁定 ☐。
- 左列标签 muted、右列值右对齐，行间 1px `separator` 分隔线。
- 点击任意值复制（沿用现有点击复制和状态提示）。
- **勾选框可修改**：
  - 权限 → `os.Chmod`；隐藏 → `chflags hidden/nohidden`；锁定 → `chflags uchg/nouchg`；隐藏扩展名 → `xattr`/`SetFile -a E` 不可用时用 `NSURLHasHiddenExtensionKey` 等价方式（用 `osascript` 设 `extension hidden`），实现前确认可行，不行就该项只读并说明。
  - 都放 `a.async`，失败显示错误；zip 内或无权限时勾选框禁用。
  - `finfo` 新增：`Added`（`mdls kMDItemDateAdded` 或 `getattrlist`，读不到为空）、`Hidden`、`Locked`（`st.Flags` 的 `UF_HIDDEN`/`UF_IMMUTABLE`）、`HideExt`、`Ext`。纯逻辑写进 `internal/finfo` 并加测试。
- 宽度：token `inspector-width` 200 → 280（`docs/design-tokens.json`、`docs/design-system.md` 同步）。

#### 边界
- 不改其他键位；`c c`、`n n` 等不变。
- 不引入第三方库或新插件。
- 不在本机实际修改 NSFileViewer、不执行 `killall Finder`。
- 删除仍只进废纸篓。

#### 执行方式
1. 主会话把阶段 14 写进 `docs/plan.md` 并提交。
2. 两个 Sonnet 子会话，各自新 worktree：
   - **14A**：一、二、三（`actions.go`、`app.go`、`menu.go`、`hints.go`、`view.go` 设置页、`sys.go`、`main.go`、`mygo.json`）。
   - **14B**：四、五（`columns.go`、`inspector.go`、`internal/finfo`、设计 token）。
3. 主会话合并、全量检查、打包签名，请用户试用；确认后删 worktree，在 plan.md 状态行标 commit 号。

#### 验证
- `go test ./...`、`go test -race ./...`、`go vet ./... && go tool mygo vet .`。
- 新测试：复制命令写剪贴板；空格调 `QuickLook`（fakeSys）、`select.toggle` 已不存在、`TestMenuCoversActions` 通过；磁盘访问按钮打开正确 URL；第 3 栏单击进入并定位、双击打开；检查器各字段（含 Added/Hidden/Locked）、勾选后 chmod/chflags 生效（临时目录文件）、八进制显示；`finfo` 单测。
- 诊断脚本验证「在 Finder 中显示」：发 reveal 请求后 goFinder 打开并选中文件（只在 NSFileViewer 已由用户设为 goFinder 时由用户确认）。
- 打包 + Apple_self_dev 签名，离屏截图核对检查器与设置页。

### 阶段 15：设置「关于」页与 Debug 编号

> 已实现 2d6bbff（打包签名未执行）。
> 来源：goFinder 开发会话（2026-11-12），用户提出。
**为什么**：用户试用时想精确指出界面上的某个元素，需要一个开关让各元素显示编号，沟通时说「7 号」即可。

**要改什么**
- 设置新增第 5 页「关于」（`view.go` 的 `ui.Tabs` 加一项，`pageAbout`）：
  - 应用名 goFinder 与版本（`mygo.json` 的 version，构建时读 `mygo.App` 提供的版本，读不到写「开发版」）、MyGo 版本（`debug.ReadBuildInfo` 里 `github.com/egoist/mygo` 的版本）、Bundle ID、数据目录（点击复制）。
  - 一个「Debug 编号」开关（按钮或复选框）。只在本次运行有效，不写 `config.json`；开启时状态栏左侧显示「Debug」标记，点击它关闭。
- **编号规则**：
  - 编号是**固定的**：一张表 `debugIDs`（新文件 `debug.go`）把元素名映射到编号，同一个元素永远是同一个号，不随显示与否变化；按区域分段：1–19 标题栏与工具栏（每个按钮各一个号），20–29 侧边栏（整体、三个分组），30–39 地址栏（胶囊、面包屑、Git 分支），40–59 三列（上级栏、当前栏、第 3 栏预览/子项、检查器及其两组、暂存架面板），60–69 状态栏（结果句、键帽提示、视图切换按钮、侧栏按钮），70–89 对话框与弹层（速查表、设置各页、重命名/新建、废纸篓确认、Workspace、右键菜单）。
  - 列表里的行不逐行编号，只给整列一个号；多窗格时号后加窗格序号（如 `41·2`）。
  - 徽章：元素左上角的小胶囊，`signal` 底 `signal-text` 字，11px 等宽数字，不拦截点击、不改变布局（画在元素之上，例如 `ui.Overlay`+`AttachTo` 或元素的绘制回调，按 MyGo 源码选不影响布局的方式）。
  - 统一入口 `a.dbg(c, e ui.Element, name string) ui.Element`：关闭时原样返回，零开销。
- **编号表文档**：`docs/debug-ids.md` 列出全部编号、元素名、所在文件和函数，由测试 `TestDebugIDsDoc` 保证与 `debugIDs` 一致（编号不重复、文档不缺项）；README 加一行用法。

**边界**
- 不改任何现有布局、键位与颜色 token；关闭时界面与现在完全一致。
- 不引入第三方库或插件；不用 MyGo 自带的 `Element.Debug()` 边框（太杂）。

**验证**
- `go test ./...`、`-race`、`go vet ./... && go tool mygo vet .`。
- 测试：关于页显示版本与 Debug 开关；开启后主要元素出现徽章（用 `tt.HasText` 查编号）、关闭后消失；`TestDebugIDsDoc`；开启时布局尺寸不变（对比某元素 bounds）。
- 用 `MYGO_SHOTS` 生成开启 Debug 的主界面、设置页离屏截图核对；打包签名。

### 阶段 16：鼠标按 Finder 习惯，键盘按 Yazi 习惯

> 来源：goFinder 开发会话（2026-11-12）。用户提出：键盘按 Yazi，目标是光标所在项；鼠标按 Finder，右键点哪项，就以哪项为目标。我盘点了 Finder 的鼠标习惯，用户从中挑选要做的项。

**改什么**
- 右键以点中的项为目标：上级栏（第 1 栏）的行现在有自己的菜单。选中菜单项后，先回到上级目录、光标停在这一项上，再执行动作（同第 3 栏 `pvEnterNow` 的做法）。以前右键会落到整栏的空白处菜单，「复制路径」得到的是这一栏所在文件夹。当前栏和第 3 栏原本就正确，补了测试。
- 工具栏的过滤按钮亮着时，再点一次就关闭过滤并清空过滤词。以前只能打开；`f` 键行为不变。
- 鼠标侧键、触控板双指轻扫：后退、前进（`back`/`forward` 的第二键 `Back`/`Forward`，`internal/keymap` 增加这两个键名）。
- 点当前栏里文件行以下的空白处：清空多选，光标保留。做法：用 `ObserveInput` 接收按下事件，再拿上一帧各行的 `Bounds` 判断点中了哪一行。不在列表上调 `Pressed`/`Clicked`，也不在行上调 `PointerPosition`：前两者会让列表接管空格，后者会让行抢走双击。

**边界**
- 用户明确不做：慢速单击改名（做了后撤掉）、拖放悬停自动展开、拖动调栏宽、⌥/⌘ 拖放强制复制或移动、⌘ 双击开新标签。
- 键盘行为和键帽提示不变。

**验证**
- `phase16_test.go`：
  - 上级栏、当前栏、第 3 栏右键后「复制路径」，复制的都是被点的项；
  - 过滤按钮点一次打开，再点一次关闭；
  - `KeyBack`/`KeyForward` 能后退、前进；
  - 点空白处后多选清空、光标保留。
- 全量 `go test ./...`、`-race`、`vet`、`mygo vet`。

### 阶段 17：拼音首字母过滤、` 回到启动位置、失焦提示

> 来源：goFinder 开发会话（2026-11-12）。用户提出：过滤只适合英文用户，希望中文名能用拼音首字母过滤；给「启动时从这里开始」配快捷键 `` ` ``；应用失去焦点时要有视觉提示（做法由开发会话决定）。

**改什么**
- 新包 `internal/pinyin`：
  - `gen/gen.swift` 调用 macOS 的 `CFStringTransform`，生成 U+4E00–U+9FFF 每个汉字的拼音首字母码表 `table.go`。码表一字一字节，约 21 KB，有 68 个字没有读音；生成后提交进仓库，平时构建不需要 Swift。
  - 提供 `HasHan`、`Initials` 两个函数。
- `pane.MatchName`：字面没匹配上，而且过滤词是全小写 ASCII、名字里有汉字时，再按拼音首字母匹配一次。smart case 不变。
- 新动作 `go.start`「启动位置」，键 `` ` ``，放在「前往」菜单。前往 `cfg.StartDir`；没设置或目录已不存在时，前往 `~`。
- 失焦提示：用 `c.WindowActive()` 判断，整窗变灰（规范见 `docs/design-system.md`）。不加遮罩，也不加动画。

**边界**
- 不做全拼匹配。多音字只取系统默认读音。
- 不引入第三方库。

**验证**
- 单元测试：`internal/pinyin`（`Initials`/`HasHan`/码表长度）、`pane.TestMatchName` 新增的中文用例。
- `phase17_test.go`：按 `f` 后输入 `mxcx` 只剩「模型查询.txt」；`` ` `` 的三种情况；失焦时焦点边框不再是蓝色，回到前台后恢复。
- 基准：一万个中文名过滤一次约 2.2ms（`BenchmarkFilterChinese10k`）；`Open10k`、界面层 `10k` 没有变慢。
- `go test ./...`、`-race`、`vet`、`mygo vet` 全部通过。

## 阶段 18：自动更新
> 来源：开发会话（2026-11-12）。用户要求参照 MyGo 的自动更新方式加上这个功能；仓库从这个阶段起公开在 github.com/aoyamashou/goFinder，用 updater 插件。

**要改什么**
- 更新源：公开仓库的 GitHub Release，标签为 `v<版本>`。`mygo.json` 的 `updates` 里写 `publicKey` 和 `github: "aoyamashou/goFinder"`。
- 密钥：用 `go tool mygo keygen` 生成，放在默认位置（`~/Library/Application Support/mygo/update-keys/`），**私钥不进仓库**。打包时通过环境变量 `MYGO_UPDATER_PRIVATE_KEY` 传入；`mygo.json` 不写 `privateKey`，所以别人克隆后没有密钥也能照常构建。
- 界面：用 `mygo.Use(native.Plugin)`（updater 的原生 UI 窗口，不用 WebView）。
  - 启动 10 秒后在后台检查一次，之后每天检查一次。
  - 窗口提供「安装 / 稍后提醒 / 跳过此版本」，文字跟随系统语言。
  - 应用菜单「关于 goFinder」下面加「检查更新…」（`updater.MenuItem()`）。
  - 设置「关于」页加「检查更新…」按钮；发现新版本时，这一行显示「有新版本 x」。
- 发布说明：新建 `CHANGELOG.md`，每个版本一节 `## 0.x.y`，更新窗口会显示其中的内容。
- 发版流程：
  1. 改 `version`，在 CHANGELOG 加一节；
  2. `MYGO_UPDATER_PRIVATE_KEY=… go tool mygo build -skip-installer -upload` 生成 draft release；
  3. `gh release edit v<版本> --draft=false --latest`；
  4. 按 CLAUDE.md 装到 `/Applications`。
- 项目规则：插件白名单加上 updater。

**边界**
- 不做 beta 通道或预发布版本，不自建服务器或 S3。
- 不改现有键位和界面布局。开发版（`mygo dev`）不检查更新。

**验证**
- `go test ./...`、`vet`、`mygo vet`；`TestMenuCoversActions` 继续通过。
- 发布 v0.18.0：Release 里有 `goFinder-0.18.0-darwin-arm64.tar.gz` 和 `update-darwin-arm64.json`，安装后「检查更新…」显示已是最新版本。
- 下一个修订版（0.18.1）发布后，从 0.18.0 实际更新一次（含增量包），确认重启后关于页显示新版本。

## 阶段 18 后修订：右键先选中、推出磁盘、状态栏提示（commit dadbc20）

> 来源：开发会话，2026-11-12，用户提出三点。

**改动**
- **右键先选中**：右键点到某一行时，像左键点了一下一样，这一行立刻显示为选中，菜单命令作用于它。
  - 当前栏由 `ui.List` 自己选中。
  - 上级栏和第 3 栏如果真的点进去，列会移动，菜单也会关掉，所以只在原地高亮：`rightclick.go` 的 `menuHot`。菜单关闭（选了命令或按 Esc）后高亮自动消失。
- **推出磁盘**：侧边栏「位置」里的每个卷都有 ⏏ 按钮（SF Symbol `eject`），右键菜单里有「推出“名称”」，和访达一样。
  - 推出前，停在这个卷里的标签页先回到 `~`，免得卷被占用。
  - 推出通过 `system.Eject` 执行：先 `diskutil eject`，失败再 `diskutil unmount`（网络共享只能卸载）。
  - 结果显示在状态栏；之后重新读取位置列表。
- **状态栏提示**：加上 `a` 启动智能体、`` ` `` 回到起点（这两个动作改为 `Hint: true`）。

**边界**
- 不改键位，也不改左键行为。
- 不判断卷是否可以推出（访达会隐藏内置分区的推出按钮）：位置里列出的卷都显示 ⏏，推不出时在状态栏报错。

**验证**
- `phase19_test.go`：右键高亮会出现，按 Esc 或选了命令后消失；推出按钮能调用 `Eject`，标签页回到 `~`，失败时报错。
- `phase10b_test.go`：默认提示里有 `agent` 和 `go.start`。
- `go test -race ./...`、`vet`、`mygo vet`。

## 阶段 18 后修订二：精简键位、常见应用列表、两行提示、菜单显示单键（commit ff600b7）

> 来源：开发会话，2026-11-12，用户提出。

**改动**
- 去掉快速查看的 `⌘Y` 和重命名的 `F2`。
- 设置里的终端、编辑器下拉框不再列出全部应用，只列出已安装的常见应用（`view.go` 的 `commonTerminals`、`commonEditors`，如终端、iTerm、Ghostty、Kaku、Warp；VS Code、Cursor、Zed、Sublime Text 等）。当前选的应用不在名单里时也保留在列表中。
- 状态栏的键位提示固定为两行（`hints.go` 的 `hintLines`）：
  - 第一行：预览、过滤、回到起点、智能体、帮助；
  - 第二行：拷贝、拷贝路径、剪切、粘贴、新文件夹、新文件、重命名、删除。
  - 每项显示动作的第一个键，跟随用户改过的键位。按住修饰键时仍在第一行列出该修饰键的全部快捷键；过滤时第二行留空。
- 右键菜单显示动作的第一个键（单键优先），放在右侧快捷键的位置；同键双击（`c c`）用 `Trailing` 显示。

**边界**
- 原生菜单栏不能把单键设成真正的快捷键：菜单会抢在窗口之前拿到按键，输入框里就打不出这些字母（见 `menuUnsafeKeys`）。所以单键仍然跟在标题后面，没法右对齐；带 ⌘ 的键照旧由系统右对齐显示。
- 不改其他键位。

**验证**
- `phase19_test.go`：`TestCommonApps`、`TestMenuSingleKeys`；`phase9a_test.go` 检查两行提示的顺序；`phase14a_test.go` 确认 ⌘Y 已经不再绑定。
- `go test -race ./...`、`vet`、`mygo vet`。

## 阶段 18 后修订三：设置面板定尺寸、默认窗口、提示文案、检查器向右伸展（commit f5bb49b）

> 来源：开发会话，2026-11-12，用户提出。

**改动**
- **设置面板大小固定**：640×600，窗口太小时缩到窗口的 90%。
  - 页面内容在面板里滚动（`setbody`）。
  - 各页的按钮统一放在面板底部，由 `settingsFooter` 绘制：通用页是「取消」「保存」，工具栏页是「恢复默认」「完成」，其他页是「完成」。
  - 按钮不会再跑到面板外面，切换页面时面板大小也不变。
- **默认窗口**：1035×692，取自用户调好的窗口大小。默认外观原本就是深色，不需要改。
- **状态栏第二行**前面加上「选中后：」，表示后面这些操作都要先选中对象。
- **检查器**：打开时窗口向右加宽「检查器宽度 + 1px 分隔线」，关闭时再缩回去，原来几栏的宽度不受影响（`fitInspector`、`growRight`）。
  - 右边贴到屏幕边缘时，窗口改为向左移。
  - 全屏或最大化的窗口不调整大小。

**边界**
- 不改设置项本身，也不改别的对话框。

**验证**
- `phase19_test.go`：
  - `TestSettingsFixedSize`：每一页的按钮都在同一位置，而且在窗口之内；
  - `TestInspectorWidensWindow`：窗口先加宽 281，再缩回 281。
- `phase14a_test.go`：通用页先滚到底，再点按钮。
- `phase9a_test.go`：第二行提示以「选中后：」开头。
- `go test -race ./...`、`vet`、`mygo vet`。

## 阶段 18 后修订四：强调色固定为品牌青、标题栏路径着色（commit cfa5a52）

> 来源：开发会话，2026-11-12，用户提出。

**改动**
- 强调色不再跟随 macOS，固定为品牌青 `#00DAFC`。
  - 选中行、焦点框、主按钮、分段控件都用这个颜色，青底上的字一律用黑字（`selection-ink`）。
  - `theme.go` 的 `applyAppearance` 每帧把 `Accent` 和 `AccentText` 设为这两个 token。
- 标题栏的文件夹名（隐藏工具栏时显示）：
  - goFinder 在前台时用品牌青 `signal-text`（浅色主题下是深一些的青，保证看得清）；
  - 失焦时，深色主题用 `muted` 灰，浅色主题用 `ink` 黑。

**边界**
- 不改其他 token 和布局。

**验证**
- `phase19_test.go`：`TestBrandAccentAndTitle`，深色和浅色主题都检查。
- `phase11b_test.go`：完成小圆点只在状态行里数像素，因为列表的焦点框现在也是青色。
- `go test -race ./...`、`vet`、`mygo vet`。

### 阶段 18 后修订五：去掉标签页，改为切换工作区；双栏各自的起点
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit 7fc6df4。

**要改什么、为什么**
1. 双栏（左右、上下）时，`` ` `` 回到的起点要两个窗格分别记忆，不再共用一个。
2. 去掉多标签：goFinder 只有一个窗口，每个窗格只有一个文件夹，因为已经有双栏。
3. 原来切标签的键改为切工作区。
4. 状态栏结果句那一行的末尾显示起点：单栏「起点：~」，左右「左：… 右：…」，上下「上：… 下：…」。没设起点时就是 Home（`~`）。

**实现**
- **起点**
  - `config.json` 新增 `startDir2`，是第二个窗格的起点；`startDir` 仍是第一个窗格的。
  - 右键「启动时从这里开始」按所在窗格分别设置（`startDirOf(pi)`）。
  - `` ` `` 前往当前窗格自己的起点；没设或文件夹已不在时回到 `~`。
  - 启动时，第二个窗格设了 `startDir2` 就从那里开始（命令行指定了文件夹时除外）。
  - 状态栏显示用 `startLabel()`，`muted` 色，路径里的 Home 缩写成 `~`。
- **去掉标签页**
  - `paneState` 只留一个 `ts`。删掉 `tab.new`、`tab.close`、`tab.next`、`tab.prev`、`tab.1–9`，以及菜单里的「新建/关闭标签页」「切换到标签页」，⌘T、⌘W 不再绑定。
  - 从 Dock、`open -a` 或默认文件浏览器打开的文件夹，改为在当前窗格打开。
- **工作区**
  - 工作区就是 `workspaces/*.json` 里保存的布局，按名称排序。
    - `ws.1–9`（`1`–`9`、⌘1–9）切到第 N 个；
    - `ws.next`、`ws.prev`（`]` `[`、⌘⇧] ⌘⇧[、⌃Tab）在列表里循环切换。
  - **切换**（`switchWorkspace`）：先把当前工作区的样子写回它自己的文件，再读目标并应用，所以切回来时还是离开时的样子，和标签页一样。
    - 一次只处理一个切换（`wsBusy`）。
    - 启动时不在任何工作区里，从那里切走不会保存任何东西。
  - 对话框（⌘⌥W，标题「工作区」）：「保存」后，当前就是这个工作区；「切换」等于按键切换。
  - 有两个或更多工作区且当前在其中一个时，窗格上方显示工作区条（`1 名称`…），点一下就切换。
  - 文件格式不变。旧文件里的多个标签只取当时选中的那个；写入时每个窗格只写一个。
  - Debug 编号 33（`addr.tabs`）随标签条一起删除。

**边界**
- 不改工作区的文件格式和保存位置。
- 不自动恢复上次会话，启动仍从起点开始。
- 不改其他键位。

**验证**
- `phase3_test.go` 的 `TestWorkspaceSaveRestore`：
  - 先保存两个工作区；
  - 用 `1`、`]`、`[`、⌘2 切换，切回来时是离开时的样子；
  - 按 `9` 但没有第 9 个时给出提示；
  - 在对话框里点「切换」。
- `phase17_test.go` 的 `TestGoStartPerPane`：左右两栏各自设起点，各自按 `` ` ``；状态栏文案（单栏、左右、上下）；没设起点的窗格回到 `~`。
- `TestOpenPath`、菜单和键位相关的测试已按单标签改写。
- `go test -race ./...`、`vet`、`mygo vet`。

### 阶段 18 后修订六：状态栏提示第一行改为「导航时：」
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit 51b5e07。

**要改什么**：状态栏键位提示的第一行以「导航时：」开头，后面依次是 `↑↓`移动、`→`打开、`←`返回、`` ` ``回到起点、`,`排序、`.`隐藏文件、`Space`预览、`f`过滤、`;`终端、`a`智能体、`?`帮助（预览、过滤、帮助是用户随后要求补回的）。第二行「选中后：」不变。

**实现**
- `hints.go` 的 `hintLines[0]` 改为 `@move, open, up, go.start, sortmenu, hidden, quicklook, filter, terminal, agent, help`。
  - `@move` 是列表自己处理的方向键，固定显示 `↑↓ 移动`。
  - 其他项取各自动作的第一个键。
- 短标题新增 `open` 打开、`up` 返回、`terminal` 终端；`agent` 改为「智能体」。

**验证**
- `TestDefaultHintsOrder` 检查第一行的顺序和键帽。
- 有 4 个测试原来用「过滤」「.」「↑」「隐藏文件」来定位界面元素，现在这些文字也出现在提示里，所以改了定位方式。
- `go test ./...`、`vet`、`mygo vet`。

### 阶段 18 后修订七：工具栏的工作区按钮与首次启动的演示工作区
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit 4d6ba68。

**要改什么**
1. 工具栏上原来的「工作区…」位置改成工作区按钮，每个工作区一个，对应 `1`–`9` 和 `[` `]`。
2. 首次打开时建 3 个演示工作区：
   - Home：三列，用户 Home；
   - Download：左右双栏，两边都是「下载」；
   - Documents：上下双栏，两边都是「文稿」（用户原文写作 Docments，按文件夹名改为 Documents）。

**实现**
- **工具栏按钮**：工具栏的 `workspace` 项改为画 `workspaceButtons`。
  - 每个工作区一个胶囊分段，当前工作区用 `selection` 底色。
  - 提示写「切换到工作区「名称」」加对应的 `ws.N` 键。
  - 分段后面保留原来的 `square.on.square` 按钮，用来打开保存/切换对话框。
  - 工具栏显示这些按钮时，窗格上方的工作区条不再出现；关掉工具栏后，工作区条照旧。
- **顺序**：`config.json` 新增 `workspaces`，记录工作区的顺序，也就是 `1`–`9`、`[` `]` 和工具栏的顺序。
  - 新保存的工作区排在最后。
  - 不在这个列表里的按名称排在后面（`orderWorkspaces`）。
- **演示工作区**：`config.json` 新增 `workspacesSeeded`。
  - 为 false 时，`listWorkspaces` 写入 3 个演示工作区（`seedWorkspaces`），把它们排在最前面，然后置为 true。
  - 已有同名文件就不覆盖，所以用户删掉以后也不会再建。
  - 已有用户（例如有 Test1、Test2 的）更新后，这 3 个会排在原有工作区之前。

**边界**
- 不改工作区的文件格式、切换逻辑和键位。
- 启动时仍不进入任何工作区，从起点开始。

**验证**
- `TestDemoWorkspaces`：
  - 顺序是 Home、Download、Documents、已有的工作区；
  - 点工具栏的 Download 进入左右双栏「下载」，按 `3` 进入上下双栏「文稿」，按 `1` 回到 Home；
  - 删除后不会再建。
- `TestWorkspaceSaveRestore`：工作区按保存顺序排列，工具栏显示按钮，关掉工具栏后显示工作区条。
- 用测试窗口截图确认了工具栏的样子。
- `go test ./...`、`vet`、`mygo vet`。

### 阶段 18 后修订八：彻底移除工具栏，Agent 移到工作区按钮左边
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit eb492f9。

**要改什么**
1. 原来的工具栏彻底不要，设置里的「工具栏」页也去掉。
2. Agent 按钮移到工作区按钮的最左边。

**实现**
- 顶部固定一条标题栏（`titleBar`）：Agent 分体按钮 → 工作区胶囊按钮和对话框按钮 → 居中的文件夹名，可拖动窗口。
- 删除：`toolbarpage.go`（设置「工具栏」页），动作 `toolbar`（⌃⌘B）及菜单项，`config.json` 的 `toolbar`、`showToolbar`（旧文件里的这两项被忽略），外观页的「显示工具栏」，`defaultToolbar`、`toolbarStrip` 等，窗格上方的工作区条（`workspaceStrip`）。
- 设置页变为「通用、快捷键、Agent、关于」，`pageAbout` 改为 3。
- Debug 编号删去工具栏各按钮，保留 `titlebar`、`tb.agent`、`tb.workspace`。

**边界**
- 不改 Agent 按钮和工作区按钮本身的行为、键位。
- 状态栏的布局切换、检查器开关保留。

**验证**
- `TestNoToolbar`、`TestNoToolbarAndDarkByDefault`：没有后退/新建等按钮，没有 `toolbar` 动作；`TestTitleBarTips`、`TestAgentTitleBar`、`TestConfigOldToolbarIgnored`。
- `go test ./...`、`go vet ./...`、`go tool mygo vet .` 通过。

### 阶段 18 后修订九：Agent 与工作区按钮移到标题栏右端；README 换成 goFinder. 字标
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit f9db5d6。

**要改什么**
1. Agent 按钮和工作区按钮移到窗口最右边；当前路径的文字在剩余宽度里居中。
2. README 顶部的应用图标换成「关于」页的字标 goFinder.。

**实现**
- `titleBar`：Spacer、文件夹名、Spacer，然后 Agent 分体按钮和工作区按钮，靠右。
- 字标：`wordmark_test.go` 的 `TestWordmarkPNG`（设 `MYGO_WORDMARK=目录` 才运行）用 `wordmark` 在白底和黑底各画一次，做差值抠图得到透明 PNG，浅色、深色各一张，放在 `docs/images/`；README 用 `<picture>` 按 GitHub 的明暗主题切换。

**边界**
- 不改按钮行为和键位；`resources/icon.png` 应用图标不变。

**验证**
- `go test ./...`、`go vet ./...`、`go tool mygo vet .` 通过；`TestChromeShots` 截图核对标题栏布局。

### 阶段 18 后修订十：工作区重命名与删除、中文演示工作区、e/s/w 单键、菜单单键小写
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit 3b9c805。

**要改什么**
1. 三个演示工作区改为：默认工作区（三列，Home）、下载（左右双栏，下载）、文档（上下双栏，文稿）。
2. 工作区对话框加「重命名」「删除」按钮；右键工作区按钮也有这两项。默认工作区只能改名，不能删除。
3. 去掉 Agent 菜单里三个 Agent 前的图标。
4. 在编辑器中打开 `e`、收藏当前文件夹 `s`、工作区 `w`，各只保留这一个默认键；它们的提示加进状态栏第一行，`?` 速查表随键位表更新。
5. 右键菜单里的单键快捷键显示为小写。

**实现**
- `config.json` 新增 `defaultWorkspace`（不能删除的那个）。已建过演示工作区（`workspacesSeeded`）而 `defaultWorkspace` 为空时，一次性把 Home/Download/Documents 改名为默认工作区/下载/文档（`migrateDemoNames`），缺默认工作区时补建。
- `workflow.go`：`renameWorkspace`（重名拒绝，同步顺序、默认、当前）、`deleteWorkspace`（移到废纸篓，默认的拒绝）、`workspaceMenu`（按钮的右键菜单）；对话框加按钮行，默认工作区的「删除」禁用并有提示。重命名复用通用输入对话框 `dlgWsRename`。
- `actions.go`：`defaultKeys` 改 `editor`/`fav.add`/`workspace`；`menuItem` 只给带修饰键的键用 `Shortcut`，单键改成小写尾注。
- `hints.go`：第一行加 `editor`、`fav.add`、`workspace`。

**边界**
- 不改工作区文件格式和其他键位；删除只移到废纸篓。

**验证**
- `TestWorkspaceRenameDelete`、`TestDemoNamesMigrate`、`TestDemoWorkspaces`；`go test ./...`、`go vet ./...`、`go tool mygo vet .` 通过。

### 阶段 18 后修订十一：s / s s / t 键位、点侧边栏不移焦点框、状态栏两行重排
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit 3eb01b6。

**要改什么**
1. 加入暂存架 `s`（保留 ⌥⌘S），收藏当前文件夹 `s s`，在终端打开 `t`（替代 `;`，保留 ⌥⌘T）；右键菜单和设置的快捷键页随键位表更新。
2. 点侧边栏时，青色焦点框留在用户离开的窗格，不移到侧边栏。
3. 状态栏两行改为：
   - 导航时：↑↓ 移动、→ 打开、← 返回、tab 切换窗口、` 回到起点、, 排序、. 隐藏文件、Space 预览、f 过滤、t 终端、a 智能体、e 编辑器、w 工作区、? 帮助；
   - 选中后：c c 拷贝路径、c 拷贝、x 剪切、v 粘贴、s 暂存、n 新文件夹、n n 新文件、↩ 重命名、d 删除、s s 收藏。

**实现**
- `actions.go` 的 `defaultKeys`；`hints.go` 的 `hintLines` 与短标题 `hintTitles`，`Tab` 在提示里写作 `tab`。
- 焦点：`updateFocusZone` 只在焦点区原本就是侧边栏（由 `Tab` 进入）时才继续算侧边栏；`sidebar()` 发现侧边栏因点击拿到焦点时置 `wantFocus`，把键盘焦点还给窗格。
- `CLAUDE.md` 总规则的同键双击加上 `s s`。

**边界**
- 不改 `Tab` 进入侧边栏后的键盘导航；其他键位不变。

**验证**
- `TestSidebarNavigation`（点收藏后焦点仍在 pane0）、`TestFocusBorderFollows`（双栏点侧边栏后仍在 pane1）、`TestDefaultHintsOrder`、`TestYaziKeys`；`go test ./...`、`go vet ./...`、`go tool mygo vet .` 通过。

### 阶段 18 后修订十二：统一界面用词，提示里的快捷键改从键位表读取
> 来源：本开发会话（2026-11-12），用户确认梳理结果后提出。已实现：commit 3df949c。

**要改什么**
1. 「拷贝」一律改「复制」（复制、复制路径、复制到此、已复制……，菜单栏「编辑」里的也是）。
2. `?` 一律叫「速查表」：动作名、状态栏、速查表标题。
3. 其他统一：
   - `` ` `` 叫「回到起点」，右键菜单「设为起点」，状态信息「已把 X 设为起点」「已取消起点」；
   - `Tab` 叫「切换窗格」；
   - 状态栏 `Space` 叫「快速查看」，`←` 叫「上级」，`a` 叫「Agent」；
   - `w` 叫「工作区…」，标题栏提示「切换、保存、重命名、删除工作区」；
   - 左右布局叫「左右分栏布局」，菜单「左右」；
   - `.` 叫「显示隐藏文件」；
   - 演示工作区「文档」改「文稿」，和侧边栏一致。
4. 写死的快捷键改从键位表读取：地址栏提示、剪切/复制后的状态、压缩包只读提示、废纸篓确认复选框、工作区对话框、打开失败时的设置键。

**实现**
- `actions.go` 的动作名和 `menuTitles`，`hints.go` 的 `hintTitles`，`menu.go`、`starthere.go`、`workflow.go`、`app.go`、`dnd.go`、`inspector.go`、`view.go` 的文案。
- `chrome.go` 新增 `firstKey(id)`（第一个键）和 `keysText(id)`（全部键，用「、」连接）。

**边界**
- 不改代码注释和键位本身；已经从 0.18.5 建好的「文档」工作区不自动改名，用户可以自己重命名。

**验证**
- 相关界面测试同步新文案；`go test ./...`、`go vet ./...`、`go tool mygo vet .` 通过。

### 阶段 18 后修订十三：对外发布前整理仓库
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit 4c6a36e（gui/ 移动的前半部分被并行的官网会话一起提交进 59deb46）。

**要改什么**：撤下和开发无关的文件；根目录源码列表太长，用文件夹整理。官网 `site/` 留在仓库。

**实现**
- 撤下 `generated-images/icon.png`（生成图标时的中间稿，`generated-images/` 加进 `.gitignore`）和 `docs/diag/reveal.swift`（一次性诊断脚本）。
- 界面层整体移到 `gui/`（`package gui`）；根目录 `main.go` 只调用 `gui.Main()`。`docs/design-tokens.json` 移到 `gui/design-tokens.json`（`go:embed` 不能引用上级目录）；`TestDebugIDsDoc` 改读 `../docs/debug-ids.md`。
- 32 个按阶段命名的测试文件按功能合并为 12 个：`main_test.go`、`keys_test.go`、`workflow_test.go`、`agents_test.go`、`theme_test.go`、`chrome_test.go`、`preview_test.go`、`inspector_test.go`、`columns_test.go`、`mouse_test.go`、`files_test.go`、`bench_view_test.go`。测试内容不变。
- 文档同步：`CLAUDE.md` 的命令和架构、设计系统、mygo-maintenance skill 的 vet 命令。

**边界**：不改任何行为；本文档里旧阶段提到的文件名（如 `phase3_test.go`、根目录的 `.go` 文件）是当时的记录，不回改。

**验证**：`go build ./...`、`go vet ./...`、`go tool mygo vet .`、`go tool mygo vet ./gui`、`go test ./...` 通过；`go tool mygo build -skip-installer` 打包成功，签名为 Apple_self_dev。

### 阶段 18 后修订十四：关于页加入 AboutKit 的内容
> 来源：本开发会话（2026-11-12），用户提出。已实现：commit 4f1b51a。

**要改什么**：参考作者的关于模块 AboutKit（github.com/aoyamashou/AboutKit，Swift），把其中的内容按 goFinder 设计系统写进设置 › 关于页，不照搬代码。

**实现**（`gui/about.go`，`aboutPage` 从 `debug.go` 移出）
- 页头：「goFinder」与「版本 X」，右侧「项目仓库」链接按钮（github.com/aoyamashou/goFinder）。
- 简介一段；作者区：「本应用由Claude Code提携 Corndog 开发。」、别名一行、四个链接按钮（作者网站、X 主页、小红书、GitHub，SF Symbols 或字形图标，悬停显示网址），右侧 94px 圆角头像（`gui/author.jpg`，`go:embed`，只解码一次）。
- 「版本信息」：原有的构建、更新、MyGo、Bundle ID、数据目录（版本已在页头，不重复）。
- 「致谢开源项目」：MyGo、Yazi、Nerd Fonts、JetBrains Mono，名字带下划线可点开，后接「作者 · 协议 · 用途」（`muted`）。
- 「声明」，最后保留原有的 Debug 编号开关。
- 链接一律通过 `system.OpenURL` 打开，测试里由 `fakeSys` 记录。

**边界**：不放应用图标（设计系统 §8）；不改 Debug 编号机制；AboutKit 的 Debug 按钮与独立关于窗口不搬。

**验证**：`TestAboutPage` 检查各区块文字、头像解码、点开链接的网址；`MYGO_SHOTS` 截图核对浅色和深色；`go vet`、`mygo vet`、`go test ./...` 通过，打包成功。

### 阶段 18 后修订十五：设置中增加可选 Vim 键位
> 来源：Codex 开发会话（2026-11-12），[Issue #2](https://github.com/aoyamashou/goFinder/issues/2) 及用户补充的设置开关需求；按 Claude 审查提交 5b752ae 修订后的 [PRD](../Codex/vim-keybindings-prd.md) 实施。已实现：commit 800f7a0。

**要改什么**：设置 › 快捷键增加默认关闭的「Vim 键位」开关，开启后支持 h/j/k/l，仍保留方向键和原有组合键；用户自定义优先。

**实现**
- `actions.go` 新增 `cursor.up/down`；`defaultKeys(vim bool)` 直接生成模式基础表，使「默认」按钮随模式切换。↑↓ 保留列表原生处理，不加入键位表。
- `config.json` 新增 `vimKeys`；开关即时保存，切换时清空录制与双击等待，用内存 `kmFile` 重建键位，不改写或重读 `keymap.json`。优先级为模式预设、自定义命令、用户按动作覆盖。
- j/k 更新光标、选择和滚动，与普通方向键移动一致；额外导航键只在当前文件列表获得焦点时分发，避开文本输入、弹窗、菜单、侧边栏与父目录列。
- 状态栏在上下两个额外移动动作均有键时显示其首键（默认 Vim 为两个键帽 k、j），否则保留 ↑↓；菜单、速查表和设置共享有效键位。移动动作旁注明「方向键始终可用」。

**边界与已知限制**
- MyGo v0.8.3 的 Shift 连选起点为私有字段，无公开重置接口。按 PRD 接受降级：j/k 后的 Shift+↑↓ 从上次原生导航的起点延伸；例如 Home、j、j、Shift+↓ 会选择第 1–4 行，而非第 3–4 行。不修改框架、不操作私有字段、不增加 Shift+J/K。
- ↑↓ 仍不可改键。关闭开关不清除用户自行保存的 hjkl；现有 `setKeys` 连带保存其他动作完整绑定的行为不变。
- README、官网、版本号和发布状态不变；未覆盖安装本机应用。

**验证**
- `vim_test.go` 覆盖开关 UI 与重启持久化、异步写入前的内存配置、自定义命令及用户覆盖优先级、清除和恢复默认、保存错误、与原生方向键的光标和多选结果对照、自动重复事件、空目录/首尾/过滤、预览、滚动、输入与菜单焦点隔离、双窗格及 Shift 起点降级。
- `gofmt -l gui internal` 无输出；`go vet ./...`、`go tool mygo vet .`、`go tool mygo vet ./gui`、`CGO_ENABLED=0 go test ./...` 通过。
- `TestVimShots` 生成浅色/深色、关闭/开启共 8 张截图，已逐项检查设置与状态栏；开启状态的 4 张验收截图附在 PRD。
- 用 `Apple_self_dev` 签名的独立原生测试程序、临时文件与配置验证开关、h/l 进出目录、连续 j 移动及跨屏滚动；测试结束后关闭窗口并清理测试夹具。长按自动重复通过 `ui.Tester.KeyRepeat` 验证，原生窗口未进行物理长按测试。
- **发布（2026-11-13）**：Claude 代码审查提交 f0a3f88 放行后，经用户明确授权发布 [v0.18.9](https://github.com/aoyamashou/goFinder/releases/tag/v0.18.9)，tag 指向 920a1dd，已设为 Latest。245 项测试、vet 三连及应用签名、DMG、全量/3 个增量包签名和远端附件摘要均通过验证；本机已安装应用未覆盖。

### 阶段 18 后修订十六：新建文件夹后的可选项目初始化

> 来源：2026-11-13 Codex 开发会话，[Issue #4](https://github.com/aoyamashou/goFinder/issues/4)；依据 Claude 审查提交 6d8a44b、ea0eb6d 及用户执行授权。需求与验收维护在 [PRD](../Codex/folder-init-prd.md)。已实现：commit ad2bc89。

**要改什么及为什么**：减少建立开发项目的重复操作，在通用设置提供独立、默认关闭的 Git 初始化、Agent 说明文件、项目 skills 目录三项。

**实现**：新建保持同步；成功后按提交时的配置与默认 Agent 快照后台初始化。Agent 文件为零字节 `AGENTS.md` 与内容 `@AGENTS.md\n` 的 `CLAUDE.md`；Claude/Claude Agent Teams 创建 `.claude/skills/`，其他或无 Agent 创建 `.agents/skills/`。Git 保留用户默认分支与模板，隔离目标重定向环境，有超时与命令行工具检查。已有项保留，失败逐项报告，不回滚目录；主线程仅更新结果和相关可见列表，保留用户当前导航与选择。

**边界**：只对本次新建文件夹生效；不对已有目录、新建文件、复制/移动/解压做初始化，不改快捷键、Agent 启动、README、官网、依赖、版本或发布渠道。配置版本保持 7。仅推送功能分支，PR、Merge、最终 Issue 回复交 Claude。

**验证**：格式检查、vet 三连、全量无 cgo 测试和 `go test -race ./gui ./internal/projectinit` 通过。覆盖组合与旧配置、取消/保存、入口及侧列、独占写入与符号链接、Git 故障、默认 Agent 与快照、异步交错和焦点隔离；临时目录真实 Git 验证默认分支/模板及环境隔离。浅深色设置截图已目视核对并附 PRD；独立签名开发窗口实际新建中文目录并核对文件、skills、Git 状态与配置。后台自动化的完成画面在后续键盘事件后读取，未宣称验证无输入时后台窗口的即时重绘。未覆盖安装、改系统设置或发版。

### 阶段 18 后修订十七：Intel Mac 双架构分发流程
> 来源：Codex 开发会话（2026-11-13），[Issue #1](https://github.com/aoyamashou/goFinder/issues/1)，按 Claude 审查提交 5ae1e87 的 [PRD](../Codex/intel-support-prd.md) 实施；用户授权在独立 worktree 工作。已实现：commit f290eb0。

**要改什么**：提供原生 amd64 打包与独立更新渠道，保留现有 arm64 更新地址；每次正式发布必须同时包含两个架构。

**实现**
- DMG 脚本支持 `--arch arm64|amd64|all`，默认 arm64 兼容旧用法。完整输入检查、暂存转换、无签名和镜像校验通过后才替换本地文件并更新摘要。
- 上传只允许 draft，所有目标完成后逐个上传 DMG，最终上传一次合并摘要；中途失败立即退出。新增临时夹具回归脚本，避免验证触碰真实 Release。
- 发布命令明确双架构目标，公开前检查两个更新清单及对应签名包、应用签名、DMG 和摘要，防止 latest 缺少某个架构使更新失败。

**边界**：不改应用逻辑、依赖、README、官网、版本或现有 Release；没有覆盖本机安装、修改默认文件浏览器。没有 Intel 真机，实际更新安装和远端附件检查留待授权发版阶段，不能宣布真机支持已验证。

**验证**
- 脚本语法及 9 项夹具测试通过；格式检查、vet 三连及全量测试通过（245 项）。
- 实际构建 arm64、amd64 两套应用，证书 `Apple_self_dev`、架构检查通过；两套 DMG 去签名、镜像与全部附件摘要校验通过。
- 本地生成两套签名更新清单与全量包，独立验证大小、SHA-256/Ed25519 签名、包内架构和内嵌更新地址；arm64 两个增量通过签名校验，amd64 首版无历史增量。没有上传。
- macOS 27.2.1 上 Rosetta 启动正式 amd64 构建，临时目录导航、中文预览、复制/移动、废纸篓、快速查看窗口和右键菜单验证通过；arm64 原生启动、导航、预览、复制回归通过。细节与验证边界见 PRD。
- **发布（2026-11-13）**：经用户明确授权发布 [v0.18.10](https://github.com/aoyamashou/goFinder/releases/tag/v0.18.10)（含修订十六、十七），tag 指向 f08ed39，已设为 Latest。首个双架构版本：两套应用 `Apple_self_dev` 签名与架构、两个不签名 DMG、两份全量包摘要均与 SHA256SUMS 一致；`update-darwin-arm64.json` 带 0.18.7–0.18.9 三个增量，`update-darwin-amd64.json` 首版无增量，`releases/latest/download/update-darwin-amd64.json` 可访问。Intel 版只经 Rosetta 验证，已在 Issue #1 回复说明并按用户要求关闭（真机有问题可重开）；本机已安装应用未覆盖。

### 阶段 18 后修订十八：新建文件夹对话框初始化选择与记忆

> 来源：2026-11-13 Codex 开发会话，[Issue #10](https://github.com/aoyamashou/goFinder/issues/10)，依据 Claude 审查提交 7cd87a3 及用户执行授权；需求和验收维护在 [PRD](../Codex/folder-init-prd.md)。已实现：commit 22d9ae6。

**要改什么及为什么**：在新建文件夹时直接选择三项初始化并记住偏好，减少创建前进入设置页的操作。

**实现**：独立对话框草稿从通用偏好载入，成功创建后应用并按变化保存；初始化使用确认时的选项和 Agent 快照。初始化与必要的保存同任务完成，保存错误追加在结果后；全关时沿用普通保存。所有配置写入用代号与互斥锁保证较新成功快照不被旧写入覆盖，后台配置快照复制可变成员。

**边界**：不改初始化产物、Git 执行规则、键位、其他对话框、依赖、README、官网、版本或发版；沿用原 Debug 编号。只推送功能分支，待 Claude 代码审查，剩余操作由人类完成。

**验证**：格式检查、vet 三连、全量无 cgo 测试及 GUI/初始化 race 检查通过。新增确认、取消、失败与持久化、反序/并发保存等测试，已有初始化与入口回归通过；浅深色对话框截图已检查并附 PRD。临时配置及目录、独立 Apple_self_dev 签名原生窗口验证三项勾选后创建，退出重启后对话框和通用页均保留偏好。未覆盖安装应用、改系统设置或发版，原生验证与自动化的边界见 PRD。

### 阶段 18 后修订十九：应用包安装菜单

> 来源：2026-11-13 Codex 开发会话，[Issue #6](https://github.com/aoyamashou/goFinder/issues/6)，依据 Claude 审查提交 6031419 及用户执行授权；需求和验证维护在 [PRD](../Codex/app-install-prd.md)。已实现：commit 4ee8c17。

**要改什么及为什么**：为未安装的本地应用包提供打开、复制到应用程序、复制并解除隔离的菜单，减少手工安装步骤，已安装应用继续直接启动。

**实现**：独立 `internal/appinstall` 处理判定、目标检查及隐藏暂存；ditto 保留包元数据，xattr 在发布前按审查备用方案解除隔离。临时夹具验证后采用暂存父目录内创建 bundle，避免预建目标丢失包根属性。界面新增三项打开菜单及右键固定两项，后台运行、防重复提交，结果主线程回传；仅将自有暂存物移到废纸篓。

**边界**：不提权、不做签名比对、进度条或取消；采用审查接受的双检查与重命名竞态。不改普通复制、动作、键位、依赖、README、官网、版本或现有 Release；本机 /Applications 未写入，安装应用未覆盖。只推送当前功能分支，待 Claude 代码审查。

**验证**：格式、vet 三连、全量无 cgo 测试及 GUI/安装逻辑 race 检查通过。临时目录真实 ditto/xattr 验证隔离属性、其他扩展属性、链接、执行位及源不变；GUI Tester 覆盖键盘/双击/三列右键/取消/失败与防重复提交，四张浅深色菜单截图已检查。细节与原生验证边界见 PRD。

### 阶段 18 后修订二十：快捷键操作轻量反馈

> 来源：2026-11-13 Codex 开发会话，[Issue #7](https://github.com/aoyamashou/goFinder/issues/7)，依据 Claude 审查提交 9ba4b0b 及用户执行授权；需求和验证维护在 [PRD](../Codex/shortcut-feedback-prd.md)。 已实现主体：commit c5c63c4；悬停暂停待裁定。

**要改什么及为什么**：复制、复制路径、剪切、回到起点、显示隐藏文件及排序变更时补充短暂提示，让键盘操作结果更容易察觉。

**实现**：沿用 MyGo 自绘 Toast，在状态栏上方显示 1.5 秒的品牌青底黑字提示，使用固定 ID 替换续时；三个排序入口共用变更比较。空选择不清空原剪贴板，ZIP 剪切显示只读提示。Toast 显示期间隐藏状态栏完成圆点以遵守品牌青上限。

**边界**：保留状态栏，不改键位、一般文件操作或其他提示，不新增依赖、动画、设置和后台定时任务。按审查约定，复制路径与导航不等待回执；MyGo 默认 Toast 会拦截提示下的点击，设为穿透后悬停暂停失效；保留穿透、悬停不暂停，Claude 代码审查已接受这一偏差并通过（c5c63c4）。

**验证**：格式检查、vet 三连、全量无 cgo 测试及 GUI race 检查通过；Tester 覆盖双击键、空选择、ZIP、起点、排序、改键、续时、消失和点击；浅深色截图已检查。未做原生窗口手动验收，细节见 PRD。
- **发布（2026-11-13）**：经用户明确授权发布 [v0.18.11](https://github.com/aoyamashou/goFinder/releases/tag/v0.18.11)（含修订十八至二十），tag 指向 d824ca6，已设为 Latest。两套应用 `Apple_self_dev` 签名与架构、两个不签名 DMG 及全部 9 个附件摘要与 SHA256SUMS 一致；`update-darwin-arm64.json` 带 0.18.8–0.18.10 三个增量，`update-darwin-amd64.json` 带首个 0.18.10 增量，两份最新清单均返回 0.18.11。本机已安装应用未覆盖。

### 阶段 18 后修订二十一：包和 ZIP 直接打开

> 来源：2026-11-13 Claude 开发会话，用户提出：遇到 macOS 当作单个文件的包和 ZIP 时，goFinder 不再预览内部，而是像 Finder 一样直接打开；ZIP 交给系统归档实用工具解压，保留右键「显示包内容」进入内部。已实现：commit ec418aa。

**要改什么及为什么**：此前 → / 双击会进入 .framework、.rtfd、.xcodeproj 等包和 ZIP，第 3 栏也列出其内部，与 Finder 习惯不一致。

**实现**：`internal/vfs/package.go` 新增包扩展名表（app、framework、bundle、plugin、appex、kext、prefPane、xcodeproj、xcworkspace、playground、xcarchive、dSYM、photoslibrary、logicx、rtfd、scptd、sparsebundle、pages/numbers/key 等，大小写不敏感）及 `IsPackage`、`IsArchive`、`Inside`。`openEntry` 对包文件夹和磁盘上的 ZIP 调用 `system.Open`（ZIP 由归档实用工具在旁边解压，goFinder 不写解压代码）；第 3 栏对它们显示图标、名称和种类卡片，不列内部、也不调 QuickLook（qlmanage 会卡到超时）。右键菜单在「打开」之后加「显示包内容」，在下一帧进入内部（ZIP 仍是只读文件夹）；在行菜单里当场导航会清空正在绘制的列表。

**边界**：不改键位和其他打开逻辑；缺 Info.plist 的不完整 .app 仍可进入（打不开）；ZIP 内的文件、压缩包里的 ZIP 照旧；pkg、dmg、tar、docx 等本来是普通文件，照旧用默认应用打开。

**验证**：`TestZipBrowse` 改为 → 打开 ZIP、右键进入后再验证只读、过滤、预览和解压复制；新增 `TestPackageOpens`（.framework 打开、预览卡片、显示包内容、普通文件夹无此项）和 `TestIsPackageName`；拖放测试改为直接进入 ZIP。gofmt、vet 三连、`go test ./...` 和 GUI race 通过。未做原生窗口手动验收。
- **发布（2026-11-13）**：经用户授权发布 [v0.18.12](https://github.com/aoyamashou/goFinder/releases/tag/v0.18.12)，tag 指向 c87947c，已设为 Latest。两套应用 `Apple_self_dev` 签名与架构正确，两个不签名 DMG 及全部附件与 SHA256SUMS 一致；`update-darwin-arm64.json` 带 0.18.9–0.18.11 三个增量，`update-darwin-amd64.json` 带 0.18.10–0.18.11 两个增量，两份最新清单均返回 0.18.12。本机已安装应用未覆盖。

### 阶段 18 后修订二十二：删除反馈与文件操作撤销（Issue #19）

- **来源**：2026-11-13 用户自动处理会话、issue #19；同日用户解除 worktree 阻塞并明确恢复。需求与两轮 Opus PRD 审查见 `Codex/prd/issue-19.md`。已实现：commit 145bb11（PR #20 squash 合并；原实现 687c1ca，审查修复 b5ee213）。
- **要改什么及原因**：删除成功后播放 macOS 废纸篓音效和 Toast；增加 ⌘Z 文件操作撤销及设置/速查表入口；主菜单「设置...」替代括号长描述。
- **实现**：内存保留最近 50 批操作的逆操作和身份快照，支持删除、同卷移动、重命名、复制/解压、新建以及粘贴/拖入/暂存架入口；恢复调用 `renamex_np(RENAME_EXCL)` 原子拒绝覆盖，撤销产物只走 `system.Trash`。后台执行文件操作和快照，主线程维护历史及进行中计数；初始化完成后才记录新文件夹。文本输入优先处理自己的撤销。
- **审查允许的方案偏差**：采用第二轮非阻断建议，通过仓库已有 purego/objc 在进程内调用 NSFileManager 获取实际废纸篓 URL，后台 OS 线程锁定并配对 autorelease pool，不启动 osascript，不扫描或猜测废纸篓位置。这样没有脚本解析/子进程超时或额外权限归属问题。purego 版本不变，仅转为直接依赖。
- **边界与限制**：不增加文件重做或持久历史，不撤销外部命令、安装、属性或工作区操作。跨卷移动因身份不连续采用 PRD 允许的不可撤销分支并提示；目录超过 10000 项或快照读取失败同样明确标记。无法撤销的最新操作形成历史屏障，避免 ⌘Z 静默撤销更早一步。第一轮代码审查发现，rename 会改变对象 ctime、增删子项会改变目录 mtime/ctime，严格比较这些值会误拦本应用的连续撤销。按 Opus 建议调整快照：每项比较相对路径/dev/ino/mode，文件及符号链接额外比较大小和 mtime，目录依赖完整后代摘要而不比较自身时间或分配大小。仍拒绝外部替换、增删和常规内容修改；外部刻意保持大小并恢复 mtime 的改写不能识别，撤销也只移到废纸篓。未测试真实 Intel 硬件。
- **验证**：全量测试与 race 各 341 项通过，gofmt/vet 三连通过；浅深色 Toast 截图、真实废纸篓同名/特殊字符往返和系统音效播放通过；Apple_self_dev 双架构签名构建通过。Opus PRD 第二轮与代码第二轮均 APPROVED，代码复核 b5ee213，无 BLOCKER/MAJOR；外部往产物目录加入 .DS_Store 等也会被安全拒绝撤销。未动本机安装应用和系统默认文件管理器。
- **合并与发布（2026-11-13）**：[PR #20](https://github.com/aoyamashou/goFinder/pull/20) squash 合并为 145bb11；[v0.18.13](https://github.com/aoyamashou/goFinder/releases/tag/v0.18.13) tag 指向 86c2d67，已公开并设为 Latest。仓库没有 CI 或分支保护必需检查，使用既有本地验证和两阶段 Opus 审查。上传后重新下载核验全部 14 个附件、11 项 SHA256SUMS；两架构全量包及从 0.18.10–0.18.12 的各三个增量的 Ed25519 更新签名全部有效；解包核对 Apple_self_dev 签名、版本和架构，两个 DMG 无签名且 hdiutil 校验通过。两个实际 latest/download 更新清单均与已核验的 0.18.13 一致。

### 阶段 18 后修订二十三：移除删除音效（Issue #21）

- **来源**：2026-11-14 GitHub issue #21，用户认为删除音效不合适，要求移除。需求、Opus PRD 审查与实施偏差见 `Codex/prd/issue-21.md`。
- **要改什么及原因**：取消修订二十二加入的「删除成功后播放 macOS 废纸篓音效」，保留 Toast、状态栏、移到废纸篓与 ⌘Z 撤销。
- **实现**：删除 `trash` 中的 `a.sys.TrashSound()` 调用、`system` 接口的 `TrashSound` 及 `realSystem` 的 afplay 实现，并删除随之无用的 `sys.go` `time` import（PRD 原判断保留有误，`qlTimeout` 定义在 `preview.go`）；测试去掉 `fakeSys.trashSounds`，原用音效计数间接验证「全部失败不算成功」改为直接断言无 Toast、不增加撤销步骤。
- **边界**：不加音效设置，不改系统音量、Finder 与用户配置；修订二十二一节和 CHANGELOG 0.18.13 的音效描述是历史记录，不改。
- **验证**：gofmt、vet 三连通过；`CGO_ENABLED=0 go test ./...` 与 `go test -race ./...` 各 341 项通过；临时去掉 `n > 0` 守卫时全部失败断言能报错（变异检验）；`grep -rn 'TrashSound\|trashSounds\|afplay\|move to trash' gui internal main.go` 无输出；Apple_self_dev 双架构构建通过、签名校验和架构正确。无布局变化未截图；用户侧经自动更新确认无音效待发版后进行。

## 测试和验证
- **纯逻辑包写单元测试，用 `go test ./...` 运行**：
  - `keymap`：按键解析、上下文过滤、用户覆盖、冲突检测；
  - `vfs`：OSFS + ZipFS，用测试里生成的 zip 文件；
  - `fsops`：在临时目录里复制/移动/重名冲突；
  - `git`：在临时仓库里解析 porcelain v2 输出；
  - `config`：workspace 往返序列化、收藏夹合并。
- **手动运行**：用 `go tool mygo dev` 启动，每个阶段按清单检查：
  - 只用键盘浏览、复制、粘贴；
  - 按住 Cmd/Alt 时状态栏提示随之变化，`?` 速查表能过滤；
  - 用活动监视器核对内存和空闲 CPU 是否达到性能目标；
  - 地址栏点开同级目录，补全可用；
  - 进入 zip 并预览；
  - 保存 Workspace，重启后能恢复。
- **端到端测试**：接入 Automation 插件，用 `mygo-automation` 跑一遍核心键盘流程的冒烟测试。
- **阶段 4 收尾**：设为默认浏览器后，在 Dock 或其他应用里"在 Finder 中显示"文件夹，确认是由 goFinder 打开；再测试恢复。
