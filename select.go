package main

import (
	"fmt"
	"sort"
)

// Selection. a.sel is the primary shape, or -1 for none. a.selset holds
// every selected shape, the primary included.
//
// Code that picks one shape just sets a.sel. syncSel notices the change and
// selects that shape's whole group. Code that builds a bigger selection
// (Shift-click, the Select tool, ^A, paste) calls setSelection.

func (a *app) syncSel() {
	if a.sel >= len(a.objs) {
		a.sel = -1
	}
	if a.sel != a.selFor || a.selset == nil {
		a.selFor = a.sel
		a.selset = map[int]bool{}
		if a.sel >= 0 {
			for _, i := range a.groupOf(a.sel) {
				a.selset[i] = true
			}
		}
	}
	for i := range a.selset {
		if i >= len(a.objs) {
			delete(a.selset, i)
		}
	}
}

func (a *app) setSelection(idx []int, primary int) {
	a.selset = map[int]bool{}
	for _, i := range idx {
		a.selset[i] = true
	}
	if len(idx) == 0 {
		primary = -1
	}
	a.sel, a.selFor = primary, primary
}

func (a *app) isSel(i int) bool {
	a.syncSel()
	return a.selset[i]
}

// selected lists the selected shapes in drawing order.
func (a *app) selected() []int {
	a.syncSel()
	out := make([]int, 0, len(a.selset))
	for i := range a.selset {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// groupOf lists the shapes in i's group, or just i if it has none.
func (a *app) groupOf(i int) []int {
	g := a.objs[i].Group
	if g == 0 {
		return []int{i}
	}
	var out []int
	for j, o := range a.objs {
		if o.Group == g {
			out = append(out, j)
		}
	}
	return out
}

// toggleSel adds i's group to the selection, or takes it out.
func (a *app) toggleSel(i int) {
	sel := a.selected()
	in := a.selset[i]
	members := map[int]bool{}
	for _, j := range a.groupOf(i) {
		members[j] = true
	}
	var next []int
	for _, j := range sel {
		if !members[j] {
			next = append(next, j)
		}
	}
	primary := i
	if in {
		primary = -1
		if len(next) > 0 {
			primary = next[len(next)-1]
		}
	} else {
		for j := range members {
			next = append(next, j)
		}
	}
	a.setSelection(next, primary)
}

func (a *app) selectAll() {
	all := make([]int, len(a.objs))
	for i := range all {
		all[i] = i
	}
	a.setSelection(all, len(all)-1)
	a.say(fmt.Sprintf("Selected all %d shapes.", len(all)))
}

// selectIn selects the shapes that sit wholly inside the rectangle, and the
// rest of any group they belong to. add keeps what was selected already.
func (a *app) selectIn(x0, y0, x1, y1 int, add bool) {
	x0, x1 = order(x0, x1)
	y0, y1 = order(y0, y1)
	pick := map[int]bool{}
	if add {
		for _, i := range a.selected() {
			pick[i] = true
		}
	}
	found := 0
	for i, o := range a.objs {
		x, y, w, h := o.bounds()
		if x >= x0 && y >= y0 && x+w-1 <= x1 && y+h-1 <= y1 {
			found++
			for _, j := range a.groupOf(i) {
				pick[j] = true
			}
		}
	}
	var idx []int
	for i := range pick {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	primary := -1
	if len(idx) > 0 {
		primary = idx[len(idx)-1]
	}
	a.setSelection(idx, primary)
	if found == 0 && !add {
		a.say("Nothing fits wholly inside that box.")
	} else {
		a.say(fmt.Sprintf("Selected %d shape(s).", len(idx)))
	}
}

// selBounds is the rectangle around the given shapes.
func selBounds(objs []object, idx []int) (x, y, w, h int) {
	if len(idx) == 0 {
		return 0, 0, 0, 0
	}
	x0, y0, x1, y1 := 1<<30, 1<<30, -1<<30, -1<<30
	for _, i := range idx {
		bx, by, bw, bh := objs[i].bounds()
		x0, y0 = min(x0, bx), min(y0, by)
		x1, y1 = max(x1, bx+bw-1), max(y1, by+bh-1)
	}
	return x0, y0, x1 - x0 + 1, y1 - y0 + 1
}

func (a *app) nextGroup() int {
	g := 0
	for _, o := range a.objs {
		g = max(g, o.Group)
	}
	return g + 1
}

// toggleGroup groups the selected shapes, or ungroups them if they are
// exactly one group already.
func (a *app) toggleGroup() {
	sel := a.selected()
	if len(sel) < 2 {
		a.sayErr("Select two or more shapes first: Shift-click them, or press v and drag a box around them.")
		return
	}
	before := cloneObjs(a.objs)
	g := a.objs[sel[0]].Group
	oneGroup := g != 0 && len(a.groupOf(sel[0])) == len(sel)
	for _, i := range sel {
		oneGroup = oneGroup && a.objs[i].Group == g
	}
	if oneGroup {
		for _, i := range sel {
			a.objs[i].Group = 0
		}
		a.say(fmt.Sprintf("Ungrouped %d shapes.", len(sel)))
	} else {
		id := a.nextGroup()
		for _, i := range sel {
			a.objs[i].Group = id
		}
		a.say(fmt.Sprintf("Grouped %d shapes. Click any of them to select the group.", len(sel)))
	}
	a.commit(before)
}

func (a *app) toggleSelectTool() {
	if a.tool == toolSelect {
		a.tool = a.lastTool
	} else {
		a.lastTool, a.tool = a.tool, toolSelect
	}
	a.say("Tool: " + tools[a.tool].name + ". v switches back.")
}
