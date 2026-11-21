package gui

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPastePathGoes(t *testing.T) {
	a, tt, _, root := setup(t)
	alpha := filepath.Join(root, "alpha")

	// A folder's path, copied elsewhere: ⌘V enters it.
	a.svc.WriteClipboard(alpha + "\n")
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	if a.tab().t.Dir != alpha {
		t.Fatalf("folder: in %s, status %q", a.tab().t.Dir, a.status)
	}

	// A file's: v shows it in its folder.
	a.svc.WriteClipboard(`"` + filepath.Join(root, "notes.md") + `"`)
	tt.Key(0, ui.KeyV)
	tt.Frame()
	if a.tab().t.Dir != root || cursorName(a) != "notes.md" {
		t.Fatalf("file: in %s on %q", a.tab().t.Dir, cursorName(a))
	}

	// Not there: says so, nothing pasted.
	a.svc.WriteClipboard(filepath.Join(root, "gone"))
	tt.Key(0, ui.KeyV)
	tt.Frame()
	if a.tab().t.Dir != root || a.status != "找不到："+filepath.Join(root, "gone") {
		t.Fatalf("missing: in %s, status %q", a.tab().t.Dir, a.status)
	}
}

func TestPastePathAfterCopy(t *testing.T) {
	a, tt, _, root := setup(t)
	alpha := filepath.Join(root, "alpha")

	// A path copied before goFinder's items: v pastes the items.
	a.svc.WriteClipboard(alpha)
	goTo(t, a, tt, "notes.md")
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	if a.tab().t.Dir != root || !exists(filepath.Join(root, "notes 2.md")) {
		t.Fatalf("items not pasted: in %s, status %q", a.tab().t.Dir, a.status)
	}

	// A path copied after them: v goes there. (The same text copied
	// again cannot be told apart from the old one.)
	beta := filepath.Join(root, "beta")
	a.svc.WriteClipboard(beta)
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	if a.tab().t.Dir != beta {
		t.Fatalf("path after copy: in %s, status %q", a.tab().t.Dir, a.status)
	}
}
