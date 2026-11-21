package tui

import (
	"image/color"

	"github.com/charmbracelet/lipgloss"

	"gofinder/internal/icons"
)

type theme struct {
	dark bool

	topBar    lipgloss.Style
	crumb     lipgloss.Style
	crumbCur  lipgloss.Style
	badge     lipgloss.Style
	sep       lipgloss.Style
	normal    lipgloss.Style
	muted     lipgloss.Style
	faint     lipgloss.Style
	cursor    lipgloss.Style
	parentSel lipgloss.Style
	selected  lipgloss.Style
	danger    lipgloss.Style
	warn      lipgloss.Style
	hintKey   lipgloss.Style
	hintText  lipgloss.Style
	statusBar lipgloss.Style
	modalBox  lipgloss.Style
	modalHead lipgloss.Style
	lineNum   lipgloss.Style
	metaKey   lipgloss.Style
	metaVal   lipgloss.Style

	kindStyles map[icons.Kind]lipgloss.Style
	imgBg      color.RGBA
}

func newTheme(dark bool) theme {
	ac := func(light, dk string) lipgloss.AdaptiveColor {
		return lipgloss.AdaptiveColor{Light: light, Dark: dk}
	}

	signal := ac("#00dafc", "#00dafc")
	signalText := ac("#00718a", "#00dafc")
	ink := ac("#0a0a0a", "#f5f7f8")
	muted := ac("#5b6168", "#9aa1a8")
	faint := ac("#80878e", "#6f767d")
	surface := ac("#f2f4f5", "#1b1d1f")
	surface2 := ac("#e7eaec", "#26292c")
	sepCol := ac("#d5dadf", "#34383c")
	danger := ac("#b42318", "#ff6b6b")
	warn := ac("#8a5a00", "#ffb020")

	imgBg := color.RGBA{R: 0x14, G: 0x15, B: 0x16, A: 0xff}
	if !dark {
		imgBg = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}

	return theme{
		dark:      dark,
		topBar:    lipgloss.NewStyle().Background(surface).Foreground(ink),
		crumb:     lipgloss.NewStyle().Background(surface).Foreground(muted),
		crumbCur:  lipgloss.NewStyle().Background(surface).Foreground(signalText).Bold(true),
		badge:     lipgloss.NewStyle().Background(surface2).Foreground(signalText).Bold(true),
		sep:       lipgloss.NewStyle().Foreground(sepCol),
		normal:    lipgloss.NewStyle().Foreground(ink),
		muted:     lipgloss.NewStyle().Foreground(muted),
		faint:     lipgloss.NewStyle().Foreground(faint),
		cursor:    lipgloss.NewStyle().Background(signal).Foreground(lipgloss.Color("#000000")).Bold(true),
		parentSel: lipgloss.NewStyle().Background(surface2).Foreground(ink).Bold(true),
		selected:  lipgloss.NewStyle().Foreground(signalText).Bold(true),
		danger:    lipgloss.NewStyle().Foreground(danger).Bold(true),
		warn:      lipgloss.NewStyle().Foreground(warn),
		hintKey:   lipgloss.NewStyle().Background(surface2).Foreground(ink).Bold(true),
		hintText:  lipgloss.NewStyle().Background(surface).Foreground(muted),
		statusBar: lipgloss.NewStyle().Background(surface).Foreground(ink),
		modalBox:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(signalText).Padding(0, 1),
		modalHead: lipgloss.NewStyle().Foreground(signalText).Bold(true),
		lineNum:   lipgloss.NewStyle().Foreground(faint),
		metaKey:   lipgloss.NewStyle().Foreground(muted),
		metaVal:   lipgloss.NewStyle().Foreground(ink),
		imgBg:     imgBg,
		kindStyles: map[icons.Kind]lipgloss.Style{
			icons.KindFolder:  lipgloss.NewStyle().Foreground(ac("#1f6feb", "#58a6ff")),
			icons.KindCode:    lipgloss.NewStyle().Foreground(ac("#bc4c00", "#ffa657")),
			icons.KindDoc:     lipgloss.NewStyle().Foreground(ac("#57606a", "#adb5bd")),
			icons.KindImage:   lipgloss.NewStyle().Foreground(ac("#bf3989", "#f778ba")),
			icons.KindMedia:   lipgloss.NewStyle().Foreground(ac("#8250df", "#bc8cff")),
			icons.KindArchive: lipgloss.NewStyle().Foreground(ac("#9a6700", "#e3b341")),
			icons.KindExec:    lipgloss.NewStyle().Foreground(ac("#1a7f37", "#3fb950")),
			icons.KindConfig:  lipgloss.NewStyle().Foreground(ac("#6e7781", "#8b949e")),
		},
	}
}

func (t theme) kindStyle(k icons.Kind, dim bool) lipgloss.Style {
	if dim {
		return t.faint
	}
	if s, ok := t.kindStyles[k]; ok {
		return s
	}
	return t.normal
}
