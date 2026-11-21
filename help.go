package main

import "github.com/gdamore/tcell/v2"

// The ? screen: every key on one page. Any key or click closes it.

var helpLeft = []string{
	"Drawing",
	"  drag          draw with the tool",
	"  click         type; inside a box, its label",
	"  ⇧drag         square box, round oval",
	"  right-drag    delete what it touches",
	"  ^T            tools, styles, widgets",
	"",
	"Typing",
	"  ←→↑↓ Home End move the cursor",
	"  Enter         new line",
	"  Esc           stop typing",
	"",
	"Files",
	"  ^S            save the drawing (YAML)",
	"  ^E            export Markdown",
	"  ^N  ^Q        new drawing, quit",
	"  ^Z  ^Y        undo, redo",
}

var helpRight = []string{
	"Selecting",
	"  click  ⇧click select, add or remove",
	"  v             Select tool: drag a box",
	"  ^A  Esc  Tab  all, none, next",
	"  g             group or ungroup",
	"",
	"The selection",
	"  drag  ←→↑↓    move, nudge",
	"  y  d  p  x    copy, cut, paste, delete",
	"  ^D            duplicate",
	"  ]  [          to the front, to the back",
	"  t             next style",
	"  Enter  Space  edit, toggle a widget",
	"",
	"View",
	"  wheel  PgUp/Dn scroll; ⇧wheel sideways",
	"  ^O            show or hide the panel",
	"  ?             this screen",
}

func (a *app) renderHelp() {
	lw, rw := 0, 0
	for _, l := range helpLeft {
		lw = max(lw, runeLen(l))
	}
	for _, l := range helpRight {
		rw = max(rw, runeLen(l))
	}
	w := lw + 4 + rw + 4
	h := max(len(helpLeft), len(helpRight)) + 4
	sw, sh := a.s.Size()
	x0, y0 := max((sw-w)/2, 0), max((sh-h)/2, 0)
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
	a.text(x0+2, y0, " ? Keys ", frame.Bold(true))
	column := func(lines []string, x int) {
		for i, l := range lines {
			st := tcell.StyleDefault
			if l != "" && l[0] != ' ' {
				st = st.Bold(true).Foreground(tcell.ColorAqua)
			}
			a.text(x, y0+1+i, l, st)
		}
	}
	column(helpLeft, x0+2)
	column(helpRight, x0+2+lw+4)
	a.text(x0+2, y0+h-2, "Any key closes this.", tcell.StyleDefault.Dim(true))
}
