# AGENTS.md

> 来源：Claude Code 开发会话，2026-11-12。项目从这天起交给 Codex 维护后续更新和补丁，本文件是交接文档，也是所有 AI 代理的总规则；`CLAUDE.md` 只引用本文件。
> 规则以本文件为准，细节写在「文档索引」列出的文档里，不在这里重复。

goFinder：轻量、键盘鼠标都优先的 macOS 文件管理器。用 [MyGo](https://mygo.egoist.dev/docs)（Go 桌面框架，不是新语言，当前锁定 v0.8.3）的原生 UI 写成，纯 Go、无 cgo、不用 WebView。仓库公开在 github.com/aoyamashou/goFinder，官网 https://gofinder.ryou.im。

## 用户与协作约定

- 用中文回复，言简意赅；先给结论。只做最精准的修复，不顺手改无关内容。
- 开工前：`git pull` 和 `git status`；发现不属于本会话的未提交改动就停下告诉用户，不提交、不覆盖、不还原。然后读本文件和本次任务相关的文档。
- 同一项目同时只有一个「开发会话」改代码和提交，其余为「讨论会话」只产出文档；用户没说当前会话是哪种时，先问。确需两个会话同时改代码，各用 `git worktree` 开分支，完成后再合并。
- **官网 `site/` 常有另一个会话在同一目录并行改动和提交**。它的提交都要保留，不要回退、覆盖或 rebase 掉；提交时只 `git add` 自己改的文件（不用 `git add -A`），提交前看一眼 `git status`。
- 仓库是唯一事实来源：结论写进仓库文档并提交才算数；文档采用「更新」而非「新增」，同一主题只维护一份最新版本。

## 总规则

- **资源优先**：低内存、低 CPU、响应快，界面朴素即可。不引入第三方库或动画，MyGo 插件只允许 SF Symbols（侧边栏和按钮图标）和 updater（自动更新）；图标和列表用本机已装的 `JetBrainsMono Nerd Font Mono`（`internal/icons`，不打包字体，缺字体时只在用户点击后下载两个 ttf，没装时回退为文字符号），预览按需加载。
- **键位是 Yazi 单键风格**：默认方向键，可选 Vim 键位 `h/j/k/l`（设置 › 快捷键，默认关闭，开启后方向键仍可用）；没有多键序列，只有 `c c`、`n n`、`s s` 三个同键双击（400 ms 内）；⌘ 组合键作为第二键位，供菜单使用。↑↓ 由列表原生处理，不可改键；`cursor.up/down` 用于额外移动键。
- 新增或修改快捷键只改 `gui/actions.go`（`actions()` 和 `defaultKeys()`）。状态栏键帽提示（`hints.go`）、`?` 速查表、设置的快捷键页、右键菜单、标题栏按钮提示里的快捷键都从这里读取；每个动作的第一个键就是提示里显示的那个。用户的 `config.json` 自定义命令和 `keymap.json` 由 `buildKeymap` 叠加上去。新动作还要放进 `menu.go` 的 `menuBar`（`TestMenuCoversActions` 会检查）。
- **界面用词统一**（2026-11-12 定）：用「复制」不用「拷贝」；`?` 叫「速查表」；`` ` `` 叫「回到起点」、右键文件夹「设为起点」；`Tab` 叫「切换窗格」；`Space` 叫「快速查看」。提示里出现的快捷键一律从键位表读，不写死。
- 改界面前先读 `docs/design-system.md`，颜色、字号、间距用 `tok()`/`px()` 取 token，不写死数值；品牌青 `signal` 只做点缀，每屏最多 3 处。
- 改 MyGo UI 代码前，先看 `.agents/skills/mygo-maintenance/SKILL.md`。API 以锁定版本的本地源码为准：`go list -m -f '{{.Dir}}' github.com/egoist/mygo`，其中 `docs/`、`ui/doc.go`、`ui/api_gen.go` 是本地文档和接口定义。

## 安全红线

- 删除只通过 `system.Trash` 移到废纸篓，从不永久删除。
- 「设为默认文件浏览器」会改用户系统设置（`NSFileViewer`），只在测试里用 `fakeSys` 验证，**不要在本机实际执行**，也不要 `killall Finder`。
- 用户本机的 `/Applications/goFinder.app` 只通过自动更新升级（模拟普通用户），**不要用 `ditto` 或复制覆盖安装**。
- 更新签名私钥只在本机（`~/Library/Application Support/mygo/update-keys/mygo-update.key`），绝不进仓库、不打印。
- 提交内容不得含邮箱、密钥等个人信息；`/generated-images/`、`/.cloudflare/`、`site/.cloudflare/`、`node_modules/`、`/build/` 不进仓库（已在 `.gitignore`）。
- 代码签名一律用本机证书 `Apple_self_dev`。

## 常用命令

```bash
go tool mygo dev                      # 开发运行，改代码自动重启
go test ./...                         # 全部测试，界面测试用 ui.Tester，不开窗口
go test -run TestFilter ./gui         # 运行单个测试
go vet ./... && go tool mygo vet . && go tool mygo vet ./gui    # mygo vet 不支持 ./...，要分别跑
go test -race ./...
go test -run '^$' -bench Open10k ./internal/pane   # 打开 1 万文件目录的性能基准
go test -run '^$' -bench 10k ./gui                  # 界面层：进入 1 万文件目录、空闲帧、光标移动
MYGO_SHOTS=/某个临时目录 go test -run 'Shots' ./gui  # 生成浅色/深色界面截图，核对改动
go tool mygo build -skip-installer    # 打包到 build/darwin-arm64/goFinder.app，用 Apple_self_dev 签名
```

每次改动的最低验证：`gofmt -l gui internal` 无输出、上面的 vet 三连、`go test ./...` 全过；改了界面再看一次截图。

## Git、提交与发版

- **改动直接 commit 并 push 到 `origin main`**（不走 PR）。推送用 `git --no-replace-objects push origin main`。
- 提交日志用中文，写清需求或问题、实现思路；不加任何 AI 署名行（Co-Authored-By 等）。
- 公开仓库从提交 41200b5 开始；更早的历史只在用户本机的 `history-local` 分支，用 `git replace --graft` 接上（只在本机生效，**不要推送 `refs/replace`**）。所以文档里记的部分旧 commit 号在 GitHub 上查不到，这是正常的。
- 改功能或修 bug 后，在 `docs/plan.md` 的「## 测试和验证」之前加一节 `### 阶段 18 后修订N：标题`（下一个是**修订十六**），写来源、要改什么、实现、边界、验证；提交后再用一个文档提交在该节「来源」行末尾补「已实现：commit xxxxxxx。」。
- 版本号在 `mygo.json` 的 `version`，格式 `0.阶段.修订`：完成新阶段时中间位改成阶段号、末位归 0；阶段之后的小修复末位 +1。关于页会显示构建的 commit 和时间。
- **打 tag 和发版由用户决定**，没有明确要求不打 tag、不发 Release。用户说「发版」时：
  1. 改 `mygo.json` 的 `version`，在 `CHANGELOG.md` 顶部加 `## <版本>` 一节（面向用户的中文说明），提交并推送；
  2. 签名打包并上传到 draft release（同时生成增量更新包）：
     ```bash
     MYGO_UPDATER_PRIVATE_KEY="$(cat ~/Library/Application\ Support/mygo/update-keys/mygo-update.key)" go tool mygo build -skip-installer -upload
     ```
  3. `scripts/unsign-dmg.sh --upload`：把 DMG 换成不签名的（自签证书签过的 DMG 会被系统拒绝打开），并替换 release 上的 DMG 和 SHA256SUMS；
  4. `gh release edit v<版本> --repo aoyamashou/goFinder --draft=false --latest`：发布后已安装的应用才能检查到更新。
  
  不带私钥的 `mygo build` 照常打包，只是不生成更新文件。

## 架构

- `internal/` 是纯逻辑包，不依赖 MyGo，可以单独测试：
  - `keymap`：动作、键位和用户覆盖，是快捷键的唯一来源；`Override` 逐条应用并返回冲突说明（覆盖优先于默认，覆盖之间按动作顺序）；
  - `vfs`：列目录、路径补全、面包屑；`zip.go` 把 zip 当只读文件夹（路径 `/x/a.zip!/sub`，`SplitZip`/`Open`/`Extract`）；
  - `pane`：一个窗格（无多标签）的位置、历史、排序、过滤和 Quick Select 匹配；
  - `fsops`：复制、移动、重命名、新建。删除只通过 `system.Trash` 移到废纸篓，从不永久删除。从 zip 复制即解压，写入 zip 返回 `vfs.ErrReadOnly`；
  - `templates`：新建文件的模板目录；`favorites`：收藏夹 `Store`、iCloud 路径、迁移和三方合并 `Merge`；
  - `shellcmd`：自定义命令的 `{path}`/`{selection}` 展开（逐个 sh 单引号转义）与运行；
  - `shelf`：暂存架列表与 `shelf.json`；`workspace`：工作区（布局与各窗格文件夹）序列化（`workspaces/*.json`，`1`–`9`、`[` `]` 切换）；`git`：查找仓库根、解析 `git status --porcelain=v2`；`finder`：只读解析 Finder 侧边栏（sfl3/sfl4：bplist + NSKeyedArchiver + bookmark）与新窗口起始位置；
  - `icons`：文件名/扩展名/属性（目录、链接、可执行、仓库根）→ Nerd Font 字形和颜色（参照 Yazi 与 nerd-fonts），隐藏项变暗；
  - `finfo`：检查器的属性（`Stat`：种类、大小、创建/修改/访问时间、权限、所有者）和文件夹统计（`Count`，有上限）。
- 根目录只有 `main.go`（`package main`，只调用 `gui.Main()`）。界面层在 `gui/`（`package gui`），测试按功能分文件（`keys_test.go`、`workflow_test.go`、`chrome_test.go` 等，公共的 `setup`/`fakeSys` 在 `main_test.go`）：
  - `app.go`：状态和 `run(actionID)`；
  - `view.go`：布局、文件列表、地址栏、状态栏、对话框，以及 `dispatch`（每帧把键位表转成 `c.Shortcut` 调用）；
  - `columns.go`：Yazi 式三列（上级｜当前｜第 4 栏）、文件行（图标+名称+Git 字母）、列宽计算、长名中间截断、焦点边框；
  - `preview.go`：第 3 栏预览（文本、图片、QuickLook 缩略图：`system.Thumbnail` 调 `qlmanage`，150ms 延迟、3s 超时、缓存在 `Caches/<id>/ql/`）；
  - `about.go`：设置 › 关于页（项目仓库、作者与链接、头像 `author.jpg`、致谢开源项目、声明，内容取自 AboutKit）；
  - `inspector.go`：检查器（动作 `preview`，默认关闭的窄第 4 栏，只显示属性）；`font.go`：列表字体检测与一键下载（`fontNotice`、设置「通用」页）；
  - `sidebar.go`：侧边栏（Finder 个人收藏、位置、自有收藏夹，SF Symbols 图标）及收藏夹同步；
  - `dnd.go`：文件拖放（拖出用 `DragDataFrom`+`transfer.FileData`，任何元素用 `acceptDrop(e, dir)` 成为拖入目标，复制/移动按 `fsops` 的 Finder 规则）；
  - `workflow.go`：暂存架面板、Workspace 对话框（启动总在 `~`，不自动恢复会话）；`gitstatus.go`：Git 状态缓存与防抖；
  - `sys.go`：`system` 接口（含 shell 命令、`NSFileViewer` 读写）和 `config.json` 读取。设为默认浏览器会改用户系统设置，只在测试里用 `fakeSys` 验证，不要在本机实际执行。
- **线程模型**：耗时的读写通过 `a.async` 放到 goroutine，结果通过 `a.post`（即 `win.Update`）回到主线程。`tabState.seq` 和 `previewState.seq` 用来丢弃过期结果。测试里把 `async` 和 `post` 换成同步执行，`system` 换成 `fakeSys`（见 `main_test.go`）。
- **文件监听**：`syncWatches` 只监听屏幕上可见窗格的目录，使用 `HintsOnly` 模式，收到变化后重新读取目录。
- **用户配置**：放在 `~/Library/Application Support/goFinder/`，包括 `config.json`（终端和编辑器的应用名、是否显示隐藏文件和侧边栏、`contextMenu`、`commands`、工作区顺序 `workspaces`）、`keymap.json`（按动作 ID 覆盖键位）、`templates/`、`shelf.json`、`workspaces/`；收藏夹在 iCloud Drive 的 `goFinder/favorites.json`（无 iCloud 时在数据目录）。

## 文档索引

- `README.md`：面向用户的介绍（截图、特点、与访达的性能对比、安装、键位、配置）；实现细节不写在这里。README 由用户提供，改动前先问。
- `CHANGELOG.md`：每个版本面向用户的更新记录，发版时更新。
- `docs/plan.md`：分阶段计划与每次修订的记录（架构、默认键位、资源原则、各阶段范围与验证方式）。
- `Codex/vim-keybindings-prd.md`：Issue #2 可选 Vim 键位的需求、审查结论、验收与已知限制。
- `Codex/intel-support-prd.md`：Issue #1 Intel Mac 支持的可行性、双架构分发计划与审查门槛（待 Claude 审查）。
- `docs/design-system.md`（及 `gui/design-tokens.json`，由 `theme.go` 嵌入）：设计系统——颜色、字体、间距、圆角、尺寸 token 与组件规范；改界面前先读。
- `docs/debug-ids.md`：设置 › 关于里「Debug 编号」的编号表，与 `gui/debug.go` 的 `debugIDs` 保持一致（`TestDebugIDsDoc` 检查）。用户沟通时常说「N 号元素」。
- `docs/website.md`：官网（gofinder.ryou.im）的目录、部署方式、设计依据与内容边界；官网源码在 `site/`，用 Cloudflare CLI `cf deploy` 部署。
- `.agents/skills/mygo-maintenance/SKILL.md`：MyGo 应用的维护要点（生命周期、身份、线程、测试）。

## 交接现状（2026-11-13）

- **当前发布版本 0.18.9**（tag 指向 920a1dd），已设为 GitHub 最新 Release；包含修订十五的可选 Vim 键位。全量及从 0.18.6–0.18.8 升级的增量包均已签名上传，DMG 不签名，应用用 Apple_self_dev 签名。
- 阶段 0–18 及修订一至十五全部完成，没有进行中的代码任务。
- **MyGo 官方 Showcase**：用户提交的 egoist/mygo#1749 因冲突被作者关闭，内容并入 egoist/mygo#1751（保留作者署名，截至交接时尚未合并）。合并后可以删掉用户 fork（aoyamashou/mygo）上的 `showcase-gofinder` 分支。
- 用户用自动更新测试新版本，所以发版后的问题通常来自真实安装的应用；修复后照常提交，等用户决定何时发版。
- 已知的测试注意点：
  - `ui.Tester` 里弹出菜单画出后的第一次点击会被菜单吃掉，测试里先做对话框部分、再做右键部分；
  - 标题栏的 Agent 按钮和状态栏的「Agent」同名，按文字查找时会有歧义；
  - 设置面板固定 640×600，内容超出要先 `tt.Scroll` 才能点到下面的元素。
