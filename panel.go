package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// The object panel: a menu-like list on the right, one row per object.

const panelW = 32

var (
	colHover   = tcell.ColorAqua
	colSel     = tcell.ColorYellow
	colPressed = tcell.ColorHotPink
)

func (a *app) panelWidth() int {
	sw, _ := a.s.Size()
	if !a.showPanel || sw < 72 {
		return 0
	}
	return panelW
}

// panelRows is how many object rows fit: border, a blank line and four help lines.
func (a *app) panelRows() int {
	_, h := a.canvasSize()
	return max(h-7, 0)
}

func (a *app) inPanel(sx, sy int) bool {
	sw, _ := a.s.Size()
	_, h := a.canvasSize()
	pw := a.panelWidth()
	return pw > 0 && sx >= sw-pw && sy < h
}

// panelItemAt gives the object on the panel row at (sx, sy), or -1.
func (a *app) panelItemAt(sx, sy int) int {
	sw, _ := a.s.Size()
	if !a.inPanel(sx, sy) || sx == sw-a.panelWidth() || sx == sw-1 {
		return -1
	}
	row := sy - 1
	if row < 0 || row >= a.panelRows() {
		return -1
	}
	if i := a.panelTop + row; i < len(a.objs) {
		return i
	}
	return -1
}

func (a *app) scrollPanel(n int) {
	a.panelTop = min(max(a.panelTop+n, 0), max(len(a.objs)-a.panelRows(), 0))
}

func fit(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	for len(r) < n {
		r = append(r, ' ')
	}
	return string(r)
}

func (a *app) renderPanel() {
	pw := a.panelWidth()
	if pw == 0 {
		return
	}
	sw, _ := a.s.Size()
	_, h := a.canvasSize()
	x0, rows, inner := sw-pw, a.panelRows(), pw-2

	// Keep the selection in view.
	if a.sel >= 0 {
		if a.sel < a.panelTop {
			a.panelTop = a.sel
		} else if a.sel >= a.panelTop+rows {
			a.panelTop = a.sel - rows + 1
		}
	}
	a.scrollPanel(0)

	frame := tcell.StyleDefault.Foreground(tcell.ColorAqua)
	for y := 0; y < h; y++ {
		for x := x0; x < sw; x++ {
			a.s.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
	border := &canvas{}
	drawBox(border, x0, 0, sw-1, h-1, styles[2])
	for y := 0; y < h; y++ {
		for x := x0; x < sw; x++ {
			if r := border.get(x, y); r != ' ' {
				a.s.SetContent(x, y, r, nil, frame)
			}
		}
	}
	a.text(x0+2, 0, fmt.Sprintf(" Objects (%d) ", len(a.objs)), frame.Bold(true))
	if a.panelTop > 0 {
		a.s.SetContent(sw-3, 0, '▲', nil, frame)
	}
	if a.panelTop+rows < len(a.objs) {
		a.s.SetContent(sw-3, h-1, '▼', nil, frame)
	}

	dim := tcell.StyleDefault.Dim(true)
	if len(a.objs) == 0 {
		a.text(x0+1, 1, " Nothing drawn yet.", dim)
	}
	for r := 0; r < rows && a.panelTop+r < len(a.objs); r++ {
		i := a.panelTop + r
		st := tcell.StyleDefault
		switch {
		case a.pressed >= 0 && a.isSel(i):
			st = st.Background(colPressed).Foreground(tcell.ColorBlack).Bold(true)
		case a.isSel(i):
			st = st.Background(colSel).Foreground(tcell.ColorBlack)
		case i == a.hover || (a.hover >= 0 && a.objs[i].Group != 0 && a.objs[i].Group == a.objs[a.hover].Group):
			st = st.Background(tcell.PaletteColor(238)).Foreground(colHover)
		}
		tag := "   "
		if g := a.objs[i].Group; g != 0 {
			tag = fmt.Sprintf("g%-2d", g)
		}
		a.text(x0+1, 1+r, fit(fmt.Sprintf("%2d %s %s", i+1, tag, a.objs[i].label()), inner), st)
	}
	if h >= 8 {
		a.text(x0+1, h-5, " ⇧click add g group v select", dim)
		a.text(x0+1, h-4, " Tab next  x/Del delete", dim)
		a.text(x0+1, h-3, " y copy  d cut  p paste", dim)
		a.text(x0+1, h-2, " corner: resize  ←→↑↓ nudge", dim)
	}
}
