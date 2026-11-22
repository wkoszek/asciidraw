package main

import (
	"fmt"
	"slices"
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

// ---- the palette ----

// The color panel has 16 palette slots, saved with the drawing. They start
// as defaultPal. p opens a grid of preset colors to fill a slot from.

const palSlots = 16

var defaultPal = []string{
	"#ffffff", "#d4d4d4", "#737373", "#404040",
	"#ef4444", "#f97316", "#f59e0b", "#84cc16",
	"#22c55e", "#14b8a6", "#06b6d4", "#3b82f6",
	"#6366f1", "#8b5cf6", "#d946ef", "#ec4899",
}

// presets is the grid p opens: one column per hue, light at the top and
// dark at the bottom. They are the Tailwind CSS colors, shades 300 to 800,
// with a gray column that runs from white to black.
var presets = []struct {
	name   string
	shades [6]string
}{
	{"gray", [6]string{"#ffffff", "#d4d4d4", "#a3a3a3", "#737373", "#404040", "#000000"}},
	{"slate", [6]string{"#cbd5e1", "#94a3b8", "#64748b", "#475569", "#334155", "#1e293b"}},
	{"red", [6]string{"#fca5a5", "#f87171", "#ef4444", "#dc2626", "#b91c1c", "#991b1b"}},
	{"orange", [6]string{"#fdba74", "#fb923c", "#f97316", "#ea580c", "#c2410c", "#9a3412"}},
	{"amber", [6]string{"#fcd34d", "#fbbf24", "#f59e0b", "#d97706", "#b45309", "#92400e"}},
	{"yellow", [6]string{"#fde047", "#facc15", "#eab308", "#ca8a04", "#a16207", "#854d0e"}},
	{"lime", [6]string{"#bef264", "#a3e635", "#84cc16", "#65a30d", "#4d7c0f", "#3f6212"}},
	{"green", [6]string{"#86efac", "#4ade80", "#22c55e", "#16a34a", "#15803d", "#166534"}},
	{"emerald", [6]string{"#6ee7b7", "#34d399", "#10b981", "#059669", "#047857", "#065f46"}},
	{"teal", [6]string{"#5eead4", "#2dd4bf", "#14b8a6", "#0d9488", "#0f766e", "#115e59"}},
	{"cyan", [6]string{"#67e8f9", "#22d3ee", "#06b6d4", "#0891b2", "#0e7490", "#155e75"}},
	{"sky", [6]string{"#7dd3fc", "#38bdf8", "#0ea5e9", "#0284c7", "#0369a1", "#075985"}},
	{"blue", [6]string{"#93c5fd", "#60a5fa", "#3b82f6", "#2563eb", "#1d4ed8", "#1e40af"}},
	{"indigo", [6]string{"#a5b4fc", "#818cf8", "#6366f1", "#4f46e5", "#4338ca", "#3730a3"}},
	{"violet", [6]string{"#c4b5fd", "#a78bfa", "#8b5cf6", "#7c3aed", "#6d28d9", "#5b21b6"}},
	{"purple", [6]string{"#d8b4fe", "#c084fc", "#a855f7", "#9333ea", "#7e22ce", "#6b21a8"}},
	{"fuchsia", [6]string{"#f0abfc", "#e879f9", "#d946ef", "#c026d3", "#a21caf", "#86198f"}},
	{"pink", [6]string{"#f9a8d4", "#f472b6", "#ec4899", "#db2777", "#be185d", "#9d174d"}},
	{"rose", [6]string{"#fda4af", "#fb7185", "#f43f5e", "#e11d48", "#be123c", "#9f1239"}},
}

var shadeNames = [6]string{"300", "400", "500", "600", "700", "800"}

// presetName is how the grid names a color, like "blue 500".
func presetName(col, row int) string {
	if col == 0 {
		return [6]string{"white", "gray 300", "gray 400", "gray 500", "gray 700", "black"}[row]
	}
	return presets[col].name + " " + shadeNames[row]
}

// presetAt finds a color in the grid.
func presetAt(c string) (col, row int, ok bool) {
	for x, p := range presets {
		for y, h := range p.shades {
			if h == c {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// ---- the C picker ----

const palRow = 2 // the picker's third row: the palette slots

type picker struct {
	row      int       // 0 text color, 1 background, 2 the palette
	target   int       // the row a palette color goes to: 0 or 1
	cur      [2]int    // slot under the cursor in each row; len(palette) is the hex slot
	hex      [2]string // the hex color in each row's hex slot, if any
	set      [2]bool   // rows changed so far; only those are applied
	slot     int       // the palette slot under the cursor
	pal      []string  // the palette as it was, for Esc
	grid     *grid     // the preset grid, while it is open
	before   []object
	selected []int
}

// grid is the preset grid's cursor, and what it was opened on, for Esc.
type grid struct {
	col, row int
	was      picker
	wasColor string
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

// setRow moves the cursor between rows. The palette remembers which of
// text and background it colors: the one you came from.
func (p *picker) setRow(r int) {
	p.row = r
	if r != palRow {
		p.target = r
	}
}

func (a *app) openPicker() {
	sel := a.selected()
	if len(sel) == 0 {
		a.sayErr("Select a shape first: click it, or press Tab.")
		return
	}
	a.stopTyping()
	p := &picker{before: cloneObjs(a.objs), selected: sel, pal: slices.Clone(a.pal)}
	o := a.objs[a.sel]
	p.load(0, o.FG)
	p.load(1, o.BG)
	if i := slices.Index(a.pal, o.FG); i >= 0 {
		p.slot = i
	}
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
		a.objs, a.pal = p.before, p.pal
		a.say("Colors unchanged.")
		return
	}
	a.commit(p.before)
	if !slices.Equal(a.pal, p.pal) {
		a.dirty = true
	}
	a.say(fmt.Sprintf("Colors: text %s, background %s.", colorName(p.value(0)), colorName(p.value(1))))
}

// choose puts the cursor on a slot in the current row and applies it.
func (a *app) choose(slot int) {
	p := a.colors
	p.cur[p.row], p.set[p.row] = slot, true
	a.applyPicker()
}

// usePal puts a palette slot's color on the text or background.
func (a *app) usePal(slot int) {
	p := a.colors
	p.slot = slot
	t := p.target
	p.load(t, a.pal[slot])
	p.set[t] = true
	a.applyPicker()
}

// step moves along the row. The hex slot only counts once it has a color.
func (a *app) step(d int) {
	p := a.colors
	if p.row == palRow {
		a.usePal(((p.slot+d)%palSlots + palSlots) % palSlots)
		return
	}
	n := hexSlot
	if p.hex[p.row] != "" {
		n++
	}
	a.choose(((p.cur[p.row]+d)%n + n) % n)
}

func rowName(row int) string {
	if row == 1 {
		return "background"
	}
	return "text"
}

// askHex asks for a hex color, in the status bar. On the palette row it
// goes into the slot under the cursor.
func (a *app) askHex() {
	p := a.colors
	row := p.row
	what := "the " + rowName(row)
	initial := "#"
	if row == palRow {
		what = fmt.Sprintf("palette slot %d", p.slot+1)
	} else if p.hex[row] != "" {
		initial = p.hex[row]
	}
	a.ask("Hex color for "+what+", like #ff8800: ", initial, func(s string) {
		if a.colors != p {
			return
		}
		h, ok := parseHex(s)
		if !ok {
			a.sayErr("That isn't a hex color. Try #ff8800, or #f80 for short.")
			return
		}
		p.setRow(row)
		if row == palRow {
			a.pal[p.slot] = h
			a.usePal(p.slot)
		} else {
			p.hex[row] = h
			a.choose(hexSlot)
		}
		a.say("Picked " + h + ". Enter keeps it.")
	})
}

// ---- the preset grid ----

// openGrid shows the preset colors for the palette slot under the cursor.
// Moving in the grid tries each color on the shapes; Enter keeps it.
func (a *app) openGrid() {
	p := a.colors
	p.setRow(palRow)
	g := &grid{was: *p, wasColor: a.pal[p.slot]}
	g.col, g.row, _ = presetAt(a.pal[p.slot])
	p.grid = g // nothing changes until you move or pick
}

func (a *app) tryGrid() {
	p := a.colors
	a.pal[p.slot] = presets[p.grid.col].shades[p.grid.row]
	a.usePal(p.slot)
}

func (a *app) closeGrid(keep bool) {
	p := a.colors
	g := p.grid
	p.grid = nil
	if keep {
		a.say(fmt.Sprintf("Palette slot %d: %s. Enter keeps the colors.", p.slot+1, a.pal[p.slot]))
		return
	}
	a.pal[g.was.slot] = g.wasColor
	g.was.grid = nil
	*p = g.was
	a.applyPicker()
}

func (a *app) gridKey(ev *tcell.EventKey) {
	g := a.colors.grid
	dx, dy := 0, 0
	switch ev.Key() {
	case tcell.KeyLeft:
		dx = -1
	case tcell.KeyRight:
		dx = 1
	case tcell.KeyUp:
		dy = -1
	case tcell.KeyDown:
		dy = 1
	case tcell.KeyEnter:
		a.tryGrid()
		a.closeGrid(true)
	case tcell.KeyEscape:
		a.closeGrid(false)
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'h':
			dx = -1
		case 'l':
			dx = 1
		case 'k':
			dy = -1
		case 'j':
			dy = 1
		case ' ':
			a.tryGrid()
			a.closeGrid(true)
		case 'p', 'q':
			a.closeGrid(false)
		}
	}
	if dx != 0 || dy != 0 {
		n := len(presets)
		g.col = ((g.col+dx)%n + n) % n
		g.row = min(max(g.row+dy, 0), len(shadeNames)-1)
		a.tryGrid()
	}
}

// gridBox sits just above the picker.
func (a *app) gridBox() (x0, y0, w, h int) {
	w, h = 2+len(presets)*3+1, len(shadeNames)+4
	sw, _ := a.s.Size()
	_, py, _, _ := a.pickerBox()
	return max((sw-w)/2, 0), max(py-h, 0), w, h
}

func (a *app) gridAt(x, y int) (col, row int, ok bool) {
	x0, y0, _, _ := a.gridBox()
	row, k := y-y0-1, x-(x0+2)
	if row < 0 || row >= len(shadeNames) || k < 0 || k%3 == 2 || k/3 >= len(presets) {
		return 0, 0, false
	}
	return k / 3, row, true
}

func (a *app) gridMouse(x, y int) {
	g := a.colors.grid
	if col, row, ok := a.gridAt(x, y); ok {
		g.col, g.row = col, row
		a.tryGrid()
		a.closeGrid(true)
		return
	}
	a.closeGrid(false) // a click anywhere else puts the slot back
}

func (a *app) renderGrid() {
	g := a.colors.grid
	x0, y0, w, h := a.gridBox()
	a.frame(x0, y0, w, h, " p Palette ")
	for col, p := range presets {
		for row, c := range p.shades {
			x, y := x0+2+col*3, y0+1+row
			a.text(x, y, "██", tcell.StyleDefault.Foreground(colorOf(c)))
			if col == g.col && row == g.row {
				mark := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorAqua)
				a.s.SetContent(x-1, y, '[', nil, mark)
				a.s.SetContent(x+2, y, ']', nil, mark)
			}
		}
	}
	a.text(x0+2, y0+h-3, presetName(g.col, g.row)+"  "+presets[g.col].shades[g.row], tcell.StyleDefault.Bold(true))
	a.text(x0+2, y0+h-2, "←→↑↓ try  Enter or click pick  Esc back", tcell.StyleDefault.Dim(true))
}

// ---- the picker's keys, mouse and drawing ----

func (a *app) pickerKey(ev *tcell.EventKey) {
	p := a.colors
	if p.grid != nil {
		a.gridKey(ev)
		return
	}
	switch ev.Key() {
	case tcell.KeyLeft:
		a.step(-1)
	case tcell.KeyRight:
		a.step(1)
	case tcell.KeyUp, tcell.KeyBacktab:
		p.setRow((p.row + 2) % 3)
	case tcell.KeyDown, tcell.KeyTab:
		p.setRow((p.row + 1) % 3)
	case tcell.KeyEnter:
		a.closePicker(true)
	case tcell.KeyEscape:
		a.closePicker(false)
	case tcell.KeyRune:
		switch ev.Rune() {
		case '#':
			a.askHex()
		case 'p':
			a.openGrid()
		case ' ':
			if p.row == palRow { // switch what the palette colors
				p.target = 1 - p.target
				a.usePal(p.slot)
			}
		case 'h':
			a.step(-1)
		case 'l':
			a.step(1)
		case 'k':
			p.setRow((p.row + 2) % 3)
		case 'j':
			p.setRow((p.row + 1) % 3)
		case 'C', 'q':
			a.closePicker(true)
		}
	}
}

// pickerBox gives the picker's top-left corner and size. It sits low in
// the window, so the shapes being colored stay in view.
func (a *app) pickerBox() (x0, y0, w, h int) {
	w, h = 8+palSlots*3+4, 7
	sw, sh := a.s.Size()
	return max((sw-w)/2, 0), max(sh-2-h-1, 0), w, h
}

const palButton = "[ p Palette ]"

// swatchAt gives the row and slot of the swatch at (x, y). The text and
// background rows space their swatches 4 apart; the palette row, 3.
func (a *app) swatchAt(x, y int) (row, slot int, ok bool) {
	x0, y0, _, _ := a.pickerBox()
	row = y - y0 - 1
	k := x - (x0 + 8)
	switch {
	case k < 0:
		return 0, 0, false
	case row == 0 || row == 1:
		return row, k / 4, k/4 <= hexSlot && k%4 != 3
	case row == palRow:
		return row, k / 3, k/3 < palSlots && k%3 != 2
	}
	return 0, 0, false
}

func (a *app) onPalButton(x, y int) bool {
	x0, y0, w, _ := a.pickerBox()
	bx := x0 + w - 2 - runeLen(palButton)
	return y == y0+4 && x >= bx-1 && x < bx+runeLen(palButton)
}

func (a *app) pickerMouse(x, y int, b tcell.ButtonMask) {
	if b&tcell.Button1 == 0 {
		return
	}
	p := a.colors
	if p.grid != nil {
		a.gridMouse(x, y)
		return
	}
	x0, y0, w, h := a.pickerBox()
	if a.onPalButton(x, y) {
		a.openGrid()
		return
	}
	if row := y - y0 - 1; row >= 0 && row <= palRow && x > x0 && x < x0+7 {
		p.setRow(row) // a click on a row's name moves there
		return
	}
	if row, slot, ok := a.swatchAt(x, y); ok {
		p.setRow(row)
		switch {
		case row == palRow:
			a.usePal(slot)
		case slot == hexSlot && p.hex[row] == "":
			a.askHex()
		default:
			a.choose(slot)
		}
		return
	}
	if x < x0 || x >= x0+w || y < y0 || y >= y0+h {
		a.closePicker(true) // a click outside keeps what you picked
	}
}

// frame clears a rectangle and draws a rounded border with a title.
func (a *app) frame(x0, y0, w, h int, title string) {
	st := tcell.StyleDefault.Foreground(tcell.ColorAqua)
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
				a.s.SetContent(x, y, r, nil, st)
			}
		}
	}
	a.text(x0+2, y0, title, st.Bold(true))
}

func (a *app) renderPicker() {
	p := a.colors
	x0, y0, w, _ := a.pickerBox()
	a.frame(x0, y0, w, 7, " C Color ")
	mark := func(x, y, row int) {
		st := tcell.StyleDefault.Bold(true)
		if row == p.row {
			st = st.Foreground(tcell.ColorAqua)
		}
		a.s.SetContent(x-1, y, '[', nil, st)
		a.s.SetContent(x+2, y, ']', nil, st)
	}
	for row, label := range []string{" Text", " Back", " Pal"} {
		y := y0 + 1 + row
		ls := tcell.StyleDefault
		switch {
		case row == p.row:
			ls = ls.Bold(true).Foreground(tcell.ColorAqua)
		case p.row == palRow && row == p.target:
			ls = ls.Bold(true) // where the palette's colors go
		}
		a.text(x0+1, y, label, ls)
		if row == palRow {
			for i, c := range a.pal {
				x := x0 + 8 + i*3
				if i == p.slot {
					mark(x, y, row)
				}
				a.text(x, y, "██", tcell.StyleDefault.Foreground(colorOf(c)))
			}
			continue
		}
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
				mark(x, y, row)
			}
			a.text(x, y, cells, sw)
		}
	}
	info := colorName(p.value(p.target)) + " on the " + rowName(p.target)
	if p.row == palRow {
		info = fmt.Sprintf("%s on the %s (Space: %s)", a.pal[p.slot], rowName(p.target), rowName(1-p.target))
	}
	a.text(x0+2, y0+4, info, tcell.StyleDefault.Bold(true))
	bs := tcell.StyleDefault.Reverse(true)
	if p.grid != nil {
		bs = bs.Foreground(tcell.ColorAqua)
	}
	a.text(x0+w-2-runeLen(palButton), y0+4, palButton, bs)
	a.text(x0+2, y0+5, "←→ color  ↑↓ row  # hex  p palette  Enter keep  Esc undo", tcell.StyleDefault.Dim(true))
	if p.grid != nil {
		a.renderGrid()
	}
}
