package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gofinder/internal/keymap"
)

type fakeSys struct {
	mu      sync.Mutex
	trashed []string
	opened  []string
	copied  []string
}

func (f *fakeSys) Trash(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trashed = append(f.trashed, path)
	return os.RemoveAll(path)
}

func (f *fakeSys) Open(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, path)
	return nil
}

func (f *fakeSys) CopyText(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.copied = append(f.copied, text)
	return nil
}

func (f *fakeSys) Watch(string, func()) func() { return func() {} }

// drain runs non-blocking Bubble Tea commands synchronously for deterministic tests.
func drain(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			continue
		}
		// Skip tick/watch messages unless tests explicitly send them.
		if _, ok := msg.(tapTimeoutMsg); ok {
			continue
		}
		if _, ok := msg.(dirChangedMsg); ok {
			continue
		}
		_, next := m.Update(msg)
		if next != nil {
			queue = append(queue, next)
		}
	}
}

func newTestModel(t *testing.T, dir string, opts ...func(*Options)) (*Model, *fakeSys, *time.Time) {
	t.Helper()
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	fs := &fakeSys{}
	cfgDir := t.TempDir()
	o := Options{
		StartDir:  dir,
		ConfigDir: cfgDir,
		System:    fs,
		Now:       func() time.Time { return now },
	}
	for _, fn := range opts {
		fn(&o)
	}
	m := New(o)
	t.Cleanup(m.Close)
	drain(m, m.loadCurrentCmd(""))
	drain(m, m.loadParentCmd())
	return m, fs, &now
}

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestDefaultKeysAndRenameOnlyEnter(t *testing.T) {
	km, warns := buildKeymap(false, "")
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	// Enter -> rename; r must not be bound.
	if act, ok := km.Lookup(keymap.MustParse("Enter")); !ok || act != "rename" {
		t.Fatalf("Enter = (%q, %v), want rename", act, ok)
	}
	if act, ok := km.Lookup(keymap.MustParse("r")); ok {
		t.Fatalf("r should be unbound, got %q", act)
	}
	// Vim keys off by default.
	for _, k := range []string{"h", "j", "k", "l"} {
		if act, ok := km.Lookup(keymap.MustParse(k)); ok {
			t.Fatalf("%s should be unbound when vim=false, got %q", k, act)
		}
	}

	kmVim, _ := buildKeymap(true, "")
	wantVim := map[string]string{"h": "up", "j": "cursor.down", "k": "cursor.up", "l": "open", "Up": "cursor.up", "Right": "open"}
	for k, want := range wantVim {
		if act, ok := kmVim.Lookup(keymap.MustParse(k)); !ok || act != want {
			t.Fatalf("vim %s = (%q, %v), want %q", k, act, ok, want)
		}
	}
}

func TestStandaloneKeymapAndConfig(t *testing.T) {
	dir := t.TempDir()
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"vimKeys": true, "showHidden": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "keymap.json"), []byte(`{"quit": ["Ctrl+Q"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(Options{StartDir: dir, ConfigDir: cfgDir, System: &fakeSys{}})
	defer m.Close()

	if !m.cfg.VimKeys || !m.tab.ShowHidden {
		t.Fatalf("expected standalone config loaded: vim=%v hidden=%v", m.cfg.VimKeys, m.tab.ShowHidden)
	}
	if act, ok := m.km.Lookup(keymap.MustParse("Ctrl+Q")); !ok || act != "quit" {
		t.Fatalf("Ctrl+Q override failed: (%q, %v)", act, ok)
	}
	if _, ok := m.km.Lookup(keymap.MustParse("q")); ok {
		t.Fatalf("q should have been replaced by override")
	}
}

func TestDoubleTapStateMachine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, fs, now := newTestModel(t, dir)

	// Single tap "c" followed by timeout -> copy.
	_, _ = m.Update(keyRune('c'))
	if len(m.clip) != 0 {
		t.Fatalf("single c should wait for double tap before firing")
	}
	*now = now.Add(keymap.DoubleTap + time.Millisecond)
	_, cmd := m.Update(tapTimeoutMsg{seq: m.tapSeq})
	drain(m, cmd)
	if len(m.clip) != 1 || filepath.Base(m.clip[0]) != "a.txt" {
		t.Fatalf("expected a.txt copied after timeout, got %v", m.clip)
	}

	// Double tap "c c" within 400ms -> copypath.
	m.clip = nil
	_, _ = m.Update(keyRune('c'))
	*now = now.Add(100 * time.Millisecond)
	_, cmd = m.Update(keyRune('c'))
	drain(m, cmd)
	if len(fs.copied) != 1 || !strings.HasSuffix(fs.copied[0], "a.txt") {
		t.Fatalf("expected copypath to copy a.txt, got %v", fs.copied)
	}
	if len(m.clip) != 0 {
		t.Fatalf("double tap c c must not trigger copy")
	}

	// Press "c" then immediately Down -> flushes copy first, then moves cursor to b.txt.
	_, cmd = m.Update(keyRune('c'))
	drain(m, cmd)
	*now = now.Add(50 * time.Millisecond)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	drain(m, cmd)
	if len(m.clip) != 1 || filepath.Base(m.clip[0]) != "a.txt" {
		t.Fatalf("expected pending c to flush on next key, got %v", m.clip)
	}
	if got := m.cursorName(); got != "b.txt" {
		t.Fatalf("expected cursor on b.txt, got %q", got)
	}

	// Double tap "n n" -> modeNewFile; single "n" timeout -> modeNewFolder.
	_, _ = m.Update(keyRune('n'))
	*now = now.Add(100 * time.Millisecond)
	_, _ = m.Update(keyRune('n'))
	if m.mode != modeNewFile {
		t.Fatalf("expected modeNewFile, got %v", m.mode)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	_, _ = m.Update(keyRune('n'))
	*now = now.Add(keymap.DoubleTap + time.Millisecond)
	_, _ = m.Update(tapTimeoutMsg{seq: m.tapSeq})
	if m.mode != modeNewFolder {
		t.Fatalf("expected modeNewFolder after timeout, got %v", m.mode)
	}
}

func TestNavigationFilterPinyinAndSelection(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "docs")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "模型查询.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "other.md"), []byte("md"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, _, _ := newTestModel(t, dir)

	// Enter "docs" with Right.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	drain(m, cmd)
	if m.tab.Dir != sub {
		t.Fatalf("expected dir %s, got %s", sub, m.tab.Dir)
	}

	// Filter with pinyin initials "mxcx".
	_, _ = m.Update(keyRune('f'))
	for _, r := range "mxcx" {
		_, cmd = m.Update(keyRune(r))
		drain(m, cmd)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if es := m.tab.Entries(); len(es) != 1 || es[0].Name != "模型查询.txt" {
		t.Fatalf("expected pinyin filter to match 模型查询.txt, got %+v", es)
	}

	// Clear filter via Esc.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	drain(m, cmd)
	if len(m.tab.Entries()) != 2 {
		t.Fatalf("expected 2 entries after clearing filter, got %d", len(m.tab.Entries()))
	}

	// Space toggles selection; Esc clears it.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	drain(m, cmd)
	if len(m.selected) != 1 {
		t.Fatalf("expected 1 selected entry, got %d", len(m.selected))
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.selected) != 0 {
		t.Fatalf("expected selection cleared")
	}

	// Left returns to parent and focuses "docs"; Alt+Left/Right navigate history.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	drain(m, cmd)
	if m.tab.Dir != dir || m.cursorName() != "docs" {
		t.Fatalf("expected back in %s focused on docs, got dir=%s cur=%q", dir, m.tab.Dir, m.cursorName())
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	drain(m, cmd)
	if m.tab.Dir != sub {
		t.Fatalf("Alt+Left back failed, dir=%s", m.tab.Dir)
	}
	// Backtick returns to startDir.
	_, cmd = m.Update(keyRune('`'))
	drain(m, cmd)
	if m.tab.Dir != dir {
		t.Fatalf("backtick go.start failed, dir=%s", m.tab.Dir)
	}
}

func TestFileOpsAndConfirmTrash(t *testing.T) {
	dir := t.TempDir()
	m, fs, now := newTestModel(t, dir)

	// Create new blank file via "n n".
	_, _ = m.Update(keyRune('n'))
	*now = now.Add(50 * time.Millisecond)
	_, _ = m.Update(keyRune('n'))
	for _, r := range "note.txt" {
		_, _ = m.Update(keyRune(r))
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	drain(m, cmd)
	if _, err := os.Stat(filepath.Join(dir, "note.txt")); err != nil {
		t.Fatalf("note.txt not created: %v", err)
	}

	// Rename via Enter.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeRename {
		t.Fatalf("Enter should open rename mode, got %v", m.mode)
	}
	m.input.SetValue("renamed.txt")
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	drain(m, cmd)
	if _, err := os.Stat(filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatalf("renamed.txt missing: %v", err)
	}

	// Trash confirms with Enter (default Y) or y; n cancels.
	_, _ = m.Update(keyRune('d'))
	if m.mode != modeConfirmTrash {
		t.Fatalf("expected confirm trash mode, got %v", m.mode)
	}
	_, _ = m.Update(keyRune('n'))
	if len(fs.trashed) != 0 {
		t.Fatalf("trash should be cancelled on n")
	}

	_, _ = m.Update(keyRune('d'))
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	drain(m, cmd)
	if len(fs.trashed) != 1 || filepath.Base(fs.trashed[0]) != "renamed.txt" {
		t.Fatalf("expected renamed.txt trashed on Enter, got %v", fs.trashed)
	}
}

func TestPreviews(t *testing.T) {
	dir := t.TempDir()
	th := newTheme(true)

	// 1. Go source file -> highlighted text preview.
	goFile := filepath.Join(dir, "hello.go")
	src := "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"
	if err := os.WriteFile(goFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// 2. Binary file -> metadata preview.
	binFile := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(binFile, []byte{0x00, 0x01, 0x02, 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	// 3. Synthetic PNG -> half-block ANSI image preview.
	imgFile := filepath.Join(dir, "pic.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 16), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imgFile, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	m, _, _ := newTestModel(t, dir)
	for _, e := range m.tab.Entries() {
		pv := buildPreview(e, false, 48, 24, th)
		switch e.Name {
		case "hello.go":
			if pv.kind != pvText || len(pv.lines) < 4 {
				t.Fatalf("hello.go preview = %+v", pv)
			}
			joined := strings.Join(pv.lines, "\n")
			if !strings.Contains(joined, "\x1b[") {
				t.Fatalf("expected chroma ANSI highlighting for hello.go")
			}
		case "data.bin":
			if pv.kind != pvMeta || len(pv.lines) == 0 {
				t.Fatalf("data.bin should render metadata preview, got %+v", pv)
			}
		case "pic.png":
			if pv.kind != pvImage || len(pv.lines) == 0 {
				t.Fatalf("pic.png should render half-block image preview, got %+v", pv)
			}
			if !strings.Contains(pv.lines[0], "▀") || !strings.Contains(pv.lines[0], "\x1b[38;2;") {
				t.Fatalf("expected TrueColor half-block output, got %q", pv.lines[0])
			}
			if !strings.Contains(pv.header, "16×16") {
				t.Fatalf("expected dimensions in header, got %q", pv.header)
			}
		}
	}
}

func TestQuitCwdBehavior(t *testing.T) {
	dir := t.TempDir()
	m, _, _ := newTestModel(t, dir)

	_, _ = m.Update(keyRune('q'))
	if got := m.FinalDir(); got != dir {
		t.Fatalf("q FinalDir = %q, want %q", got, dir)
	}

	m2, _, _ := newTestModel(t, dir)
	_, _ = m2.Update(keyRune('Q'))
	if got := m2.FinalDir(); got != "" {
		t.Fatalf("Q FinalDir = %q, want empty", got)
	}
}

func TestViewLayoutsAndModals(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "very_long_chinese_文件名测试_document.go"), []byte("package a\n"), 0o644)
	m, _, _ := newTestModel(t, dir)

	for _, sz := range [][2]int{{120, 36}, {58, 20}} {
		_, cmd := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		drain(m, cmd)
		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) != sz[1] {
			t.Fatalf("size %v: got %d lines, want %d", sz, len(lines), sz[1])
		}
		for i, ln := range lines {
			if w := lipgloss.Width(ln); w > sz[0] {
				t.Fatalf("size %v line %d width %d > %d", sz, i, w, sz[0])
			}
		}
	}

	// Sort modal and help modal render cleanly.
	_, _ = m.Update(keyRune(','))
	if out := m.View(); !strings.Contains(out, "排序方式") {
		t.Fatalf("sort modal did not render")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	_, _ = m.Update(keyRune('?'))
	if out := m.View(); !strings.Contains(out, "快捷键速查表") || !strings.Contains(out, "浏览") {
		t.Fatalf("help modal did not render")
	}
	for _, r := range "重命名" {
		_, _ = m.Update(keyRune(r))
	}
	if out := m.View(); !strings.Contains(out, "重命名") || !strings.Contains(out, "↩") {
		t.Fatalf("help modal search did not filter rename binding")
	}
}
