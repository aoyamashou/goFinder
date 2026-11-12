package gui

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

// 关于页的内容取自作者的 AboutKit（github.com/aoyamashou/AboutKit）：
// 项目仓库、简介、作者与链接、头像、致谢开源项目、声明，按 goFinder 的
// 设计系统重画，不照搬代码。

const repoURL = "https://github.com/aoyamashou/goFinder"

const aboutSummary = "轻量、键盘鼠标都优先的 macOS 文件管理器，把访达和 Yazi 的长处融在了一起：" +
	"Yazi 式三栏、单键操作、边打字边过滤，用 MyGo 的原生界面写成，纯 Go、不用 WebView。"

const aboutDisclaimer = "goFinder 是个人开发的开源软件，与 Apple、访达（Finder）及 Yazi 项目没有任何关系。" +
	"删除只会把文件移到废纸篓，从不永久删除。"

// aboutLink is a link button: an SF Symbol, or a glyph when sym is "".
type aboutLink struct{ title, url, sym, glyph string }

var authorLinks = []aboutLink{
	{"作者网站", "https://ryou.im", "globe", ""},
	{"X 主页", "https://x.com/corndogjpn2", "", "𝕏"},
	{"小红书", "https://xhslink.cn/o/2M9LOaUzn01", "", "书"},
	{"GitHub", "https://github.com/aoyamashou", "person.crop.circle", ""},
}

// credit is an open-source project goFinder uses or learned from.
type credit struct{ name, url, author, license, usage string }

var credits = []credit{
	{"MyGo", "https://github.com/egoist/mygo", "egoist", "MIT", "原生界面框架"},
	{"Yazi", "https://github.com/sxyazi/yazi", "sxyazi", "MIT", "三栏布局与单键键位的灵感"},
	{"Nerd Fonts", "https://github.com/ryanoasis/nerd-fonts", "Ryan L McIntyre", "MIT", "文件图标字形"},
	{"JetBrains Mono", "https://github.com/JetBrains/JetBrainsMono", "JetBrains", "OFL-1.1", "列表字体"},
}

//go:embed author.jpg
var authorJPG []byte

// authorAvatar is the author's picture, decoded once.
var authorAvatar = sync.OnceValue(func() *ui.Bitmap {
	b, err := ui.DecodeBitmap(authorJPG)
	if err != nil {
		return nil
	}
	return b
})

const avatarSize = 94

func (a *app) aboutPage(c *ui.Context) {
	ui.Row(c).Gap(px("space-3")).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(px("space-1")).Children(func() {
			ui.Text(c, "goFinder").Bold()
			uiCaption(c, "版本 "+appVersion(), "muted")
		})
		a.linkButton(c, aboutLink{"项目仓库", repoURL, "chevron.left.forwardslash.chevron.right", ""})
	})
	ui.Text(c, aboutSummary).FontSize(fontBody)

	ui.Row(c).Gap(px("space-3")).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(px("space-2")).Children(func() {
			ui.Text(c, "本应用由Claude Code提携 Corndog 开发。").Bold()
			ui.Text(c, "aka. 狗大V | リョウ | aoyamashou | 性感玉米。").FontSize(fontBody)
			ui.Row(c).Gap(px("space-2")).Wrap().Children(func() {
				for _, l := range authorLinks {
					a.linkButton(c, l)
				}
			})
		})
		if bmp := authorAvatar(); bmp != nil {
			ui.Image(c.Key("about.avatar"), bmp).Fit(ui.Cover).Width(avatarSize).Height(avatarSize).
				Radius(px("radius-row") + 3).Clip().Shrink(0)
		}
	})

	ui.Divider(c)
	ui.Text(c, "版本信息").Bold()
	a.aboutInfo(c)

	ui.Divider(c)
	ui.Text(c, "致谢开源项目").Bold()
	for _, cr := range credits {
		ui.Row(c).Gap(px("space-1.5")).AlignItems(ui.Center).Children(func() {
			b := ui.ButtonBase(c.Key("credit." + cr.name)).Label(cr.name).Tooltip(cr.url).Cursor(ui.CursorPointer)
			if b.Clicked() {
				a.openLink(cr.url)
			}
			b.Children(func() {
				ui.Text(c, cr.name).FontSize(fontBody).Underline().SingleLine()
			})
			uiCaption(c, strings.Join([]string{"作者 " + cr.author, cr.license + " 协议", cr.usage}, " · "), "muted").
				SingleLine().Shrink(1).MinWidth(0)
		})
	}

	ui.Divider(c)
	ui.Text(c, "声明").Bold()
	uiCaption(c, aboutDisclaimer, "muted")

	ui.Divider(c)
	ui.Checkbox(c.Key("about.debug"), &a.debugOn, "Debug 编号")
	uiCaption(c, "开启后界面各元素左上角显示固定编号，沟通时说编号即可；只在本次运行有效，编号表见 docs/debug-ids.md", "muted")
}

// aboutInfo is the build's and this Mac's details: build, updates, MyGo,
// bundle ID and the data folder.
func (a *app) aboutInfo(c *ui.Context) {
	bundle := a.bundleID
	if bundle == "" {
		bundle = fallbackBundleID
	}
	row := func(label, value string) {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).Width(72).TextColor(tok(c, "muted"))
			ui.Text(c, value).Font("").FontSize(12).SingleLine().Shrink(1).MinWidth(0)
		})
	}
	if b := buildInfo(); b != "" {
		row("构建", b)
	}
	if a.updates {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "更新").Width(72).TextColor(tok(c, "muted"))
			if v := updater.AvailableVersion(); v != "" {
				ui.Text(c, "有新版本 "+v).TextColor(tok(c, "signal"))
			}
			if ui.Button(c.Key("about.update"), "检查更新…").Clicked() {
				updater.CheckForUpdates()
			}
		})
	}
	row("MyGo", mygoVersion())
	row("Bundle ID", bundle)
	if a.dataDir != "" {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "数据目录").Width(72).TextColor(tok(c, "muted"))
			if ui.Button(c.Key("about.datadir"), a.dataDir).Tooltip("点击复制路径").Clicked() {
				if a.hasSvc {
					a.svc.WriteClipboard(a.dataDir)
				}
				a.status = "已复制数据目录路径"
			}
		})
	}
}

// linkButton is faceButton with a 14px icon before the title; it opens
// the URL, which is its tooltip.
func (a *app) linkButton(c *ui.Context, l aboutLink) {
	const s = 14
	b := ui.ButtonBase(c.Key("link."+l.title)).Height(toolbarButtonH).Radius(px("radius-control")).
		Padding(0, px("space-2")).Label(l.title).Tooltip(l.url)
	bg := tok(c, "surface-200")
	if b.Hovered() {
		bg = tok(c, "separator")
	}
	col := tok(c, "ink")
	b.Background(bg).TextColor(col).Shrink(0).Children(func() {
		ui.Row(c).Gap(px("space-1.5")).AlignItems(ui.Center).Children(func() {
			switch {
			case l.sym != "" && sfsymbols.Available():
				ui.Box(c).Width(s).Height(s).Draw(func(p *ui.Painter, r ui.Rect) {
					if src, ok := sfsymbols.Image(l.sym, int(s*max(p.Scale(), 1)+0.5), col, sfsymbols.Options{Natural: true}); ok {
						p.Image(src, r, ui.Contain)
					}
				})
			case l.glyph != "":
				ui.Text(c, l.glyph).FontSize(fontBody).Bold().SingleLine().Width(s).Center()
			}
			ui.Text(c, l.title).FontSize(fontBody).SingleLine()
		})
	})
	if b.Clicked() {
		a.openLink(l.url)
	}
}

// openLink opens url in the browser and says so.
func (a *app) openLink(url string) {
	if err := a.sys.OpenURL(url); err != nil {
		a.status = "无法打开：" + err.Error()
	}
}
