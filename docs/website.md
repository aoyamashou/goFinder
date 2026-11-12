# goFinder 官网

> 来源：goFinder 主会话，2026-11-12。线上地址：https://gofinder.ryou.im
> 状态：首版已上线（Cloudflare Workers Static Assets，域名 `ryou.im` 在同一 Cloudflare 账号下）。

## 做了什么、为什么

落地页是一个静态单页：介绍特点、性能、键位、常见问题，并把所有下载入口指向 GitHub Releases（`/releases/latest`）。目的是给软件一个对外的门面，不替代 README。

## 目录与部署

- `site/public/index.html`：整页（内联 CSS 与少量 JS，无构建、无外部依赖、无外部字体）。
- `site/public/assets/icon.png`：应用图标，来自 `resources/icon.png`，原样使用（不裁切、不改色）。
- `site/cloudflare.config.ts`：Worker 名 `gofinder`，路由 `gofinder.ryou.im/*`（zone `ryou.im`）；`site/wrangler.config.ts` 指向 `public/`。
- 部署（在 `site/` 下，已登录 `cf`）：

  ```bash
  cd site && npm install && cf deploy
  ```

- DNS：`gofinder` 是一条开启代理的 `AAAA 100::` 占位记录，Worker 路由靠它进入 Cloudflare；不要删。
- `site/.cloudflare/`、`node_modules/` 不进仓库。

## 设计

- 依据 `docs/design-system.md`：默认深色、跟随系统浅色；灰阶承担一切，品牌青只用在字标句点、下载按钮、过滤高亮和展开的常见问题；选中行用系统蓝；零渐变、零阴影、无装饰动画（入场动画只用透明度和位移，并遵守 `prefers-reduced-motion`）。
- 构图用「平铺方块」：12 列、8px 间距、统一圆角；页面里的图片全部是用 HTML 按设计系统 token 还原的界面片段（`data-ui`），不放真实截图，避免露出个人文件名。
- 首屏下方、「小，而且快」上方有一块上下双栏的界面图：按软件真实截图重绘成 HTML（`data-ui`），文件名、用户名、Agent 等都换成占位内容，颜色按设计系统（选中行与工作区高亮用品牌青）；手机宽度只留一个窗格。
- 首屏只有两块：字标和一句话说明加下载按钮；2026-11-12 按用户要求删掉了首屏的界面演示窗、应用图标块和「常用操作只要一个键」块。
- 字体只用系统字体栈；网页里的界面片段用系统字体，不用等宽字体。

## 内容边界

- 性能数字、键位、安装说明都取自 `README.md`；改了 README 的这些内容要同步改这里。
- 页面不写未证实的内容：没有用户评价、下载量、许可证、价格；下载按钮不写「免费」。
- 不放个人信息（邮箱、账号 ID）。

## 待办

- 暂无真实录屏；有了就放进首屏或「性能」。
- 发新版后无需改页面（下载链接指向 latest）；若 Intel 版本出现，改常见问题与下载说明。
