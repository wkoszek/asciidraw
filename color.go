package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// Colors. Each shape can have a text color (FG) and a background (BG),
// stored by name. "" is the terminal's own default. They are the
// terminal's palette colors, so they follow its theme. The Markdown
// export leaves colors out.

var palette = []struct {
	name  string
	color tcell.Color
}{
	{"", tcell.ColorDefault},
	{"black", tcell.PaletteColor(0)},
	{"red", tcell.PaletteColor(1)},
	{"green", tcell.PaletteColor(2)},
	{"yellow", tcell.PaletteColor(3)},
	{"blue", tcell.PaletteColor(4)},
	{"magenta", tcell.PaletteColor(5)},
	{"cyan", tcell.PaletteColor(6)},
	{"white", tcell.PaletteColor(7)},
	{"gray", tcell.PaletteColor(8)},
}

func colorIndex(name string) (int, bool) {
	for i, p := range palette {
		if p.name == name {
			return i, true
		}
	}
	return 0, false
}

func colorOf(name string) tcell.Color {
	i, _ := colorIndex(name)
	return palette[i].color
}

// takesBG says whether a background color fills the inside of a shape.
func takesBG(kind string) bool { return kind == "box" || kind == "oval" || kind == "window" }

// ---- the C picker ----

type picker struct {
	row      int // 0 text color, 1 background
	fg, bg   int // palette index under the cursor in each row
	fgSet    bool
	bgSet    bool // rows changed so far; only those are applied
	before   []object
	selected []int
}

func (a *app) openPicker() {
	sel := a.selected()
	if len(sel) == 0 {
		a.sayErr("Select a shape first: click it, or press Tab.")
		return
	}
	a.stopTyping()
	p := &picker{before: cloneObjs(a.objs), selected: sel}
	o := a.objs[a.sel]
	p.fg, _ = colorIndex(o.FG)
	p.bg, _ = colorIndex(o.BG)
	a.colors = p
}

// applyPicker puts the picker's colors on the selected shapes, live.
func (a *app) applyPicker() {
	p := a.colors
	a.objs = cloneObjs(p.before)
	for _, i := range p.selected {
		if p.fgSet {
			a.objs[i].FG = palette[p.fg].name
		}
		if p.bgSet {
			a.objs[i].BG = palette[p.bg].name
		}
	}
}

func (a *app) closePicker(keep bool) {
	p := a.colors
	a.colors = nil
	if !keep {
		a.objs = p.before
		a.say("Colors unchanged.")
		return
	}
	a.commit(p.before)
	name := func(i int) string {
		if palette[i].name == "" {
			return "default"
		}
		return palette[i].name
	}
	a.say(fmt.Sprintf("Colors: text %s, background %s.", name(p.fg), name(p.bg)))
}

func (a *app) pickerKey(ev *tcell.EventKey) {
	p := a.colors
	n := len(palette)
	step := func(d int) {
		if p.row == 0 {
			p.fg, p.fgSet = (p.fg+d+n)%n, true
		} else {
			p.bg, p.bgSet = (p.bg+d+n)%n, true
		}
		a.applyPicker()
	}
	switch ev.Key() {
	case tcell.KeyLeft:
		step(-1)
	case tcell.KeyRight:
		step(1)
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyTab:
		p.row = 1 - p.row
	case tcell.KeyEnter:
		a.closePicker(true)
	case tcell.KeyEscape:
		a.closePicker(false)
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'h':
			step(-1)
		case 'l':
			step(1)
		case 'j', 'k':
			p.row = 1 - p.row
		case 'C', 'q':
			a.closePicker(true)
		}
	}
}

// pickerBox gives the picker's top-left corner and size. It sits low in
// the window, so the shapes being colored stay in view.
func (a *app) pickerBox() (x0, y0, w, h int) {
	w, h = 8+len(palette)*4+3, 6
	sw, sh := a.s.Size()
	return max((sw-w)/2, 0), max(sh-2-h-1, 0), w, h
}

// swatchAt gives the row and palette index of the swatch at (x, y).
func (a *app) swatchAt(x, y int) (row, idx int, ok bool) {
	x0, y0, _, _ := a.pickerBox()
	row = y - y0 - 1
	k := x - (x0 + 8)
	if row < 0 || row > 1 || k < 0 || k/4 >= len(palette) || k%4 == 3 {
		return 0, 0, false
	}
	return row, k / 4, true
}

func (a *app) pickerMouse(x, y int, b tcell.ButtonMask) {
	if b&tcell.Button1 == 0 {
		return
	}
	p := a.colors
	x0, y0, w, h := a.pickerBox()
	if row, idx, ok := a.swatchAt(x, y); ok {
		p.row = row
		if row == 0 {
			p.fg, p.fgSet = idx, true
		} else {
			p.bg, p.bgSet = idx, true
		}
		a.applyPicker()
		return
	}
	if x < x0 || x >= x0+w || y < y0 || y >= y0+h {
		a.closePicker(true) // a click outside keeps what you picked
	}
}

func (a *app) renderPicker() {
	p := a.colors
	x0, y0, w, h := a.pickerBox()
	frame := tcell.StyleDefault.Foreground(tcell.ColorAqua)
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			a.s.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
	c := &canvas{}
	drawBox(c, x0, y0, x0+w-1, y0+h-1, styles[2])
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if r := c.get(x, y); r != ' ' {
				a.s.SetContent(x, y, r, nil, frame)
			}
		}
	}
	a.text(x0+2, y0, " C Color ", frame.Bold(true))
	for row, label := range []string{" Text", " Back"} {
		y := y0 + 1 + row
		ls := tcell.StyleDefault
		if row == p.row {
			ls = ls.Bold(true).Foreground(tcell.ColorAqua)
		}
		a.text(x0+1, y, label, ls)
		cur := p.fg
		if row == 1 {
			cur = p.bg
		}
		for i, col := range palette {
			x := x0 + 8 + i*4
			sw := tcell.StyleDefault.Foreground(col.color)
			cells := "██"
			if col.name == "" {
				cells, sw = "··", tcell.StyleDefault.Dim(true)
			}
			if i == cur {
				mark := tcell.StyleDefault.Bold(true)
				if row == p.row {
					mark = mark.Foreground(tcell.ColorAqua)
				}
				a.s.SetContent(x-1, y, '[', nil, mark)
				a.s.SetContent(x+2, y, ']', nil, mark)
			}
			a.text(x, y, cells, sw)
		}
	}
	name := palette[p.fg].name
	if p.row == 1 {
		name = palette[p.bg].name
	}
	if name == "" {
		name = "default"
	}
	a.text(x0+2, y0+3, name, tcell.StyleDefault.Bold(true))
	a.text(x0+2, y0+4, "←→ color  ↑↓ text/back  Enter keep  Esc undo", tcell.StyleDefault.Dim(true))
}
