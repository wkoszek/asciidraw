package main

// Lines and arrows can attach to shapes. An end that sits on a shape's edge,
// or in the cell just outside it, records which shape, which side, how far
// along that side, and whether it sits on the edge (gap 0) or just outside
// (gap 1). When the shape moves or changes size, the end follows.
//
// Shapes are named by a stable ID, not by their place in the list, since
// that changes whenever something is deleted.

type anchor struct {
	ID   int  // the shape attached to; 0 means not attached
	Side byte // 'l', 'r', 't' or 'b'
	At   int  // cells along the side from its top or left end
	Gap  int  // 0 on the edge, 1 just outside it
}

func isLine(kind string) bool { return kind == "line" || kind == "arrow" }

// canTake says whether lines can attach to a shape of this kind.
func canTake(o object) bool { return o.boxy() && o.Kind != "fill" }

// ensureIDs gives every shape without an ID a new one.
func (a *app) ensureIDs() {
	next := 1
	for _, o := range a.objs {
		next = max(next, o.ID+1)
	}
	for i := range a.objs {
		if a.objs[i].ID == 0 {
			a.objs[i].ID = next
			next++
		}
	}
}

func (a *app) indexByID() map[int]int {
	m := map[int]int{}
	for i, o := range a.objs {
		if o.ID != 0 {
			m[o.ID] = i
		}
	}
	return m
}

// anchorAt finds the topmost shape, other than self, whose edge touches
// (x, y), and how.
func anchorAt(objs []object, self, x, y int) (int, anchor, bool) {
	for i := len(objs) - 1; i >= 0; i-- {
		o := objs[i]
		if i == self || !canTake(o) {
			continue
		}
		bx, by, bw, bh := o.bounds()
		x1, y1 := bx+bw-1, by+bh-1
		inX, inY := x >= bx && x <= x1, y >= by && y <= y1
		switch {
		case inY && (x == bx-1 || x == bx):
			return i, anchor{o.ID, 'l', y - by, bx - x}, true
		case inY && (x == x1+1 || x == x1):
			return i, anchor{o.ID, 'r', y - by, x - x1}, true
		case inX && (y == by-1 || y == by):
			return i, anchor{o.ID, 't', x - bx, by - y}, true
		case inX && (y == y1+1 || y == y1):
			return i, anchor{o.ID, 'b', x - bx, y - y1}, true
		}
	}
	return -1, anchor{}, false
}

// anchorPoint is where an attached end sits on its shape right now.
func anchorPoint(target object, an anchor) (int, int) {
	bx, by, bw, bh := target.bounds()
	clamp := func(v, n int) int { return min(max(v, 0), max(n-1, 0)) }
	switch an.Side {
	case 'l':
		return bx - an.Gap, by + clamp(an.At, bh)
	case 'r':
		return bx + bw - 1 + an.Gap, by + clamp(an.At, bh)
	case 't':
		return bx + clamp(an.At, bw), by - an.Gap
	default:
		return bx + clamp(an.At, bw), by + bh - 1 + an.Gap
	}
}

// detect works out what line i's two ends are attached to, from where
// they are now.
func (a *app) detect(i int) {
	a.ensureIDs()
	o := &a.objs[i]
	if !isLine(o.Kind) {
		return
	}
	_, o.From, _ = anchorAt(a.objs, i, o.X0, o.Y0)
	_, o.To, _ = anchorAt(a.objs, i, o.X1, o.Y1)
}

// reattach moves every attached end to its shape. Lines in skip are being
// dragged and keep their place. An end whose shape is gone lets go.
func (a *app) reattach(skip map[int]bool) {
	a.ensureIDs()
	byID := a.indexByID()
	for i := range a.objs {
		o := &a.objs[i]
		if !isLine(o.Kind) || skip[i] {
			continue
		}
		if o.From.ID != 0 {
			if j, ok := byID[o.From.ID]; ok && j != i {
				o.X0, o.Y0 = anchorPoint(a.objs[j], o.From)
			} else {
				o.From = anchor{}
			}
		}
		if o.To.ID != 0 {
			if j, ok := byID[o.To.ID]; ok && j != i {
				o.X1, o.Y1 = anchorPoint(a.objs[j], o.To)
			} else {
				o.To = anchor{}
			}
		}
	}
}

// uGap is how far a U reaches past the end nearest its turn.
const uGap = 2

// route lists the corners a line runs through. An end attached to a left or
// right side leaves or arrives sideways; one attached to a top or bottom
// leaves or arrives up or down. Side to side gets a Z with the bend halfway
// across; top to bottom gets the Z turned on its side. Two ends on the same
// kind of side, such as two tops, get a U that clears the outer end by
// uGap cells.
func (o object) route() [][2]int {
	p0, p1 := [2]int{o.X0, o.Y0}, [2]int{o.X1, o.Y1}
	sideways := func(an anchor) bool { return an.ID != 0 && (an.Side == 'l' || an.Side == 'r') }
	upDown := func(an anchor) bool { return an.ID != 0 && (an.Side == 't' || an.Side == 'b') }
	same := o.From.ID != 0 && o.To.ID != 0 && o.From.Side == o.To.Side
	switch {
	case same && o.From.Side == 't':
		y := max(min(o.Y0, o.Y1)-uGap, 0)
		return [][2]int{p0, {o.X0, y}, {o.X1, y}, p1}
	case same && o.From.Side == 'b':
		y := max(o.Y0, o.Y1) + uGap
		return [][2]int{p0, {o.X0, y}, {o.X1, y}, p1}
	case same && o.From.Side == 'l':
		x := max(min(o.X0, o.X1)-uGap, 0)
		return [][2]int{p0, {x, o.Y0}, {x, o.Y1}, p1}
	case same && o.From.Side == 'r':
		x := max(o.X0, o.X1) + uGap
		return [][2]int{p0, {x, o.Y0}, {x, o.Y1}, p1}
	case sideways(o.From) && sideways(o.To):
		mx := (o.X0 + o.X1) / 2
		return [][2]int{p0, {mx, o.Y0}, {mx, o.Y1}, p1}
	case upDown(o.From) && upDown(o.To):
		my := (o.Y0 + o.Y1) / 2
		return [][2]int{p0, {o.X0, my}, {o.X1, my}, p1}
	case upDown(o.From) || sideways(o.To):
		return [][2]int{p0, {o.X0, o.Y1}, p1} // up or down first, then across
	default:
		return [][2]int{p0, {o.X1, o.Y0}, p1} // across first, then up or down
	}
}

// drawPath draws straight runs between the points, turning a corner
// wherever the path turns. Each cell gets arms toward the cells before and
// after it, so the ends only point back along the line, and a line that
// stops on a box edge joins it as a tee. start and end put a head on the
// first or the last cell.
func drawPath(c *canvas, pts [][2]int, st boxStyle, start, end bool) {
	sign := func(v int) int {
		switch {
		case v > 0:
			return 1
		case v < 0:
			return -1
		}
		return 0
	}
	// Every cell along the path, in order.
	var cells [][2]int
	for k, q := range pts {
		if k == 0 {
			cells = append(cells, q)
			continue
		}
		p := cells[len(cells)-1]
		dx, dy := sign(q[0]-p[0]), sign(q[1]-p[1])
		for p != q {
			p = [2]int{p[0] + dx, p[1] + dy}
			cells = append(cells, p)
		}
	}
	toward := func(from, to [2]int) uint8 {
		switch {
		case to[0] > from[0]:
			return armR
		case to[0] < from[0]:
			return armL
		case to[1] > from[1]:
			return armD
		case to[1] < from[1]:
			return armU
		}
		return 0
	}
	for k, p := range cells {
		var arms uint8
		if k > 0 {
			arms |= toward(p, cells[k-1])
		}
		if k+1 < len(cells) {
			arms |= toward(p, cells[k+1])
		}
		if arms == 0 { // a path one cell long
			arms = armL | armR
		}
		c.line(p[0], p[1], arms, st)
	}
	// A head points out of the line: from the cell before it to its own.
	head := func(p, prev [2]int, alone rune) {
		r := alone
		switch toward(prev, p) {
		case armR:
			r = '>'
		case armL:
			r = '<'
		case armD:
			r = 'v'
		case armU:
			r = '^'
		}
		c.set(p[0], p[1], r)
	}
	n := len(cells)
	if end && n > 0 {
		head(cells[n-1], cells[max(n-2, 0)], '>')
	}
	if start && n > 1 {
		head(cells[0], cells[1], '<')
	} else if start && n == 1 && !end {
		c.set(cells[0][0], cells[0][1], '<')
	}
}

// shapeUnder finds the topmost shape, other than self, that lines can
// attach to and whose bounds hold (x, y), or -1.
func shapeUnder(objs []object, self, x, y int) int {
	for i := len(objs) - 1; i >= 0; i-- {
		o := objs[i]
		if i == self || !canTake(o) {
			continue
		}
		bx, by, bw, bh := o.bounds()
		if x >= bx && x < bx+bw && y >= by && y < by+bh {
			return i
		}
	}
	return -1
}

// facing is the middle of the side of o that faces (ax, ay). Each offset
// from the centre is measured against the shape's own width or height, so
// a wide box gets its left or right side unless the point is well above
// or below it.
func facing(o object, ax, ay, gap int) anchor {
	bx, by, bw, bh := o.bounds()
	dx, dy := 2*ax-(2*bx+bw-1), 2*ay-(2*by+bh-1) // doubled, so odd sizes stay whole
	an := anchor{ID: o.ID, Gap: gap}
	switch {
	case abs(dx)*bh >= abs(dy)*bw && dx < 0:
		an.Side, an.At = 'l', bh/2
	case abs(dx)*bh >= abs(dy)*bw:
		an.Side, an.At = 'r', bh/2
	case dy < 0:
		an.Side, an.At = 't', bw/2
	default:
		an.Side, an.At = 'b', bw/2
	}
	return an
}

func centre(o object) (int, int) {
	bx, by, bw, bh := o.bounds()
	return bx + bw/2, by + bh/2
}

// connect attaches line o's ends to the shapes they touch. An end on a
// shape's edge, or just outside it, attaches where it is. An end that
// inner allows and that sits further inside a shape moves to the middle of
// the side facing the other end. An end inside the shape the other end is
// on stays loose, so a line can still be drawn inside a box. It returns the
// shapes the two ends attach to, or -1.
func (a *app) connect(o *object, self int, inner [2]bool) (int, int) {
	a.ensureIDs()
	pts := [2][2]int{{o.X0, o.Y0}, {o.X1, o.Y1}}
	var at [2]int
	var an [2]anchor
	var edge [2]bool
	for k, p := range pts {
		at[k], an[k], edge[k] = anchorAt(a.objs, self, p[0], p[1])
		if !edge[k] {
			at[k] = -1
			if inner[k] {
				at[k] = shapeUnder(a.objs, self, p[0], p[1])
			}
		}
	}
	if at[0] >= 0 && at[0] == at[1] {
		for k := range at {
			if !edge[k] {
				at[k], an[k] = -1, anchor{}
			}
		}
	}
	// The point an inside end aims at: the other end if it is loose or on
	// an edge, or the middle of the shape the other end is inside.
	aim := func(k int) (int, int) {
		j := 1 - k
		if at[j] >= 0 && !edge[j] {
			return centre(a.objs[at[j]])
		}
		return pts[j][0], pts[j][1]
	}
	for k := range pts {
		if at[k] < 0 || edge[k] {
			continue
		}
		gap := 0 // a tee on the edge
		if o.headAt(k == 1) {
			gap = 1 // a head sits just outside, where you can see it
		}
		ax, ay := aim(k)
		an[k] = facing(a.objs[at[k]], ax, ay, gap)
	}
	for k := range pts { // move inside ends out to their sides
		if at[k] >= 0 && !edge[k] {
			pts[k][0], pts[k][1] = anchorPoint(a.objs[at[k]], an[k])
		}
	}
	o.X0, o.Y0, o.X1, o.Y1 = pts[0][0], pts[0][1], pts[1][0], pts[1][1]
	o.From, o.To = an[0], an[1]
	return at[0], at[1]
}
