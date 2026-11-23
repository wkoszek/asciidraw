package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// The T dialog: where a box's or oval's label sits. Nine buttons in a 3×3
// grid; the arrow keys move between them and the label follows at once.

var placeRows = [3]string{"top", "center", "bottom"}
var placeCols = [3]string{"left", "center", "right"}

// placeName names a position the way the dialog and the YAML do:
// "top left", "center", "bottom center".
func placeName(h, v int) string {
	if h == 0 && v == 0 {
		return "center"
	}
	return placeRows[v+1] + " " + placeCols[h+1]
}

// parsePlace reads a name from placeName. It also takes "middle" for
// center, hyphens for spaces, and either word order.
func parsePlace(s string) (h, v int, ok bool) {
	s = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "-", " ")))
	s = strings.ReplaceAll(s, "middle", "center")
	for v := -1; v <= 1; v++ {
		for h := -1; h <= 1; h++ {
			name := placeName(h, v)
			rev := placeCols[h+1] + " " + placeRows[v+1]
			if s == name || s == rev || strings.Join(strings.Fields(s), " ") == name {
				return h, v, true
			}
		}
	}
	return 0, 0, false
}

type placer struct {
	h, v     int // the button under the cursor
	before   []object
	selected []int // the boxes and ovals it changes
}

func (a *app) openPlacer() {
	var sel []int
	for _, i := range a.selected() {
		if hasLabel(a.objs[i].Kind) {
			sel = append(sel, i)
		}
	}
	if len(sel) == 0 {
		a.sayErr("Select a box or an oval first. T places its label.")
		return
	}
	a.stopTyping()
	o := a.objs[sel[0]]
	if a.sel >= 0 && a.sel < len(a.objs) && hasLabel(a.objs[a.sel].Kind) {
		o = a.objs[a.sel]
	}
	a.placer = &placer{h: o.HAlign, v: o.VAlign, before: cloneObjs(a.objs), selected: sel}
}

// tryPlace moves the labels to the button under the cursor.
func (a *app) tryPlace() {
	p := a.placer
	a.objs = cloneObjs(p.before)
	for _, i := range p.selected {
		a.objs[i].HAlign, a.objs[i].VAlign = p.h, p.v
	}
}

func (a *app) closePlacer(keep bool) {
	p := a.placer
	if keep {
		a.tryPlace() // a number key or a click picks without moving first
	}
	a.placer = nil
	if !keep {
		a.objs = p.before
		a.say("Label left where it was.")
		return
	}
	a.commit(p.before)
	a.say("Label: " + placeName(p.h, p.v) + ".")
}

func (a *app) movePlace(dh, dv int) {
	p := a.placer
	p.h = min(max(p.h+dh, -1), 1)
	p.v = min(max(p.v+dv, -1), 1)
	a.tryPlace()
}

// numpad is the number keys laid out like a keypad: 7 is top left.
var numpad = map[rune][2]int{
	'7': {-1, -1}, '8': {0, -1}, '9': {1, -1},
	'4': {-1, 0}, '5': {0, 0}, '6': {1, 0},
	'1': {-1, 1}, '2': {0, 1}, '3': {1, 1},
}

func (a *app) placerKey(ev *tcell.EventKey) {
	p := a.placer
	switch ev.Key() {
	case tcell.KeyLeft:
		a.movePlace(-1, 0)
	case tcell.KeyRight:
		a.movePlace(1, 0)
	case tcell.KeyUp:
		a.movePlace(0, -1)
	case tcell.KeyDown:
		a.movePlace(0, 1)
	case tcell.KeyEnter:
		a.closePlacer(true)
	case tcell.KeyEscape:
		a.closePlacer(false)
	case tcell.KeyRune:
		r := ev.Rune()
		if hv, ok := numpad[r]; ok {
			p.h, p.v = hv[0], hv[1]
			a.closePlacer(true)
			return
		}
		switch r {
		case 'h':
			a.movePlace(-1, 0)
		case 'l':
			a.movePlace(1, 0)
		case 'k':
			a.movePlace(0, -1)
		case 'j':
			a.movePlace(0, 1)
		case ' ', 'T', 'q':
			a.closePlacer(true)
		}
	}
}

// placeColW is each column's button width: its longest name plus padding.
func placeColW(h int) int {
	w := 0
	for v := -1; v <= 1; v++ {
		w = max(w, runeLen(placeName(h, v)))
	}
	return w + 4
}

func (a *app) placerBox() (x0, y0, w, h int) {
	w = 2 + placeColW(-1) + 1 + placeColW(0) + 1 + placeColW(1) + 2
	h = 7
	sw, sh := a.s.Size()
	return max((sw-w)/2, 0), max(sh-2-h-1, 0), w, h
}

// placeButtonAt finds the button at (x, y).
func (a *app) placeButtonAt(x, y int) (h, v int, ok bool) {
	x0, y0, _, _ := a.placerBox()
	v = y - y0 - 1 - 1
	if v < -1 || v > 1 {
		return 0, 0, false
	}
	bx := x0 + 2
	for h := -1; h <= 1; h++ {
		if x >= bx && x < bx+placeColW(h) {
			return h, v, true
		}
		bx += placeColW(h) + 1
	}
	return 0, 0, false
}

func (a *app) placerMouse(x, y int, b tcell.ButtonMask) {
	if b&tcell.Button1 == 0 {
		return
	}
	p := a.placer
	if h, v, ok := a.placeButtonAt(x, y); ok {
		p.h, p.v = h, v
		a.closePlacer(true)
		return
	}
	x0, y0, w, h := a.placerBox()
	if x < x0 || x >= x0+w || y < y0 || y >= y0+h {
		a.closePlacer(true) // a click outside keeps what the arrows picked
	}
}

func (a *app) renderPlacer() {
	p := a.placer
	x0, y0, w, h := a.placerBox()
	a.frame(x0, y0, w, h, " T Label position ")
	for v := -1; v <= 1; v++ {
		bx := x0 + 2
		for hh := -1; hh <= 1; hh++ {
			cw := placeColW(hh)
			label := fmt.Sprintf("[ %-*s ]", cw-4, placeName(hh, v))
			st := tcell.StyleDefault
			if hh == p.h && v == p.v {
				st = st.Reverse(true).Bold(true).Foreground(tcell.ColorAqua)
			}
			a.text(bx, y0+2+v, label, st)
			bx += cw + 1
		}
	}
	a.text(x0+2, y0+h-2, "←→↑↓ move  1-9 keypad  Enter keep  Esc undo", tcell.StyleDefault.Dim(true))
}
