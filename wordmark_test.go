package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestWordmarkPNG writes the settings' wordmark ("goFinder." as the 关于
// page shows it) with a transparent background, for the README: one for
// light pages, one for dark. Run with MYGO_WORDMARK set to a folder.
func TestWordmarkPNG(t *testing.T) {
	dir := os.Getenv("MYGO_WORDMARK")
	if dir == "" {
		t.Skip("MYGO_WORDMARK not set")
	}
	for _, dark := range []bool{false, true} {
		shot := func(bg ui.Color) *image.RGBA {
			tt := ui.NewTester(func(c *ui.Context) {
				applyAppearance(c, map[bool]string{false: "light", true: "dark"}[dark])
				ui.Column(c).Grow(1).Padding(8).Background(bg).AlignItems(ui.Start).Children(func() { wordmark(c) })
			}, 200, 50)
			tt.SetScale(4)
			tt.Frame()
			tt.Frame()
			return tt.Image()
		}
		w, b := shot(ui.Color{R: 255, G: 255, B: 255, A: 255}), shot(ui.Color{A: 255})
		r := w.Bounds()
		out := image.NewNRGBA(r)
		box := image.Rectangle{Min: r.Max, Max: r.Min}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				pw, pb := w.RGBAAt(x, y), b.RGBAAt(x, y)
				// Difference matting: over white and black, a = 1 - (w - b).
				d := (int(pw.R) - int(pb.R) + int(pw.G) - int(pb.G) + int(pw.B) - int(pb.B)) / 3
				a := max(0, min(255, 255-d))
				if a == 0 {
					continue
				}
				un := func(v uint8) uint8 { return uint8(min(255, int(v)*255/a)) }
				out.SetNRGBA(x, y, color.NRGBA{R: un(pb.R), G: un(pb.G), B: un(pb.B), A: uint8(a)})
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
		crop := out.SubImage(box.Inset(-4))
		name := map[bool]string{false: "wordmark-light.png", true: "wordmark-dark.png"}[dark]
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, crop); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
