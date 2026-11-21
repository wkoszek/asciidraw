package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// Copy and paste. y or ^C keeps the selected object, and also puts its
// picture on the system clipboard as plain text for other apps. p or ^V
// pastes the object, unless the system clipboard holds other text (copied
// from somewhere else since), which it pastes as text.

// The system clipboard. Variables so tests can swap them out.
var (
	clipWrite = func(s string) error {
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(s)
		return cmd.Run()
	}
	clipRead = func() (string, error) {
		out, err := exec.Command("pbpaste").Output()
		return string(out), err
	}
)

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.TrimRight(s, "\n")
}

func (a *app) copySel(cut bool) {
	if a.sel < 0 || a.sel >= len(a.objs) {
		a.sayErr("Select a shape first: click it, or press Tab.")
		return
	}
	a.stopTyping()
	o := a.objs[a.sel].clone()
	x, y, _, _ := o.bounds()
	o.move(-x, -y) // stored at 0,0; ^V moves it to the mouse
	a.clip = []object{o}
	a.clipText = cleanText(compose(a.clip).String())
	onSystem := clipWrite(a.clipText+"\n") == nil

	verb := "Copied"
	if cut {
		verb = "Cut"
		before := cloneObjs(a.objs)
		a.remove(a.sel)
		a.sel, a.hover = -1, -1
		a.commit(before)
	}
	msg := verb + " " + kindNames[o.Kind] + "."
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
	for _, o := range a.clip {
		o = o.clone()
		o.move(x, y)
		a.objs = append(a.objs, o)
	}
	a.sel = len(a.objs) - 1
	a.commit(before)
	a.say(fmt.Sprintf("Pasted %s at %d,%d.", kindNames[a.clip[0].Kind], x, y))
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
