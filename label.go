package main

// Box and oval labels. A label lives in the shape's Text, so it moves,
// copies and deletes with the shape. It is drawn centered inside unless T
// put it somewhere else (HAlign, VAlign), and the shape grows to fit it.

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
	w := 0
	for _, l := range o.Text {
		w = max(w, runeLen(l))
	}
	return o.placeLine(k, runeLen(o.Text[k]), w, len(o.Text))
}

// labelArea is the rectangle a label can sit in when it isn't centered:
// inside the border with a blank column each side for a box, and the
// rectangle that fits inside the curve for an oval.
func (o object) labelArea() (ax0, ay0, ax1, ay1 int) {
	x0, x1 := order(o.X0, o.X1)
	y0, y1 := order(o.Y0, o.Y1)
	if o.Kind != "oval" {
		return x0 + 2, y0 + 1, x1 - 2, y1 - 1
	}
	// An ellipse's widest inside rectangle reaches 1/√2 of each half-axis.
	cx2, cy2 := x0+x1, y0+y1 // twice the center, to keep it in whole numbers
	hw, hh := (x1-x0)*707/2000, (y1-y0)*707/2000
	return (cx2+1)/2 - hw + 1, (cy2+1)/2 - hh, cx2/2 + hw - 1, cy2/2 + hh
}

// placeLine gives where line k starts, for a line lw wide in a label w
// wide and n lines tall. A centered label, or one too big for labelArea
// either way, sits in the middle of the inside, so it never crosses an
// oval's curve.
func (o object) placeLine(k, lw, w, n int) (int, int) {
	x0, x1 := order(o.X0, o.X1)
	y0, y1 := order(o.Y0, o.Y1)
	innerW, innerH := x1-x0-1, y1-y0-1
	x := x0 + 1 + max(innerW-lw, 0)/2
	y := y0 + 1 + max(innerH-n, 0)/2
	ax0, ay0, ax1, ay1 := o.labelArea()
	if w > ax1-ax0+1 || n > ay1-ay0+1 {
		return x, y + k
	}
	switch o.HAlign {
	case -1:
		x = ax0
	case 1:
		x = ax1 - lw + 1
	}
	switch o.VAlign {
	case -1:
		y = ay0
	case 1:
		y = ay1 - n + 1
	}
	return x, y + k
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
		if o.HAlign == 0 && o.VAlign == 0 {
			x0, x1 := order(o.X0, o.X1)
			y0, y1 := order(o.Y0, o.Y1)
			return (x0 + x1) / 2, (y0 + y1) / 2
		}
		return o.placeLine(0, 0, 0, 1)
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
