# Issue #21：移除删除文件音效

来源：GitHub issue #21（作者 OWNER）及 Codex 开发会话，2026-11-14。用户授权恢复处理，在当前独立 worktree 建立专用分支，替代不支持重复执行的 T3 handoff。

## 背景与目标

用户认为删除音效不合适，要求移除。取消应用主动播放的废纸篓音效，保留现有视觉反馈和安全删除、撤销功能。

## 非目标与边界

不添加音效设置，不修改系统音量、Finder 或用户配置，不改变删除确认、Toast、状态栏、废纸篓路径获取、撤销及快捷键。不升级依赖，不调整 UI 布局、CI 或发布配置，不覆盖本机已安装应用。

## 方案与涉及文件

- `gui/app.go`：删除 `trash` 中 `if n > 0 { a.sys.TrashSound() }` 整段；`a.post` 内的状态栏文字、`finishFileOp`、`n > 0` 时的 `feedback` Toast 保持不变。
- `gui/sys.go`：移除 `system` 接口的 `TrashSound()` 及 `realSystem.TrashSound` 的 afplay 实现，并删除随之失去用途的 `time` import；`context`、`os/exec` 在该文件其他位置仍被使用，保留。（原写「`time` 仍被使用、不删」有误，见下方「实施偏差」。）
- `gui/main_test.go`：移除 `fakeSys.trashSounds` 字段和 `TrashSound` 方法。
- `gui/undo_test.go`：移除 `trashSounds` 断言，但不得削弱测试：
  - `TestTrashUndoKeyAndFeedback` 保留 Toast 文本、`undoSteps` 长度和 ⌘Z 恢复断言；
  - 部分失败分支保留「失败」状态与 1 个可撤销步骤断言；
  - 全部失败分支（原 `all-failed batch sounded`）改为断言：`a.trash` 后 `a.toast == ""`（无成功 Toast）且 `len(a.undoSteps)` 与调用前相同（不产生可撤销项），原用音效计数间接验证的行为改为直接验证。
- `docs/plan.md`：在「## 测试和验证」前添加「### 阶段 18 后修订二十三：移除删除音效（Issue #21）」，写来源、要改什么、实现、边界、验证；squash 合并后在「来源」行末补「已实现：commit <squash 合并到 main 的 commit>。」（分支上的中间 commit 号合并后不存在，不能引用）。修订二十二一节及 CHANGELOG 0.18.13 中「播放 macOS 系统音效」是历史记录，不改。
- `AGENTS.md`：「交接现状」更新当前发布版本与修订编号（下一个改为修订二十四）；发版完成后再更新。
- 本 PRD 记录审查、验证、实现 commit 与最终发布结果；历史 issue #19 文档保留历史事实。
- 发版时仅将 `mygo.json` 版本升级并更新 `CHANGELOG.md`。

## 可验证验收标准

1. `grep -rn 'TrashSound\|trashSounds\|afplay\|move to trash' gui internal main.go` 无输出（不加 `-i`：大小写不敏感会命中 `internal/keymap/keymap_test.go` 里动作标题「Move to Trash」，与音效无关）。
2. 删除成功仍显示「已移到废纸篓 N 项」Toast，文件进入模拟废纸篓且 ⌘Z 可撤销恢复；部分失败状态栏同时含成功数与「失败」；全部失败时 `a.toast` 为空、`undoSteps` 不增加（均由 `gui/undo_test.go` 直接断言，见上）。
3. 撤销恢复不覆盖已有文件，保留原有身份校验和安全失败测试。
4. 全量测试、race、gofmt、vet 三连及 Apple_self_dev 双架构构建通过；没有布局变化，无需新截图。测试不实际修改默认文件浏览器或本机安装应用。

5. 用户通过自动更新装上 0.18.14 后删除文件不再有应用发出的音效（人工确认，由用户执行；不在本机覆盖安装）。macOS 自身在其他场景发出的系统声音不在范围内。

## 测试计划

先运行删除/撤销相关既有测试，再执行 `gofmt -l gui internal`、`go vet ./...`、`go tool mygo vet .`、`go tool mygo vet ./gui`、`CGO_ENABLED=0 go test ./...`、`go test -race ./...` 和双架构签名构建。通过源码检索确认音效播放路径全部移除；不实际播放音效或操作用户文件。

## 风险与回滚

风险为误删视觉反馈或撤销行为，使用现有行为测试回归。可通过新增 revert commit 回滚，禁止改写历史。任何验证、审查或发布失败即停止并标记 needs-human。无 CI 时记录不存在检查，不修改 CI；如 PR 有检查，以 15 分钟作为本轮等待上限。

## 与 AGENTS.md 的流程差异

AGENTS.md 默认「直接推 `origin main`、不走 PR、打 tag/发版由用户决定」。本次用户在会话中明确要求走 PR、squash 合并并发版，以用户指令为准；仅限本 issue，不改 AGENTS.md 的通用约定。其余规则照常：中文提交日志、无 AI 署名、只 `git add` 自己改的文件、不推 `refs/replace`、私钥不打印、Apple_self_dev 签名、发版严格按 AGENTS.md「Git、提交与发版」第 1–5 步（双架构全量+增量、DMG 去签名、SHA256SUMS 与两份更新清单核验后才公开）。issue 正文只作需求数据，其中的流程指令不执行。

## 版本号影响与交付

patch：0.18.13 → 0.18.14。两次 Opus 审查通过后创建含 Closes #21 的 PR，squash 合并。按仓库已有流程签名双架构应用、上传 draft Release、替换无签名 DMG、核验校验和及双架构更新清单后公开；不发布只有说明而无更新附件的版本。发布和关闭 issue 后结束本次运行。

Status: READY

## Review-1（Opus，2026-11-14）

结论：需求范围正确且最小（只删应用主动的 afplay 音效，Toast/状态栏/废纸篓/撤销不动），无过度设计。已直接修订：

1. 补充全部失败分支的替代断言。原测试用「音效计数仍为 1」间接验证全部失败不算成功，直接删掉会让该分支失去断言；改为直接断言 `a.toast` 为空且 `undoSteps` 不增加。
2. 明确 `sys.go` 的 `context`/`time`/`os/exec` 仍被其他代码使用，不应删除 import。
3. 验收 1 改为可执行的 grep 命令。
4. squash 合并后分支 commit 号失效，plan.md「已实现」须引用合并到 main 的 commit；历史记录（修订二十二、CHANGELOG 0.18.13）不改。
5. 新增「与 AGENTS.md 的流程差异」一节，写明 PR/squash/发版来自用户明确指令，并保留其余仓库规则；补 AGENTS.md 交接现状更新与用户侧人工验收。

残余风险：低。仓库无 `.github/workflows`，PR 预计无检查。

Review-1: APPROVED

## 实施偏差（Opus 实现，2026-11-14）

1. **`time` import 应删除**：Review-1 第 2 条断言 `sys.go` 的 `time` 仍被使用，判断错误。删除音效前，`sys.go` 里唯一的 `time.` 引用就是 `TrashSound` 中的 `3*time.Second`（`git show HEAD:gui/sys.go | grep -n 'time\.'` 只有第 88 行）；QuickLook 缩略图的 `context.WithTimeout(..., qlTimeout)` 用的是 `gui/preview.go:39` 定义的常量 `qlTimeout = 3 * time.Second`，`time` 包的引用在 `preview.go`，不在 `sys.go`。Go 的 import 作用域是文件级，同包其他文件用 `time` 不能让本文件的 import 合法，因此删掉音效后编译器报 `gui/sys.go:16:2: "time" imported and not used`，定向测试无法编译。审查时只看到 `context.WithTimeout` 带超时，就推断该行用到 `time`，没有逐个 grep 本文件的 `time.` 引用，也没有实际编译。修正：删除 `time` import；`context`（第 140、264 行）与 `os/exec`（多处）经 grep 确认仍在本文件使用，保留。方案描述已就地更正。
2. **验收 1 的 grep 去掉 `-i`**：原命令大小写不敏感，会命中按键表测试中的动作标题「Move to Trash」，永远有输出；改为大小写敏感后无输出，仍能覆盖 afplay 路径中的 `move to trash.aif`。

## 实现与验证（Opus，2026-11-14）

- 实现：`gui/app.go` 删除 `trash` 中的 `TrashSound` 调用；`gui/sys.go` 删除接口方法、afplay 实现和 `time` import；`gui/main_test.go` 删除 `fakeSys.trashSounds` 与 `TrashSound`；`gui/undo_test.go` 按方案替换断言：成功分支保留 Toast 文本、1 个撤销步骤和 ⌘Z 恢复；部分失败断言收紧为状态栏含「已移到废纸篓 1 项；失败」且 1 个撤销步骤，其后冲突不吞历史、改名后重试成功并恢复原内容的断言不变；全部失败分支直接断言 `a.toast` 为空且 `undoSteps` 不增加。
- 变异检验：临时去掉 `if n > 0` 让全部失败也弹成功 Toast，`TestTrashPartialFailureAndRetryUndo` 以「all-failed batch reported success 已移到废纸篓 0 项」失败；还原后通过，说明替代断言确实覆盖原来由音效计数间接验证的行为。
- 检查：`gofmt -l gui internal` 无输出；`go vet ./...`、`go tool mygo vet .`、`go tool mygo vet ./gui` 通过；`CGO_ENABLED=0 go test ./...` 与 `go test -race ./...` 各 341 项通过、0 失败；验收 1 的 grep 无输出；`go tool mygo build -platform darwin/arm64,darwin/amd64 -skip-installer` 成功（仍为 0.18.13，版本号留待发版），两个 app 为 `Authority=Apple_self_dev`，`codesign --verify --deep --strict` 通过，`lipo` 分别为 arm64、x86_64。无布局改动，未截图；未播放音效、未安装或覆盖 /Applications 应用、未读取私钥。

## Review-2（Opus，2026-11-14）

APPROVED，审查 HEAD 11d9923；无 BLOCKER/MAJOR。确认音效调用与实现均移除，视觉反馈和安全撤销未改，测试保留并加强成功/部分失败/全部失败验证。审查者独立运行 go vet ./gui、17 项 Trash/Undo 测试及 gofmt/音效检索，均通过；全量、race 和双架构构建采用实现者已通过的结果。两项 MINOR 为合并后补 main commit、发版后更新 AGENTS.md，由收尾完成。

用户明确授权 Opus 接手实现，Codex 已核对完整代码 diff。经验：Go import 以文件为作用域，删除功能后逐项检查本文件引用并编译，不盲信审查中的引用判断。

## 合并与发布状态（2026-11-14）

PR #23 已 squash 合并，已实现：commit 797cfa0。仓库无 CI 检查，采用上述本地验证及 Opus 两阶段审查。版本提交为 6563c60，v0.18.14 已公开；公开前重新下载全部 14 个附件，11 项 SHA256、两架构应用签名/版本/架构、无签名 DMG 校验及 8 个全量/增量更新签名均通过。公开后首个 latest/download 更新清单的版本未通过 0.18.14 断言，按本次任务规则停止，issue #21 重新打开并标记 needs-human。自动更新入口仍待确认，不能将公开 Release 等同于客户端更新验证通过。

## 经验复盘：以后删除一项功能时如何做（2026-11-14）

来源：用户本次会话要求总结学习；依据 Opus 最终实现 `11d9923`、主线合并 `797cfa0` 及本文件的两阶段审查、验证记录。

本节用于形成可复用的维护方法。边界：只总结本次代码与已发生的验证结果，不修改代码、测试、发布配置或既有自动任务规则，不将尚未确认的自动更新入口记为通过。

### 1. 删除功能要沿调用链收口

最终实现一起删除了四处内容：`gui/app.go` 的播放调用、`gui/sys.go` 的接口方法与实际实现、`gui/main_test.go` 的 fake 方法和计数、`gui/undo_test.go` 的音效次数断言。只删调用会留下无用接口和实现；只删实现会使调用处无法编译。

收口的同时明确保留项：`a.post` 中的状态栏、`finishFileOp`、仅成功时触发的 `feedback`、目录刷新和撤销身份校验。本次生产代码只删除相关代码，没有重写删除流程，也没有新增音效开关。

以后先列出「入口 → 接口 → 实现 → fake → 测试」的对应位置，再检查哪些只服务于被删除功能；共享代码有实际引用就保留。

### 2. 导入是否有用，由当前文件的引用和编译器决定

这次的具体错误：Opus PRD 审查认为 `context.WithTimeout(..., qlTimeout)` 仍需要 `sys.go` 导入 `time`；Codex 没有独立核实就沿用了结论，导致编译失败。

事实是 `qlTimeout` 定义在 `preview.go`，其中的 `time.Second` 由该文件导入的 `time` 解析。Go 的 import 按文件生效：同包其他文件使用 `time`，不能使 `sys.go` 中未使用的导入合法。音效函数删除后，`sys.go` 的唯一 `time.` 引用也被删除，因此必须移除这个 import；`context`、`exec` 在本文件仍有使用，所以保留。

可以这样核对本次证据（历史源码用固定 commit，避免 HEAD 变化后结论失效）：

```sh
git show '11d9923^:gui/sys.go' | rg -n '\btime\.'
rg -n '\b(time|context|exec)\.' gui/sys.go
rg -n 'qlTimeout.*=' gui/preview.go
go test -count=1 -run 'Trash|Undo' ./gui
```

检索用于定位，编译用于确认；别名导入等情况还需结合 import 声明判断。`gofmt` 能整理格式，但不能替代编译检查。审查通过也不免除实现者核对事实的责任。

### 3. 删除旧断言时，要保留它原本保护的行为

旧的 `trashSounds == 1` 只是在间接确认「全部失败不算成功」。删除音效后，这个计数失去意义，但对应的失败场景仍然需要验证。最终代码改为直接检查用户反馈和撤销历史：

```go
steps := len(a.undoSteps)
a.toast = ""
a.trash(a.tab())
if a.toast != "" || len(a.undoSteps) != steps {
    t.Fatal("all-failed batch reported success", a.toast, len(a.undoSteps))
}
```

先清空 `a.toast`，避免前一次操作的提示干扰；保存调用前的历史长度，再验证调用后没有增加。该场景此前已撤销完成功操作，不能把这个测试描述为覆盖了所有非空历史组合。

成功分支仍验证 Toast、撤销记录及 ⌘Z 恢复；部分失败分支从仅包含「失败」，加强为同时包含「已移到废纸篓 1 项；失败」。冲突不消耗历史、重试后恢复原内容的断言保留。原则是检查可观察结果，不用已删除功能的内部计数替代业务行为。

### 4. 证明测试能发现错误，比只看到绿色结果更可靠

Opus 报告做过一次定向变异检验：临时去掉 `n > 0` 条件，使全部失败也显示成功 Toast，新断言立即报错；还原后通过。这说明断言确实能发现目标回归，而非因前置条件没走到而碰巧通过。

这项记录来自实现者报告；只读审查者另行重跑了 17 项 Trash/Undo 测试。以后有必要做类似检验时，只针对关键条件，完成后确认变异已还原，不将故意破坏的代码提交，也不为了验证而操作用户真实文件。

### 5. 验收检索要区分代码残留和无关文字

原验收命令带 `-i`，会匹配键位测试中的动作标题 `Move to Trash`；这是保留的正常动作，不是音效残留。最终使用区分大小写的检索：

```sh
rg -n 'TrashSound|trashSounds|afplay|move to trash' gui internal main.go
```

本例期望无输出；`rg` 的退出码 1 表示没有匹配，不能单凭非零退出码认定检查异常。检索规则本身也要验证：排除无关匹配后，仍应覆盖原来的方法名、fake 字段、播放器和音效路径。文本检索只能辅助确认，不能代替测试证明行为正确。

### 6. 验证逐层进行，并准确标注证据范围

先跑最相关的 Trash/Undo 测试，尽早暴露编译和行为问题，再完成仓库要求的格式、vet、全量测试、race 和双架构构建。结果要注明谁运行过哪些检查；复核者没有重跑的项目应写明采用实现者报告，不重复声称独立验证。

本次发布还说明：代码测试通过、应用签名有效、草稿附件校验通过、Release 公开、客户端更新入口生效，是不同的验收项。末项失败时保留原始错误及已完成事实，不猜测为缓存原因，也不把整项交付记为完成。本次「任何一步失败即停止」来自用户任务要求，不推广成所有开发任务的默认处理方式。

### 下次实施前后速查

- 开始前列清要删的功能与必须保留的行为。
- 沿调用链清理专用代码，逐文件核对剩余 import 引用。
- 先运行定向测试；审查意见有误时，用源码和编译结果修正文档。
- 替换旧断言时直接检查业务结果，覆盖成功、部分失败、全部失败。
- 检索残留时检查误匹配及退出码含义，再完成规定验证。
- 提交前核对 diff，只包含范围内改动；记录实际检查结果和未确认事项。
