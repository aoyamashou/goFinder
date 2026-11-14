# Issue #21：移除删除文件音效

来源：GitHub issue #21（作者 OWNER）及 Codex 开发会话，2026-11-14。用户授权恢复处理，在当前独立 worktree 建立专用分支，替代不支持重复执行的 T3 handoff。

## 背景与目标

用户认为删除音效不合适，要求移除。取消应用主动播放的废纸篓音效，保留现有视觉反馈和安全删除、撤销功能。

## 非目标与边界

不添加音效设置，不修改系统音量、Finder 或用户配置，不改变删除确认、Toast、状态栏、废纸篓路径获取、撤销及快捷键。不升级依赖，不调整 UI 布局、CI 或发布配置，不覆盖本机已安装应用。

## 方案与涉及文件

- `gui/app.go`：删除成功后不再调用 `TrashSound`。
- `gui/sys.go`：移除 `system.TrashSound` 接口及 realSystem 的 afplay 实现，仅移除因此不再使用的 import。
- `gui/main_test.go`：移除专用 fake 音效方法和计数。
- `gui/undo_test.go`：移除旧的播放次数断言；保留并核验删除、撤销、部分失败和全部失败的行为断言。
- `docs/plan.md`：添加修订二十三记录及验证；提交后补实现 commit。
- 本 PRD 记录审查、验证、实现 commit 与最终发布结果；历史 issue #19 文档保留历史事实。
- 发版时仅将 `mygo.json` 版本升级并更新 `CHANGELOG.md`。

## 可验证验收标准

1. 应用源码不再包含 `TrashSound`、`trashSounds` 或用于废纸篓音效的 `afplay` 调用。
2. 删除成功仍显示「已移到废纸篓 N 项」，文件进入模拟废纸篓且可撤销恢复；部分失败准确反馈成功与失败，全部失败不出现成功 Toast，不产生可撤销成功项。
3. 撤销恢复不覆盖已有文件，保留原有身份校验和安全失败测试。
4. 全量测试、race、gofmt、vet 三连及 Apple_self_dev 双架构构建通过；没有布局变化，无需新截图。测试不实际修改默认文件浏览器或本机安装应用。

## 测试计划

先运行删除/撤销相关既有测试，再执行 `gofmt -l gui internal`、`go vet ./...`、`go tool mygo vet .`、`go tool mygo vet ./gui`、`CGO_ENABLED=0 go test ./...`、`go test -race ./...` 和双架构签名构建。通过源码检索确认音效播放路径全部移除；不实际播放音效或操作用户文件。

## 风险与回滚

风险为误删视觉反馈或撤销行为，使用现有行为测试回归。可通过新增 revert commit 回滚，禁止改写历史。任何验证、审查或发布失败即停止并标记 needs-human。无 CI 时记录不存在检查，不修改 CI；如 PR 有检查，以 15 分钟作为本轮等待上限。

## 版本号影响与交付

patch：0.18.13 → 0.18.14。两次 Opus 审查通过后创建含 Closes #21 的 PR，squash 合并。按仓库已有流程签名双架构应用、上传 draft Release、替换无签名 DMG、核验校验和及双架构更新清单后公开；不发布只有说明而无更新附件的版本。发布和关闭 issue 后结束本次运行。

Status: DRAFT
