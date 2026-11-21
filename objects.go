package main

import (
	"fmt"
	"sort"
	"strings"
)

// object is one shape on the canvas. Boxes, ovals, lines, arrows and
// widgets use both points; text sits at (X0, Y0).
type object struct {
	Kind  string // "box", "line", "arrow", "text"
	X0    int
	Y0    int
	X1    int
	Y1    int
	Style int // index into styles
	Text  []string
	Value int    // widget state: checked, active tab, or percent
	Group int    // shapes with the same nonzero Group move and select together
	ID    int    // stable name, for lines to attach to; see connect.go
	From  anchor // what a line's start is attached to
	To    anchor // what a line's end is attached to
}

var kindNames = map[string]string{"box": "Box", "oval": "Oval", "line": "Line", "arrow": "Arrow", "text": "Text", "fill": "Fill"}

func init() {
	for k, d := range widgets {
		kindNames[k] = d.name
	}
}

// boxy kinds are sized by a rectangle and have four corner handles.
func (o object) boxy() bool {
	return o.Kind == "box" || o.Kind == "oval" || o.Kind == "fill" || isWidget(o.Kind)
}

// fillPatterns are the shades a fill cycles through with Space.
var fillPatterns = [][]rune{
	{'.', ':', '%', '#'}, // ascii
	{'░', '▒', '▓', '█'}, // the other styles
}

func (o object) fillRune() rune {
	set := fillPatterns[1]
	if o.Style == 0 {
		set = fillPatterns[0]
	}
	return set[min(max(o.Value, 0), len(set)-1)]
}

// squarable kinds turn square or round with Shift.
func (o object) squarable() bool { return o.Kind == "box" || o.Kind == "oval" }

func (o object) clone() object {
	if o.Text != nil {
		o.Text = append([]string(nil), o.Text...)
	}
	return o
}

func cloneObjs(objs []object) []object {
	if objs == nil {
		return nil
	}
	out := make([]object, len(objs))
	for i, o := range objs {
		out[i] = o.clone()
	}
	return out
}

func (o object) draw(c *canvas) {
	st := styles[min(max(o.Style, 0), len(styles)-1)]
	switch o.Kind {
	case "box":
		drawBox(c, o.X0, o.Y0, o.X1, o.Y1, st)
		drawLabel(c, o)
	case "oval":
		drawOval(c, o.X0, o.Y0, o.X1, o.Y1, st)
		drawLabel(c, o)
	case "fill":
		x0, x1 := order(o.X0, o.X1)
		y0, y1 := order(o.Y0, o.Y1)
		r := o.fillRune()
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				c.set(x, y, r)
			}
		}
	default:
		if isWidget(o.Kind) {
			drawWidget(c, o, st)
		}
	case "line", "arrow":
		drawPath(c, o.route(), st, o.Kind == "arrow")
	case "text":
		// Spaces are see-through, so text can sit on top of other shapes.
		for i, line := range o.Text {
			for j, r := range []rune(line) {
				if r != ' ' {
					c.set(o.X0+j, o.Y0+i, r)
				}
			}
		}
	}
}

// compose draws objects in order, later ones on top.
func compose(objs []object) *canvas {
	c := &canvas{}
	for i, o := range objs {
		c.pen = i + 1
		o.draw(c)
	}
	return c
}

func (o *object) move(dx, dy int) {
	o.X0, o.Y0, o.X1, o.Y1 = o.X0+dx, o.Y0+dy, o.X1+dx, o.Y1+dy
}

// normalize keeps a box's, oval's or widget's first point at its top-left,
// and a widget to a shape it can have.
func (o *object) normalize() {
	if o.boxy() {
		o.X0, o.X1 = order(o.X0, o.X1)
		o.Y0, o.Y1 = order(o.Y0, o.Y1)
	}
	o.fitLabel() // a box or oval is never smaller than its label
	if isWidget(o.Kind) {
		o.fixSize()
	}
}

func (o object) bounds() (x, y, w, h int) {
	if o.Kind == "text" {
		for _, l := range o.Text {
			w = max(w, len([]rune(l)))
		}
		return o.X0, o.Y0, max(w, 1), len(o.Text)
	}
	x0, x1 := order(o.X0, o.X1)
	y0, y1 := order(o.Y0, o.Y1)
	return x0, y0, x1 - x0 + 1, y1 - y0 + 1
}

// handle names a point that a drag moves: x1 picks X1 over X0, y1 Y1 over Y0.
type handle struct{ ok, x1, y1 bool }

// handleAt finds a resize handle at (x, y): a box or oval corner, or a
// line end.
func (o object) handleAt(x, y int) handle {
	switch o.Kind {
	case "box", "oval", "fill", "button", "checkbox", "radio", "input", "dropdown", "tabs", "scrollbar", "progress", "window":
		for _, hx := range []bool{false, true} {
			for _, hy := range []bool{false, true} {
				px, py := o.X0, o.Y0
				if hx {
					px = o.X1
				}
				if hy {
					py = o.Y1
				}
				if px == x && py == y {
					return handle{true, hx, hy}
				}
			}
		}
	case "line", "arrow":
		if x == o.X1 && y == o.Y1 {
			return handle{true, true, true}
		}
		if x == o.X0 && y == o.Y0 {
			return handle{true, false, false}
		}
	}
	return handle{}
}

func (o object) label() string {
	switch o.Kind {
	case "fill":
		x, y, w, h := o.bounds()
		return fmt.Sprintf("%-6s %c %d×%d at %d,%d", "Fill", o.fillRune(), w, h, x, y)
	case "box", "oval":
		x, y, w, h := o.bounds()
		if len(o.Text) > 0 {
			return fmt.Sprintf("%-6s \"%s\" %d×%d", kindNames[o.Kind], strings.Join(o.Text, " "), w, h)
		}
		return fmt.Sprintf("%-6s %d×%d at %d,%d", kindNames[o.Kind], w, h, x, y)
	case "line", "arrow":
		return fmt.Sprintf("%-6s %d,%d→%d,%d", kindNames[o.Kind], o.X0, o.Y0, o.X1, o.Y1)
	case "progress", "scrollbar":
		return fmt.Sprintf("%-6s %d%%", kindNames[o.Kind], o.Value)
	case "text":
		first := ""
		if len(o.Text) > 0 {
			first = strings.TrimSpace(o.Text[0])
		}
		if len(o.Text) > 1 {
			first += "…"
		}
		return "Text   \"" + first + "\""
	}
	if isWidget(o.Kind) {
		return fmt.Sprintf("%-6s \"%s\"", kindNames[o.Kind], strings.Join(o.Text, ", "))
	}
	return o.Kind
}

// objectsIn lists objects with a cell inside the rectangle, highest index first.
func objectsIn(objs []object, x0, y0, x1, y1 int) []int {
	x0, x1 = order(x0, x1)
	y0, y1 = order(y0, y1)
	c := compose(objs)
	seen := map[int]bool{}
	var out []int
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if i := c.owner(x, y); i >= 0 && !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}
