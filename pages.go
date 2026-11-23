package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// Pages. A drawing is a list of pages, each with its own shapes and its own
// undo history. The page on screen lives in a.objs, a.undo and a.redo, as
// it always has; the others wait in a.pages. Moving past the last page
// starts an empty one. + copies the page on screen to a new page after it.

type page struct {
	objs       []object
	undo, redo [][]object
	lay        layerState
}

// allPages gives every page's shapes, the one on screen included.
func (a *app) allPages() [][]object {
	out := make([][]object, len(a.pages))
	for i, p := range a.pages {
		out[i] = p.objs
	}
	out[a.page] = a.objs
	return out
}

// setPages replaces the whole drawing and shows the first page.
func (a *app) setPages(pp [][]object) {
	if len(pp) == 0 {
		pp = [][]object{nil}
	}
	a.pages = make([]page, len(pp))
	for i, objs := range pp {
		a.pages[i].objs = objs
	}
	a.page = 0
	a.objs, a.undo, a.redo, a.lay = a.pages[0].objs, nil, nil, layerState{}
}

// setLayers gives each page its layer state, as read from a file.
func (a *app) setLayers(ls []layerState) {
	for i := range a.pages {
		if i < len(ls) {
			a.pages[i].lay = ls[i]
		}
	}
	a.lay = a.pages[a.page].lay
}

func (a *app) gotoPage(n int) {
	a.stopTyping()
	a.forget()
	a.drag = nil
	a.pages[a.page] = page{a.objs, a.undo, a.redo, a.lay}
	// Going back from an untouched empty page at the end drops it, so the
	// page count only counts pages you used. One with undo or redo steps
	// stays, so nothing you could get back is lost.
	last := len(a.pages) - 1
	if n < a.page && a.page == last && last > 0 && len(a.objs) == 0 && len(a.undo) == 0 && len(a.redo) == 0 && a.lay == (layerState{}) {
		a.pages = a.pages[:last]
	}
	a.page = n
	p := a.pages[n]
	a.objs, a.undo, a.redo, a.lay = p.objs, p.undo, p.redo, p.lay
}

// Each page has a one-character name: 0-9, then a-z, then A-Z. That caps a
// drawing at 62 pages and lets the ruler in the corner show every page in
// one cell each.
const pageNames = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

const maxPages = len(pageNames)

func pageChar(i int) string { return pageNames[i : i+1] }

func (a *app) pageName() string { return "page " + pageChar(a.page) }

func (a *app) tooManyPages() bool {
	if len(a.pages) < maxPages {
		return false
	}
	a.sayErr(fmt.Sprintf("A drawing holds at most %d pages, 0 to Z.", maxPages))
	return true
}

const pagerLabel = "Pages:"

// pagerSlots is how many page names the ruler shows. It never grows: as you
// move, the names slide through it and the page you're on stays in view.
const pagerSlots = 5

// pager lays out the ruler at the right end of the legend line: where the
// names start, and the first page shown.
func (a *app) pager() (x0, first int) {
	sw, _ := a.s.Size()
	first = min(max(a.page-pagerSlots/2, 0), maxPages-pagerSlots)
	return sw - 1 - pagerSlots, first
}

// pagerWidth is how much of the legend line the ruler takes.
func (a *app) pagerWidth() int { return runeLen(pagerLabel) + pagerSlots + 2 }

// renderPager draws the ruler. The page on screen is yellow, other pages
// are bold, and names of pages that don't exist yet are dim.
func (a *app) renderPager(y int, leg tcell.Style) {
	x0, first := a.pager()
	a.text(x0-runeLen(pagerLabel), y, pagerLabel, leg)
	on := tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack).Bold(true)
	for i := 0; i < pagerSlots; i++ {
		n, st := first+i, leg.Bold(true)
		switch {
		case n == a.page:
			st = on
		case n >= len(a.pages):
			st = leg.Dim(true)
		}
		a.text(x0+i, y, pageChar(n), st)
	}
}

// pagerAt gives the page whose name is at screen cell (sx, sy), or -1.
// Names of pages that don't exist yet don't count.
func (a *app) pagerAt(sx, sy int) int {
	_, sh := a.s.Size()
	x0, first := a.pager()
	if n := first + sx - x0; sy == sh-1 && sx >= x0 && sx < x0+pagerSlots && n < len(a.pages) {
		return n
	}
	return -1
}

// pickPage goes to a page clicked in the ruler.
func (a *app) pickPage(n int) {
	if n != a.page {
		a.gotoPage(n)
		a.say(a.pageName())
	}
}

func (a *app) nextPage() {
	if a.page == len(a.pages)-1 {
		if len(a.objs) == 0 {
			a.sayErr("This page is empty. Draw on it before you add another.")
			return
		}
		if a.tooManyPages() {
			return
		}
		a.pages = append(a.pages, page{})
		a.gotoPage(a.page + 1)
		a.say("New empty page: " + a.pageName() + ". , goes back.")
		return
	}
	a.gotoPage(a.page + 1)
	a.say(a.pageName())
}

func (a *app) prevPage() {
	if a.page == 0 {
		a.sayErr("This is the first page.")
		return
	}
	a.gotoPage(a.page - 1)
	a.say(a.pageName())
}

// clonePage copies the page on screen to a new page right after it, and
// goes there.
func (a *app) clonePage() {
	if a.tooManyPages() {
		return
	}
	a.stopTyping()
	from := a.page + 1
	a.pages[a.page] = page{a.objs, a.undo, a.redo, a.lay}
	a.pages = append(a.pages[:from], append([]page{{objs: cloneObjs(a.objs), lay: a.lay}}, a.pages[from:]...)...)
	a.gotoPage(from)
	a.dirty = true
	a.say(fmt.Sprintf("Copied page %s to a new %s.", pageChar(from-1), a.pageName()))
}

// trimPages drops empty pages at the end; a drawing keeps at least one.
func trimPages(pp [][]object) [][]object {
	for len(pp) > 1 && len(pp[len(pp)-1]) == 0 {
		pp = pp[:len(pp)-1]
	}
	return pp
}
