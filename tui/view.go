package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"gofinder/internal/icons"
	"gofinder/internal/pane"
	"gofinder/internal/vfs"
)

func (m *Model) contentHeight() int {
	return max(m.height-2, 1)
}

func (m *Model) layoutWidths() (parentW, curW, prevW int) {
	total := max(m.width, 20)
	if total < 70 {
		avail := total - 1
		curW = max(avail*45/100, 10)
		prevW = max(avail-curW, 8)
		return 0, curW, prevW
	}
	avail := total - 2
	parentW = max(avail*20/100, 12)
	curW = max(avail*38/100, 20)
	prevW = max(avail-parentW-curW, 16)
	return parentW, curW, prevW
}

// View renders the full terminal screen.
func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.mode == modeHelp {
		return m.renderHelpScreen()
	}
	if m.mode == modeSort {
		return m.renderSortScreen()
	}

	top := m.renderTopBar()
	body := m.renderColumns()
	bottom := m.renderBottomBar()
	return strings.Join([]string{top, body, bottom}, "\n")
}

func (m *Model) renderTopBar() string {
	dir := m.tab.Dir
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if dir == home {
			dir = "~"
		} else if strings.HasPrefix(dir, home+string(filepath.Separator)) {
			dir = "~" + dir[len(home):]
		}
	}

	var crumbs []string
	if dir == "~" {
		crumbs = []string{"~"}
	} else if strings.HasPrefix(dir, "~/") {
		crumbs = append([]string{"~"}, strings.Split(strings.TrimPrefix(dir, "~/"), "/")...)
	} else {
		crumbs = vfs.Crumbs(dir)
	}

	var left strings.Builder
	left.WriteString(" ")
	for i, c := range crumbs {
		if i > 0 {
			left.WriteString(m.th.crumb.Render(" ▸ "))
		}
		if i == len(crumbs)-1 {
			left.WriteString(m.th.crumbCur.Render(c))
		} else {
			left.WriteString(m.th.crumb.Render(c))
		}
	}

	var badges []string
	if m.tab.Filter != "" {
		badges = append(badges, m.th.badge.Render(" 过滤:"+m.tab.Filter+" "))
	}
	if len(m.selected) > 0 {
		badges = append(badges, m.th.badge.Render(fmt.Sprintf(" 已选:%d ", len(m.selected))))
	}
	if len(m.clip) > 0 {
		verb := "复制"
		if m.clipCut {
			verb = "剪切"
		}
		badges = append(badges, m.th.badge.Render(fmt.Sprintf(" %s:%d ", verb, len(m.clip))))
	}
	if m.tab.ShowHidden {
		badges = append(badges, m.th.crumb.Render(" · 隐藏 "))
	}
	badges = append(badges, m.th.crumb.Render(" "+sortShortLabel(m.tab.Sort, m.tab.Desc)+" "))
	right := strings.Join(badges, " ")

	return composeBar(left.String(), right, m.width, m.th.topBar)
}

func (m *Model) renderColumns() string {
	h := m.contentHeight()
	parentW, curW, prevW := m.layoutWidths()

	curLines := m.renderCurrentColumn(curW, h)
	prevLines := m.renderPreviewColumn(prevW, h)
	sep := m.th.sep.Render("│")

	rows := make([]string, h)
	if parentW > 0 {
		parentLines := m.renderParentColumn(parentW, h)
		for i := 0; i < h; i++ {
			rows[i] = parentLines[i] + sep + curLines[i] + sep + prevLines[i]
		}
	} else {
		for i := 0; i < h; i++ {
			rows[i] = curLines[i] + sep + prevLines[i]
		}
	}
	return strings.Join(rows, "\n")
}

func (m *Model) renderParentColumn(w, h int) []string {
	lines := make([]string, h)
	if len(m.parentEntries) == 0 {
		for i := range lines {
			lines[i] = strings.Repeat(" ", w)
		}
		return lines
	}
	curBase := filepath.Base(m.tab.Dir)
	activeIdx := -1
	for i, e := range m.parentEntries {
		if e.Name == curBase {
			activeIdx = i
			break
		}
	}
	start := 0
	if activeIdx >= h {
		start = activeIdx - h/2
	}
	if maxStart := max(len(m.parentEntries)-h, 0); start > maxStart {
		start = maxStart
	}

	for i := 0; i < h; i++ {
		idx := start + i
		if idx >= len(m.parentEntries) {
			lines[i] = strings.Repeat(" ", w)
			continue
		}
		e := m.parentEntries[idx]
		isCur := idx == activeIdx
		lines[i] = m.renderEntryRow(e, w, false, isCur, false, isCur)
	}
	return lines
}

func (m *Model) renderCurrentColumn(w, h int) []string {
	lines := make([]string, h)
	if m.loadErr != "" {
		lines[0] = fitWidth(m.th.danger.Render(" "+m.loadErr), w)
		for i := 1; i < h; i++ {
			lines[i] = strings.Repeat(" ", w)
		}
		return lines
	}
	es := m.tab.Entries()
	if len(es) == 0 {
		msg := " (空目录)"
		if m.tab.Filter != "" {
			msg = " (无匹配项)"
		}
		lines[0] = fitWidth(m.th.muted.Render(msg), w)
		for i := 1; i < h; i++ {
			lines[i] = strings.Repeat(" ", w)
		}
		return lines
	}

	for i := 0; i < h; i++ {
		idx := m.scroll + i
		if idx >= len(es) {
			lines[i] = strings.Repeat(" ", w)
			continue
		}
		e := es[idx]
		isCursor := idx == m.cursor
		isSel := m.selected[e.Path]
		lines[i] = m.renderEntryRow(e, w, isCursor, false, isSel, false)
	}
	return lines
}

func (m *Model) renderEntryRow(e vfs.Entry, w int, isCursor, isParentSel, isSelected, openFolder bool) string {
	if w <= 0 {
		return ""
	}
	mark := " "
	if isSelected {
		mark = "●"
	}

	ic := icons.Of(e.Name, icons.Attr{
		Dir:  e.IsDir,
		Open: openFolder,
		Link: e.IsLink,
		Exec: !e.IsDir && e.Mode&0o111 != 0,
	})
	glyph := ic.Glyph
	if !m.nerd {
		glyph = icons.Fallback(e.IsDir, openFolder)
	}

	// Layout: [mark 1][space 1][glyph ~1-2][space 1][name...][trailing space 1]
	prefixPlain := mark + " " + glyph + " "
	prefixW := runewidth.StringWidth(prefixPlain)
	nameMax := max(w-prefixW-1, 1)
	name := truncateMiddle(e.Name, nameMax)
	pad := max(w-prefixW-runewidth.StringWidth(name), 0)

	if isCursor {
		return m.th.cursor.Render(prefixPlain + name + strings.Repeat(" ", pad))
	}
	if isParentSel {
		return m.th.parentSel.Render(prefixPlain + name + strings.Repeat(" ", pad))
	}

	markStr := mark
	if isSelected {
		markStr = m.th.selected.Render(mark)
	}
	glyphStr := m.th.kindStyle(ic.Kind, ic.Dim).Render(glyph)
	nameStyle := m.th.normal
	if ic.Dim {
		nameStyle = m.th.faint
	}
	return markStr + " " + glyphStr + " " + nameStyle.Render(name) + strings.Repeat(" ", pad)
}

func (m *Model) renderPreviewColumn(w, h int) []string {
	lines := make([]string, h)
	if w <= 0 {
		return lines
	}
	fillEmpty := func(from int) {
		for i := from; i < h; i++ {
			lines[i] = strings.Repeat(" ", w)
		}
	}

	if m.pv.loading {
		lines[0] = fitWidth(m.th.muted.Render(" 加载预览中…"), w)
		fillEmpty(1)
		return lines
	}

	switch m.pv.kind {
	case pvNone:
		fillEmpty(0)
		return lines

	case pvError:
		lines[0] = fitWidth(m.th.danger.Render(" "+m.pv.header), w)
		for i := 1; i < h && i-1 < len(m.pv.lines); i++ {
			lines[i] = fitWidth(m.th.muted.Render(" "+m.pv.lines[i-1]), w)
		}
		fillEmpty(min(len(m.pv.lines)+1, h))
		return lines

	case pvDir:
		lines[0] = fitWidth(m.th.muted.Render(" "+m.pv.header), w)
		if len(m.pv.entries) == 0 && h > 1 {
			lines[1] = fitWidth(m.th.faint.Render(" (空目录)"), w)
			fillEmpty(2)
			return lines
		}
		for i := 1; i < h; i++ {
			idx := i - 1
			if idx >= len(m.pv.entries) {
				lines[i] = strings.Repeat(" ", w)
				continue
			}
			lines[i] = m.renderEntryRow(m.pv.entries[idx], w, false, false, false, false)
		}
		return lines

	case pvImage:
		lines[0] = fitWidth(m.th.muted.Render(" "+m.pv.header), w)
		for i := 1; i < h; i++ {
			idx := i - 1
			if idx >= len(m.pv.lines) {
				lines[i] = strings.Repeat(" ", w)
				continue
			}
			lines[i] = fitWidth(" "+m.pv.lines[idx], w)
		}
		return lines

	case pvText:
		hdr := " " + m.pv.header
		if m.pv.scroll > 0 {
			hdr += fmt.Sprintf(" · L%d", m.pv.scroll+1)
		}
		lines[0] = fitWidth(m.th.muted.Render(hdr), w)
		numW := len(fmt.Sprint(max(len(m.pv.lines), 1)))
		for i := 1; i < h; i++ {
			idx := m.pv.scroll + (i - 1)
			if idx >= len(m.pv.lines) {
				lines[i] = strings.Repeat(" ", w)
				continue
			}
			ln := m.th.lineNum.Render(fmt.Sprintf(" %*d ", numW, idx+1))
			codeW := max(w-numW-2, 1)
			code := ansi.Truncate(m.pv.lines[idx], codeW, "…")
			lines[i] = fitWidth(ln+code, w)
		}
		return lines

	case pvMeta:
		lines[0] = fitWidth(m.th.muted.Render(" "+m.pv.header), w)
		for i := 1; i < h; i++ {
			idx := m.pv.scroll + (i - 1)
			if idx >= len(m.pv.lines) {
				lines[i] = strings.Repeat(" ", w)
				continue
			}
			lines[i] = fitWidth(" "+m.pv.lines[idx], w)
		}
		return lines
	}
	fillEmpty(0)
	return lines
}

func (m *Model) renderBottomBar() string {
	switch m.mode {
	case modeFilter, modeGoto, modeRename, modeNewFolder, modeNewFile:
		hint := "Enter 确认 · Esc 取消"
		if m.mode == modeGoto {
			hint = "Tab 补全 · Enter 跳转 · Esc 取消"
		} else if m.mode == modeFilter {
			hint = "支持拼音首字母 · Enter 保留 · Esc 清除"
		}
		return composeBar(" "+m.input.View(), m.th.hintText.Render(hint+" "), m.width, m.th.statusBar)
	case modeConfirmTrash:
		n := len(m.trashTargets)
		var prompt string
		if n == 1 {
			prompt = fmt.Sprintf(" 移到废纸篓「%s」？ [Y/n]", filepath.Base(m.trashTargets[0]))
		} else {
			prompt = fmt.Sprintf(" 将 %d 个项目移到废纸篓？ [Y/n]", n)
		}
		return composeBar(m.th.danger.Render(prompt), "", m.width, m.th.statusBar)
	}

	left := m.buildStatusLeft()
	right := m.buildCompactHints(max(m.width-lipgloss.Width(left)-2, 0))
	return composeBar(left, right, m.width, m.th.statusBar)
}

func (m *Model) buildStatusLeft() string {
	if ch, pending := m.taps.Pending(); pending {
		return " " + m.th.hintKey.Render(ch.String()) + m.th.hintText.Render(" …等待双击")
	}
	if m.status != "" {
		return " " + m.th.statusBar.Render(m.status)
	}
	es := m.tab.Entries()
	if e, ok := m.cursorEntry(); ok {
		pos := fmt.Sprintf("%d/%d", m.cursor+1, len(es))
		size := "目录"
		if !e.IsDir {
			size = humanSize(e.Size)
		}
		return " " + m.th.statusBar.Render(fmt.Sprintf("%s · %s · %s", pos, size, e.ModTime.Format("01-02 15:04")))
	}
	return " " + m.th.hintText.Render("0 项")
}

func (m *Model) buildCompactHints(maxW int) string {
	if maxW < 10 {
		return ""
	}
	hints := m.km.Hints(0)
	var parts []string
	used := 1 // trailing space
	for _, h := range hints {
		piece := m.th.hintKey.Render(h.Keys) + m.th.hintText.Render(" "+h.Title)
		pw := lipgloss.Width(piece)
		sepW := 0
		if len(parts) > 0 {
			sepW = 2
		}
		if used+sepW+pw > maxW {
			break
		}
		parts = append(parts, piece)
		used += sepW + pw
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, m.th.hintText.Render("  ")) + " "
}

func (m *Model) renderSortScreen() string {
	var b strings.Builder
	b.WriteString(m.th.modalHead.Render("排序方式") + "\n\n")
	for i, c := range sortChoices {
		prefix := "  "
		if c.key == m.tab.Sort && c.desc == m.tab.Desc {
			prefix = "✓ "
		}
		line := prefix + c.label
		if i == m.sortCursor {
			b.WriteString(m.th.cursor.Render(" " + line + " "))
		} else {
			b.WriteString(m.th.normal.Render(" " + line))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n" + m.th.muted.Render("↑↓ 选择 · Enter 确认 · Esc 取消"))
	box := m.th.modalBox.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) renderHelpScreen() string {
	items := m.km.Sheet(m.input.Value())
	boxW := min(max(m.width-6, 44), 76)
	boxH := max(m.height-4, 10)
	innerH := max(boxH-6, 3)

	var rows []string
	lastGroup := ""
	for _, it := range items {
		if it.Group != lastGroup {
			if lastGroup != "" {
				rows = append(rows, "")
			}
			rows = append(rows, m.th.modalHead.Render(it.Group))
			lastGroup = it.Group
		}
		keyCol := m.th.hintKey.Render(fmt.Sprintf(" %-14s ", it.Keys))
		rows = append(rows, ansi.Truncate(keyCol+"  "+m.th.normal.Render(it.Title), max(boxW-4, 10), "…"))
	}
	if len(rows) == 0 {
		rows = []string{m.th.muted.Render("(无匹配快捷键)")}
	}

	maxScroll := max(len(rows)-innerH, 0)
	if m.helpScroll > maxScroll {
		m.helpScroll = maxScroll
	}
	end := min(m.helpScroll+innerH, len(rows))
	visible := rows[m.helpScroll:end]

	var b strings.Builder
	b.WriteString(m.th.modalHead.Render("快捷键速查表") + "  " + m.th.muted.Render(fmt.Sprintf("(%d 项)", len(items))) + "\n")
	b.WriteString(m.input.View() + "\n")
	b.WriteString(m.th.sep.Render(strings.Repeat("─", max(boxW-4, 10))) + "\n")
	b.WriteString(strings.Join(visible, "\n"))
	for i := len(visible); i < innerH; i++ {
		b.WriteString("\n")
	}
	b.WriteString("\n" + m.th.muted.Render("直接输入过滤 · ↑↓/PgUp/PgDn 滚动 · Esc/? 关闭"))

	box := m.th.modalBox.Width(boxW).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func composeBar(left, right string, width int, bg lipgloss.Style) string {
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	if lw+rw > width {
		availLeft := max(width-rw-1, 0)
		left = ansi.Truncate(left, availLeft, "…")
		lw = lipgloss.Width(left)
	}
	gap := max(width-lw-rw, 0)
	return left + bg.Render(strings.Repeat(" ", gap)) + right
}

func fitWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	cur := lipgloss.Width(s)
	if cur < w {
		s += strings.Repeat(" ", w-cur)
	}
	return s
}

func truncateMiddle(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}
	rs := []rune(s)
	headBudget := (maxW - 1) / 2
	tailBudget := maxW - 1 - headBudget

	var head []rune
	hw := 0
	for _, r := range rs {
		rw := runewidth.RuneWidth(r)
		if hw+rw > headBudget {
			break
		}
		head = append(head, r)
		hw += rw
	}

	var tailRev []rune
	tw := 0
	for i := len(rs) - 1; i >= len(head); i-- {
		rw := runewidth.RuneWidth(rs[i])
		if tw+rw > tailBudget {
			break
		}
		tailRev = append(tailRev, rs[i])
		tw += rw
	}
	for i, j := 0, len(tailRev)-1; i < j; i, j = i+1, j-1 {
		tailRev[i], tailRev[j] = tailRev[j], tailRev[i]
	}
	return string(head) + "…" + string(tailRev)
}

func sortShortLabel(k pane.SortKey, desc bool) string {
	name := "名称"
	switch k {
	case pane.BySize:
		name = "大小"
	case pane.ByDate:
		name = "时间"
	case pane.ByKind:
		name = "类型"
	}
	if desc {
		return name + "↓"
	}
	return name + "↑"
}
