package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// The ^T menu: one list holding the tools and then the styles. The cursor
// moves through both. The status bar previews whatever the cursor is on.

type menu struct{ cur int } // index into the combined item list

func itemCount() int { return len(tools) + len(styles) }

type menuLine struct {
	text string
	item int // -1 for headers and blank lines
}

func (a *app) menuLines() []menuLine {
	mark := func(on bool) string {
		if on {
			return "●"
		}
		return " "
	}
	lines := []menuLine{{" Tool", -1}}
	for i, t := range tools {
		lines = append(lines, menuLine{fmt.Sprintf(" %s %-6s %s", mark(i == a.tool), t.name, t.help), i})
	}
	lines = append(lines, menuLine{"", -1}, menuLine{" Style", -1})
	for i, st := range styles {
		lines = append(lines, menuLine{fmt.Sprintf(" %s %-6s %s", mark(i == a.style), st.name, st.sample()), len(tools) + i})
	}
	return append(lines, menuLine{"", -1},
		menuLine{" ↑↓ move  Space pick  Enter pick+close  Esc close", -1})
}

// menuBox gives the top-left corner and size of the menu, border included.
func (a *app) menuBox(lines []menuLine) (x0, y0, w, h int) {
	for _, l := range lines {
		w = max(w, len([]rune(l.text)))
	}
	w, h = w+3, len(lines)+2
	sw, sh := a.s.Size()
	return max((sw-w)/2, 0), max((sh-2-h)/2, 0), w, h
}

func (a *app) openMenu() {
	a.menu = &menu{cur: a.tool}
	a.drag, a.pressed = nil, -1
}

// previewTool and previewStyle are what the status bar shows: the item under
// the menu cursor while the menu is open, the picked ones otherwise.
func (a *app) previewTool() int {
	if a.menu != nil && a.menu.cur < len(tools) {
		return a.menu.cur
	}
	return a.tool
}

func (a *app) previewStyle() int {
	if a.menu != nil && a.menu.cur >= len(tools) {
		return a.menu.cur - len(tools)
	}
	return a.style
}

func (a *app) pick(item int) {
	if item < len(tools) {
		a.tool = item
	} else {
		a.style = item - len(tools)
	}
	a.say(fmt.Sprintf("Tool: %s, style: %s", tools[a.tool].name, styles[a.style].name))
}

func (a *app) menuKey(ev *tcell.EventKey) {
	m := a.menu
	switch ev.Key() {
	case tcell.KeyUp:
		m.cur = (m.cur + itemCount() - 1) % itemCount()
	case tcell.KeyDown, tcell.KeyTab:
		m.cur = (m.cur + 1) % itemCount()
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
			m.cur = (m.cur + itemCount() - 1) % itemCount()
		case 'j':
			m.cur = (m.cur + 1) % itemCount()
		case 'q':
			a.menu = nil
		}
	}
}

// menuMouse: hovering moves the cursor, clicking picks, clicking outside closes.
func (a *app) menuMouse(x, y int, b tcell.ButtonMask) {
	lines := a.menuLines()
	x0, y0, w, h := a.menuBox(lines)
	inside := x > x0 && x < x0+w-1 && y > y0 && y < y0+h-1
	item := -1
	if inside {
		item = lines[y-y0-1].item
	}
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
	lines := a.menuLines()
	x0, y0, w, h := a.menuBox(lines)
	st := styles[2] // round border for the menu itself
	frame := tcell.StyleDefault.Foreground(tcell.ColorAqua)
	body := tcell.StyleDefault
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			a.s.SetContent(x, y, ' ', nil, body)
		}
	}
	c := &canvas{}
	drawBox(c, x0, y0, x0+w-1, y0+h-1, st)
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if r := c.get(x, y); r != ' ' {
				a.s.SetContent(x, y, r, nil, frame)
			}
		}
	}
	a.text(x0+2, y0, " ^T Tools ", frame.Bold(true))
	for i, l := range lines {
		y := y0 + 1 + i
		ls := body
		switch {
		case l.item == a.menu.cur:
			ls = body.Reverse(true)
			for x := x0 + 1; x < x0+w-1; x++ {
				a.s.SetContent(x, y, ' ', nil, ls)
			}
		case l.item < 0 && l.text != "" && i < len(lines)-1:
			ls = body.Bold(true).Foreground(tcell.ColorAqua)
		case i == len(lines)-1:
			ls = body.Dim(true)
		}
		a.text(x0+1, y, l.text, ls)
	}
}
