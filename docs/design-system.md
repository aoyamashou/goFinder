# goFinder 设计系统

> 来源：goFinder 主会话，2026-11-11；以 Artifacts 中的「Unlocknote.com 设计系统」为底产出（阶段 11 第一步）。
> 本文件与 `docs/design-tokens.json` 是唯一事实来源；Artifact「goFinder 设计系统」（https://claude.ai/artifact/LcapwwLodgW55ZtBdYS7b9）只作展示，含组件预览。
> 状态：待用户确认后套用到软件（阶段 11 第二步）。

> 简洁、高速、实用。灰阶承担一切，系统强调色负责交互，一点青色是品牌。

goFinder 是键盘优先的 macOS 文件管理器（Finder 替代品）。这套系统以 Unlocknote.com 设计系统为底，换成桌面应用的密度与 macOS 的规矩：13pt 正文、22px 行高、4px 网格，强调色交给用户的系统设置。

## 1. 原则（按优先级）

1. **信息密度第一。** 一屏尽量多的文件名。行高 `row-height` 22px，列表用 `list`（默认 14px 等宽）；不放大字号、不加留白来“显得高级”。
2. **每个像素都为文件服务。** 文件名之外的东西（大小、日期、类型）默认不显示，交给图标颜色和检查器。
3. **键盘在哪里，一眼可见。** 拿着键盘焦点的那一栏画 `focus` 边框；选中行在有焦点时是 `selection`，失焦时退成 `selection-inactive`。
4. **快就是不动。** 没有装饰性动画、没有过渡、没有加载骨架；状态变化立即生效。
5. **平，不浮。** 层次只靠灰阶：`bg` → `surface-100` → `surface-200`。零阴影、零渐变；唯一的线是 `separator`。

## 2. 从 Unlocknote 继承什么、改了什么

| | Unlocknote.com（网站） | goFinder（macOS 应用） |
|---|---|---|
| 阅读色 | 黑白 + 五级灰 | 相同：`ink` `muted` `faint` + `surface-100/200` |
| 交互强调 | 主操作用 `ink` | **跟随用户的 macOS 强调色** `selection` |
| 青色 | “解锁信号”，≤3% | **品牌点缀** `signal`，不承担交互状态 |
| 层次 | 灰阶，不描边不阴影 | 相同，另加一条 `separator` 分栏线 |
| 形状 | 胶囊 | 胶囊只给路径、分段、键帽；行和按钮用小圆角 `radius-row` `radius-control` |
| 字体 | Inter / Inter Display（打包） | 系统字体 `ui` + JetBrains Mono Nerd Font `mono`，**不打包** |
| 密度 | 正文 17px，分区留白 180px | 正文 13px，行高 22px，间距最大 `space-4` 16px |
| 标志 | 文字字标 | 界面内沿用文字字标 `wordmark`；另有儿童简笔画风应用图标（§6），只用于 Dock、关于窗口、Finder，不进入界面布局 |
| 标题 | 800、负字距 | 只有字标 `wordmark` 用 800；界面标题 `title` 600 |

## 3. 颜色

用色比例：灰阶面 90% · 墨（文字）8% · 强调色 ≤2%（选中行除外）· 青色只几个点。

**系统强调色负责交互（`selection`）**
- 选中行、有焦点时的光标行：`selection` 底 + `selection-ink` 字（默认蓝上 5.5:1）。运行时读用户的强调色，不写死。
- 列表失去焦点：选中行换成 `selection-inactive`（= `surface-200`），文字仍是 `ink`，图标仍是类型色——**绝不出现白底白字**。
- 键盘焦点栏：1.5px（`focus-width`）`focus` 边框，与选中行同色。
- 应用失焦（窗口不在前台）：焦点栏边框改为 `separator`，工具栏、侧边栏的 SF Symbols 改为 `faint`（删除按钮的红色也一样），选中行退成 `selection-inactive`；回到前台后恢复。
- 主按钮、开关、分段控件的选中段用 `selection`。

**品牌青只做点缀（`signal`）**，每屏最多 3 处，只出现在：
- 字标 **goFinder.** 的句点（关于窗口、设置页页眉）；
- Agent（“交给 AI”）的三道光字形：工具栏 Agent 按钮、右键菜单里的 Agent 项、设置 › Agent 页；默认 Agent 的“默认”标签用 `signal-text`，那一行底用 `signal-soft`；
- 状态栏结果句前的“已完成”小圆点（6px，`radius-pill`）；
- 过滤生效时状态栏的“过滤：xx”用 `signal-text`。

青底永远配黑字 `signal-ink`；白底上要青色文字时用 `signal-text`（5.6:1）。不要把选中行、焦点、按钮做成青色。

**其他**
- `danger` 只给“移到废纸篓”和错误，永远配文字。
- Git 标记永远是字母 + 颜色：`M` `git-modified`、`A` `git-added`、`?` `git-untracked`、`U` `git-conflict`。
- 文件类型色板 `type-folder` `type-code` `type-doc` `type-image` `type-media` `type-archive` `type-exec` `type-config` 只给 Nerd Font 字形上色，文件名永远是 `ink`（隐藏文件 `faint`）。两套主题下对 `bg` 都 ≥4.5:1。

## 4. 字体

- 外观：**默认深色**（`appearance` = `dark`），可改为浅色或跟随系统；与 macOS 外观不一致时整窗仍按所选主题绘制（自绘标题栏，侧边栏不用毛玻璃改用 `sidebar` 实底）。强调色始终取用户的 macOS 设置。
- 界面：`ui`（SF Pro + PingFang SC，跟随系统，不打包）。对话框和设置标题 `title`，正文 `body`，次要 `caption`，侧边栏分组名 `section`。
- 列表与路径：`mono`（JetBrains Mono Nerd Font Mono）。文件列表 `list` 默认 14px（设置里可调 13–18）、行高取 1.25 倍字号且不低于 22px，字形比文字大 3pt，路径胶囊 `path`，键帽 `keycap` 11px / 600，用系统字体（等宽字体会把 ⇧⌘⌥⌃ 各压进一格，挤在一起）。没装字体时状态栏提示一键下载（只下两个 ttf），回退为 `SF Mono`。
- 数字一律等宽（大小、时间、计数），检查器的值用 `body` 加等宽数字。
- 字标 `wordmark`：“goFinder.”，800、−0.035em，只出现在关于窗口和设置页页眉，是界面内唯一的品牌标识；应用图标见 §6，界面里不画图形 Logo，设置页页眉不放图标。

## 5. 版式与尺寸

- 4px 网格：`space-1` 4 · `space-1.5` 6 · `space-2` 8 · `space-3` 12 · `space-4` 16，`space-0.5` 2 只用于字母与字形间。
- 固定尺寸：`toolbar-height` 38、`statusbar-row` 22（两行）、`sidebar-width` 190、`inspector-width` 280。
- 列宽贴合本栏最长文件名，上限后中间截断并保留扩展名（`build.gra….kts`）。
- 圆角：行 `radius-row` 5，控件 `radius-control` 6，菜单与对话框 `radius-menu` 10，路径胶囊、分段、键帽 `radius-pill`。

## 6. 图标

- 侧边栏：SF Symbols 线性图标（`house`、`arrow.down.circle`、`doc`、`icloud`、`externaldrive`…），`muted`，选中且有焦点时 `selection-ink`。
- 文件列表：Nerd Font 字形（参照 Yazi 与 nerd-fonts 的 seti/dev/md），按类别用 `type-*` 着色；光标所在的文件夹换成“打开的文件夹”字形。
- 工具栏、状态栏：SF Symbols，`ink`；Agent 按钮用三道光字形，`signal`。
- 不用 emoji，不用彩色插画（应用图标除外，见下）。

### 应用图标

- 来源文件：`resources/icon.png`（1024×1024，透明外圈）；MyGo 打包时自动生成 `AppIcon.icns`，不要手改 icns。
- 风格：儿童简笔画风，缝合四宫格——四叶草、叉车、Finder 式笑脸、橡皮鸭，中间是缝线十字。刻意与界面的灰阶极简形成反差，只在图标里出现彩色。
- 用在哪里：Dock、关于窗口（macOS 自带 RoleAbout 自动取应用图标）、Finder 与系统选择器。设置页页眉沿用文字字标“goFinder.”，不放图标。
- 最小尺寸：32px。32px 以上四个图案都可辨认；16px 只剩四色块，仅由系统在列表里缩放，不要在界面里主动使用。
- 禁止：不重新着色（含深色模式变体、单色化）、不加阴影或描边、不裁切（保留透明外圈）、不拉伸变形、不在其上叠文字。

## 7. 文案

- 动词开头、短：“移到废纸篓”“在这里打开 Claude”“复制路径”。
- 状态栏写结果句：“已拷贝 3 项”“已复制 文件名”“过滤：.jpg · 12 项”；出错写原因：“无法打开：没有权限”。
- 确认框问一句、给两个按钮：“将 3 个项目移到废纸篓？”→「取消」「移到废纸篓」（`danger`，↩ 确认，Esc 取消）。
- 键位写法：单键小写 `c`、双击 `c c`、组合 `⌘⌥A`；菜单每项一行字一个键。

## 8. 不要这样做

- 给面板加阴影、给按钮加描边、做渐变或毛玻璃之外的透明叠层。
- 把选中行、焦点或主按钮做成青色；把青色铺成大块背景。
- 用颜色单独表达状态（Git、删除、错误都要有字母或文字）。
- 加入场动画、悬停放大、加载骨架。
- 在列表里显示大小/日期列，或把文件名染成类型色。
- 失焦选中用白色或浅灰底配白字。
- 把应用图标放进界面（页眉、侧边栏、工具栏），或对它重新着色、加阴影、裁切。

## 9. Token 表

### 颜色（浅色 / 深色）

| token | 浅色 | 深色 | 用途 |
|---|---|---|---|
| `bg` | `#ffffff` | `#141516` | 文件列表区（上级栏、当前栏、预览栏、检查器）的底色，占窗口面积最大。 |
| `sidebar` | `#f2f4f5` | `#1b1d1f` | 侧边栏底色，仅在窗口没有 VibrancySidebar 材质时使用；有材质时侧边栏透明。 |
| `surface-100` | `#f2f4f5` | `#1b1d1f` | 一级面：工具栏、两行状态栏、路径胶囊、过滤输入框、检查器值的悬停底。靠它和 bg 分层，不加描边、不加阴影。 |
| `surface-200` | `#e7eaec` | `#26292c` | 二级面：行悬停、按钮按下、分段控件底、键帽底、失焦时的选中行。ink 在其上 16.4:1（深色 13.6:1）。 |
| `separator` | `#d5dadf` | `#34383c` | 全应用唯一的“线”：栏与栏之间、两行状态栏之间、工具栏与内容之间的 1px 分隔。装饰性，不承载含义，故不要求 3:1。 |
| `ink` | `#0a0a0a` | `#f5f7f8` | 文件名、标题、正文、图标默认色，读在 bg、surface-100、surface-200 上（≥13.6:1）。 |
| `muted` | `#5b6168` | `#9aa1a8` | 次要文字：检查器字段名、状态栏结果句、侧边栏分组名、键帽旁的动作名。在 bg/surface-100/surface-200 上 ≥5.2:1。 |
| `faint` | `#80878e` | `#6f767d` | 第三级：隐藏文件名、禁用项、占位文字、Git 未跟踪标记。对 bg 3.6:1（深色 4.0:1），只用于非关键信息，不做正文。 |
| `selection` | `#0a64d6` | `#0a64d6` | 系统强调色：选中行、光标行（列表有焦点时）、主按钮、开关与分段的选中态。运行时读用户的 macOS 强调色，这里是默认蓝的回退值。上面的字用 selection-ink（5.5:1）。 |
| `selection-ink` | `#ffffff` | `#ffffff` | selection 底上的文字与字形。 |
| `selection-inactive` | `{surface-200}` | `{surface-200}` | 列表失去键盘焦点时的选中行底色，文字保持原色（ink），图标保持类型色。 |
| `focus` | `{selection}` | `{selection}` | 键盘焦点所在栏的 1.5px 边框（侧边栏、当前栏、双栏中的活动栏）。跟随系统强调色，与选中行同色，对 bg ≥3:1。 |
| `signal` | `#00dafc` | `#00dafc` | 品牌青，只做点缀、不承担交互状态：字标 goFinder. 的句点、Agent（交给 AI）的三道光、“已完成”小圆点、过滤生效标记。每屏 ≤3 处。上面的字用 signal-ink。 |
| `signal-ink` | `#000000` | `#000000` | 青底上的文字，12.4:1。青底永远配黑字，不配白字。 |
| `signal-text` | `#00718a` | `#00dafc` | 需要青色“文字”时用：Agent 菜单项里的“默认”标签、状态栏“过滤：xx”。浅色 5.6:1；深色下 #00DAFC 在 bg 上 10.8:1。 |
| `signal-soft` | `#e0f9fe` | `#00262d` | 青色浅底：设置 › Agent 页里默认 Agent 那一行的底，配 signal-text。 |
| `danger` | `#b42318` | `#ff6b6b` | 删除（移到废纸篓）菜单项与确认按钮文字、错误提示、Git 冲突。在 bg 上 6.6:1。永远配文字，不单靠颜色。 |
| `warning` | `#ffb020` | `#ffb020` | 琥珀填充：缺字体提示的小圆点。只做填充，上面的字用 ink 的浅色值 #0a0a0a。 |
| `warning-text` | `#8a5a00` | `#ffb020` | 琥珀文字：Git 已修改标记 M、“部分完成”。浅色 5.9:1，深色 10.0:1。 |
| `git-added` | `#1a7f37` | `#3fb950` | Git 新增标记 A。永远是字母 + 颜色，不靠色相区分（与 danger 同在时字母不同）。 |
| `git-modified` | `{warning-text}` | `{warning-text}` | Git 已修改标记 M、含改动的文件夹的 •。 |
| `git-untracked` | `{muted}` | `{muted}` | Git 未跟踪标记 ?。 |
| `git-conflict` | `{danger}` | `{danger}` | Git 冲突标记 U。 |
| `type-folder` | `#1f6feb` | `#58a6ff` | 文件列表字形：文件夹（含打开态）、.app、Git 仓库根。对 bg 4.6:1（深色 7.2:1）。 |
| `type-code` | `#bc4c00` | `#ffa657` | 字形：源代码与脚本（go、ts、py、sh…）。 |
| `type-doc` | `#57606a` | `#adb5bd` | 字形：文档、文本、Markdown、PDF、表格、演示文稿。 |
| `type-image` | `#bf3989` | `#f778ba` | 字形：图片与设计稿。 |
| `type-media` | `#8250df` | `#bc8cff` | 字形：视频与音频。 |
| `type-archive` | `#9a6700` | `#e3b341` | 字形：压缩包、磁盘映像、安装包。 |
| `type-exec` | `#1a7f37` | `#3fb950` | 字形：可执行文件（有 x 权限的普通文件）。 |
| `type-config` | `#6e7781` | `#8b949e` | 字形：配置、锁文件、证书与密钥、日志、隐藏的点文件。对 bg 4.6:1（深色 5.9:1）。 |

### 字体

- `ui`：-apple-system, "SF Pro Text", "PingFang SC", "Hiragino Sans", "Hiragino Sans GB", system-ui, sans-serif
- `mono`："JetBrainsMono Nerd Font Mono", "JetBrainsMono Nerd Font", "SF Mono", ui-monospace, Menlo, monospace

| 样式 | 字族 | 字号 / 行高 / 字重 | 用途 |
|---|---|---|---|
| `wordmark` | ui | 28px / 1 / 800 | 关于窗口、设置页页眉的字标；句点用 signal。只有这里用 800。 |
| `title` | ui | 15px / 20px / 600 | 对话框标题、设置分页标题、检查器文件名。 |
| `body` | ui | 13px / 18px / 400 | 菜单项、按钮、设置项、侧边栏项、检查器值。 |
| `body-strong` | ui | 13px / 18px / 600 | 设置里的分组标题、对话框主按钮文字。 |
| `caption` | ui | 11px / 14px / 400 | 检查器字段名、状态栏结果句、提示文字。 |
| `section` | ui | 11px / 14px / 700 | 侧边栏分组名，muted。 |
| `list` | mono | 14px / 22px / 400 | 文件列表（上级、当前、子项）。默认 14，可在设置中调整为 13–18；字形比文字大 3pt；行高取 1.25 倍字号，不低于 row-height 22px。 |
| `path` | mono | 13px / 18px / 400 | 路径胶囊里的各级目录名、过滤输入框。 |
| `keycap` | 系统字体，字距 1px | 11px / 16px / 600 | 状态栏、速查表、菜单右侧的键帽字母。 |

### 间距

| token | 值 | 用途 |
|---|---|---|
| `space-0.5` | 2px | 字形与文件名之间以外的最小间隙：键帽内上下、Git 字母与文件名。 |
| `space-1` | 4px | 键帽之间、图标按钮内边距、分段控件内边距。 |
| `space-1.5` | 6px | 列表行左右内边距、字形与文件名间距、侧边栏项图标与文字间距。 |
| `space-2` | 8px | 工具栏按钮间距、状态栏左右内边距、提示组内间距。 |
| `space-3` | 12px | 提示组之间、检查器字段之间、对话框元素之间。 |
| `space-4` | 16px | 对话框、设置页、菜单的外边距；工具栏按钮分组之间。 |

### 圆角

| token | 值 | 用途 |
|---|---|---|
| `radius-row` | 5px | 文件列表行、侧边栏项、检查器值悬停底。 |
| `radius-control` | 6px | 按钮、输入框、工具栏图标按钮。 |
| `radius-menu` | 10px | 右键菜单、下拉、对话框、暂存架面板。 |
| `radius-pill` | 999px | 路径胶囊、分段控件、键帽、“已完成”圆点——从 Unlocknote 继承的胶囊形状。 |

### 尺寸

| token | 值 | 用途 |
|---|---|---|
| `row-height` | 22px | 文件列表、侧边栏、菜单项的行高。 |
| `toolbar-height` | 38px | 工具栏高度（含上下 space-1）。 |
| `statusbar-row` | 22px | 两行状态栏每一行的高度。 |
| `sidebar-width` | 190px | 侧边栏宽度。 |
| `inspector-width` | 280px | 检查器（第 4 栏）固定宽度。 |
| `focus-width` | 1.5px | 键盘焦点栏边框宽度。 |

## 10. 组件规范

组件预览见 Artifact；规范如下。

### ColumnView

Yazi 式三列：上级｜当前｜预览（光标在文件夹上时列子项），可选第 4 栏检查器。

- 栏与栏之间只有 1px `separator`；栏宽贴合最长文件名。
- 拿着键盘焦点的栏画 `focus-width` 1.5px 的 `focus` 边框，圆角 `radius-row`；其他栏无边框。
- 上级栏里当前路径所在项用 `selection-inactive`（它不是焦点栏）。
- 双栏布局时每个窗格都是完整的三列，焦点边框只在活动窗格的当前栏。

### ConfirmDialog

确认框：`radius-menu`，`bg` 底，内边距 `space-4`。标题 `title` 一句问话，下面最多列 5 个名称（`list`），底部两个按钮。

- 主按钮“移到废纸篓”：`danger` 文字，`surface-200` 底，↩ 确认；“取消”：Esc。
- 左下角“不再提醒”复选框；设置的「通用」页可再打开。
- 遮罩用半透明 `ink`，不模糊。

### FileRow

文件列表的一行：Nerd Font 字形 + 文件名 + 可选 Git 字母，高 `row-height` 22px，字体 `list`。

- 消费方提供：字形与类型色（`type-*`）、名称、Git 字母、状态（普通 / 悬停 / 选中 / 失焦选中 / 光标所在文件夹）。
- 选中且列表有焦点：`selection` 底 + `selection-ink`，字形也变白。失焦：`selection-inactive` 底，文字 `ink`、字形保持类型色。
- 光标停在文件夹上：字形换成“打开的文件夹”。隐藏文件：名称 `faint`。
- 名称过长：中间截断并保留扩展名。文件名永远不染色。
- 不显示大小、日期列；行之间没有分隔线。

### FilterField

按 `f` 打开的过滤输入：在当前栏底部，`surface-100` 胶囊（`radius-pill`），`path` 字体，左侧“过滤”标签 `muted`。

- 边输入边过滤，smart case；↩ 保留并回到列表，Esc 清除。
- 生效后输入框收起，状态栏上一行显示“过滤：xx”（`signal-text`）。

### Inspector

检查器是宽 `inspector-width` 280px 的第 4 栏，只显示属性，不显示内容预览。参照 Finder 的属性面板。

- 顶部：文件名（`title`，粗），下一行“种类 - 大小”（`caption` `muted`）。
- 两个可折叠的组，标题 `body` 粗体、右侧 ▾/▸：
  - **通用**：路径、大小（`1.5 MB（1 469 302 字节）`）、修改/创建/添加时间、扩展名、隐藏扩展名 ☐、隐藏 ☐；文件夹改为子项数与总大小。
  - **权限**：所有者/组/其他 × R/W/X 勾选网格；八进制行（`rw-r--r--` 与 `644` 小底块）；所有者、组（只读文本）；锁定 ☐。
- 行：左标签 `muted`，右值右对齐（`body`，数字等宽），行间 1px `separator`。
- 点击值复制：悬停 `surface-100` 底 + 手形光标，复制后状态栏写“已复制 字段名”。
- 勾选框会直接改文件：权限 `chmod`、隐藏/锁定 `chflags`、隐藏扩展名写 `com.apple.FinderInfo` 的标志位；zip 内、符号链接、非本人所有的文件勾选框禁用；失败时状态栏显示错误。
- 底部两个按钮纵向排列：“加入暂存架”“打开暂存架”。

### Menu

右键菜单与工具栏下拉由应用自绘：`radius-menu` 圆角、`bg` 底、`separator` 细分隔，项高 `row-height`，左右内边距 `space-3`。

- 每项一行字 `body` + 右侧一个键 `keycap`（`muted`，无键帽底）；悬停/键盘高亮用 `selection`。
- 顺序：在终端中打开，然后是全部已启用的 Agent（三道光 `signal`，默认 Agent 标 `a`），再是文件操作；“移到废纸篓”文字 `danger` 放最后。
- 原生菜单栏同样一行字一个键，样式由 macOS 绘制。

### PathBar

路径栏是一整条 `radius-pill` 胶囊，`surface-100` 底，内容用 `path` 字体。

- 各级目录名可点击跳转；级间的 `▸` 可点开子目录菜单；深层路径中段折叠为 `…`。
- 点空白处或 `⌘L` 进入输入，输入框同样是胶囊。
- 结尾可带 Git 分支（`caption`，`muted`）。没有描边，没有阴影。

### Segmented

分段控件：`radius-pill` 胶囊，`surface-200` 底，选中段 `selection` 底 + `selection-ink`。用于状态栏的“三列 / 左右 / 上下”和设置里的“启用 / 禁用”。

- 每段 22px 高，内边距 `space-2`；段之间没有分隔线。
- 选中色跟随系统强调色，不用青色。

### SidebarItem

侧边栏项：SF Symbols 线性图标 + 名称（`body`），高 22px，圆角 `radius-row`；分组名用 `section`（11/700，`muted`）。

- 底色：有窗口材质时透明，否则 `sidebar`。
- 选中且侧边栏有焦点：`selection` 底 + `selection-ink`；失焦：`selection-inactive`。
- 条目、顺序、分组只读跟随 Finder；自有收藏夹放最下。

### StatusBar

两行状态栏，每行 `statusbar-row` 22px，`surface-100` 底，两行之间一条 `separator`。

- 上一行：左边是结果句（`caption`，`muted`），完成时前面一个 6px `signal` 圆点；过滤生效时显示“过滤：xx”（`signal-text`）。右边是视图分段控件和检查器开关。不显示项目数。
- 下一行：键帽提示。键帽 `keycap` + 动作名 `caption`，按组排列，组间 `space-3`。默认顺序：f 过滤、c 拷贝、c c 拷贝路径、x 剪切、v 粘贴、n 新文件夹、n n 新文件、↩ 重命名、d 删除、? 帮助。
- 按住修饰键时第二行换成带该修饰键的全部快捷键。

### Toolbar

工具栏**默认隐藏**（⌃⌘B 或设置开关显示）。窗口不用系统标题栏：隐藏时顶部是一条与红黄绿按钮同高的 `surface-100` 标题条，居中显示当前文件夹名（`caption`，`muted`），可拖动窗口；显示时工具栏就是标题栏，左侧让出红黄绿按钮的位置。工具栏高 `toolbar-height` 38px（不低于标题栏高度），`surface-100` 底，下方一条 `separator`。

- 按钮是无边框图标按钮（SF Symbols，`ink`），28×24，圆角 `radius-control`；悬停 `surface-200`，开启态（暂存架、隐藏文件、过滤生效时的过滤）用 `surface-200` 常驻。
- 默认按钮，从左到右：后退、前进、上级 ｜ 新建▾、排序▾、显示隐藏文件 ｜ 移到废纸篓 ｜ 终端、Agent▾ ｜ 弹性空白 ｜ Workspace、暂存架、过滤。
- 分组之间留 `space-4`。Agent 是一个分体按钮：`surface-200` 底、`radius-control`，左段三道光（`signal`）加默认 Agent 名、点击在这里打开；中间 1px `separator`；右段 ▾ 列出已启用 Agent。悬停只高亮当前段；没有启用的 Agent 时左段禁用。
- 工具栏提示 0 等待，写成“名称 · 一句话说明 · 快捷键”；窗口其他地方的提示保持默认延迟。
- 移到废纸篓：图标平时 `ink`，悬停时 `danger`；没有选中项或在 zip 内时禁用（`muted`）。上级在根目录时禁用。
- 设置的「工具栏」页：左右两列，底色 `bg`，行高约 26px，选中行 `selection`；上移、下移、移除用同样的 28×24 图标按钮；「恢复默认」是 `surface-200` 的普通按钮。
- 悬停提示写“名称 + 键”。

