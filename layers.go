package main

import (
	"fmt"
	"slices"
	"strings"
)

// Layers. Each page has up to 62 layers, named like pages: 0-9, a-z, A-Z.
// Every shape sits on one, layer 0 unless moved. Higher layers draw on top
// of lower ones; within a layer, list order decides, as it always has.
//
// a.objs stays sorted by layer, so drawing, stacking and the panel need to
// know nothing about layers. sortLayers puts things right after a change.
//
// A hidden layer isn't drawn, can't be clicked and isn't exported. A
// locked layer is drawn and exported, but clicks go through it. Hiding and
// locking aren't undo steps; moving shapes to a layer is.

const maxLayers = len(pageNames)

// layerBit is a layer's bit in a page's hidden or locked set.
func layerBit(l int) uint64 { return 1 << uint(l) }

// layerOf reads a layer name, one of 0-9, a-z, A-Z.
func layerOf(r rune) (int, bool) {
	i := strings.IndexRune(pageNames, r)
	return i, i >= 0
}

// layerState is what a page keeps about its layers.
type layerState struct {
	hidden, locked uint64
	active         int // the layer new shapes go on
}

// layerNames writes a set as its names, such as "02a".
func layerNames(set uint64) string {
	var b strings.Builder
	for l := 0; l < maxLayers; l++ {
		if set&layerBit(l) != 0 {
			b.WriteString(pageChar(l))
		}
	}
	return b.String()
}

// parseLayerNames reads a set written by layerNames.
func parseLayerNames(s string) (uint64, error) {
	var set uint64
	for _, r := range s {
		l, ok := layerOf(r)
		if !ok {
			return 0, fmt.Errorf("%q is not a layer; layers are 0-9, a-z and A-Z", string(r))
		}
		set |= layerBit(l)
	}
	return set, nil
}

// composeLayers draws objs like compose, leaving out the layers in skip.
// Cells still record each shape's index in objs.
func composeLayers(objs []object, skip uint64) *canvas {
	c := &canvas{}
	for i, o := range objs {
		if skip&layerBit(o.Layer) != 0 {
			continue
		}
		c.pen, c.bg = i+1, o.BG
		o.draw(c)
	}
	return c
}

// visible leaves out the shapes on hidden layers, for export.
func visible(objs []object, hidden uint64) []object {
	if hidden == 0 {
		return objs
	}
	var out []object
	for _, o := range objs {
		if hidden&layerBit(o.Layer) == 0 {
			out = append(out, o)
		}
	}
	return out
}

// scene is the page as drawn; pickScene is what clicks can land on.
func (a *app) scene() *canvas     { return composeLayers(a.objs, a.lay.hidden) }
func (a *app) pickScene() *canvas { return composeLayers(a.objs, a.lay.hidden|a.lay.locked) }

// labelBoxAt is boxAt for clicks: boxes and ovals on hidden or locked
// layers don't count, so a click there can't start typing their label.
func (a *app) labelBoxAt(x, y int) int {
	for i := len(a.objs) - 1; i >= 0; i-- {
		if a.pickable(i) && hasLabel(a.objs[i].Kind) && a.objs[i].inside(x, y) {
			return i
		}
	}
	return -1
}

// pickable says whether a shape can be clicked, selected or erased.
func (a *app) pickable(i int) bool {
	return i >= 0 && i < len(a.objs) && (a.lay.hidden|a.lay.locked)&layerBit(a.objs[i].Layer) == 0
}

// sortLayers puts a.objs in layer order, keeping the order within each
// layer, and moves every index the app holds along with its shape.
func (a *app) sortLayers() {
	if slices.IsSortedFunc(a.objs, func(x, y object) int { return x.Layer - y.Layer }) {
		return
	}
	idx := make([]int, len(a.objs))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(x, y int) int { return a.objs[x].Layer - a.objs[y].Layer })
	to := make([]int, len(idx)) // old index → new
	objs := make([]object, len(idx))
	for n, o := range idx {
		to[o] = n
		objs[n] = a.objs[o]
	}
	a.objs = objs
	re := func(i int) int {
		if i >= 0 && i < len(to) {
			return to[i]
		}
		return i
	}
	set := map[int]bool{}
	for i := range a.selset {
		set[re(i)] = true
	}
	a.selset = set
	a.sel, a.selFor = re(a.sel), re(a.selFor)
	a.hover, a.pressed, a.edit, a.snap = re(a.hover), re(a.pressed), re(a.edit), re(a.snap)
}

// pageLayers gives every page's layer state, the one on screen included.
func (a *app) pageLayers() []layerState {
	out := make([]layerState, len(a.pages))
	for i, p := range a.pages {
		out[i] = p.lay
	}
	out[a.page] = a.lay
	return out
}

// visiblePages is every page as export sees it: hidden layers left out.
func (a *app) visiblePages() [][]object {
	pages, lays := a.allPages(), a.pageLayers()
	for i := range pages {
		pages[i] = visible(pages[i], lays[i].hidden)
	}
	return pages
}

// ---- the keys ----

// startLayerKey waits for a layer name after l, H or K.
func (a *app) startLayerKey(cmd rune) {
	a.stopTyping()
	a.layerCmd = cmd
}

var layerHelp = map[rune]string{
	'l': "Layer for the selection and new shapes: press 0-9, a-z or A-Z. Esc cancels.",
	'H': "Hide or show a layer: press 0-9, a-z or A-Z. Esc cancels.",
	'K': "Lock or unlock a layer: press 0-9, a-z or A-Z. Esc cancels.",
}

// layerKey takes the name after l, H or K. Anything else cancels.
func (a *app) layerKey(r rune) {
	cmd := a.layerCmd
	a.layerCmd = 0
	l, ok := layerOf(r)
	if !ok {
		a.say("Cancelled.")
		return
	}
	name := pageChar(l)
	switch cmd {
	case 'l':
		a.toLayer(l)
	case 'H':
		a.lay.hidden ^= layerBit(l)
		a.dropUnpickable()
		a.dirty = true
		if a.lay.hidden&layerBit(l) != 0 {
			a.say(fmt.Sprintf("Layer %s is hidden: not drawn, not exported. H%s shows it.", name, name) + a.currentNote(l))
		} else {
			a.say("Layer " + name + " shows again.")
		}
	case 'K':
		a.lay.locked ^= layerBit(l)
		a.dropUnpickable()
		a.dirty = true
		if a.lay.locked&layerBit(l) != 0 {
			a.say(fmt.Sprintf("Layer %s is locked: clicks go through it. K%s unlocks it.", name, name) + a.currentNote(l))
		} else {
			a.say("Layer " + name + " is unlocked.")
		}
	}
}

// toLayer puts the selected shapes on layer l, on top of what is there,
// and makes l the layer new shapes go on.
func (a *app) toLayer(l int) {
	a.lay.active = l
	sel := a.selected()
	name := pageChar(l)
	if len(sel) == 0 {
		a.say("New shapes go on layer " + name + "." + a.activeNote())
		return
	}
	before := cloneObjs(a.objs)
	// On top of the layer: after its other shapes, in the same order.
	var rest, picked []object
	for i, o := range a.objs {
		if a.selset[i] {
			o.Layer = l
			picked = append(picked, o)
		} else {
			rest = append(rest, o)
		}
	}
	a.objs = append(rest, picked...)
	idx := make([]int, len(picked))
	for k := range idx {
		idx[k] = len(rest) + k
	}
	a.setSelection(idx, idx[len(idx)-1])
	if a.commit(before) {
		a.say(fmt.Sprintf("Moved %s to layer %s. New shapes go there too.", a.describe(a.selected()), name) + a.activeNote())
	} else {
		a.say("Already on layer " + name + ".")
	}
	a.dropUnpickable()
}

// activeNote warns when new shapes would go on a hidden or locked layer.
func (a *app) activeNote() string {
	bit := layerBit(a.lay.active)
	switch {
	case a.lay.hidden&bit != 0:
		return " It is hidden, so you won't see them."
	case a.lay.locked&bit != 0:
		return " It is locked, so you can't pick them."
	}
	return ""
}

// currentNote warns, after hiding or locking layer l, if it is the
// current layer.
func (a *app) currentNote(l int) string {
	if l != a.lay.active {
		return ""
	}
	if a.lay.hidden&layerBit(l) != 0 {
		return " New shapes still go on it, and won't show."
	}
	if a.lay.locked&layerBit(l) != 0 {
		return " New shapes still go on it, and can't be picked."
	}
	return ""
}

// dropUnpickable lets go of selected shapes on hidden or locked layers.
func (a *app) dropUnpickable() {
	var keep []int
	for _, i := range a.selected() {
		if a.pickable(i) {
			keep = append(keep, i)
		}
	}
	if len(keep) != len(a.selected()) {
		primary := -1
		if len(keep) > 0 {
			primary = keep[len(keep)-1]
		}
		a.setSelection(keep, primary)
	}
	if !a.pickable(a.hover) {
		a.hover = -1
	}
}

// layerStatus is the status bar's note on layers, or "" when only layer 0
// is in use and nothing is hidden or locked.
func (a *app) layerStatus() string {
	if a.lay.active == 0 && a.lay.hidden == 0 && a.lay.locked == 0 && !slices.ContainsFunc(a.objs, func(o object) bool { return o.Layer != 0 }) {
		return ""
	}
	s := "layer " + pageChar(a.lay.active)
	if a.lay.hidden != 0 {
		s += " hide:" + layerNames(a.lay.hidden)
	}
	if a.lay.locked != 0 {
		s += " lock:" + layerNames(a.lay.locked)
	}
	return s
}
