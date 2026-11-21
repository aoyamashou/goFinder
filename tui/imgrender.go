package tui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// RenderHalfBlock decodes an image and renders it into ANSI TrueColor lines
// using the Unicode upper half-block glyph (▀), where each terminal cell
// displays 2 vertical pixels. Works universally across terminals, tmux, and SSH.
func RenderHalfBlock(data []byte, maxCols, maxRows int, bg color.RGBA) (lines []string, origW, origH int, format string, err error) {
	if maxCols < 2 || maxRows < 1 {
		return nil, 0, 0, "", fmt.Errorf("预览区过小")
	}
	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, "", err
	}
	b := src.Bounds()
	origW, origH = b.Dx(), b.Dy()
	if origW <= 0 || origH <= 0 {
		return nil, 0, 0, format, fmt.Errorf("无效图片尺寸")
	}

	dstW, dstH := fitPixels(origW, origH, maxCols, maxRows*2)
	scaled := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(scaled, scaled.Bounds(), src, b, draw.Over, nil)

	rows := dstH / 2
	lines = make([]string, 0, rows)
	for y := 0; y < dstH; y += 2 {
		var sb strings.Builder
		sb.Grow(dstW * 24)
		var prevTop, prevBot color.RGBA
		hasPrev := false
		for x := 0; x < dstW; x++ {
			top := blendOver(scaled.RGBAAt(x, y), bg)
			bot := bg
			if y+1 < dstH {
				bot = blendOver(scaled.RGBAAt(x, y+1), bg)
			}
			if !hasPrev || top != prevTop || bot != prevBot {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm",
					top.R, top.G, top.B, bot.R, bot.G, bot.B)
				prevTop, prevBot = top, bot
				hasPrev = true
			}
			sb.WriteString("▀")
		}
		sb.WriteString("\x1b[0m")
		lines = append(lines, sb.String())
	}
	return lines, origW, origH, format, nil
}

// fitPixels fits origW×origH inside maxW×maxH while preserving aspect ratio
// and keeping height even so every terminal row pairs 2 pixel rows cleanly.
func fitPixels(origW, origH, maxW, maxH int) (int, int) {
	scaleW := float64(maxW) / float64(origW)
	scaleH := float64(maxH) / float64(origH)
	scale := min(scaleW, scaleH)
	if scale > 1 {
		scale = 1
	}
	w := max(int(float64(origW)*scale+0.5), 1)
	h := max(int(float64(origH)*scale+0.5), 2)
	if w > maxW {
		w = maxW
	}
	if h > maxH {
		h = maxH
	}
	if h%2 != 0 {
		if h > 2 {
			h--
		} else if h+1 <= maxH {
			h++
		}
	}
	return w, h
}

// blendOver composites premultiplied RGBA c over opaque background bg.
func blendOver(c, bg color.RGBA) color.RGBA {
	if c.A == 0xff {
		return c
	}
	if c.A == 0 {
		return bg
	}
	inv := uint32(0xff - c.A)
	r := uint32(c.R) + (uint32(bg.R)*inv)/0xff
	g := uint32(c.G) + (uint32(bg.G)*inv)/0xff
	b := uint32(c.B) + (uint32(bg.B)*inv)/0xff
	return color.RGBA{
		R: uint8(min(r, 0xff)),
		G: uint8(min(g, 0xff)),
		B: uint8(min(b, 0xff)),
		A: 0xff,
	}
}
