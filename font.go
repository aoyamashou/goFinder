package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/icons"
)

// The lists' font is downloaded, only when the user asks, from the
// nerd-fonts repository: its two faces, into ~/Library/Fonts.
const fontBaseURL = "https://github.com/ryanoasis/nerd-fonts/raw/HEAD/patched-fonts/JetBrainsMono/Ligatures/"

var fontFiles = []string{"JetBrainsMonoNerdFontMono-Regular.ttf", "JetBrainsMonoNerdFontMono-Bold.ttf"}

const fontMaxSize = 20 << 20 // a face is about 2.5 MB

// fontState is whether the lists' font is installed, and its download.
type fontState struct {
	checked bool // the system's fonts were looked through
	mono    bool // JetBrainsMono Nerd Font Mono is there

	busy  bool
	done  int // faces downloaded
	err   string
	saved bool // downloaded in this run

	// Where the download comes from and goes, and how a face is put to
	// use: tests change them.
	baseURL  string
	dir      string
	client   *http.Client
	register func(data []byte, family string) error
}

func newFontState() fontState {
	dir := ""
	if home, err := os.UserHomeDir(); err == nil {
		dir = filepath.Join(home, "Library", "Fonts")
	}
	return fontState{
		baseURL: fontBaseURL, dir: dir, register: ui.RegisterFont,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// checkFonts looks for the Nerd Fonts off the main thread: the Mono one
// the lists use, or the other, for the glyphs alone.
func (a *app) checkFonts() {
	a.async(func() {
		mono := hasFont(icons.Font)
		nerd := mono || hasFont(icons.FontAlt)
		a.post(func() {
			a.nerd, a.font.mono, a.font.checked = nerd, mono, true
		})
	})
}

// downloadFont fetches the font's faces off the main thread, saying how
// far it got, and puts them to use as they are written: no restart.
func (a *app) downloadFont() {
	f := &a.font
	if f.busy || f.dir == "" {
		return
	}
	f.busy, f.done, f.err = true, 0, ""
	base, dir, client := f.baseURL, f.dir, f.client
	a.async(func() {
		var faces [][]byte
		for i, name := range fontFiles {
			data, err := fetchFont(client, base+name)
			if err == nil {
				err = writeFile(filepath.Join(dir, name), data)
			}
			if err != nil {
				a.post(func() {
					f.busy, f.err = false, fmt.Sprintf("%s：%v", name, err)
				})
				return
			}
			faces = append(faces, data)
			a.post(func() { f.done = i + 1 })
		}
		a.post(func() {
			f.busy = false
			for _, data := range faces {
				if err := f.register(data, icons.Font); err != nil {
					f.err = "已下载，但无法载入：" + err.Error()
					return
				}
			}
			f.mono, f.saved, a.nerd = true, true, true
			a.charW = 0 // measure the new font
			a.status = "已安装 JetBrains Mono Nerd Font Mono 到 " + dir
		})
	})
}

// fetchFont downloads one face, checking it is a font.
func fetchFont(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, fontMaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > fontMaxSize {
		return nil, fmt.Errorf("文件过大")
	}
	if !isFont(data) {
		return nil, fmt.Errorf("下载的不是字体文件")
	}
	return data, nil
}

// isFont tells a TrueType or OpenType file by its first bytes.
func isFont(b []byte) bool {
	return len(b) >= 4 && (bytes.Equal(b[:4], []byte{0, 1, 0, 0}) || string(b[:4]) == "true" || string(b[:4]) == "OTTO")
}

// writeFile writes data to p whole or not at all.
func writeFile(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p+".part", data, 0o644); err != nil {
		return err
	}
	return os.Rename(p+".part", p)
}

// fontNotice offers the download while the lists' font is missing, and
// says how it goes. The status bar shows it.
func (a *app) fontNotice(c *ui.Context) {
	f := &a.font
	if !f.checked || (f.mono && f.err == "" && !f.busy) {
		return
	}
	t := c.Theme()
	ui.Row(c.Key("fontnotice")).Gap(6).AlignItems(ui.Center).Children(func() {
		switch {
		case f.busy:
			ui.Text(c, fmt.Sprintf("正在下载 JetBrains Mono Nerd Font %d/%d…", f.done, len(fontFiles))).FontSize(12).TextColor(t.TextMuted).SingleLine()
		case f.err != "":
			ui.Text(c, "字体下载失败："+f.err).FontSize(12).TextColor(t.Danger).SingleLine().Shrink(1)
			if a.flatButton(c.Key("font.retry"), "重试", "再次下载字体", false).Clicked() {
				a.downloadFont()
			}
		default:
			ui.Text(c, "未安装 JetBrains Mono Nerd Font").FontSize(12).TextColor(t.TextMuted).SingleLine()
			ui.Text(c, "·").FontSize(12).TextColor(t.TextMuted)
			if a.flatButton(c.Key("font.dl"), "下载", fontTip(), false).TextColor(t.Accent).Clicked() {
				a.downloadFont()
			}
		}
	})
}

func fontTip() string {
	return "从 nerd-fonts 的 GitHub 下载 " + strings.Join(fontFiles, " 和 ") + "（各约 2.5 MB）到 ~/Library/Fonts"
}

// fontSettings is the settings' row of the lists' font.
func (a *app) fontSettings(c *ui.Context) {
	t := c.Theme()
	f := &a.font
	ui.Text(c, "列表字体").Bold()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		state := "JetBrainsMono Nerd Font Mono · 13pt"
		switch {
		case !f.checked:
			state += "（正在检查…）"
		case f.busy:
			state += fmt.Sprintf("（正在下载 %d/%d…）", f.done, len(fontFiles))
		case f.mono:
			state += "（已安装）"
		default:
			state += "（未安装，暂用系统等宽字体）"
		}
		ui.Text(c, state).FontSize(12).Grow(1)
		if ui.Button(c.Key("font.get"), "下载字体").Tooltip(fontTip()).Disabled(!f.checked || f.busy || f.mono).Clicked() {
			a.downloadFont()
		}
	})
	if f.err != "" {
		ui.Text(c, "下载失败："+f.err).FontSize(11).TextColor(t.Danger)
	}
}
