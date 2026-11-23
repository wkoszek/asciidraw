package main

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
)

// Align and spread. Press a, then a second key. The selected shapes of each
// group count as one block, and so does each loose shape. A line with
// an end attached to a shape isn't a block: it follows its shape.

const alignHelp = "Align h:left l:right k:top j:bottom c:center m:middle  Spread x:across y:down"

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
	a.aligning = false
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
		how = string(ev.Rune())
	}
	switch how {
	case "h", "l", "k", "j", "c", "m":
		a.align(how)
	case "x", "y":
		a.spread(how == "x")
	default:
		a.say("Align cancelled.")
	}
}

var alignNames = map[string]string{
	"h": "left edges", "l": "right edges", "k": "top edges", "j": "bottom edges",
	"c": "centers, left to right", "m": "middles, top to bottom",
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
	a.finishMove(before, fmt.Sprintf("Lined up %d by their %s.", len(bs), alignNames[how]))
}

// spread keeps the first and last blocks where they are and spaces the
// rest so the gaps between them are equal. Whole cells don't always divide
// evenly; then the gaps differ by one.
func (a *app) spread(across bool) {
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
	a.finishMove(before, fmt.Sprintf("Spread %d out evenly, %s.", n, dir))
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
func (a *app) finishMove(before []object, msg string) {
	for _, i := range a.selected() {
		if isLine(a.objs[i].Kind) && !attached(before[i]) {
			a.detect(i)
		}
	}
	if !a.commit(before) {
		a.say("Already lined up.")
		return
	}
	a.say(msg)
}
