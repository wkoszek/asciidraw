package main

import "strings"

// canvas is a grid of characters. It also records which object drew each
// cell, so the mouse can find the object under it.
type canvas struct {
	rows [][]rune
	own  [][]int // object index + 1; 0 means nothing
	pen  int     // owner recorded by set
}

func (c *canvas) get(x, y int) rune {
	if y < 0 || y >= len(c.rows) || x < 0 || x >= len(c.rows[y]) {
		return ' '
	}
	return c.rows[y][x]
}

// owner gives the index of the object that drew (x, y), or -1.
func (c *canvas) owner(x, y int) int {
	if y < 0 || y >= len(c.own) || x < 0 || x >= len(c.own[y]) {
		return -1
	}
	return c.own[y][x] - 1
}

func (c *canvas) set(x, y int, r rune) {
	if x < 0 || y < 0 {
		return
	}
	for len(c.rows) <= y {
		c.rows = append(c.rows, nil)
		c.own = append(c.own, nil)
	}
	for len(c.rows[y]) <= x {
		c.rows[y] = append(c.rows[y], ' ')
		c.own[y] = append(c.own[y], 0)
	}
	c.rows[y][x] = r
	c.own[y][x] = c.pen
}

// String trims trailing spaces on each line and trailing blank lines.
func (c *canvas) String() string {
	lines := make([]string, len(c.rows))
	for i, r := range c.rows {
		lines[i] = strings.TrimRight(string(r), " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// ---- styles and tools ----

type boxStyle struct {
	name                 string
	h, v, tl, tr, bl, br rune
	up, down             rune // diagonals for ovals: rising and falling
	lt, rt, bt           rune // tees: left side, right side, bottom
}

var styles = []boxStyle{
	{"ascii", '-', '|', '+', '+', '+', '+', '/', '\\', '+', '+', '+'},
	{"line", '─', '│', '┌', '┐', '└', '┘', '╱', '╲', '├', '┤', '┴'},
	{"round", '─', '│', '╭', '╮', '╰', '╯', '╱', '╲', '├', '┤', '┴'},
	{"double", '═', '║', '╔', '╗', '╚', '╝', '╱', '╲', '╠', '╣', '╩'},
}

// sample shows a style's pieces: a box top, a box bottom, a side, an arrow.
func (st boxStyle) sample() string {
	h := string(st.h)
	return string(st.tl) + h + h + string(st.tr) + " " +
		string(st.bl) + h + h + string(st.br) + " " +
		string(st.v) + " " + h + h + ">"
}

const (
	toolBox = iota
	toolOval
	toolLine
	toolArrow
	toolErase
	toolSelect
)

// shapeTools is how many of the tools are shapes; the rest are widgets.
const shapeTools = toolSelect + 1

var tools = []struct{ name, kind, help string }{
	{"Box", "box", "drag a box, Shift: square"},
	{"Oval", "oval", "drag an oval, Shift: circle"},
	{"Line", "line", "drag a line, bends once"},
	{"Arrow", "arrow", "line with a head"},
	{"Erase", "", "drag to delete objects"},
	{"Select", "", "drag a box to select (v)"},
	{"Button", "button", "[ OK ]"},
	{"Check", "checkbox", "[x] Remember me"},
	{"Radio", "radio", "(•) Option"},
	{"Input", "input", "[user______]"},
	{"Drop", "dropdown", "[ Choose  ▼ ]"},
	{"Tabs", "tabs", "│ Home │ Settings"},
	{"Scroll", "scrollbar", "▲ ░█░░ ▼"},
	{"Prog", "progress", "[████░░░░] 40%"},
	{"Window", "window", "title bar and body"},
}

func order(a, b int) (int, int) {
	if a > b {
		return b, a
	}
	return a, b
}

// ---- primitives ----

// drawBox draws a box, or a straight line when it is one row or column.
func drawBox(c *canvas, x0, y0, x1, y1 int, st boxStyle) {
	x0, x1 = order(x0, x1)
	y0, y1 = order(y0, y1)
	switch {
	case y0 == y1:
		for x := x0; x <= x1; x++ {
			c.set(x, y0, st.h)
		}
	case x0 == x1:
		for y := y0; y <= y1; y++ {
			c.set(x0, y, st.v)
		}
	default:
		for x := x0 + 1; x < x1; x++ {
			c.set(x, y0, st.h)
			c.set(x, y1, st.h)
		}
		for y := y0 + 1; y < y1; y++ {
			c.set(x0, y, st.v)
			c.set(x1, y, st.v)
		}
		c.set(x0, y0, st.tl)
		c.set(x1, y0, st.tr)
		c.set(x0, y1, st.bl)
		c.set(x1, y1, st.br)
	}
}

// drawLine runs across row y0, then down or up column x1, with a corner
// where the two meet. arrow puts a head on the end.
func drawLine(c *canvas, x0, y0, x1, y1 int, st boxStyle, arrow bool) {
	drawPath(c, [][2]int{{x0, y0}, {x1, y0}, {x1, y1}}, st, arrow)
}

// drawOval draws the oval that fits the rectangle. The outline is one cell
// thin, and each cell's character follows the line through its two
// neighbors on the outline.
func drawOval(c *canvas, x0, y0, x1, y1 int, st boxStyle) {
	x0, x1 = order(x0, x1)
	y0, y1 = order(y0, y1)
	if x1-x0 < 2 || y1-y0 < 1 {
		drawBox(c, x0, y0, x1, y1, st) // too small to curve
		return
	}
	pts := ellipsePoints(x0, y0, x1, y1)
	on := map[[2]int]bool{}
	for _, p := range pts {
		on[p] = true
	}
	for _, p := range pts {
		c.set(p[0], p[1], ovalChar(p, on, st))
	}
}

// ovalChar picks the character for one outline cell from its neighbors.
func ovalChar(p [2]int, on map[[2]int]bool, st boxStyle) rune {
	has := func(dx, dy int) bool { return on[[2]int{p[0] + dx, p[1] + dy}] }
	switch {
	case has(-1, 0) && has(1, 0):
		return st.h
	case has(0, -1) && has(0, 1):
		return st.v
	}
	var nb [][2]int
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx != 0 || dy != 0) && has(dx, dy) {
				nb = append(nb, [2]int{dx, dy})
			}
		}
	}
	// The two neighbors farthest apart give the direction of the line.
	vx, vy, best := 0, 0, -1
	for i := range nb {
		for j := i + 1; j < len(nb); j++ {
			dx, dy := nb[j][0]-nb[i][0], nb[j][1]-nb[i][1]
			if d := dx*dx + dy*dy; d > best {
				vx, vy, best = dx, dy, d
			}
		}
	}
	ax, ay := abs(vx), abs(vy)
	switch {
	case best < 0 || ax >= 2*ay:
		return st.h
	case ay >= 2*ax:
		return st.v
	case vx*vy > 0:
		return st.down // falls to the right: \
	default:
		return st.up // rises to the right: /
	}
}

// ellipsePoints is Alois Zingl's ellipse-in-a-rectangle rasterizer
// ("A Rasterizing Algorithm for Drawing Curves", 2012). It returns a thin,
// connected outline that is symmetric for any width and height.
func ellipsePoints(x0, y0, x1, y1 int) [][2]int {
	seen := map[[2]int]bool{}
	var pts [][2]int
	set := func(x, y int) {
		k := [2]int{x, y}
		if !seen[k] {
			seen[k] = true
			pts = append(pts, k)
		}
	}
	a, b := x1-x0, y1-y0
	b1 := b & 1
	dx := float64(4 * (1 - a) * b * b)
	dy := float64(4 * (b1 + 1) * a * a)
	err := dx + dy + float64(b1*a*a)
	y0 += (b + 1) / 2
	y1 = y0 - b1
	aa, bb := float64(8*a*a), float64(8*b*b)
	for x0 <= x1 {
		set(x1, y0)
		set(x0, y0)
		set(x0, y1)
		set(x1, y1)
		e2 := 2 * err
		if e2 <= dy { // step in y
			y0++
			y1--
			dy += aa
			err += dy
		}
		if e2 >= dx || 2*err > dy { // step in x
			x0++
			x1--
			dx += bb
			err += dx
		}
	}
	for y0-y1 < b { // flat ellipses stop early: finish the tips
		set(x0-1, y0)
		set(x1+1, y0)
		y0++
		set(x0-1, y1)
		set(x1+1, y1)
		y1--
	}
	return pts
}

// symmetric moves (x, y) so the rectangle from the anchor looks square on
// screen: twice as many columns as rows, since cells are about twice as tall
// as they are wide. It never goes past the top or left edge.
func symmetric(ax, ay, x, y int) (int, int) {
	sx, sy := 1, 1
	if x < ax {
		sx = -1
	}
	if y < ay {
		sy = -1
	}
	r := max((abs(x-ax)+1)/2, abs(y-ay))
	if sx < 0 {
		r = min(r, ax/2)
	}
	if sy < 0 {
		r = min(r, ay)
	}
	return ax + sx*2*r, ay + sy*r
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
