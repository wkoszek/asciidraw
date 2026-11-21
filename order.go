package main

import "fmt"

// Stacking order. Shapes are drawn in list order, so a later shape covers
// an earlier one. ] brings the selection to the front, [ sends it to the
// back, and dragging a panel row moves it to another place in the list.

// restack moves the selected shapes, in their order, to the front or back.
func (a *app) restack(front bool) {
	sel := a.selected()
	if len(sel) == 0 {
		a.sayErr("Select a shape first: click it, or press Tab.")
		return
	}
	at := 0
	if front {
		at = len(a.objs)
	}
	a.moveSelTo(at)
	if front {
		a.say("Brought " + a.describe(a.selected()) + " to the front.")
	} else {
		a.say("Sent " + a.describe(a.selected()) + " to the back.")
	}
}

// moveSelTo moves the selected shapes so they sit just before the shape now
// at index at (or at the end, for len(objs)). Their order among themselves
// stays the same, and they stay selected.
func (a *app) moveSelTo(at int) {
	sel := a.selected()
	if len(sel) == 0 {
		return
	}
	a.stopTyping()
	a.ensureIDs()
	before := cloneObjs(a.objs)
	primaryID := -1
	if a.sel >= 0 && a.sel < len(a.objs) {
		primaryID = a.objs[a.sel].ID
	}
	var picked, rest []object
	insert := 0 // where, in rest, the picked shapes go
	for i, o := range a.objs {
		if i == at {
			insert = len(rest)
		}
		if a.selset[i] {
			picked = append(picked, o)
		} else {
			rest = append(rest, o)
		}
	}
	if at >= len(a.objs) {
		insert = len(rest)
	}
	a.objs = append(append(append([]object{}, rest[:insert]...), picked...), rest[insert:]...)
	var idx []int
	primary := insert + len(picked) - 1
	for k := range picked {
		i := insert + k
		idx = append(idx, i)
		if a.objs[i].ID == primaryID {
			primary = i
		}
	}
	a.setSelection(idx, primary)
	a.hover = -1
	a.commit(before)
}

// duplicate copies the selection in place, a little down and to the right,
// and selects the copies. The clipboard is left alone.
func (a *app) duplicate() {
	a.stopTyping()
	sel := a.selected()
	if len(sel) == 0 {
		a.sayErr("Select a shape first: click it, or press Tab.")
		return
	}
	before := cloneObjs(a.objs)
	var src []object
	for _, i := range sel {
		src = append(src, a.objs[i])
	}
	added := a.addCopies(src, 2, 1)
	a.setSelection(added, added[len(added)-1])
	a.commit(before)
	a.say(fmt.Sprintf("Duplicated %s.", a.describe(added)))
}
