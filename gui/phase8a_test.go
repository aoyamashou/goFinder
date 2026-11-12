package gui

import (
	"archive/zip"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/vfs"
)

func TestQuickLookKey(t *testing.T) {
	m := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	k := qlKey("/a/b.pdf", m, 800)
	if k != qlKey("/a/b.pdf", m.In(time.Local), 800) || !strings.HasSuffix(k, ".png") {
		t.Fatalf("unstable key %s", k)
	}
	for _, other := range []string{qlKey("/a/c.pdf", m, 800), qlKey("/a/b.pdf", m.Add(time.Nanosecond), 800), qlKey("/a/b.pdf", m, 400)} {
		if other == k {
			t.Fatal("path, time and size must each change the key")
		}
	}
}

// thumbSetup is setup with a cache folder and the file name in root,
// the cursor on it.
func thumbSetup(t *testing.T, name string) (*app, *ui.Tester, *fakeSys, string) {
	t.Helper()
	a, tt, fs, root := setup(t)
	a.cacheDir = t.TempDir()
	os.WriteFile(filepath.Join(root, name), []byte("%PDF-1.4 fake"), 0o644)
	a.load(a.tab())
	tt.Frame()
	goTo(t, a, tt, name)
	tt.Frame()
	return a, tt, fs, root
}

func TestQuickLookThumbnail(t *testing.T) {
	a, tt, fs, root := thumbSetup(t, "doc.pdf")
	pv := &a.panes[0].pv
	if !pv.ql || len(fs.thumbs) != 0 || !tt.HasText("正在生成预览…") {
		t.Fatalf("before the delay: ql %v thumbs %v", pv.ql, fs.thumbs)
	}
	tt.Advance(qlDelay + 10*time.Millisecond)
	tt.Frame()
	if len(fs.thumbs) != 1 || fs.thumbs[0] != filepath.Join(root, "doc.pdf") {
		t.Fatalf("thumbs %v", fs.thumbs)
	}
	if pv.kind != pvImage || !pv.thumb || pv.ql {
		t.Fatalf("after: kind %d thumb %v ql %v", pv.kind, pv.thumb, pv.ql)
	}
	e, _ := a.tab().cursorEntry()
	if _, err := os.Stat(a.qlFile(e)); err != nil || filepath.Dir(a.qlFile(e)) != filepath.Join(a.cacheDir, "ql") {
		t.Fatalf("cache %s: %v", a.qlFile(e), err)
	}
	// Away and back: the thumbnail comes from the cache.
	goTo(t, a, tt, "notes.md")
	tt.Frame()
	goTo2 := func(name string) {
		for cursorName(a) != name {
			tt.Key(0, ui.KeyUp)
			tt.Frame()
		}
	}
	goTo2("doc.pdf")
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if len(fs.thumbs) != 1 || pv.kind != pvImage || !pv.thumb {
		t.Fatalf("cache miss: thumbs %v kind %d", fs.thumbs, pv.kind)
	}
}

// TestQuickLookWaits: moving on before the delay starts no qlmanage.
func TestQuickLookWaits(t *testing.T) {
	a, tt, fs, _ := thumbSetup(t, "doc.pdf")
	tt.Advance(qlDelay / 2)
	goTo(t, a, tt, "notes.md")
	tt.Advance(time.Second)
	tt.Frame()
	if len(fs.thumbs) != 0 {
		t.Fatalf("qlmanage ran for an item passed by: %v", fs.thumbs)
	}
}

func TestQuickLookFails(t *testing.T) {
	a, tt, fs, _ := thumbSetup(t, "deck.key")
	fs.thumbErr = errors.New("QuickLook 超时")
	tt.Advance(time.Second)
	tt.Frame()
	pv := &a.panes[0].pv
	if len(fs.thumbs) != 1 || pv.ql || pv.kind == pvImage || !tt.HasText("deck.key") || tt.HasText("正在生成预览…") {
		t.Fatalf("failure: thumbs %v ql %v kind %d %q", fs.thumbs, pv.ql, pv.kind, tt.Texts())
	}
	if a.nerd && !tt.HasText("") { // the icon instead
		t.Fatalf("no icon: %q", tt.Texts())
	}
	// Not asked for again.
	a.panes[0].pv.path = ""
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if len(fs.thumbs) != 1 {
		t.Fatalf("asked again: %v", fs.thumbs)
	}
}

// TestQuickLookStale: a thumbnail arriving after the cursor moved on is
// dropped.
func TestQuickLookStale(t *testing.T) {
	a, tt, _, _ := thumbSetup(t, "doc.pdf")
	var queue []func()
	a.async = func(f func()) { queue = append(queue, f) }
	tt.Advance(time.Second)
	tt.Frame()
	if len(queue) != 1 {
		t.Fatalf("queued %d", len(queue))
	}
	a.async = func(f func()) { f() }
	goTo(t, a, tt, "notes.md")
	tt.Frame()
	queue[0]()
	if pv := a.panes[0].pv; pv.thumb || pv.kind != pvText {
		t.Fatalf("stale thumbnail shown: kind %d", pv.kind)
	}
}

func TestQuickLookInZip(t *testing.T) {
	a, tt, fs, root := setup(t)
	a.cacheDir = t.TempDir()
	p := filepath.Join(root, "docs.zip")
	f, _ := os.Create(p)
	w := zip.NewWriter(f)
	fw, _ := w.Create("in/page.html")
	fw.Write([]byte("<h1>hi</h1>"))
	w.Close()
	f.Close()
	a.navigate(a.tab(), vfs.ZipRoot(p)+"/in")
	tt.Frame()
	if cursorName(a) != "page.html" {
		t.Fatalf("cursor %q", cursorName(a))
	}
	tt.Advance(time.Second)
	tt.Frame()
	if len(fs.thumbs) != 1 || !strings.HasPrefix(fs.thumbs[0], filepath.Join(a.cacheDir, "zip")) {
		t.Fatalf("thumbs %v", fs.thumbs)
	}
	if b, err := os.ReadFile(fs.thumbs[0]); err != nil || string(b) != "<h1>hi</h1>" {
		t.Fatalf("extracted %q %v", b, err)
	}
	if !a.panes[0].pv.thumb {
		t.Fatal("no thumbnail")
	}
}

func TestPruneThumbs(t *testing.T) {
	dir := t.TempDir()
	old, fresh := filepath.Join(dir, "old.png"), filepath.Join(dir, "new.png")
	os.WriteFile(old, nil, 0o644)
	os.WriteFile(fresh, nil, 0o644)
	now := time.Now()
	os.Chtimes(old, now.Add(-qlMaxAge-time.Hour), now.Add(-qlMaxAge-time.Hour))
	pruneThumbs(dir, now)
	if _, err := os.Stat(old); err == nil {
		t.Fatal("old thumbnail kept")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("fresh thumbnail removed")
	}
}

func countText(tt *ui.Tester, s string) int {
	n := 0
	for _, x := range tt.Texts() {
		if strings.Contains(x, s) {
			n++
		}
	}
	return n
}

func TestInspectorFile(t *testing.T) {
	a, tt, _, root := setup(t)
	if a.showPreview || tt.HasText("八进制") {
		t.Fatal("the inspector is on by default")
	}
	goTo(t, a, tt, "notes.md")
	os.Chmod(filepath.Join(root, "notes.md"), 0o644)
	tt.Key(ui.Cmd|ui.Alt, ui.KeyP)
	tt.Frame()
	tt.Frame()
	if !a.showPreview {
		t.Fatal("⌘⌥P did not show the inspector")
	}
	for _, s := range []string{"Markdown 文稿 - 14 B", "通用", "大小", "14 B（14 字节）", "创建时间", "修改时间", "扩展名", "隐藏扩展名", "隐藏", "权限", "八进制", "rw-r--r--", "644", "所有者", "组", "锁定", "路径", filepath.Join(root, "notes.md")} {
		if !tt.HasText(s) && countText(tt, s) == 0 {
			t.Errorf("inspector lacks %q", s)
		}
	}
	// No preview of its own: the text shows once, in the third column.
	if n := countText(tt, "hello notes.md"); n != 1 {
		t.Fatalf("text shown %d times: %q", n, tt.Texts())
	}
	if a.actionTitle("preview") != "检查器" {
		t.Fatalf("title %q", a.actionTitle("preview"))
	}
}

func TestInspectorFolder(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.run("preview")
	tt.Frame()
	tt.Frame()
	if cursorName(a) != "alpha" {
		t.Fatalf("cursor %q", cursorName(a))
	}
	if !tt.HasText("2 项（文件 1 · 文件夹 1）") || !tt.HasText("22 B") || countText(tt, "rwx") == 0 {
		t.Fatalf("folder: %q", tt.Texts())
	}
	if tt.HasText("隐藏扩展名") {
		t.Fatalf("a folder shows counts, not a file's size: %q", tt.Texts())
	}
	// Several chosen: their count and size.
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	if countText(tt, "已选 5 项") == 0 {
		t.Fatalf("selection: %q", tt.Texts())
	}
}

func TestInspectorCapped(t *testing.T) {
	a, tt, _, root := setup(t)
	a.run("preview")
	tt.Frame()
	p := a.panes[0]
	p.insp.path = ""
	e, _ := a.tab().cursorEntry()
	_ = root
	// A tiny cap stands in for 100 000 items.
	in := &p.insp
	a.ensureInspect(p, e)
	if in.counting || in.stats.Capped {
		t.Fatalf("stats %+v", in.stats)
	}
	in.stats.Capped, in.stats.Scanned = true, 100000
	tt.Frame()
	if countText(tt, "≥ ") == 0 {
		t.Fatalf("no ≥: %q", tt.Texts())
	}
}

// TestInspectorStale: the attributes of an item the cursor left are dropped.
func TestInspectorStale(t *testing.T) {
	a, tt, _, root := setup(t)
	p := a.panes[0]
	var queue []func()
	a.async = func(f func()) { queue = append(queue, f) }
	es := a.tab().t.Entries()
	a.ensureInspect(p, es[0]) // alpha
	a.ensureInspect(p, es[3]) // a file
	if len(queue) != 2 {
		t.Fatalf("queued %d", len(queue))
	}
	queue[1]()
	queue[0]() // late
	if p.insp.info.Path != es[3].Path || p.insp.counting {
		t.Fatalf("stale result kept: %+v", p.insp.info)
	}
	_, _ = tt, root
}

// fontServer serves the two faces, or fails with status.
func fontServer(t *testing.T, status int) (*httptest.Server, *[]string) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path)
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Write(append([]byte{0, 1, 0, 0}, []byte(r.URL.Path)...))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func fontApp(t *testing.T, status int) (*app, *ui.Tester, string, *[]string, *[]string) {
	t.Helper()
	a, _, _, _ := setup(t)
	srv, got := fontServer(t, status)
	dir := t.TempDir()
	var registered []string
	a.font = fontState{
		checked: true, baseURL: srv.URL + "/fonts/", dir: dir, client: srv.Client(),
		register: func(data []byte, family string) error {
			registered = append(registered, family+":"+string(data[4:]))
			return nil
		},
	}
	tt := ui.NewTester(func(c *ui.Context) { a.fontNotice(c); a.fontSettings(c) }, 800, 200)
	tt.Frame()
	return a, tt, dir, got, &registered
}

func TestFontNoticeAndDownload(t *testing.T) {
	a, tt, dir, got, registered := fontApp(t, http.StatusOK)
	if !tt.HasText("未安装 JetBrains Mono Nerd Font") || !tt.HasText("下载") {
		t.Fatalf("no notice: %q", tt.Texts())
	}
	if len(*got) != 0 {
		t.Fatal("downloaded before asked")
	}
	if err := tt.Click("下载"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(*got) != 2 || (*got)[0] != "/fonts/JetBrainsMonoNerdFontMono-Regular.ttf" || (*got)[1] != "/fonts/JetBrainsMonoNerdFontMono-Bold.ttf" {
		t.Fatalf("requests %v", *got)
	}
	for _, f := range fontFiles {
		if b, err := os.ReadFile(filepath.Join(dir, f)); err != nil || !strings.HasSuffix(string(b), f) {
			t.Fatalf("%s: %q %v", f, b, err)
		}
	}
	if len(*registered) != 2 || !strings.HasPrefix((*registered)[0], "JetBrainsMono Nerd Font Mono:") {
		t.Fatalf("registered %v", *registered)
	}
	if !a.font.mono || !a.nerd || a.font.busy || tt.HasText("未安装 JetBrains Mono Nerd Font") || !tt.HasText("JetBrainsMono Nerd Font Mono · 13pt（已安装）") {
		t.Fatalf("after: %+v %q", a.font, tt.Texts())
	}
}

func TestFontDownloadFails(t *testing.T) {
	a, tt, dir, _, registered := fontApp(t, http.StatusNotFound)
	a.downloadFont()
	tt.Frame()
	if a.font.busy || a.font.mono || !strings.Contains(a.font.err, "HTTP 404") || !tt.HasText("重试") || len(*registered) != 0 {
		t.Fatalf("failure: %+v %q", a.font, tt.Texts())
	}
	if es, _ := os.ReadDir(dir); len(es) != 0 {
		t.Fatalf("wrote %v", es)
	}
}

func TestFontRejectsNonFont(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) }))
	defer srv.Close()
	if _, err := fetchFont(srv.Client(), srv.URL); err == nil {
		t.Fatal("an HTML page taken for a font")
	}
}

func TestFontNoticeHiddenWhenInstalled(t *testing.T) {
	a, tt, _, _, _ := fontApp(t, http.StatusOK)
	a.font.mono = true
	tt.Frame()
	if tt.HasText("未安装 JetBrains Mono Nerd Font") {
		t.Fatal("notice shown with the font installed")
	}
	a.font.checked, a.font.mono = false, false
	tt.Frame()
	if tt.HasText("未安装 JetBrains Mono Nerd Font") {
		t.Fatal("notice shown before the fonts were looked through")
	}
}

func TestListFont(t *testing.T) {
	a, _, _, _ := setup(t)
	if a.listFont() != "JetBrainsMono Nerd Font Mono, JetBrainsMono Nerd Font, monospace" {
		t.Fatal(a.listFont())
	}
}

func TestRowIcons(t *testing.T) {
	a, tt, _, root := setup(t)
	a.nerd = true
	os.WriteFile(filepath.Join(root, "tool"), []byte("#!/bin/sh\n"), 0o755)
	os.Symlink(filepath.Join(root, "notes.md"), filepath.Join(root, "link.md"))
	a.load(a.tab())
	tt.Frame()
	for _, g := range []string{"", ""} { // executable, link
		if !tt.HasText(g) {
			t.Errorf("no glyph %q: %q", g, tt.Texts())
		}
	}
}
