package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func clip(a *app) string { return a.svc.ReadClipboard() }

func TestInspectorClickCopies(t *testing.T) {
	a, tt, _, root := setup(t)
	goTo(t, a, tt, "notes.md")
	a.run("preview")
	tt.Frame()
	tt.Frame()
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatalf("click %s: %v", label, err)
		}
		tt.Frame()
	}
	for _, c := range []struct{ field, label, want string }{
		{"大小", "14 B（14 字节）", "14 B（14 字节）"},
		{"扩展名", "md", "md"},
		{"路径", filepath.Join(root, "notes.md"), filepath.Join(root, "notes.md")},
	} {
		a.svc.WriteClipboard("")
		click(c.field)
		if got := clip(a); got != c.want {
			t.Errorf("%s: clipboard %q, want %q", c.field, got, c.want)
		}
		if a.status != "已复制 "+c.field {
			t.Errorf("status %q", a.status)
		}
	}
	// A time is copied as shown.
	a.svc.WriteClipboard("")
	for _, s := range tt.Texts() {
		if len(s) == 19 && s[4] == '-' && s[13] == ':' {
			tt.Click(s)
			tt.Frame()
			if clip(a) != s {
				t.Errorf("time: clipboard %q, want %q", clip(a), s)
			}
			break
		}
	}
	// The octal is copied.
	a.svc.WriteClipboard("")
	tt.Click("644")
	tt.Frame()
	if clip(a) != "644" {
		t.Errorf("octal: clipboard %q", clip(a))
	}
}

func TestInspectorShelfButtons(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.sh.file = filepath.Join(t.TempDir(), "shelf.json")
	goTo(t, a, tt, "notes.md")
	a.run("preview")
	tt.Frame()
	tt.Frame()
	if tt.HasText("关闭暂存架") || !tt.HasText("打开暂存架") {
		t.Fatalf("texts %q", tt.Texts())
	}
	tt.Click("加入暂存架")
	tt.Frame()
	if len(a.sh.items) != 1 || filepath.Base(a.sh.items[0]) != "notes.md" {
		t.Fatalf("shelf %v", a.sh.items)
	}
	tt.Click("打开暂存架")
	tt.Frame()
	tt.Frame()
	if !a.sh.show || !tt.HasText("关闭暂存架") {
		t.Fatalf("shelf not shown: %q", tt.Texts())
	}
	tt.Click("关闭暂存架")
	tt.Frame()
	if a.sh.show {
		t.Fatal("shelf not hidden")
	}
}

func TestShelfButtonsWithinPanel(t *testing.T) {
	a, tt, _, root := setup(t)
	a.sh.file = filepath.Join(t.TempDir(), "shelf.json")
	a.shelve([]string{filepath.Join(root, "a-very-long-file-name-that-would-widen-the-panel-report-final-v2.txt")})
	a.sh.show = true
	tt.SetSize(800, 600)
	tt.Frame()
	tt.Frame()
	panel, ok := tt.Find("暂存架（1）")
	if !ok {
		t.Fatalf("no shelf: %q", tt.Texts())
	}
	_ = panel
	var rects []ui.Rect
	for _, s := range []string{"复制到此", "移动到此", "清空"} {
		r, ok := tt.Find(s)
		if !ok {
			t.Fatalf("no %s", s)
		}
		rects = append(rects, r)
	}
	win := float32(800)
	right := rects[2].X + rects[2].W
	if right > win {
		t.Errorf("buttons run to %v, past the window %v", right, win)
	}
	// The panel is 220 wide with 6 of padding on each side (no border, as
	// the design system has none); the title's text starts at the padding.
	if limit := panel.X - 6 + 220 - 6; right > limit+0.5 {
		t.Errorf("buttons end at %v, past the panel's content edge %v", right, limit)
	}
}
