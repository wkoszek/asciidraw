package main

import "github.com/gdamore/tcell/v2"

// The ? screen: every key on one page. Any key or click closes it.

var helpLeft = []string{
	"Drawing",
	"  drag           draw with the tool",
	"  click          type, or box label",
	"  ⇧drag          square or circle",
	"  right-drag     delete shapes",
	"  ^T             tools and styles",
	"",
	"Typing",
	"  ←→↑↓ Home End  move the cursor",
	"  Enter  Esc     new line, stop",
	"",
	"Files",
	"  ^S  ^E         save, export .md",
	"  ^N  ^Q         new drawing, quit",
	"  ^Z  ^Y         undo, redo",
	"",
	"View",
	"  wheel  ⇧wheel  scroll, sideways",
	"  PgUp PgDn Home scroll, top left",
	"  ^O  ?          panel, this screen",
}

var helpRight = []string{
	"Selecting",
	"  click  ⇧click  select, add/remove",
	"  v              Select tool",
	"  ^A  Esc        all, none",
	"  Tab  ⇧Tab      next, previous",
	"  g              group or ungroup",
	"",
	"The selection",
	"  drag  ←→↑↓     move, nudge",
	"  y d p x        copy/cut/paste/del",
	"  ^D             duplicate",
	"  ]  [           front, back",
	"  t  T           style, label spot",
	"  C  a           colors, align",
	"  Enter          edit label or text",
	"  Space          toggle, next shade",
	"",
	"Pages",
	"  > .  / < ,     next / previous page",
	"  +              copy this page",
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
