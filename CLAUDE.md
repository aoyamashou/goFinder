# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

goFinder：轻量、键盘优先的 macOS 文件管理器。用 [MyGo](https://mygo.egoist.dev/docs)（Go 桌面框架，不是新语言）的原生 UI 写成，纯 Go、无 cgo、不用 WebView。

## 总规则

- **资源优先**：低内存、低 CPU、响应快，界面朴素即可。不引入第三方库或动画，MyGo 插件只允许 SF Symbols（侧边栏图标）和 updater（`native.Plugin`，自动更新）；图标和列表用本机已装的 `JetBrainsMono Nerd Font Mono`（`internal/icons`，不打包字体，缺字体时只在用户点击后下载两个 ttf，没装时回退为文字符号），预览按需加载。
- **键位是 Yazi 单键风格**：不含 hjkl，也没有多键序列，只有 `c c`、`n n` 两个同键双击；方向键移动；⌘ 组合键作为第二键位，供菜单使用。
- 新增或修改快捷键只改 `actions.go`（`actions()` 和 `defaultKeys()`）。状态栏键帽提示（`hints.go`）、`?` 速查表、设置的快捷键页、右键菜单、工具栏上的快捷键标注都从这里读取；每个动作的第一个键就是提示里显示的那个。用户的 `config.json` 自定义命令和 `keymap.json` 由 `buildKeymap` 叠加上去。新动作还要放进 `menu.go` 的 `menuBar`（`TestMenuCoversActions` 会检查），菜单项的快捷键也从键位表生成。
- 改 MyGo UI 代码前，先看 `.agents/skills/mygo-maintenance/SKILL.md`。API 以锁定版本的本地源码为准：`go list -m -f '{{.Dir}}' github.com/egoist/mygo`，其中 `docs/`、`ui/doc.go`、`ui/api_gen.go` 是本地文档和接口定义。

## 常用命令

```bash
go tool mygo dev                      # 开发运行，改代码自动重启
go test ./...                         # 全部测试，界面测试用 ui.Tester，不开窗口
go test -run TestQuickSelect .        # 运行单个测试
go vet ./... && go tool mygo vet .    # mygo vet 检查是否把构建期的值存进了长期状态
go test -race ./...
go test -run '^$' -bench Open10k ./internal/pane   # 打开 1 万文件目录的性能基准
go test -run '^$' -bench 10k .                      # 界面层：进入 1 万文件目录、空闲帧、光标移动
go tool mygo build -skip-installer    # 打包到 build/darwin-arm64/goFinder.app，用 Apple_self_dev 签名
```

- **发版**（自动更新，见 `docs/plan.md` 阶段 18）：改 `version` 并在 `CHANGELOG.md` 加 `## <版本>` 一节，然后：
  ```bash
  MYGO_UPDATER_PRIVATE_KEY="$(cat ~/Library/Application\ Support/mygo/update-keys/mygo-update.key)" go tool mygo build -skip-installer -upload   # 签名并上传到 draft release
  gh release edit v<版本> --draft=false --latest   # 发布后已安装的应用才能检查到
  ```
  私钥只在本机，绝不进仓库；不带私钥的 `mygo build` 照常打包，只是不生成更新文件。
- 仓库公开在 github.com/aoyamashou/goFinder（2026-11-12 起），提交后推送 `origin main`；提交内容不得含邮箱、密钥等个人信息。公开仓库从提交 41200b5 开始，之前的历史只留在本机：分支 `history-local`，并用 `git replace --graft` 接在 41200b5 之后（只在本机生效，不推送 `refs/replace`），所以本机 `git log` 能看到完整历史，文档里记的旧 commit 号在 GitHub 上查不到。
- 版本号在 `mygo.json` 的 `version`，格式 `0.阶段.修订`：完成一个新阶段时把中间位改成阶段号、末位归 0，阶段之后的小修复末位 +1；关于页还会显示构建的 commit 和时间。
- **改动直接 commit 并 push**；**打 tag 和发版由用户决定**，未经用户明确要求不打 tag、不发布 Release。用户本机的 `/Applications/goFinder.app` 只通过自动更新升级（模拟普通用户），不要再用 `ditto` 覆盖安装（用户要求，2026-11-12）。

## 架构

- `internal/` 是纯逻辑包，不依赖 MyGo，可以单独测试：
  - `keymap`：动作、键位和用户覆盖，是快捷键的唯一来源；`Override` 逐条应用并返回冲突说明（覆盖优先于默认，覆盖之间按动作顺序）；
  - `vfs`：列目录、路径补全、面包屑；`zip.go` 把 zip 当只读文件夹（路径 `/x/a.zip!/sub`，`SplitZip`/`Open`/`Extract`）；
  - `pane`：单个标签页的位置、历史、排序、过滤和 Quick Select 匹配；
  - `fsops`：复制、移动、重命名、新建。删除只通过 `system.Trash` 移到废纸篓，从不永久删除。从 zip 复制即解压，写入 zip 返回 `vfs.ErrReadOnly`；
  - `templates`：新建文件的模板目录；`favorites`：收藏夹 `Store`、iCloud 路径、迁移和三方合并 `Merge`；
  - `shellcmd`：自定义命令的 `{path}`/`{selection}` 展开（逐个 sh 单引号转义）与运行；
  - `shelf`：暂存架列表与 `shelf.json`；`workspace`：布局/标签页序列化（`workspaces/*.json`）；`git`：查找仓库根、解析 `git status --porcelain=v2`；`finder`：只读解析 Finder 侧边栏（sfl3/sfl4：bplist + NSKeyedArchiver + bookmark）与新窗口起始位置；
  - `icons`：文件名/扩展名/属性（目录、链接、可执行、仓库根）→ Nerd Font 字形和颜色（参照 Yazi 与 nerd-fonts），隐藏项变暗；
  - `finfo`：检查器的属性（`Stat`：种类、大小、创建/修改/访问时间、权限、所有者）和文件夹统计（`Count`，有上限）。
- 根目录的 `package main` 是界面层：
  - `app.go`：状态和 `run(actionID)`；
  - `view.go`：布局、文件列表、地址栏、状态栏、对话框，以及 `dispatch`（每帧把键位表转成 `c.Shortcut` 调用）；
  - `columns.go`：Yazi 式三列（上级｜当前｜第 4 栏）、文件行（图标+名称+Git 字母）、列宽计算、长名中间截断、焦点边框；
  - `preview.go`：第 3 栏预览（文本、图片、QuickLook 缩略图：`system.Thumbnail` 调 `qlmanage`，150ms 延迟、3s 超时、缓存在 `Caches/<id>/ql/`）；
  - `inspector.go`：检查器（动作 `preview`，默认关闭的窄第 4 栏，只显示属性）；`font.go`：列表字体检测与一键下载（`fontNotice`、设置「通用」页）；
  - `sidebar.go`：侧边栏（Finder 个人收藏、位置、自有收藏夹，SF Symbols 图标）及收藏夹同步；
  - `dnd.go`：文件拖放（拖出用 `DragDataFrom`+`transfer.FileData`，任何元素用 `acceptDrop(e, dir)` 成为拖入目标，复制/移动按 `fsops` 的 Finder 规则）；
  - `workflow.go`：暂存架面板、Workspace 对话框（启动总在 `~`，不自动恢复会话）；`gitstatus.go`：Git 状态缓存与防抖；
  - `sys.go`：`system` 接口（含 shell 命令、`NSFileViewer` 读写）和 `config.json` 读取。设为默认浏览器会改用户系统设置，只在测试里用 `fakeSys` 验证，不要在本机实际执行。
- **线程模型**：耗时的读写通过 `a.async` 放到 goroutine，结果通过 `a.post`（即 `win.Update`）回到主线程。`tabState.seq` 和 `previewState.seq` 用来丢弃过期结果。测试里把 `async` 和 `post` 换成同步执行，`system` 换成 `fakeSys`（见 `main_test.go`）。
- **文件监听**：`syncWatches` 只监听屏幕上可见标签页的目录，使用 `HintsOnly` 模式，收到变化后重新读取目录。
- **用户配置**：放在 `~/Library/Application Support/goFinder/`，包括 `config.json`（终端和编辑器的应用名、是否显示隐藏文件和侧边栏、`contextMenu`、`toolbar`、`commands`）、`keymap.json`（按动作 ID 覆盖键位）、`templates/`、`shelf.json`、`workspaces/`；收藏夹在 iCloud Drive 的 `goFinder/favorites.json`（无 iCloud 时在数据目录）。

## 文档索引

- `README.md`：面向用户的介绍（特点、与访达的性能对比、安装、键位、配置）；实现细节不写在这里，写进 `docs/plan.md` 的对应阶段。
- `docs/design-system.md`（及 `docs/design-tokens.json`）：goFinder 设计系统——颜色、字体、间距、圆角、尺寸 token 与组件规范；改界面前先读。
- `docs/plan.md`：分阶段计划（架构、默认键位、资源原则、各阶段范围与验证方式）。完成一个阶段后在文档开头的状态行标注 commit 号。
