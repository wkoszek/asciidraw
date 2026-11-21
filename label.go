package main

// Box and oval labels. A label lives in the shape's Text, so it moves,
// copies and deletes with the shape. It is drawn centered inside, and the
// shape grows to fit it.

func hasLabel(kind string) bool { return kind == "box" || kind == "oval" }

// labelRoom is the space a label needs around it: border and a blank column
// each side for a box; more for an oval, whose curve eats the corners.
func labelRoom(kind string) (int, int) {
	if kind == "oval" {
		return 6, 2
	}
	return 4, 2
}

// labelStart gives where line k of the label starts.
func (o object) labelStart(k int) (int, int) {
	x0, x1 := order(o.X0, o.X1)
	y0, y1 := order(o.Y0, o.Y1)
	innerW, innerH := x1-x0-1, y1-y0-1
	top := y0 + 1 + max(innerH-len(o.Text), 0)/2
	return x0 + 1 + max(innerW-runeLen(o.Text[k]), 0)/2, top + k
}

func drawLabel(c *canvas, o object) {
	for k, line := range o.Text {
		x, y := o.labelStart(k)
		for j, r := range []rune(line) {
			if r != ' ' {
				c.set(x+j, y, r)
			}
		}
	}
}

// fitLabel grows the shape right and down until its label fits. It never
// shrinks it.
func (o *object) fitLabel() {
	if !hasLabel(o.Kind) || len(o.Text) == 0 {
		return
	}
	padW, padH := labelRoom(o.Kind)
	w := 0
	for _, l := range o.Text {
		w = max(w, runeLen(l))
	}
	o.X1 = max(o.X1, o.X0+w+padW-1)
	o.Y1 = max(o.Y1, o.Y0+len(o.Text)+padH-1)
}

// labelCursor is where the next typed letter of a label goes.
func (o object) labelCursor() (int, int) {
	if len(o.Text) == 0 {
		x0, x1 := order(o.X0, o.X1)
		y0, y1 := order(o.Y0, o.Y1)
		return (x0 + x1) / 2, (y0 + y1) / 2
	}
	k := len(o.Text) - 1
	x, y := o.labelStart(k)
	return x + runeLen(o.Text[k]), y
}

// inside says whether (x, y) is inside the shape, not on its border.
func (o object) inside(x, y int) bool {
	bx, by, bw, bh := o.bounds()
	return x > bx && x < bx+bw-1 && y > by && y < by+bh-1
}

// boxAt finds the topmost box or oval whose inside holds (x, y), or -1.
func boxAt(objs []object, x, y int) int {
	for i := len(objs) - 1; i >= 0; i-- {
		if hasLabel(objs[i].Kind) && objs[i].inside(x, y) {
			return i
		}
	}
	return -1
}
