package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// The ^T menu has two columns: shape tools and styles on the left, widgets
// and arrowheads on the right. The cursor walks one list holding all of
// them, in that order. The status bar previews whatever the cursor is on.

type menu struct{ cur int } // index into menuItems()

// What a menu row picks.
const (
	itemTool = iota
	itemStyle
	itemHead
)

type menuItem struct {
	kind int // itemTool, itemStyle or itemHead
	idx  int // into tools, styles or heads
}

func menuItems() []menuItem {
	var items []menuItem
	for i := 0; i < shapeTools; i++ {
		items = append(items, menuItem{itemTool, i})
	}
	for i := range styles {
		items = append(items, menuItem{itemStyle, i})
	}
	for i := shapeTools; i < len(tools); i++ {
		items = append(items, menuItem{itemTool, i})
	}
	for i := range heads {
		items = append(items, menuItem{itemHead, i})
	}
	return items
}

func itemOfTool(t int) int  { return indexOf(menuItem{itemTool, t}) }
func itemOfStyle(s int) int { return indexOf(menuItem{itemStyle, s}) }
func itemOfHead(h int) int  { return indexOf(menuItem{itemHead, h}) }

func indexOf(it menuItem) int {
	for i, m := range menuItems() {
		if m == it {
			return i
		}
	}
	return 0
}

type menuLine struct {
	text string
	item int // -1 for headers and blank lines
}

func (a *app) menuColumns() (left, right []menuLine) {
	mark := func(on bool) string {
		if on {
			return "●"
		}
		return " "
	}
	tool := func(i int) menuLine {
		return menuLine{fmt.Sprintf(" %s %-6s %s", mark(i == a.tool), tools[i].name, tools[i].help), itemOfTool(i)}
	}
	left = []menuLine{{" Tool", -1}}
	for i := 0; i < shapeTools; i++ {
		left = append(left, tool(i))
	}
	left = append(left, menuLine{"", -1}, menuLine{" Style", -1})
	for i, st := range styles {
		left = append(left, menuLine{fmt.Sprintf(" %s %-6s %s", mark(i == a.style), st.name, st.sample()), itemOfStyle(i)})
	}
	right = []menuLine{{" Widgets", -1}}
	for i := shapeTools; i < len(tools); i++ {
		right = append(right, tool(i))
	}
	right = append(right, menuLine{"", -1}, menuLine{" Arrow heads", -1})
	for i, h := range heads {
		right = append(right, menuLine{fmt.Sprintf(" %s %s  %s", mark(i == a.heads), h.sample, h.name), itemOfHead(i)})
	}
	return left, right
}

const menuFooter = " ↑↓ move  ←→ column  Space pick  Enter pick+close  Esc close"

func colWidth(lines []menuLine) int {
	w := 0
	for _, l := range lines {
		w = max(w, runeLen(l.text))
	}
	return w
}

// menuBox gives the menu's top-left corner and size, border included, and
// the x where the right column starts.
func (a *app) menuBox() (x0, y0, w, h, rightX int) {
	left, right := a.menuColumns()
	lw := colWidth(left)
	inner := max(lw+2+colWidth(right), runeLen(menuFooter))
	w, h = inner+3, max(len(left), len(right))+4
	sw, sh := a.s.Size()
	x0, y0 = max((sw-w)/2, 0), max((sh-2-h)/2, 0)
	return x0, y0, w, h, x0 + 1 + lw + 2
}

// menuItemAt gives the item at screen position (x, y), or -1.
func (a *app) menuItemAt(x, y int) int {
	x0, y0, w, _, rightX := a.menuBox()
	left, right := a.menuColumns()
	col := left
	if x >= rightX {
		col = right
	}
	row := y - y0 - 1
	if x <= x0 || x >= x0+w-1 || row < 0 || row >= len(col) {
		return -1
	}
	return col[row].item
}

// menuItemPos gives the screen position of an item's row.
func (a *app) menuItemPos(item int) (int, int) {
	x0, y0, _, _, rightX := a.menuBox()
	left, right := a.menuColumns()
	for i, l := range left {
		if l.item == item {
			return x0 + 2, y0 + 1 + i
		}
	}
	for i, l := range right {
		if l.item == item {
			return rightX + 1, y0 + 1 + i
		}
	}
	return x0, y0
}

func (a *app) openMenu() {
	a.menu = &menu{cur: itemOfTool(a.tool)}
	a.drag, a.pressed = nil, -1
}

// previewTool and previewStyle are what the status bar shows: the item under
// the menu cursor while the menu is open, the picked ones otherwise.
func (a *app) previewTool() int {
	if a.menu != nil {
		if it := menuItems()[a.menu.cur]; it.kind == itemTool {
			return it.idx
		}
		if menuItems()[a.menu.cur].kind == itemHead {
			return toolArrow
		}
	}
	return a.tool
}

func (a *app) previewStyle() int {
	if a.menu != nil {
		if it := menuItems()[a.menu.cur]; it.kind == itemStyle {
			return it.idx
		}
	}
	return a.style
}

func (a *app) previewHeads() int {
	if a.menu != nil {
		if it := menuItems()[a.menu.cur]; it.kind == itemHead {
			return it.idx
		}
	}
	return a.heads
}

// pick takes the item on a menu row. Picking arrowheads picks the Arrow
// tool too, and changes any arrows that are selected.
func (a *app) pick(item int) {
	it := menuItems()[item]
	switch it.kind {
	case itemStyle:
		a.style = it.idx
	case itemHead:
		a.heads, a.tool = it.idx, toolArrow
		a.restyleHeads()
	default:
		a.tool = it.idx
	}
	msg := fmt.Sprintf("Tool: %s, style: %s", tools[a.tool].name, styles[a.style].name)
	if a.tool == toolArrow {
		msg += ", heads: " + heads[a.heads].name
	}
	a.say(msg)
}

// restyleHeads gives the selected arrows the heads picked for new ones.
func (a *app) restyleHeads() {
	before := cloneObjs(a.objs)
	changed := false
	for _, i := range a.selected() {
		if a.objs[i].Kind == "arrow" && a.objs[i].Heads != a.heads {
			a.objs[i].Heads = a.heads
			changed = true
		}
	}
	if changed {
		a.commit(before)
	}
}

func (a *app) menuKey(ev *tcell.EventKey) {
	m := a.menu
	n := len(menuItems())
	inWidgets := func() bool { // in the right column
		it := menuItems()[m.cur]
		return it.kind == itemHead || it.kind == itemTool && it.idx >= shapeTools
	}
	switch ev.Key() {
	case tcell.KeyUp:
		m.cur = (m.cur + n - 1) % n
	case tcell.KeyDown, tcell.KeyTab:
		m.cur = (m.cur + 1) % n
	case tcell.KeyRight:
		if !inWidgets() {
			m.cur = itemOfTool(shapeTools)
		}
	case tcell.KeyLeft:
		if inWidgets() {
			m.cur = itemOfTool(0)
		}
	case tcell.KeyEnter:
		a.pick(m.cur)
		a.menu = nil
	case tcell.KeyEscape, tcell.KeyCtrlT, tcell.KeyCtrlC:
		a.menu = nil
	case tcell.KeyRune:
		switch ev.Rune() {
		case ' ':
			a.pick(m.cur)
		case 'k':
			m.cur = (m.cur + n - 1) % n
		case 'j':
			m.cur = (m.cur + 1) % n
		case 'q':
			a.menu = nil
		}
	}
}

// menuMouse: hovering moves the cursor, clicking picks, clicking outside closes.
func (a *app) menuMouse(x, y int, b tcell.ButtonMask) {
	x0, y0, w, h, _ := a.menuBox()
	inside := x > x0 && x < x0+w-1 && y > y0 && y < y0+h-1
	item := a.menuItemAt(x, y)
	if item >= 0 {
		a.menu.cur = item
	}
	if b&tcell.Button1 != 0 {
		switch {
		case item >= 0:
			a.pick(item)
		case !inside:
			a.menu = nil
		}
	}
}

func (a *app) renderMenu() {
	x0, y0, w, h, rightX := a.menuBox()
	left, right := a.menuColumns()
	frame := tcell.StyleDefault.Foreground(tcell.ColorAqua)
	body := tcell.StyleDefault
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			a.s.SetContent(x, y, ' ', nil, body)
		}
	}
	c := &canvas{}
	drawBox(c, x0, y0, x0+w-1, y0+h-1, styles[2]) // round border for the menu itself
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if r := c.get(x, y); r != ' ' {
				a.s.SetContent(x, y, r, nil, frame)
			}
		}
	}
	a.text(x0+2, y0, " ^T Tools ", frame.Bold(true))
	column := func(lines []menuLine, x, end int) {
		for i, l := range lines {
			y := y0 + 1 + i
			ls := body
			switch {
			case l.item >= 0 && l.item == a.menu.cur:
				ls = body.Reverse(true)
				for xx := x; xx < end; xx++ {
					a.s.SetContent(xx, y, ' ', nil, ls)
				}
			case l.item < 0 && l.text != "":
				ls = body.Bold(true).Foreground(tcell.ColorAqua)
			}
			a.text(x, y, l.text, ls)
		}
	}
	column(left, x0+1, rightX-1)
	column(right, rightX, x0+w-1)
	a.text(x0+1, y0+h-2, menuFooter, body.Dim(true))
}
