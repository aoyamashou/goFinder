package gui

import (
	"github.com/egoist/mygo/ui"
)

// The chrome's measures that docs/design-tokens.json has no token for yet
// (design-system.md §4 and §10 give them in prose): kept here by name so
// the chrome files carry no bare numbers. Colors and the lengths the
// tokens do name always come from tok and px.
const (
	toolbarButtonW = 28 // a toolbar icon button, Toolbar
	toolbarButtonH = 24
	segmentH       = 22 // one segment of a Segmented
	doneDotSize    = 6  // the "done" dot before the status sentence

	fontCaption  = 11 // caption: field names, status sentence, hints
	fontBody     = 13 // body: menu items, buttons, inspector values
	fontTitle    = 15 // title: dialog headings
	fontWordmark = 28 // wordmark in the settings' header
	fontKeycap   = 11 // keycap

	weightTitle    = 600
	weightKeycap   = 600
	weightWordmark = 800
	wordmarkSpace  = -0.035 * fontWordmark // letter spacing of the wordmark, in DIPs

	dialogW = 360 // the confirm dialog's width
)

// uiCaption is a caption-styled text in the color token col.
func uiCaption(c *ui.Context, s, col string) ui.Element {
	return ui.Text(c, s).FontSize(fontCaption).TextColor(tok(c, col))
}

// wordmark is "goFinder." with the period in signal: the settings'
// header.
func wordmark(c *ui.Context) {
	ui.Row(c.Key("wordmark")).Gap(0).AlignItems(ui.End).Children(func() {
		for i, part := range []string{"goFinder", "."} {
			col := "ink"
			if i == 1 {
				col = "signal"
			}
			ui.Text(c.Key("wm"+part), part).FontSize(fontWordmark).FontWeight(weightWordmark).
				LetterSpacing(wordmarkSpace).LineHeight(1).SingleLine().TextColor(tok(c, col))
		}
	})
}

// segmented is a pill-shaped Segmented over labels: surface-200 track, the
// chosen segment in selection with selection-ink. A click on segment i
// sets *sel to i and reports i; otherwise it reports -1. The segment's
// text is its label, so tests and assistive technology find it.
func segmented(c *ui.Context, sel *int, name string, labels ...string) int {
	return segmentedWith(c, sel, name, nil, labels...)
}

// segmentedWith is segmented, handing its track to track (when not nil).
func segmentedWith(c *ui.Context, sel *int, name string, track func(ui.Element), labels ...string) int {
	parts := ui.SegmentedBase(c, sel, len(labels))
	parts.Track.Label(name).Radius(px("radius-pill")).Background(tok(c, "surface-200")).
		AlignItems(ui.Center).Gap(0).Shrink(0)
	if track != nil {
		track(parts.Track)
	}
	hit := -1
	for i, label := range labels {
		s := parts.Segment(i).Height(segmentH).Padding(0, px("space-2")).Radius(px("radius-pill")).Label(label)
		col := tok(c, "ink")
		if i == *sel {
			s.Background(tok(c, "selection"))
			col = tok(c, "selection-ink")
		}
		s.TextColor(col).Children(func() {
			ui.Text(c, label).FontSize(fontCaption).SingleLine().TextColor(col)
		})
		if s.Clicked() {
			hit = i
		}
	}
	if hit >= 0 {
		*sel = hit
	}
	return hit
}

// faceButton is a button on surface-200 with radius-control, no border:
// the inspector's, the dialogs' and the shelf's. textTok is the label's
// color token ("danger" for the trash confirmation).
func faceButton(c *ui.Context, label, textTok string, disabled bool) ui.Element {
	b := ui.ButtonBase(c).Height(toolbarButtonH).Radius(px("radius-control")).
		Padding(0, px("space-2")).Label(label).Disabled(disabled)
	bg := tok(c, "surface-200")
	if !disabled && b.Hovered() {
		bg = tok(c, "separator")
	}
	col := tok(c, textTok)
	if disabled {
		col = tok(c, "faint")
	}
	return b.Background(bg).TextColor(col).Center().Children(func() {
		ui.Text(c, label).FontSize(fontBody).SingleLine().TextColor(col)
	})
}

// dialogModal is ui.Modal in the design system's look: an ink scrim with
// no blur, a bg panel of radius-menu with space-4 inside, no shadow or
// border.
// dialogModalSized is dialogModal with its panel w×h, shrunk to fit a
// smaller window; fn's content should grow into it.
func dialogModalSized(c *ui.Context, open *bool, w, h float32, fn func()) {
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(tok(c, "ink").Alpha(0.4))
		panel.Padding(px("space-4")).Gap(px("space-3")).Radius(px("radius-menu")).
			Background(tok(c, "bg")).Width(w).Height(h).MaxWidthPercent(90).MaxHeightPercent(90)
		fn()
	})
}

func dialogModal(c *ui.Context, open *bool, fn func()) {
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(tok(c, "ink").Alpha(0.4))
		panel.Padding(px("space-4")).Gap(px("space-3")).Radius(px("radius-menu")).
			Background(tok(c, "bg")).MaxWidthPercent(90).MaxHeightPercent(90)
		fn()
	})
}
