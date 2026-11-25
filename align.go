package main

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
)

// Align and spread. Press a, and a window shows the choices; press one
// more key, or click a choice. The selected shapes of each group count as
// one block, and so does each loose shape. A line with an end attached to
// a shape isn't a block: it follows its shape.

const alignHelp = "Align: press a key or click a choice."

// alignLines is the window's text. Each choice starts with its key. The
// pictures are drawn in blocks, so the brackets only ever mean keys.
var alignLines = []string{
	"Line up",
	"  [  left edges          ^  top edges",
	"  |  centers (column)    -  middles (row)",
	"  ]  right edges         _  bottom edges",
	"",
	"Spread out (three or more shapes)",
	"  x  █<->█████<->███     equal gaps, across",
	"  X  █    █████   ███    centers evenly spaced, across",
	"     └──────┴──────┘",
	"  y  ███     Y  ███┐     y  equal gaps, down",
	"      ↕            │     Y  centers evenly spaced, down",
	"     ███        ███┤",
	"      ↕            │",
	"     ███        ███┘",
}

const alignFoot = "Esc cancels."

// alignSpot is a place in the window to click a choice: rows r0 to r1 and
// columns c0 up to c1, counted inside the border. A choice can have more
// than one, as y and Y do: the picture and the words.
type alignSpot struct {
	key            string
	r0, r1, c0, c1 int
}

var alignSpots = []alignSpot{
	{"[", 1, 1, 0, 24}, {"|", 2, 2, 0, 24}, {"]", 3, 3, 0, 24},
	{"^", 1, 1, 24, 99}, {"-", 2, 2, 24, 99}, {"_", 3, 3, 24, 99},
	{"x", 6, 6, 0, 99}, {"X", 7, 8, 0, 99},
	{"y", 9, 13, 0, 12}, {"Y", 9, 13, 12, 24},
	{"y", 9, 9, 24, 99}, {"Y", 10, 10, 24, 99},
}

// alignKeys turns the window's keys, and the older vim keys, into what
// align and spread take.
var alignKeys = map[string]string{
	"[": "h", "]": "l", "^": "k", "_": "j", "|": "c", "-": "m",
	"h": "h", "l": "l", "k": "k", "j": "j", "c": "c", "m": "m",
	"x": "x", "y": "y", "X": "X", "Y": "Y",
}

// block is one piece that aligning moves: its shapes and the box around them.
type block struct {
	idx        []int
	x, y, w, h int
}

func attached(o object) bool { return isLine(o.Kind) && (o.From.ID != 0 || o.To.ID != 0) }

// blocks splits the selection into blocks, in drawing order.
func (a *app) blocks() []block {
	var out []block
	seen := map[int]bool{} // groups already taken
	for _, i := range a.selected() {
		o := a.objs[i]
		var idx []int
		switch {
		case o.Group != 0:
			if seen[o.Group] {
				continue
			}
			seen[o.Group] = true
			for _, j := range a.groupOf(i) {
				if a.isSel(j) { // one shape picked out of a group moves alone
					idx = append(idx, j)
				}
			}
		case attached(o):
			continue
		default:
			idx = []int{i}
		}
		b := block{idx: idx}
		b.x, b.y, b.w, b.h = selBounds(a.objs, idx)
		out = append(out, b)
	}
	return out
}

func (a *app) startAlign() {
	if len(a.selected()) == 0 {
		a.sayErr("Select the shapes to line up first: Shift-click them, or press v and drag a box around them.")
		return
	}
	a.aligning = true
}

func (a *app) alignKey(ev *tcell.EventKey) {
	how := ""
	switch ev.Key() {
	case tcell.KeyLeft:
		how = "h"
	case tcell.KeyRight:
		how = "l"
	case tcell.KeyUp:
		how = "k"
	case tcell.KeyDown:
		how = "j"
	case tcell.KeyRune:
		how = alignKeys[string(ev.Rune())]
	}
	a.alignRun(how)
}

// alignRun closes the window and does what how says: an align or a spread,
// or nothing for "".
func (a *app) alignRun(how string) {
	a.aligning = false
	switch how {
	case "h", "l", "k", "j", "c", "m":
		a.align(how)
	case "x", "y":
		a.spread(how == "x", false)
	case "X", "Y":
		a.spread(how == "X", true)
	default:
		a.say("Align cancelled.")
	}
}

// alignBox is where the window goes: centered across, and above or below
// the selection so it stays in view.
func (a *app) alignBox() (x0, y0, w, h int) {
	w, h = 60, len(alignLines)+5
	sw, sh := a.s.Size()
	x0 = max((sw-w)/2, 0)
	_, sy, _, sbh := selBounds(a.objs, a.selected())
	if sy+sbh/2 < (sh-2)/2 {
		return x0, max(sh-2-h, 0), w, h
	}
	return x0, 0, w, h
}

// alignMouse runs the choice under a click; a click anywhere else cancels.
func (a *app) alignMouse(x, y int) {
	x0, y0, w, h := a.alignBox()
	if x >= x0 && x < x0+w && y >= y0 && y < y0+h {
		r, c := y-y0-2, x-x0-2
		for _, sp := range alignSpots {
			if r >= sp.r0 && r <= sp.r1 && c >= sp.c0 && c < sp.c1 {
				a.alignRun(alignKeys[sp.key])
				return
			}
		}
		return // a click on the window's empty space does nothing
	}
	a.alignRun("")
}

func (a *app) renderAlign() {
	x0, y0, w, h := a.alignBox()
	a.frame(x0, y0, w, h, " Align and spread out ")
	keyst := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorYellow)
	dim := tcell.StyleDefault.Dim(true)
	for r, line := range alignLines {
		st := tcell.StyleDefault
		if r == 0 || r == 5 {
			st = st.Bold(true)
		}
		a.text(x0+2, y0+2+r, line, st)
	}
	for _, sp := range alignSpots { // the keys stand out
		row := []rune(alignLines[sp.r0])
		for c := sp.c0; c < min(sp.c1, len(row)); c++ {
			if string(row[c]) == sp.key {
				a.text(x0+2+c, y0+2+sp.r0, sp.key, keyst)
				break
			}
		}
	}
	a.text(x0+2, y0+h-2, alignFoot, dim)
}

var alignNames = map[string]string{
	"h": "left edges", "l": "right edges", "k": "top edges", "j": "bottom edges",
	"c": "centers, in a column", "m": "middles, in a row",
}

// align lines the blocks up on the outermost edge, or on the center of the
// box around them all.
func (a *app) align(how string) {
	bs := a.blocks()
	if len(bs) < 2 {
		a.sayErr("Select two or more shapes or groups to line up.")
		return
	}
	var all []int
	for _, b := range bs {
		all = append(all, b.idx...)
	}
	x0, y0, w, h := selBounds(a.objs, all)
	before := cloneObjs(a.objs)
	for _, b := range bs {
		dx, dy := 0, 0
		switch how {
		case "h":
			dx = x0 - b.x
		case "l":
			dx = (x0 + w) - (b.x + b.w)
		case "k":
			dy = y0 - b.y
		case "j":
			dy = (y0 + h) - (b.y + b.h)
		case "c":
			dx = (x0 + (w-1)/2) - (b.x + (b.w-1)/2)
		case "m":
			dy = (y0 + (h-1)/2) - (b.y + (b.h-1)/2)
		}
		a.moveBlock(b, dx, dy)
	}
	a.finishMove(before, fmt.Sprintf("Lined up %d by their %s.", len(bs), alignNames[how]), "Already lined up.")
}

// spread keeps the first and last blocks where they are and spaces the
// rest so the gaps between them are equal, or with centers set the steps
// from one center to the next. Whole cells don't always divide evenly;
// then they differ by one.
func (a *app) spread(across, centers bool) {
	bs := a.blocks()
	if len(bs) < 3 {
		a.sayErr("Select three or more shapes or groups to spread out.")
		return
	}
	pos := func(b block) (int, int) { // start and size along the axis
		if across {
			return b.x, b.w
		}
		return b.y, b.h
	}
	sort.SliceStable(bs, func(i, j int) bool {
		pi, _ := pos(bs[i])
		pj, _ := pos(bs[j])
		return pi < pj
	})
	n := len(bs)
	if centers {
		sort.SliceStable(bs, func(i, j int) bool {
			pi, si := pos(bs[i])
			pj, sj := pos(bs[j])
			return pi+(si-1)/2 < pj+(sj-1)/2
		})
		mid := func(b block) int { p, s := pos(b); return p + (s-1)/2 }
		c0, c1 := mid(bs[0]), mid(bs[n-1])
		before := cloneObjs(a.objs)
		for k, b := range bs {
			d := c0 + floorDiv((c1-c0)*k, n-1) - mid(b)
			if across {
				a.moveBlock(b, d, 0)
			} else {
				a.moveBlock(b, 0, d)
			}
		}
		dir := "left to right"
		if !across {
			dir = "top to bottom"
		}
		a.finishMove(before, fmt.Sprintf("Spaced the centers of %d evenly, %s.", n, dir), "Already evenly spaced.")
		return
	}
	first, _ := pos(bs[0])
	lastStart, lastSize := pos(bs[n-1])
	sizes := 0
	for _, b := range bs {
		_, s := pos(b)
		sizes += s
	}
	room := lastStart + lastSize - first - sizes // all the gaps together
	before := cloneObjs(a.objs)
	at := first
	for k, b := range bs {
		start, size := pos(b)
		want := at + floorDiv(room*k, n-1)
		if across {
			a.moveBlock(b, want-start, 0)
		} else {
			a.moveBlock(b, 0, want-start)
		}
		at += size
	}
	dir := "left to right"
	if !across {
		dir = "top to bottom"
	}
	a.finishMove(before, fmt.Sprintf("Spaced %d with even gaps, %s.", n, dir), "Already evenly spaced.")
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func (a *app) moveBlock(b block, dx, dy int) {
	for _, i := range b.idx {
		a.objs[i].move(dx, dy)
	}
}

// finishMove lets moved lines find what their ends touch now, as a nudge
// does, and records one undo step.
func (a *app) finishMove(before []object, msg, same string) {
	for _, i := range a.selected() {
		if isLine(a.objs[i].Kind) && !attached(before[i]) {
			a.detect(i)
		}
	}
	if !a.commit(before) {
		a.say(same)
		return
	}
	a.say(msg)
}
