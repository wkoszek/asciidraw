package main

import (
	"fmt"
	"strconv"
	"strings"

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

// parseHex reads "#rrggbb" or "#rgb", and gives it back as "#rrggbb".
func parseHex(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	h := s[1:]
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return "", false
	}
	if _, err := strconv.ParseUint(h, 16, 32); err != nil {
		return "", false
	}
	return "#" + h, true
}

// cleanColor accepts a palette name, "" or a hex color in any form a
// person might type (#FF8800, #f80), and gives back the stored form.
func cleanColor(name string) (string, bool) {
	if _, ok := colorIndex(name); ok {
		return name, true
	}
	if strings.HasPrefix(strings.TrimSpace(name), "#") {
		return parseHex(name)
	}
	return "", false
}

func colorOf(name string) tcell.Color {
	if strings.HasPrefix(name, "#") {
		if v, err := strconv.ParseUint(name[1:], 16, 32); err == nil {
			return tcell.NewHexColor(int32(v))
		}
	}
	i, _ := colorIndex(name)
	return palette[i].color
}

// takesBG says whether a background color fills the inside of a shape.
func takesBG(kind string) bool { return kind == "box" || kind == "oval" || kind == "window" }

// ---- the C picker ----

type picker struct {
	row      int       // 0 text color, 1 background
	cur      [2]int    // slot under the cursor in each row; len(palette) is the hex slot
	hex      [2]string // the hex color in each row's hex slot, if any
	set      [2]bool   // rows changed so far; only those are applied
	before   []object
	selected []int
}

var hexSlot = len(palette) // the slot after the named colors

func (p *picker) value(row int) string {
	if p.cur[row] == hexSlot {
		return p.hex[row]
	}
	return palette[p.cur[row]].name
}

func (p *picker) load(row int, name string) {
	if i, ok := colorIndex(name); ok {
		p.cur[row] = i
		return
	}
	p.cur[row], p.hex[row] = hexSlot, name
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
	p.load(0, o.FG)
	p.load(1, o.BG)
	a.colors = p
}

// applyPicker puts the picker's colors on the selected shapes, live.
func (a *app) applyPicker() {
	p := a.colors
	a.objs = cloneObjs(p.before)
	for _, i := range p.selected {
		if p.set[0] {
			a.objs[i].FG = p.value(0)
		}
		if p.set[1] {
			a.objs[i].BG = p.value(1)
		}
	}
}

func colorName(c string) string {
	if c == "" {
		return "default"
	}
	return c
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
	a.say(fmt.Sprintf("Colors: text %s, background %s.", colorName(p.value(0)), colorName(p.value(1))))
}

// choose puts the cursor on a slot in the current row and applies it.
func (a *app) choose(slot int) {
	p := a.colors
	p.cur[p.row], p.set[p.row] = slot, true
	a.applyPicker()
}

// step moves along the row. The hex slot only counts once it has a color.
func (a *app) step(d int) {
	p := a.colors
	n := hexSlot
	if p.hex[p.row] != "" {
		n++
	}
	a.choose(((p.cur[p.row]+d)%n + n) % n)
}

// askHex asks for a hex color for the current row, in the status bar.
func (a *app) askHex() {
	p := a.colors
	row := p.row
	what := "text"
	if row == 1 {
		what = "background"
	}
	initial := p.hex[row]
	if initial == "" {
		initial = "#"
	}
	a.ask("Hex color for the "+what+", like #ff8800: ", initial, func(s string) {
		if a.colors != p {
			return
		}
		h, ok := parseHex(s)
		if !ok {
			a.sayErr("That isn't a hex color. Try #ff8800, or #f80 for short.")
			return
		}
		p.row, p.hex[row] = row, h
		a.choose(hexSlot)
		a.say("Picked " + h + ". Enter keeps it.")
	})
}

func (a *app) pickerKey(ev *tcell.EventKey) {
	p := a.colors
	switch ev.Key() {
	case tcell.KeyLeft:
		a.step(-1)
	case tcell.KeyRight:
		a.step(1)
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyTab:
		p.row = 1 - p.row
	case tcell.KeyEnter:
		a.closePicker(true)
	case tcell.KeyEscape:
		a.closePicker(false)
	case tcell.KeyRune:
		switch ev.Rune() {
		case '#':
			a.askHex()
		case 'h':
			a.step(-1)
		case 'l':
			a.step(1)
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
	w, h = 8+(hexSlot+1)*4+3, 6
	sw, sh := a.s.Size()
	return max((sw-w)/2, 0), max(sh-2-h-1, 0), w, h
}

// swatchAt gives the row and slot of the swatch at (x, y).
func (a *app) swatchAt(x, y int) (row, slot int, ok bool) {
	x0, y0, _, _ := a.pickerBox()
	row = y - y0 - 1
	k := x - (x0 + 8)
	if row < 0 || row > 1 || k < 0 || k/4 > hexSlot || k%4 == 3 {
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
	if row, slot, ok := a.swatchAt(x, y); ok {
		p.row = row
		if slot == hexSlot && p.hex[row] == "" {
			a.askHex()
			return
		}
		a.choose(slot)
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
		for i := 0; i <= hexSlot; i++ {
			x := x0 + 8 + i*4
			var cells string
			var sw tcell.Style
			switch {
			case i == hexSlot && p.hex[row] == "":
				cells, sw = " #", tcell.StyleDefault.Dim(true)
			case i == hexSlot:
				cells, sw = "██", tcell.StyleDefault.Foreground(colorOf(p.hex[row]))
			case palette[i].name == "":
				cells, sw = "··", tcell.StyleDefault.Dim(true)
			default:
				cells, sw = "██", tcell.StyleDefault.Foreground(palette[i].color)
			}
			if i == p.cur[row] {
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
	a.text(x0+2, y0+3, colorName(p.value(p.row)), tcell.StyleDefault.Bold(true))
	a.text(x0+2, y0+4, "←→ color  ↑↓ text/back  # hex  Enter keep  Esc undo", tcell.StyleDefault.Dim(true))
}
