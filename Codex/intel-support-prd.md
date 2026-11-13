# PRD：Intel Mac 支持（Issue #1）

> 来源：Codex Issue 受理与需求规划会话，2026-11-13；[Issue #1](https://github.com/aoyamashou/goFinder/issues/1)。
> 要改什么及为什么：提供 Intel Mac 的原生安装包和独立更新渠道，让 Intel 用户能够安装并持续更新 goFinder。
> 边界：仅补齐双架构构建、DMG 处理及发布操作文档；不改应用交互、依赖、官网、README，不自动发版。
> 状态：Claude 已审查（2026-11-13），**可以执行**；脚本与分发流程已实现并完成本机验证，待 Claude 代码审查。Intel 真机与实际更新仍未验证，不能宣布 Intel 支持已完全交付。
> 功能分支：`feature/issue-1-intel-support`；规划基线：`cb04bb8`（当前版本 0.18.9）。

## 1. 需求判断与完成标准

需求明确、合理且有必要：当前分发流程只处理 arm64，Intel 用户无法直接使用；锁定框架已有多架构构建能力，无需为此升级或引入依赖。

完成应同时满足：

1. 同一版本可构建独立的 arm64、amd64 应用及安装 DMG，应用均使用 `Apple_self_dev` 签名。
2. DMG 均不带签名且通过校验；两个架构的附件和 SHA256SUMS 互不覆盖。
3. 发布准备流程同时生成两个架构的签名全量更新与独立清单；旧 arm64 客户端继续使用原更新地址。
4. Intel 真机验证启动、主要文件操作、系统集成与更新；缺少真机结果时明确标记未完成，不宣布支持已交付。
5. 最低代码验证和脚本边界验证通过，Claude 审查后负责 PR、合并及 Issue 最终回复。

失败必须先修好：错误架构的下载或更新、单架构附件丢失、摘要不符、签名失败、Intel 启动崩溃、脚本漏处理任一目标。

## 2. 已核实的现状

依据仓库及本机 MyGo v0.8.3 源码，未采用其他版本文档推断接口。

| 位置 | 事实与影响 |
|---|---|
| `go.mod` | 锁定 MyGo v0.8.3，本次不升级 |
| `mygo.json` | 当前 0.18.9，已配置证书与 GitHub 更新源；未固定构建架构 |
| MyGo `cmd/mygo/build.go` | `-platform` 支持逗号分隔目标，默认仅为构建主机架构；目标写入更新参数 |
| MyGo `cmd/mygo/updates.go` | 全量包、增量和更新清单按 target 区分；没有私钥时不生成 macOS 更新包 |
| MyGo `cmd/mygo/config.go` | 未配置时最低 macOS 版本为 12.0；Intel 支持不等于支持更老系统 |
| `scripts/unsign-dmg.sh` | 固定 `build/darwin-arm64` 与 arm64 文件名，当前不能处理 amd64 |
| `AGENTS.md` | 发布命令未显式指定架构；安全约束禁止覆盖本机安装与实际修改默认文件浏览器 |

规划阶段已经运行：

```sh
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /临时目录/goFinder .
```

结果成功，`file` 确认为 `Mach-O 64-bit executable x86_64`。产物仅在 `/tmp`，未签名、未启动、未安装；这只证明当前代码可以交叉编译，不证明 Intel 系统 API、原生 UI、签名打包或更新已验证。

## 3. 方案与实施范围

采用两个独立安装包，保留 arm64 的既有更新渠道；不切换 Universal，避免改变老客户端渠道及包体积。

### 3.1 构建与发布流程

在 `AGENTS.md` 的发布流程中明确使用：

```sh
go tool mygo build -platform darwin/arm64,darwin/amd64 -skip-installer
```

只有用户明确要求发版时，才在既有安全私钥注入方式下追加 `-upload`，随后执行双架构 DMG 处理，再由发布负责人检查附件后公开 Release。本次实现验收不上传、不发布。

- 输出目录分别为 `build/darwin-arm64/`、`build/darwin-amd64/`。
- DMG 分别为 `gofinder-<版本>-arm64.dmg`、`gofinder-<版本>-amd64.dmg`。
- 更新清单分别为 `update-darwin-arm64.json`、`update-darwin-amd64.json`；逐项确认其 URL 和包内容架构正确。
- 首次 Intel 发布允许没有历史增量，但必须有有效的 amd64 全量包和清单；不得为了凑齐增量跨架构生成补丁。
- SHA256SUMS 保留两个架构及现有其他附件的记录；框架已有合并能力，先验证，不自行重写框架。
- **审查补充：从首个双架构版本起，每次发版都必须同时构建两个架构。** 已安装应用的更新地址是 `releases/latest/download/update-darwin-<arch>.json`（MyGo `updateFeed`），只要某次 latest Release 缺少 amd64 清单，所有 Intel 用户的检查更新就会 404。`AGENTS.md` 发版步骤必须写成双架构命令，并把「两个清单都在 Release 上」列为公开前的检查项。

### 3.2 精准修改 DMG 脚本

修改 `scripts/unsign-dmg.sh`，保留现有无参数和 `--upload` 的 arm64 默认行为，新增显式 `--arch arm64|amd64|all`；双架构发布使用 `scripts/unsign-dmg.sh --arch all --upload`。

- `all` 固定处理 arm64、amd64，先验证两个输入均存在；缺任意输入时立即失败，不先转换或上传已有的一份。
- 未知参数、无效架构或缺参数必须非零退出，并给出用法。
- 每个 DMG 转换后检查无签名及 `hdiutil verify`，只替换其对应摘要行，保留其他记录。
- 所有目标完成并验证后才进入上传步骤；每个 DMG 上传一次，最终 SHA256SUMS 上传一次。任一失败立即退出，不报告整体成功。
- 上传阶段不具备远端事务性；中途失败可能留下部分 draft 附件。保持 draft，重跑完整命令并核对附件后才能公开。
- 不读取或打印更新签名私钥，不改变应用包本身的签名。

### 3.3 文档与允许修改的文件

- `scripts/unsign-dmg.sh`：架构参数和双架构处理。
- `AGENTS.md`：更新发布命令、操作顺序和文档索引；保留其他现有总规则。
- 本 PRD：Claude 审查、实现证据、剩余限制与提交号。
- `docs/plan.md`：实现后按届时最新编号增加修订记录，并引用本文；规划阶段不写成已实现。
- 如需脚本回归辅助，只增加与参数、缺文件、摘要保留和上传顺序有关的最小脚本验证，不为文案或简单参数映射建立测试体系。

若交叉编译或真机验证暴露出需要改 GUI、框架或依赖的兼容问题，暂停该部分实现，先更新本文交 Claude 复审，不擅自扩大范围。

## 4. 验收计划与阻塞条件

| 验证 | 通过条件 |
|---|---|
| 参数与缺文件 | 默认行为兼容；两种单架构及 all 选择正确；非法输入失败；all 缺一份时零上传 |
| 双架构正式打包 | 使用配置证书在独立构建目录产出两套 app/DMG；`file` 或 `lipo -archs` 对应 arm64/x86_64 |
| 签名与镜像 | `codesign --verify --deep --strict` 验证 app，签名身份正确；DMG 无签名且镜像校验通过 |
| 摘要与重复执行 | SHA256SUMS 两套记录均正确、不重复、不丢其他记录；重复运行仍正确 |
| 上传顺序 | 用临时夹具和替身 gh 验证转换或校验失败时不上传；全部成功后才上传两份 DMG 与最终摘要，不接触真实 Release |
| 更新准备 | 经授权的发布阶段核对两个清单、全量包摘要与 Ed25519 签名，旧 arm64 地址保持有效；首版 amd64 无增量可接受 |
| Intel 真机 | macOS 12+ 记录具体系统版本；启动、目录导航、预览、复制/移动、废纸篓、快速查看、拖放、菜单和快捷键正常；默认浏览器设置仅 fakeSys 验证 |
| Intel 更新 | 发布负责人在明确授权的发布阶段验证同架构更新；首次安装无旧 Intel 版时记录不适用，后续版本补验证，不伪造完成 |
| arm64 回归 | 打包和核心操作正常，更新清单保持 arm64，不被 amd64 覆盖 |

实施时执行 `sh -n scripts/unsign-dmg.sh`、有意义的临时夹具验证及项目最低验证：`gofmt -l gui internal`、`go vet ./...`、`go tool mygo vet .`、`go tool mygo vet ./gui`、`CGO_ENABLED=0 go test ./...`。没有 UI 变更则无需新增界面截图。

Intel 真机当前尚未验证，也未确认可用设备。

**审查修订：Rosetta 冒烟是必做项。** 在 Apple Silicon 上用 Rosetta 运行签名后的 amd64 应用（如 `arch -x86_64 build/darwin-amd64/goFinder.app/Contents/MacOS/goFinder`，或在 Finder 里直接打开），验证启动、目录导航、预览、复制/移动、废纸篓、快速查看和菜单，并记录 macOS 版本。它不能替代真机，但能在发版前挡住启动崩溃和打包错误。

没有 Intel 真机时：脚本实现、代码审查和合并照常进行；是否带 amd64 包发版由用户决定。若发版，Release 说明和 Issue 回复须写明「Intel 版只经过 Rosetta 验证，欢迎 Intel 用户反馈」，Issue 保持 Open 等反馈，不宣布 Intel 支持已验证。

## 5. 审查与交付门槛

1. 本提交只受理一个 Issue、提出 PRD 并推送功能分支；Issue 保持 Open，不创建 PR、不合并。
2. Claude 在同一分支提交审查修改，并在本文明确写下“可以执行”及放行条件。Codex 拉取并阅读该提交后才能实施。
3. Codex 严格按审定正文实施、验证并提交；本文及 plan 修订记录补充“已实现：commit <提交号>”和实际验证边界，不把未测项目写成通过。
4. Claude 审查实现后负责 PR、Merge 与 Issue 最终回复；发版仍需用户明确要求。

## Claude 审查记录

> 审查：Claude Code 审查会话，2026-11-13，基于 09fa1d5。已核对 MyGo v0.8.3 的 `cmd/mygo/build.go`（`-platform` 逗号分隔多目标）、`updates.go`（按 target 生成 `update-<target>.json`，feed 为 `releases/latest/download/…`）、`checksums.go`（SHA256SUMS 合并），以及现有 `scripts/unsign-dmg.sh` 和 `mygo.json`。

**结论：✅ 可以执行。** 按修订后的正文实施。

方案合理：选两个单架构包，不选 Universal，既保住旧 arm64 客户端的更新地址，又不增大包体积；框架原生支持多目标构建、按架构生成清单和合并摘要，不需要升级或改框架；改动只集中在 DMG 脚本和发版文档，风险低。

审查修订（已改入正文）：

1. **每次发版都必须双架构**（第 3.1 节）：更新地址固定指向 latest Release，漏打 amd64 会让所有 Intel 用户更新失败。
2. **Rosetta 冒烟改为必做**（第 4 节），并明确没有真机时的发版口径：是否发版由用户决定；若发版，要标明只经 Rosetta 验证，Issue 保持 Open。

放行条件：

- 只改第 3.3 节列出的文件；脚本保持 POSIX sh，`sh -n` 通过；无参数时的 arm64 行为不变。
- 第 4 节中除「Intel 真机」「Intel 更新」外的各项必须通过；这两项如实标记为未验证或不适用。
- 本轮验证不上传、不改真实 Release，也不覆盖安装 `/Applications/goFinder.app`。
- README 和官网的下载说明不在本轮范围，是否补充由用户另行决定。
- 实现后交回 Claude 做代码审查，再进入合并流程。

## 实施与验证记录（2026-11-13）

本需求在独立 worktree `intel-support`、分支 `feature/issue-1-intel-support` 实施，未改动主目录内 Issue #4 的未提交文件。

### 已实现

- `unsign-dmg.sh` 保留无参数与 `--upload` 默认 arm64，新增 `--arch arm64|amd64|all`，拒绝缺参数和非法参数。
- 先检查所有输入；转换结果先暂存并验证无签名及镜像完整性，全部通过后再替换本地产物。精确更新对应摘要行，保留其他附件；缺少 SHA256SUMS 时生成它。
- `--upload` 仅允许对应 draft release；转换全部通过后逐个上传 DMG，最后上传一次 SHA256SUMS。失败非零退出，不报告整体成功；远端部分失败仍须由发布负责人保持 draft 并重跑检查。
- 发布文档改为显式双架构构建，列出两份清单、对应更新包、签名及摘要的公开前检查。后续每次发版均须保留两个架构。
- 增加 `python3 scripts/test-unsign-dmg.py`，只在临时夹具使用替身工具，不操作真实 GitHub Release。

### 验证结果

- `sh -n scripts/unsign-dmg.sh`、Git 差异检查通过。
- 脚本 9 项回归测试通过：默认及单架构、双架构重复执行与上传顺序、旧 `--upload`、非法参数、缺输入零转换/上传、转换或校验失败保留输入、拒绝公开版本、上传失败停止、缺摘要文件自动生成。
- `gofmt -l gui internal` 无输出；`go vet ./...`、`go tool mygo vet .`、`go tool mygo vet ./gui`、`CGO_ENABLED=0 go test ./...` 通过（245 项，18 个包）。未改共享状态或 UI 实现。
- `go tool mygo build -platform darwin/arm64,darwin/amd64 -skip-installer` 产出两套应用与 DMG；`file` 确认 arm64/x86_64，`codesign --verify --deep --strict` 均通过，证书均为 `Apple_self_dev`。
- 实际执行双架构去签名，两份 DMG 均不带签名且 `hdiutil verify` 通过；核对 SHA256SUMS 中全部附件。两目录的同名 `install.sh` 内容一致，摘要文件保留一个对应记录。
- 另在本地安全注入既有更新签名密钥，执行相同双架构构建（**不加 `-upload`**）。产出两份更新清单、两个签名全量包及 arm64 的两个历史增量；amd64 无历史版本、不生成增量，符合首版条件。使用 `mygo.json` 公钥独立验证全量及增量的 SHA-256/Ed25519 签名与大小；检查清单 URL、包内可执行文件架构及两套二进制中各自的 latest feed 地址，均正确。生成后再次处理 DMG，摘要仍保留全部更新附件。
- macOS **27.2.1（26B131）**、Apple Silicon 上，以 `arch -x86_64` 启动签名后的 amd64 应用；经 computer-use 操作并读回状态，确认临时目录导航、中文文本预览、复制、剪切移动、移到废纸篓及右键菜单“复制路径”。复制/移动后核对磁盘内容，废纸篓操作后确认原路径消失且应用报告成功，菜单操作后读回剪贴板路径。Space 启动 `qlmanage -p`，观察到快速查看窗口。主界面与右键菜单截图已目视核对，未将含本机侧边栏信息的截图提交。
- 签名后的 arm64 构建也在同一临时目录验证启动、导航、文本预览与复制，并核对复制文件内容。

### 剩余验证与交接

- **Intel 真机、实际自动更新安装、远端 draft 附件完整性：尚未验证。** 本轮只验证本地生成物，不上传、不改现有 0.18.9 Release，不将生成的 0.18.9 测试包作为正式发版。
- 没有修改默认文件浏览器，没有覆盖 `/Applications/goFinder.app`，没有改版本号、README、官网或依赖。
- 真实原生测试使用原构建应用和现有用户配置，只指定临时文件目录；未调整设置。
- 交 Claude 做代码审查、PR、Merge 和最终 Issue 回复；是否发版仍等用户决定。无真机时按审定口径说明仅经过 Rosetta 验证，Issue 保持 Open 等 Intel 用户反馈。

已实现：commit f290eb0（待 Claude 代码审查；真机及实际更新验证边界见上文）。
