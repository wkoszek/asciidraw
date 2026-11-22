package main

import (
	"fmt"
	"strings"
)

// Copy and paste. y or ^C keeps the selected object, and also puts its
// picture on the system clipboard as plain text for other apps. p or ^V
// pastes the object, unless the system clipboard holds other text (copied
// from somewhere else since), which it pastes as text.

// The system clipboard; see sysclip.go. Variables so tests can swap them out.
var clipWrite, clipRead = pickClipboard()

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.TrimRight(s, "\n")
}

func (a *app) copySel(cut bool) {
	a.stopTyping()
	sel := a.selected()
	if len(sel) == 0 {
		a.sayErr("Select a shape first: click it, or press Tab.")
		return
	}
	x, y, _, _ := selBounds(a.objs, sel)
	a.clip = nil
	for _, i := range sel {
		o := a.objs[i].clone()
		o.move(-x, -y) // stored from 0,0; paste moves it to the mouse
		a.clip = append(a.clip, o)
	}
	a.clipText = cleanText(compose(a.clip).String())
	onSystem := clipWrite(a.clipText+"\n") == nil
	what := a.describe(sel)

	verb := "Copied"
	if cut {
		verb = "Cut"
		before := cloneObjs(a.objs)
		for k := len(sel) - 1; k >= 0; k-- {
			a.remove(sel[k])
		}
		a.sel, a.hover = -1, -1
		a.commit(before)
	}
	msg := verb + " " + what + "."
	if onSystem {
		msg += " Its picture is on the clipboard too."
	}
	a.say(msg + " p or ^V pastes at the mouse.")
}

func (a *app) paste() {
	text, err := clipRead()
	text = cleanText(text)
	if err == nil && text != "" && text != a.clipText {
		a.pasteText(text)
		return
	}
	if len(a.clip) == 0 {
		a.sayErr("Nothing to paste. Select a shape and press y or ^C first.")
		return
	}
	a.stopTyping()
	before := cloneObjs(a.objs)
	x, y := a.mx, a.my
	added := a.addCopies(a.clip, x, y)
	a.setSelection(added, added[len(added)-1])
	a.commit(before)
	a.say(fmt.Sprintf("Pasted %s at %d,%d.", a.describe(added), x, y))
}

// pasteText types text from the clipboard: into the text being typed, or
// as a new text object at the mouse.
func (a *app) pasteText(text string) {
	if !a.typing {
		a.typing, a.edit, a.typed = true, -1, false
		a.cx, a.cy = a.mx, a.my
		defer a.stopTyping()
	}
	for _, r := range text {
		if r == '\n' {
			a.newline()
		} else {
			a.typeRune(r)
		}
	}
	a.say(fmt.Sprintf("Pasted %d line(s) of text from the clipboard.", strings.Count(text, "\n")+1))
}

// addCopies appends copies of src, moved by dx, dy, and returns their
// indexes. Copied groups stay groups, but new ones, apart from the
// originals. Copies get new IDs; a copied line stays attached to a shape
// copied with it, and lets go of anything else.
func (a *app) addCopies(src []object, dx, dy int) []int {
	newGroup, next := map[int]int{}, a.nextGroup()
	a.ensureIDs()
	newID, nextID := map[int]int{}, 1
	for _, o := range a.objs {
		nextID = max(nextID, o.ID+1)
	}
	for _, o := range src {
		if o.ID != 0 {
			newID[o.ID], nextID = nextID, nextID+1
		}
	}
	var added []int
	for _, o := range src {
		o = o.clone()
		o.move(dx, dy)
		o.ID = newID[o.ID]
		o.From.ID, o.To.ID = newID[o.From.ID], newID[o.To.ID]
		if o.From.ID == 0 {
			o.From = anchor{}
		}
		if o.To.ID == 0 {
			o.To = anchor{}
		}
		if o.Group != 0 {
			if newGroup[o.Group] == 0 {
				newGroup[o.Group], next = next, next+1
			}
			o.Group = newGroup[o.Group]
		}
		a.objs = append(a.objs, o)
		added = append(added, len(a.objs)-1)
	}
	return added
}
