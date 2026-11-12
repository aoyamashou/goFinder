package gui

// A right-click on a row of the parent or the third column lights the row
// up as chosen at once, as a left click would, while its menu shows; a
// command chosen then goes to the row (contextMenuPre's pre). The current
// column's list chooses its row itself (ui.List with a Selection).

// hotMenu notes, from the row menu's build, that the menu of path shows.
func (a *app) hotMenu(path string) {
	if a.menuHot != path {
		a.menuHot, a.hotNudge = path, true
	}
	a.hotSeen = true
}

// dropHot forgets the row, as a command of its menu was chosen.
func (a *app) dropHot() {
	if a.menuHot != "" {
		a.menuHot, a.hotNudge = "", true
	}
}

// endHot runs as a frame's build ends: a menu that built no more has
// closed, and its row goes back to how it was. A change draws once more,
// as the rows were built before the menu.
func (a *app) endHot() {
	if !a.hotSeen {
		a.dropHot()
	}
	a.hotSeen = false
	if a.hotNudge {
		a.hotNudge = false
		a.post(func() {})
	}
}

// hotLook is a side column's row look while a right-click's menu shows:
// the row it is on in the accent, the others plain.
func (a *app) hotLook(path string, look rowLook) rowLook {
	switch {
	case a.menuHot == "":
		return look
	case a.menuHot == path:
		return lookAccent
	}
	return lookPlain
}
