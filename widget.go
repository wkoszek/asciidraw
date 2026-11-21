package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Widgets are ready-made UI pieces. Each is one object, sized by its
// rectangle like a box. Text holds the label (tab names for tabs); Value
// holds the state: checked, the active tab, or a percent.

type widgetDef struct {
	name  string   // shown in the panel
	label []string // label a new widget starts with
	value int
	w, h  int    // size when stamped with a click; 0 means "as small as it goes"
	edit  string // prompt for Enter
}

var widgets = map[string]widgetDef{
	"button":    {"Button", []string{"OK"}, 0, 0, 0, "Label: "},
	"checkbox":  {"Check", []string{"Remember me"}, 1, 0, 0, "Label: "},
	"radio":     {"Radio", []string{"Option"}, 1, 0, 0, "Label: "},
	"input":     {"Input", []string{""}, 0, 20, 1, "Text: "},
	"dropdown":  {"Drop", []string{"Choose"}, 0, 16, 1, "Label: "},
	"tabs":      {"Tabs", []string{"Home", "Settings", "About"}, 0, 0, 0, "Tabs, separated by commas: "},
	"scrollbar": {"Scroll", nil, 0, 1, 8, "Percent (0-100): "},
	"progress":  {"Prog", nil, 40, 20, 1, "Percent (0-100): "},
	"window":    {"Window", []string{"Window"}, 0, 30, 8, "Title: "},
}

func isWidget(kind string) bool { _, ok := widgets[kind]; return ok }

func runeLen(s string) int { return len([]rune(s)) }

func (o object) label0() string {
	if len(o.Text) == 0 {
		return ""
	}
	return o.Text[0]
}

// tabsWidth is the width of the tab row: "│ Home │ Settings │".
func tabsWidth(names []string) int {
	w := 1
	for _, n := range names {
		w += runeLen(n) + 3
	}
	return w
}

// fixSize keeps a widget's rectangle to a shape that widget can have, and
// its value in range. The top-left corner never moves.
func (o *object) fixSize() {
	w, h, lab := o.X1-o.X0+1, o.Y1-o.Y0+1, runeLen(o.label0())
	clamp := func(v, lo, hi int) int { return min(max(v, lo), hi) }
	switch o.Kind {
	case "button":
		if h < 3 {
			o.Y1 = o.Y0
		}
		o.X1 = max(o.X1, o.X0+lab+3)
	case "checkbox", "radio":
		o.Y1, o.X1 = o.Y0, o.X0+lab+3
		o.Value = clamp(o.Value, 0, 1)
	case "input":
		o.Y1, o.X1 = o.Y0, max(o.X1, o.X0+lab+1)
	case "dropdown":
		o.Y1, o.X1 = o.Y0, max(o.X1, o.X0+lab+5)
	case "tabs":
		o.Y1, o.X1 = o.Y0+1, max(o.X1, o.X0+tabsWidth(o.Text)-1)
		o.Value = clamp(o.Value, 0, max(len(o.Text)-1, 0))
	case "progress":
		o.Y1, o.X1 = o.Y0, max(o.X1, o.X0+11)
		o.Value = clamp(o.Value, 0, 100)
	case "scrollbar":
		if w > 1 && w-1 >= 2*(h-1) { // wider than tall on screen: horizontal
			o.Y1, o.X1 = o.Y0, max(o.X1, o.X0+2)
		} else {
			o.X1, o.Y1 = o.X0, max(o.Y1, o.Y0+2)
		}
		o.Value = clamp(o.Value, 0, 100)
	case "window":
		o.X1, o.Y1 = max(o.X1, o.X0+lab+7), max(o.Y1, o.Y0+3)
	}
}

func center(s string, w int) string {
	pad := max(w-runeLen(s), 0)
	return strings.Repeat(" ", pad/2) + s + strings.Repeat(" ", pad-pad/2)
}

func padRight(s string, w int) string {
	return s + strings.Repeat(" ", max(w-runeLen(s), 0))
}

// glyphs a widget uses besides the box pieces: plain ASCII, or block and
// geometric characters for the other styles.
type glyphSet struct {
	dot, drop, fill, empty, thumb, trackV, trackH rune
	up, down, left, right                         rune
}

func glyphs(st boxStyle) glyphSet {
	if st.name == "ascii" {
		return glyphSet{'*', 'v', '#', '.', '#', '|', '-', '^', 'v', '<', '>'}
	}
	return glyphSet{'•', '▼', '█', '░', '█', '░', '░', '▲', '▼', '◀', '▶'}
}

func drawWidget(c *canvas, o object, st boxStyle) {
	g := glyphs(st)
	x0, y0, x1, y1 := o.X0, o.Y0, o.X1, o.Y1
	w, lab := x1-x0+1, o.label0()
	row := func(x, y int, s string) { // widgets are opaque: spaces count
		for i, r := range []rune(s) {
			c.set(x+i, y, r)
		}
	}
	switch o.Kind {
	case "button":
		if y1 == y0 {
			row(x0, y0, "[ "+center(lab, w-4)+" ]")
			return
		}
		drawBox(c, x0, y0, x1, y1, st)
		for y := y0 + 1; y < y1; y++ {
			row(x0+1, y, strings.Repeat(" ", w-2))
		}
		row(x0+1, (y0+y1)/2, center(lab, w-2))
	case "checkbox":
		mark := " "
		if o.Value != 0 {
			mark = "x"
		}
		row(x0, y0, "["+mark+"] "+lab)
	case "radio":
		mark := " "
		if o.Value != 0 {
			mark = string(g.dot)
		}
		row(x0, y0, "("+mark+") "+lab)
	case "input":
		row(x0, y0, "["+lab+strings.Repeat("_", max(w-2-runeLen(lab), 0))+"]")
	case "dropdown":
		row(x0, y0, "[ "+padRight(lab, w-6)+" "+string(g.drop)+" ]")
	case "tabs":
		drawTabs(c, o, st, row)
	case "progress":
		bar := w - 7
		n := bar * o.Value / 100
		row(x0, y0, "["+strings.Repeat(string(g.fill), n)+strings.Repeat(string(g.empty), bar-n)+"]"+fmt.Sprintf("%4d%%", o.Value))
	case "scrollbar":
		drawScrollbar(c, o, g)
	case "window":
		drawBox(c, x0, y0, x1, y1, st)
		row(x0+1, y0+1, " "+padRight(lab, w-6)+" x ")
		c.set(x0, y0+2, st.lt)
		for x := x0 + 1; x < x1; x++ {
			c.set(x, y0+2, st.h)
		}
		c.set(x1, y0+2, st.rt)
		// The body stays see-through, so you can draw inside the window.
	}
}

// drawTabs draws the names on one row and a line under them, open below
// the active tab:
//
//	│ Home │ Settings │ About │
//	└──────┘          └───────┘
func drawTabs(c *canvas, o object, st boxStyle, row func(int, int, string)) {
	x := o.X0
	seps := []int{x}
	c.set(x, o.Y0, st.v)
	for _, n := range o.Text {
		row(x+1, o.Y0, " "+n+" ")
		x += runeLen(n) + 3
		c.set(x, o.Y0, st.v)
		seps = append(seps, x)
	}
	for xx := o.X0; xx <= o.X1; xx++ {
		c.set(xx, o.Y0+1, st.h)
	}
	last := len(seps) - 1
	lineEnds := seps[last] == o.X1 // no line runs on past the last tab
	for i, sx := range seps {
		var r rune
		switch {
		case i == o.Value: // left wall of the active tab
			r = st.br
			if i == 0 {
				r = st.v
			}
		case i == o.Value+1: // right wall of the active tab
			r = st.bl
			if i == last && lineEnds {
				r = st.v
			}
		case i == 0:
			r = st.bl
		case i == last && lineEnds:
			r = st.br
		default:
			r = st.bt
		}
		c.set(sx, o.Y0+1, r)
	}
	if o.Value < len(seps)-1 {
		for xx := seps[o.Value] + 1; xx < seps[o.Value+1]; xx++ {
			c.set(xx, o.Y0+1, ' ')
		}
	}
}

func drawScrollbar(c *canvas, o object, g glyphSet) {
	vertical := o.X0 == o.X1 && o.Y1 > o.Y0
	n, first, last, track := o.X1-o.X0+1, g.left, g.right, g.trackH
	if vertical {
		n, first, last, track = o.Y1-o.Y0+1, g.up, g.down, g.trackV
	}
	at := func(i int, r rune) {
		if vertical {
			c.set(o.X0, o.Y0+i, r)
		} else {
			c.set(o.X0+i, o.Y0, r)
		}
	}
	inner := n - 2
	thumb := max(1, inner/4)
	pos := (inner - thumb) * o.Value / 100
	at(0, first)
	for i := 0; i < inner; i++ {
		r := track
		if i >= pos && i < pos+thumb {
			r = g.thumb
		}
		at(i+1, r)
	}
	at(n-1, last)
}

// ---- editing ----

// editWidget asks for a new label (or tab names, or percent) in the status bar.
func (a *app) editWidget() {
	i := a.sel
	o := a.objs[i]
	def := widgets[o.Kind]
	initial := strings.Join(o.Text, "")
	switch o.Kind {
	case "tabs":
		initial = strings.Join(o.Text, ", ")
	case "progress", "scrollbar":
		initial = strconv.Itoa(o.Value)
	}
	a.ask(def.edit, initial, func(s string) {
		if i >= len(a.objs) || a.objs[i].Kind != o.Kind {
			return
		}
		before := cloneObjs(a.objs)
		w := &a.objs[i]
		switch w.Kind {
		case "tabs":
			var names []string
			for _, n := range strings.Split(s, ",") {
				if n = strings.TrimSpace(n); n != "" {
					names = append(names, n)
				}
			}
			if len(names) == 0 {
				a.sayErr("Tabs need at least one name.")
				return
			}
			w.Text = names
		case "progress", "scrollbar":
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%")))
			if err != nil {
				a.sayErr("Type a number from 0 to 100.")
				return
			}
			w.Value = n
		default:
			w.Text = []string{s}
		}
		w.normalize()
		a.commit(before)
	})
}

// toggleWidget is Space on a selected widget: flip a checkbox or radio,
// move to the next tab, or add 10% to a bar.
func (a *app) toggleWidget() {
	if a.sel < 0 || a.sel >= len(a.objs) || !isWidget(a.objs[a.sel].Kind) {
		return
	}
	before := cloneObjs(a.objs)
	w := &a.objs[a.sel]
	switch w.Kind {
	case "checkbox", "radio":
		w.Value = 1 - w.Value
	case "tabs":
		w.Value = (w.Value + 1) % max(len(w.Text), 1)
	case "progress", "scrollbar":
		w.Value = (w.Value + 10) % 110
	default:
		a.say("Press Enter to change the label.")
		return
	}
	a.commit(before)
}
