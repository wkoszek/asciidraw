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
func canTake(o object) bool { return o.boxy() }

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

// route lists the corners a line runs through. An end attached to a left or
// right side leaves or arrives sideways; one attached to a top or bottom
// leaves or arrives up or down. Side to side gets a Z with the bend halfway
// across; top to bottom gets the Z turned on its side.
func (o object) route() [][2]int {
	p0, p1 := [2]int{o.X0, o.Y0}, [2]int{o.X1, o.Y1}
	sideways := func(an anchor) bool { return an.ID != 0 && (an.Side == 'l' || an.Side == 'r') }
	upDown := func(an anchor) bool { return an.ID != 0 && (an.Side == 't' || an.Side == 'b') }
	switch {
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
// stops on a box edge joins it as a tee. arrow puts a head on the end.
func drawPath(c *canvas, pts [][2]int, st boxStyle, arrow bool) {
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
	if arrow && len(cells) > 0 {
		last := cells[len(cells)-1]
		head := '>'
		if len(cells) > 1 {
			switch toward(cells[len(cells)-2], last) {
			case armL:
				head = '<'
			case armD:
				head = 'v'
			case armU:
				head = '^'
			}
		}
		c.set(last[0], last[1], head)
	}
}
