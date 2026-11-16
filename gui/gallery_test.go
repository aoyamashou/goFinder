package gui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestGalleryToggleAndChoose(t *testing.T) {
	a, tt, _, root := setup(t)
	for _, n := range []string{"a.png", "b.png", "c.png"} {
		os.WriteFile(filepath.Join(root, n), tinyPNG, 0o644)
	}
	a.load(a.tab())
	tt.Frame()
	if a.gallery {
		t.Fatal("gallery starts on")
	}
	press9(tt, 0, ui.KeyQ)
	tt.Frame()
	tt.Frame()
	if !a.gallery || len(a.gal.bmp) != 3 {
		t.Fatalf("after Q: gallery %v, %d thumbnails", a.gallery, len(a.gal.bmp))
	}
	// A chosen thumbnail moves the cursor and joins the selection.
	ts := a.tab()
	ts.cursor = ts.t.Index("b.png")
	ts.sel.Clear()
	ts.sel.Add(filepath.Join(root, "b.png"))
	ts.sel.Add(filepath.Join(root, "c.png"))
	tt.Frame()
	if got := paths(ts.chosen()); len(got) != 2 {
		t.Fatalf("chosen %v", got)
	}
	press9(tt, 0, ui.KeyQ)
	tt.Frame()
	if a.gallery || a.gal.bmp != nil {
		t.Fatal("Q again must leave the gallery and free the thumbnails")
	}
}

func TestGalleryEntriesOnlyImages(t *testing.T) {
	a, _, _, root := setup(t)
	os.WriteFile(filepath.Join(root, "p.png"), tinyPNG, 0o644)
	a.load(a.tab())
	got := galleryEntries(a.tab().t.Entries())
	if len(got) != 1 || got[0].Name != "p.png" {
		t.Fatalf("entries %v", got)
	}
}

func TestShrinkAndOrient(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	for y := 0; y < 500; y++ {
		for x := 0; x < 1000; x++ {
			src.Set(x, y, color.RGBA{200, 0, 0, 255})
		}
	}
	s := shrink(src, 100)
	if s.Bounds().Dx() != 100 || s.Bounds().Dy() != 50 {
		t.Fatalf("size %v", s.Bounds())
	}
	if c := s.RGBAAt(50, 25); c.R != 200 || c.A != 255 {
		t.Fatalf("pixel %v", c)
	}
	if o := orient(s, 6); o.Bounds().Dx() != 50 || o.Bounds().Dy() != 100 {
		t.Fatalf("rotated %v", o.Bounds())
	}
}

func TestGalleryHEIC(t *testing.T) {
	a, tt, fs, root := setup(t)
	a.cacheDir = t.TempDir()
	os.WriteFile(filepath.Join(root, "IMG_1.HEIC"), []byte("heic"), 0o644)
	a.load(a.tab())
	tt.Frame()
	press9(tt, 0, ui.KeyQ)
	tt.Frame()
	tt.Frame()
	if len(fs.thumbs) != 1 || len(a.gal.bmp) != 1 {
		t.Fatalf("thumbs asked %v, shown %d", fs.thumbs, len(a.gal.bmp))
	}
}

func TestGalleryHintAndEmptyText(t *testing.T) {
	a, tt, _, _ := setup(t)
	if !tt.HasText("图库视图") {
		t.Fatalf("status bar lacks the gallery hint: %q", tt.Texts())
	}
	press9(tt, 0, ui.KeyQ)
	tt.Frame()
	if !a.gallery || !tt.HasText("无法启用图库视图，当前文件夹下没有图片，按 q 返回文件列表") {
		t.Fatalf("empty text missing: %q", tt.Texts())
	}
}

func TestOpenInFinder(t *testing.T) {
	a, tt, fs, root := setup(t)
	goTo(t, a, tt, "alpha")
	a.run("finder")
	if len(fs.openedWith) != 1 || fs.openedWith[0] != [2]string{"Finder", filepath.Join(root, "alpha")} {
		t.Fatalf("folder: %v", fs.openedWith)
	}
	goTo(t, a, tt, "notes.md")
	a.run("finder")
	if n := len(fs.revealed); n != 1 || fs.revealed[0] != filepath.Join(root, "notes.md") {
		t.Fatalf("file: %v", fs.revealed)
	}
}
