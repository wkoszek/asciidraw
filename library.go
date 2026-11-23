package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// The L library: ready-made pieces to drop into a drawing. The builtin tab
// has pieces compiled into asciidraw, from the library folder of the
// source. The custom tab reads ~/.config/asciidraw/library: an .adraw file
// is a drawing, and its first page is the piece; an .md or .txt file is
// one text shape, the way opening it as a drawing would bring it in.

//go:embed library/*.adraw
var builtinLib embed.FS

type libItem struct {
	name string
	objs []object
	err  error // why a custom file couldn't be read; shown instead of a preview
}

// libraryDir is where custom pieces live: $XDG_CONFIG_HOME/asciidraw/library,
// or ~/.config/asciidraw/library.
func libraryDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "asciidraw", "library")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "asciidraw", "library")
}

// pieceName turns "1-login-dialog.adraw" into "login dialog". A number in
// front only sets the order in the list.
func pieceName(file string) string {
	name := strings.TrimSuffix(file, filepath.Ext(file))
	if i := strings.IndexByte(name, '-'); i > 0 && strings.Trim(name[:i], "0123456789") == "" {
		name = name[i+1:]
	}
	return strings.ReplaceAll(name, "-", " ")
}

func builtinItems() []libItem {
	ents, _ := builtinLib.ReadDir("library")
	var out []libItem
	for _, e := range ents {
		data, _ := builtinLib.ReadFile("library/" + e.Name())
		pages, _, err := decodeDoc(data)
		it := libItem{name: pieceName(e.Name()), err: err}
		if err == nil {
			it.objs = pages[0]
		}
		out = append(out, it)
	}
	return out
}

func isPieceFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".adraw", ".yaml", ".yml", ".md", ".txt":
		return true
	}
	return false
}

// customItems reads the custom library folder. A folder that isn't there
// is an empty library, not an error.
func customItems() []libItem {
	dir := libraryDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []libItem
	for _, e := range ents {
		if e.IsDir() || !isPieceFile(e.Name()) || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		pages, _, _, _, err := loadDrawing(filepath.Join(dir, e.Name()))
		it := libItem{name: pieceName(e.Name()), err: err}
		if err == nil && len(pages) > 0 {
			it.objs = pages[0]
		}
		out = append(out, it)
	}
	return out // in file name order, so a number in front sets the order
}

const (
	libBuiltin = iota
	libCustom
)

type library struct {
	tab     int
	row     [2]int
	items   [2][]libItem
	clicked int // the row clicked last, or -1; a second click on it adds it
}

// openLibrary also makes the custom folder, so it is there to save into.
func (a *app) openLibrary() {
	a.stopTyping()
	os.MkdirAll(libraryDir(), 0o755)
	a.library = &library{items: [2][]libItem{builtinItems(), customItems()}, clicked: -1}
}

func (l *library) current() (libItem, bool) {
	items := l.items[l.tab]
	if len(items) == 0 {
		return libItem{}, false
	}
	return items[min(l.row[l.tab], len(items)-1)], true
}

// insertPiece drops the piece at the mouse, the way paste does, as one
// group, and selects it.
func (a *app) insertPiece(it libItem) {
	a.library = nil
	if it.err != nil {
		a.sayErr(it.name + ": " + it.err.Error())
		return
	}
	if len(it.objs) == 0 {
		a.sayErr(it.name + " is empty.")
		return
	}
	idx := make([]int, len(it.objs))
	for i := range idx {
		idx[i] = i
	}
	x0, y0, _, _ := selBounds(it.objs, idx)
	src := cloneObjs(it.objs)
	solo := len(src) == 1
	for i := range src {
		src[i].move(-x0, -y0)
		if !solo {
			src[i].Group = 1 // one piece moves as one; addCopies gives it a fresh group
		}
	}
	before := cloneObjs(a.objs)
	added := a.addCopies(src, a.mx, a.my)
	a.setSelection(added, added[len(added)-1])
	a.commit(before)
	a.say(fmt.Sprintf("Added %s at %d,%d.", it.name, a.mx, a.my))
}

func (a *app) libraryKey(ev *tcell.EventKey) {
	l := a.library
	n := len(l.items[l.tab])
	move := func(d int) {
		if n > 0 {
			l.row[l.tab] = (min(l.row[l.tab], n-1) + d + n) % n
		}
	}
	switch ev.Key() {
	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyTab, tcell.KeyBacktab:
		l.tab = 1 - l.tab
	case tcell.KeyUp:
		move(-1)
	case tcell.KeyDown:
		move(1)
	case tcell.KeyEnter:
		if it, ok := l.current(); ok {
			a.insertPiece(it)
		}
	case tcell.KeyEscape:
		a.library = nil
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'h', 'l':
			l.tab = 1 - l.tab
		case 'k':
			move(-1)
		case 'j':
			move(1)
		case 'b':
			l.tab = libBuiltin
		case 'c':
			l.tab = libCustom
		case ' ':
			if it, ok := l.current(); ok {
				a.insertPiece(it)
			}
		case 'L', 'q':
			a.library = nil
		}
	}
}

const libListW = 22 // the list of names on the left

func (a *app) libraryBox() (x0, y0, w, h int) {
	sw, sh := a.s.Size()
	w, h = min(sw-2, 100), min(sh-3, 30)
	return max((sw-w)/2, 0), max((sh-2-h)/2, 0), w, h
}

// libTabsAt gives the tab under (x, y), or -1. The tabs read
// "[ builtin | custom ]".
func (a *app) libTabsAt(x, y int) int {
	x0, y0, _, _ := a.libraryBox()
	if y != y0+1 {
		return -1
	}
	switch {
	case x >= x0+2 && x < x0+2+11:
		return libBuiltin
	case x >= x0+13 && x < x0+13+9:
		return libCustom
	}
	return -1
}

func (a *app) libraryMouse(x, y int, b tcell.ButtonMask) {
	l := a.library
	if b&tcell.Button1 == 0 {
		if b&tcell.WheelUp != 0 && len(l.items[l.tab]) > 0 {
			l.row[l.tab] = max(l.row[l.tab]-1, 0)
		}
		if b&tcell.WheelDown != 0 && len(l.items[l.tab]) > 0 {
			l.row[l.tab] = min(l.row[l.tab]+1, len(l.items[l.tab])-1)
		}
		return
	}
	if t := a.libTabsAt(x, y); t >= 0 {
		l.tab, l.clicked = t, -1
		return
	}
	x0, y0, w, h := a.libraryBox()
	if x < x0 || x >= x0+w || y < y0 || y >= y0+h {
		a.library = nil // a click outside closes it
		return
	}
	// A click on a name picks it; a second click on the same name adds it.
	if r := y - (y0 + 3); x >= x0+2 && x < x0+2+libListW && r >= 0 && r < len(l.items[l.tab]) {
		if l.clicked == r {
			a.insertPiece(l.items[l.tab][r])
			return
		}
		l.row[l.tab], l.clicked = r, r
		return
	}
	l.clicked = -1
}

func (a *app) renderLibrary() {
	l := a.library
	x0, y0, w, h := a.libraryBox()
	a.frame(x0, y0, w, h, " L Library ")
	dim := tcell.StyleDefault.Dim(true)
	on := tcell.StyleDefault.Reverse(true).Bold(true).Foreground(tcell.ColorAqua)
	x := a.text(x0+2, y0+1, "[", tcell.StyleDefault)
	for t, name := range []string{"builtin", "custom"} {
		st := tcell.StyleDefault
		if t == l.tab {
			st = on
		}
		if t > 0 {
			x = a.text(x, y0+1, "|", tcell.StyleDefault)
		}
		x = a.text(x, y0+1, " "+name+" ", st)
	}
	a.text(x, y0+1, "]", tcell.StyleDefault)

	items := l.items[l.tab]
	listH := h - 5
	if len(items) == 0 {
		msg := []string{"Nothing here yet."}
		if l.tab == libCustom {
			msg = []string{
				"No pieces yet. Put drawings (.adraw) or Markdown",
				"(.md) files in " + tildePath(libraryDir()),
				"and they show up here.",
			}
		}
		for i, m := range msg {
			if runeLen(m) > w-4 {
				m = "…" + string([]rune(m)[runeLen(m)-(w-5):])
			}
			a.text(x0+2, y0+3+i, m, dim)
		}
	}
	cur, _ := l.current()
	for i, it := range items {
		if i >= listH {
			break
		}
		st := tcell.StyleDefault
		if i == min(l.row[l.tab], len(items)-1) {
			st = on
		}
		name := it.name
		if runeLen(name) > libListW-2 {
			name = string([]rune(name)[:libListW-3]) + "…"
		}
		a.text(x0+2, y0+3+i, " "+padRight(name, libListW-2)+" ", st)
	}
	// The preview, cut to fit.
	px, py := x0+2+libListW+2, y0+3
	pw, ph := x0+w-2-px, listH
	if len(items) > 0 {
		if cur.err != nil {
			a.text(px, py, "Can't read this piece:", tcell.StyleDefault.Foreground(tcell.ColorYellow))
			a.text(px, py+1, cur.err.Error(), dim)
		} else {
			scene := compose(cur.objs)
			for y := 0; y < ph; y++ {
				for x := 0; x < pw; x++ {
					if r := scene.get(x, y); r != ' ' {
						a.s.SetContent(px+x, py+y, r, nil, tcell.StyleDefault)
					}
				}
			}
		}
	}
	a.text(x0+2, y0+h-2, "←→ builtin/custom  ↑↓ choose  Enter add at the mouse  Esc close", dim)
}

// tildePath shows a path under the home folder with ~.
func tildePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}
