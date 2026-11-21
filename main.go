// asciidraw: draw boxes, lines, arrows and text in the terminal with the
// mouse, move them around as objects, and save the result as plain text.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type prompt struct {
	label string
	buf   []rune
	done  func(string)
}

// What a mouse drag is doing.
const (
	dCreate = iota // drawing a new shape
	dMove          // moving an object
	dResize        // dragging a box corner or a line end
	dErase         // deleting what the rectangle touches
	dPanel         // holding a row in the object panel
)

type drag struct {
	mode           int
	x0, y0, x1, y1 int
	obj            int      // object being moved or resized
	orig           object   // that object when the drag began
	h              handle   // which point a resize moves
	wasSel         bool     // object was already selected at press
	before         []object // all objects at press, for undo
}

func (d *drag) moved() bool { return d.x0 != d.x1 || d.y0 != d.y1 }

type app struct {
	s      tcell.Screen
	objs   []object
	undo   [][]object
	redo   [][]object
	file   string
	dirty  bool
	style  int
	tool   int
	drag   *drag
	menu   *menu
	prompt *prompt

	sel     int // selected object, or -1
	hover   int // object under the mouse, on the canvas or in the panel
	pressed int // object under a held mouse button

	typing bool // text cursor is out
	edit   int  // text object being typed into; -1 until the first key
	cx, cy int  // where a new text object will start
	typed  bool // undo snapshot taken for this run of typing

	msg       string
	msgErr    bool
	confirm   string   // action waiting for a second press: "new" or "quit"
	mx, my    int      // last mouse position, for the readout
	swallow   bool     // ignore mouse events until the button is released
	shift     bool     // Shift held during the last mouse event
	clip      []object // what ^C copied, moved to 0,0
	clipText  string   // its picture, as put on the system clipboard
	showPanel bool
	panelTop  int    // first object shown in the panel
	suggest   string // name offered by the first save of an imported file
	exported  string // where the last ^E export went
}

func newApp(s tcell.Screen) *app {
	return &app{s: s, sel: -1, hover: -1, pressed: -1, edit: -1, showPanel: true}
}

func (a *app) reset() {
	n := newApp(a.s)
	n.style, n.tool, n.showPanel = a.style, a.tool, a.showPanel
	*a = *n
}

func (a *app) canvasSize() (int, int) {
	sw, sh := a.s.Size()
	return max(sw-a.panelWidth(), 1), max(sh-2, 1)
}

func (a *app) say(m string)    { a.msg, a.msgErr = m, false }
func (a *app) sayErr(m string) { a.msg, a.msgErr = m, true }

func (a *app) ask(label, initial string, done func(string)) {
	a.prompt = &prompt{label: label, buf: []rune(initial), done: done}
}

// ---- undo ----

func (a *app) pushUndo(state []object) {
	a.undo = append(a.undo, state)
	if len(a.undo) > 500 {
		a.undo = a.undo[1:]
	}
	a.redo = nil
	a.dirty = true
}

// commit records an undo step if the objects changed since before.
func (a *app) commit(before []object) bool {
	if reflect.DeepEqual(before, a.objs) {
		return false
	}
	a.pushUndo(before)
	return true
}

func (a *app) forget() {
	a.stopTyping()
	a.sel, a.hover, a.pressed = -1, -1, -1
}

func (a *app) undoOp() {
	if len(a.undo) == 0 {
		return
	}
	a.forget()
	a.redo = append(a.redo, a.objs)
	a.objs = a.undo[len(a.undo)-1]
	a.undo = a.undo[:len(a.undo)-1]
	a.dirty = true
}

func (a *app) redoOp() {
	if len(a.redo) == 0 {
		return
	}
	a.forget()
	a.undo = append(a.undo, a.objs)
	a.objs = a.redo[len(a.redo)-1]
	a.redo = a.redo[:len(a.redo)-1]
	a.dirty = true
}

func (a *app) remove(i int) {
	a.objs = append(a.objs[:i], a.objs[i+1:]...)
}

func (a *app) ascii() string { return compose(a.objs).String() }

// ---- saving ----

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func (a *app) save() {
	if a.file == "" {
		a.askPath("Save as: ", ".yaml", a.suggest, a.write)
		return
	}
	a.write(a.file)
}

// askPath asks for a file name, makes sure it ends in ext, and asks before
// replacing a file that exists.
func (a *app) askPath(label, ext, initial string, done func(string)) {
	a.ask(label, initial, func(name string) {
		name = expandHome(strings.TrimSpace(name))
		if name == "" {
			a.say("Cancelled.")
			return
		}
		if e := strings.ToLower(filepath.Ext(name)); e != ext && !(ext == ".yaml" && e == ".yml") {
			name = withExt(name, ext)
		}
		a.unlessExists(name, done)
	})
}

// unlessExists calls done(path) right away if path doesn't exist, and
// otherwise asks first.
func (a *app) unlessExists(path string, done func(string)) {
	if _, err := os.Stat(path); err != nil {
		done(path)
		return
	}
	a.ask(path+" exists. Overwrite? (y/n): ", "", func(ans string) {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ans)), "y") {
			done(path)
		} else {
			a.say("Cancelled.")
		}
	})
}

func (a *app) write(path string) {
	a.stopTyping()
	if err := saveYAML(path, a.objs); err != nil {
		a.sayErr("Save failed: " + err.Error())
		return
	}
	if path != a.file {
		a.exported = "" // a new name exports next to the new file
	}
	a.file, a.dirty, a.suggest = path, false, ""
	a.say(fmt.Sprintf("Saved %s (%d objects)", path, len(a.objs)))
}

// export writes the picture as Markdown: next to the YAML file if there is
// one, or under a name it asks for. It asks before replacing a file, except
// the one this session already exported to.
func (a *app) export() {
	switch {
	case a.exported != "":
		a.writeMD(a.exported)
	case a.file != "":
		a.unlessExists(withExt(a.file, ".md"), a.writeMD)
	default:
		a.askPath("Export as: ", ".md", "", a.writeMD)
	}
}

func (a *app) writeMD(path string) {
	a.stopTyping()
	pic := a.ascii()
	if err := os.WriteFile(path, []byte(markdown(pic)), 0o644); err != nil {
		a.sayErr("Export failed: " + err.Error())
		return
	}
	a.exported = path
	a.say(fmt.Sprintf("Exported %s (%d lines)", path, strings.Count(pic, "\n")))
}

// ---- keyboard ----

// key handles a key press and reports whether the program should quit.
func (a *app) key(ev *tcell.EventKey) bool {
	if a.prompt != nil {
		a.promptKey(ev)
		return false
	}
	if a.menu != nil {
		a.menuKey(ev)
		return false
	}
	pending := a.confirm
	a.confirm = ""

	switch ev.Key() {
	case tcell.KeyCtrlQ:
		if a.dirty && pending != "quit" {
			a.confirm = "quit"
			a.sayErr("Unsaved changes. ^Q again to quit, ^S to save.")
			return false
		}
		return true
	case tcell.KeyCtrlN:
		if a.dirty && pending != "new" {
			a.confirm = "new"
			a.sayErr("Unsaved changes. ^N again to discard, ^S to save.")
			return false
		}
		a.reset()
		a.say("New file. Draw, then ^S to name it and save.")
	case tcell.KeyCtrlS:
		a.save()
	case tcell.KeyCtrlZ:
		a.undoOp()
	case tcell.KeyCtrlY:
		a.redoOp()
	case tcell.KeyCtrlT:
		a.openMenu()
	case tcell.KeyCtrlE:
		a.export()
	case tcell.KeyCtrlC:
		a.copySel(false)
	case tcell.KeyCtrlX:
		a.copySel(true)
	case tcell.KeyCtrlV:
		a.paste()
	case tcell.KeyCtrlO:
		a.showPanel = !a.showPanel
	default:
		if a.typing {
			a.typeKey(ev)
		} else {
			a.objectKey(ev)
		}
	}
	return false
}

func (a *app) typeKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape:
		a.stopTyping()
	case tcell.KeyEnter:
		a.newline()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.backspace()
	case tcell.KeyRune:
		a.typeRune(ev.Rune())
	}
}

func (a *app) newline() {
	if a.edit < 0 {
		_, h := a.canvasSize()
		a.cy = min(a.cy+1, h-1)
		return
	}
	a.typeSnapshot()
	a.objs[a.edit].Text = append(a.objs[a.edit].Text, "")
}

func (a *app) typeSnapshot() {
	if !a.typed {
		a.pushUndo(cloneObjs(a.objs))
		a.typed = true
	}
	a.dirty = true
}

func (a *app) typeRune(r rune) {
	a.typeSnapshot()
	if a.edit < 0 {
		a.objs = append(a.objs, object{Kind: "text", X0: a.cx, Y0: a.cy, Text: []string{""}})
		a.edit = len(a.objs) - 1
	}
	a.sel = a.edit
	t := a.objs[a.edit].Text
	t[len(t)-1] += string(r)
}

func (a *app) backspace() {
	if a.edit < 0 {
		return
	}
	a.typeSnapshot()
	o := &a.objs[a.edit]
	last := []rune(o.Text[len(o.Text)-1])
	switch {
	case len(last) > 0:
		o.Text[len(o.Text)-1] = string(last[:len(last)-1])
	case len(o.Text) > 1:
		o.Text = o.Text[:len(o.Text)-1]
	}
	if len(o.Text) == 1 && o.Text[0] == "" {
		// Nothing left: drop the object, keep the cursor where it started.
		a.cx, a.cy = o.X0, o.Y0
		a.remove(a.edit)
		a.edit, a.sel = -1, -1
	}
}

func (a *app) stopTyping() {
	if a.edit >= 0 && a.edit < len(a.objs) {
		o := &a.objs[a.edit]
		for len(o.Text) > 1 && o.Text[len(o.Text)-1] == "" {
			o.Text = o.Text[:len(o.Text)-1]
		}
	}
	a.typing, a.edit = false, -1
}

func (a *app) startEditing(i int) {
	a.typing, a.edit, a.typed, a.sel = true, i, false, i
}

// cursor is where the next typed character goes.
func (a *app) cursor() (int, int) {
	if a.edit >= 0 && a.edit < len(a.objs) {
		o := a.objs[a.edit]
		return o.X0 + len([]rune(o.Text[len(o.Text)-1])), o.Y0 + len(o.Text) - 1
	}
	return a.cx, a.cy
}

// objectKey handles keys when not typing: picking and changing the selection.
func (a *app) objectKey(ev *tcell.EventKey) {
	n := len(a.objs)
	switch ev.Key() {
	case tcell.KeyTab:
		if n > 0 {
			a.sel = (a.sel + 1) % n
		}
		return
	case tcell.KeyBacktab:
		if n > 0 {
			a.sel = (a.sel + n - 1) % n
		}
		return
	case tcell.KeyEscape:
		a.sel = -1
		return
	case tcell.KeyRune: // vim-style, only when not typing
		switch ev.Rune() {
		case 'y':
			a.copySel(false)
		case 'p':
			a.paste()
		case 'd':
			a.copySel(true)
		case 'x':
			a.deleteSel()
		case ' ':
			a.toggleWidget()
		}
		return
	}
	if a.sel < 0 || a.sel >= n {
		return
	}
	switch ev.Key() {
	case tcell.KeyDelete, tcell.KeyBackspace, tcell.KeyBackspace2:
		a.deleteSel()
	case tcell.KeyLeft:
		a.nudge(-1, 0)
	case tcell.KeyRight:
		a.nudge(1, 0)
	case tcell.KeyUp:
		a.nudge(0, -1)
	case tcell.KeyDown:
		a.nudge(0, 1)
	case tcell.KeyEnter:
		switch k := a.objs[a.sel].Kind; {
		case k == "text":
			a.startEditing(a.sel)
		case isWidget(k):
			a.editWidget()
		}
	}
}

// deleteSel deletes the selected shape and leaves the clipboard alone.
func (a *app) deleteSel() {
	if a.sel < 0 || a.sel >= len(a.objs) {
		a.sayErr("Select a shape first: click it, or press Tab.")
		return
	}
	before := cloneObjs(a.objs)
	a.say("Deleted " + kindNames[a.objs[a.sel].Kind] + ".")
	a.remove(a.sel)
	a.sel, a.hover = -1, -1
	a.commit(before)
}

func (a *app) nudge(dx, dy int) {
	before := cloneObjs(a.objs)
	o := &a.objs[a.sel]
	x, y, _, _ := o.bounds()
	o.move(max(dx, -x), max(dy, -y))
	a.commit(before)
}

func (a *app) promptKey(ev *tcell.EventKey) {
	p := a.prompt
	switch ev.Key() {
	case tcell.KeyEnter:
		a.prompt = nil
		p.done(string(p.buf))
	case tcell.KeyEscape, tcell.KeyCtrlC:
		a.prompt = nil
		a.say("Cancelled.")
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(p.buf) > 0 {
			p.buf = p.buf[:len(p.buf)-1]
		}
	case tcell.KeyRune:
		p.buf = append(p.buf, ev.Rune())
	}
}

// ---- mouse ----

func (a *app) mouse(ev *tcell.EventMouse) {
	if a.prompt != nil {
		return
	}
	sx, sy := ev.Position()
	b := ev.Buttons()
	held := b&(tcell.Button1|tcell.Button2) != 0
	a.shift = ev.Modifiers()&tcell.ModShift != 0

	// A click that the menu used must not turn into a drag on the canvas.
	if a.swallow {
		a.swallow = held
		return
	}
	if a.menu != nil {
		a.menuMouse(sx, sy, b)
		a.swallow = held
		return
	}
	cw, h := a.canvasSize()
	x, y := min(max(sx, 0), cw-1), min(max(sy, 0), h-1)

	if a.drag != nil {
		a.mx, a.my = x, y
		a.dragTo(x, y)
		if !held {
			a.release()
		}
		return
	}
	if !held { // hover, or the wheel
		a.hover = -1
		switch {
		case a.inPanel(sx, sy):
			a.hover = a.panelItemAt(sx, sy)
			if b&tcell.WheelUp != 0 {
				a.scrollPanel(-1)
			}
			if b&tcell.WheelDown != 0 {
				a.scrollPanel(1)
			}
		case sx < cw && sy < h:
			a.mx, a.my = x, y
			a.hover = compose(a.objs).owner(x, y)
		}
		return
	}
	// A button just went down.
	switch {
	case a.inPanel(sx, sy):
		a.stopTyping()
		a.drag = &drag{mode: dPanel, obj: -1}
		if i := a.panelItemAt(sx, sy); i >= 0 {
			a.sel, a.pressed = i, i
		}
	case sy >= h: // status bar or legend
		a.swallow = true
	default:
		a.mx, a.my = x, y
		a.press(x, y, b&tcell.Button2 != 0)
	}
}

func (a *app) press(x, y int, right bool) {
	a.stopTyping()
	a.confirm = ""
	d := &drag{x0: x, y0: y, x1: x, y1: y, obj: -1, before: cloneObjs(a.objs)}
	a.drag = d
	if right || a.tool == toolErase {
		d.mode = dErase
		return
	}
	// The selected shape's handles come first. An oval's corners are empty
	// cells, so this is the only way to grab them.
	if i := a.sel; i >= 0 && i < len(a.objs) {
		if hd := a.objs[i].handleAt(x, y); hd.ok {
			d.mode, d.obj, d.orig, d.h, d.wasSel = dResize, i, a.objs[i].clone(), hd, true
			a.pressed = i
			return
		}
	}
	if i := compose(a.objs).owner(x, y); i >= 0 {
		d.mode, d.obj, d.orig, d.wasSel = dMove, i, a.objs[i].clone(), a.sel == i
		if hd := a.objs[i].handleAt(x, y); hd.ok {
			d.mode, d.h = dResize, hd
		}
		a.sel, a.pressed = i, i
		return
	}
	d.mode = dCreate
	a.sel = -1
}

func (a *app) dragTo(x, y int) {
	d := a.drag
	d.x1, d.y1 = x, y
	switch d.mode {
	case dMove:
		o := d.orig.clone()
		bx, by, _, _ := o.bounds()
		o.move(max(x-d.x0, -bx), max(y-d.y0, -by))
		a.objs[d.obj] = o
	case dResize:
		o := d.orig.clone()
		if a.shift && o.squarable() {
			ax, ay := o.X1, o.Y1 // the corner that stays put
			if d.h.x1 {
				ax = o.X0
			}
			if d.h.y1 {
				ay = o.Y0
			}
			x, y = symmetric(ax, ay, x, y)
		}
		if d.h.x1 {
			o.X1 = x
		} else {
			o.X0 = x
		}
		if d.h.y1 {
			o.Y1 = y
		} else {
			o.Y0 = y
		}
		o.normalize() // widgets keep their shape while you drag
		a.objs[d.obj] = o
	}
}

// newShape makes the object a drag (or, for a widget, a click) creates.
func (a *app) newShape(d *drag) object {
	o := object{Kind: tools[a.tool].kind, X0: d.x0, Y0: d.y0, X1: d.x1, Y1: d.y1, Style: a.style}
	if a.shift && o.squarable() {
		o.X1, o.Y1 = symmetric(d.x0, d.y0, d.x1, d.y1)
	}
	if def, ok := widgets[o.Kind]; ok {
		o.Text, o.Value = append([]string(nil), def.label...), def.value
		if !d.moved() && def.w > 0 { // stamped with a click: default size
			o.X1, o.Y1 = o.X0+def.w-1, o.Y0+def.h-1
		}
	}
	o.normalize()
	return o
}

func (a *app) widgetTool() bool { return isWidget(tools[a.tool].kind) }

func (a *app) release() {
	d := a.drag
	a.drag, a.pressed = nil, -1
	switch d.mode {
	case dMove, dResize:
		a.objs[d.obj].normalize()
		// A second click on selected text goes back to typing in it.
		if !a.commit(d.before) && d.wasSel && a.objs[d.obj].Kind == "text" {
			a.startEditing(d.obj)
		}
	case dCreate:
		if !d.moved() && !a.widgetTool() {
			// A click on empty space puts out the text cursor.
			a.typing, a.edit, a.typed = true, -1, false
			a.cx, a.cy = d.x0, d.y0
			return
		}
		a.objs = append(a.objs, a.newShape(d))
		a.sel = len(a.objs) - 1
		a.commit(d.before)
	case dErase:
		hits := objectsIn(a.objs, d.x0, d.y0, d.x1, d.y1)
		for _, i := range hits {
			a.remove(i)
		}
		if len(hits) > 0 {
			a.sel, a.hover = -1, -1
			a.commit(d.before)
			a.say(fmt.Sprintf("Deleted %d object(s).", len(hits)))
		}
	}
}

// ---- drawing the screen ----

func (a *app) text(x, y int, s string, st tcell.Style) int {
	w, _ := a.s.Size()
	for _, r := range s {
		if x >= w {
			break
		}
		a.s.SetContent(x, y, r, nil, st)
		x++
	}
	return x
}

// readout is the size readout at the right of the status bar.
func (a *app) readout() string {
	if d := a.drag; d != nil {
		switch d.mode {
		case dMove, dResize:
			x, y, w, h := a.objs[d.obj].bounds()
			return fmt.Sprintf("x=%d y=%d w=%d h=%d", x, y, w, h) + a.shiftMark(a.objs[d.obj])
		case dCreate:
			if d.moved() {
				o := a.newShape(d)
				x, y, w, h := o.bounds()
				return fmt.Sprintf("x=%d y=%d w=%d h=%d", x, y, w, h) + a.shiftMark(o)
			}
			fallthrough
		case dErase:
			x0, x1 := order(d.x0, d.x1)
			y0, y1 := order(d.y0, d.y1)
			return fmt.Sprintf("x=%d y=%d w=%d h=%d", x0, y0, x1-x0+1, y1-y0+1)
		}
	}
	return fmt.Sprintf("x=%d y=%d", a.mx, a.my)
}

// shiftMark notes in the readout that Shift is keeping a shape symmetric.
func (a *app) shiftMark(o object) string {
	if a.shift && o.squarable() && a.drag != nil && a.drag.mode != dMove {
		return " ⇧"
	}
	return ""
}

// objStyle colors an object's cells: held, then selected, then hovered.
func (a *app) objStyle(i int, doomed map[int]bool) tcell.Style {
	st := tcell.StyleDefault
	switch {
	case i < 0:
	case doomed[i]:
		st = st.Foreground(tcell.ColorRed).Bold(true)
	case i == a.pressed:
		st = st.Foreground(colPressed).Bold(true)
	case i == a.sel:
		st = st.Foreground(colSel).Bold(true)
	case i == a.hover:
		st = st.Foreground(colHover)
	}
	return st
}

func (a *app) render() {
	s := a.s
	cw, h := a.canvasSize()
	sw, sh := s.Size()
	s.Clear()

	objs := a.objs
	d := a.drag
	if d != nil && d.mode == dCreate && (d.moved() || a.widgetTool()) {
		objs = append(cloneObjs(a.objs), a.newShape(d))
	}
	scene := compose(objs)
	doomed := map[int]bool{}
	if d != nil && d.mode == dErase {
		for _, i := range objectsIn(a.objs, d.x0, d.y0, d.x1, d.y1) {
			doomed[i] = true
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < cw; x++ {
			st := a.objStyle(scene.owner(x, y), doomed)
			if d != nil && d.mode == dErase {
				x0, x1 := order(d.x0, d.x1)
				y0, y1 := order(d.y0, d.y1)
				if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
					st = st.Background(tcell.ColorMaroon)
				}
			}
			s.SetContent(x, y, scene.get(x, y), nil, st)
		}
	}
	// A selected oval shows its corners, since that is where it resizes.
	if i := a.sel; i >= 0 && i < len(a.objs) && a.objs[i].Kind == "oval" {
		o := a.objs[i]
		for _, p := range [][2]int{{o.X0, o.Y0}, {o.X1, o.Y0}, {o.X0, o.Y1}, {o.X1, o.Y1}} {
			if p[0] < cw && p[1] < h && scene.get(p[0], p[1]) == ' ' {
				s.SetContent(p[0], p[1], '+', nil, a.objStyle(i, doomed))
			}
		}
	}
	a.renderPanel()

	// Status line.
	bar := tcell.StyleDefault.Background(tcell.ColorDarkSlateGray).Foreground(tcell.ColorWhite)
	for x := 0; x < sw; x++ {
		s.SetContent(x, sh-2, ' ', nil, bar)
	}
	s.HideCursor()
	if a.prompt != nil {
		end := a.text(1, sh-2, a.prompt.label+string(a.prompt.buf), bar.Bold(true))
		s.ShowCursor(min(end, sw-1), sh-2)
	} else {
		name := a.file
		if name == "" {
			name = "[untitled]"
		}
		if a.dirty {
			name += " *"
		}
		x := a.text(1, sh-2, name, bar.Bold(true))
		// Tool and a sample of the style. While the menu is open this
		// follows the menu cursor, so a style shows before you pick it.
		ps := bar.Foreground(tcell.ColorWhite)
		if a.menu != nil {
			ps = bar.Foreground(tcell.ColorYellow).Bold(true)
		}
		x = a.text(x+2, sh-2, "drag: "+tools[a.previewTool()].name+"  "+styles[a.previewStyle()].sample(), ps)
		if a.msg != "" {
			ms := bar.Foreground(tcell.ColorLightGreen).Bold(true)
			if a.msgErr {
				ms = bar.Foreground(tcell.ColorYellow).Bold(true)
			}
			a.text(x+3, sh-2, a.msg, ms)
		}
		// Position readout, right-aligned. Drawn last so it stays visible.
		pos := a.readout()
		a.text(max(sw-len(pos)-1, 0), sh-2, pos, bar.Foreground(tcell.ColorAqua).Bold(true))
		if a.typing && a.menu == nil {
			s.ShowCursor(a.cursor())
		}
	}
	if a.menu != nil {
		a.renderMenu()
	}

	// Legend: as many items as fit.
	leg := tcell.StyleDefault.Reverse(true)
	for x := 0; x < sw; x++ {
		s.SetContent(x, sh-1, ' ', nil, leg)
	}
	items := [][2]string{
		{"click", "type"}, {"r-drag", "erase"}, {"^T", "tools"}, {"^S", "save"},
		{"^E", "export .md"}, {"^C/^V", "copy/paste"}, {"^N", "new"}, {"^Q", "quit"},
		{"^Z/^Y", "undo/redo"}, {"^O", "objects"},
	}
	x := 1
	for _, it := range items {
		if x+len([]rune(it[0]+it[1]))+1 > sw {
			break
		}
		x = a.text(x, sh-1, it[0], leg.Bold(true))
		x = a.text(x+1, sh-1, it[1], leg)
		x += 2
	}
	s.Show()
}

func main() {
	var objs []object
	file, suggest, note := "", "", ""
	if len(os.Args) > 1 {
		arg := os.Args[1]
		var err error
		objs, file, note, err = loadDrawing(arg)
		switch {
		case os.IsNotExist(err): // a new drawing under that name
			file = arg
			if !isYAML(arg) {
				file = withExt(arg, ".yaml")
			}
		case err != nil:
			fmt.Fprintln(os.Stderr, "asciidraw:", err)
			os.Exit(1)
		case file == "": // imported .txt or .md: offer a YAML name on first save
			suggest = withExt(arg, ".yaml")
		}
	}

	s, err := tcell.NewScreen()
	if err == nil {
		err = s.Init()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "asciidraw:", err)
		os.Exit(1)
	}
	s.EnableMouse(tcell.MouseMotionEvents) // motion without a button feeds hover and the readout
	// Ghostty and xterm keep Shift+drag for selecting text unless the program
	// asks for it (XTSHIFTESCAPE). Ask, and hand it back on exit.
	tty, hasTTY := s.Tty()
	if hasTTY {
		tty.Write([]byte("\x1b[>1s"))
	}
	defer func() {
		if hasTTY {
			tty.Write([]byte("\x1b[>0s"))
		}
		s.Fini()
	}()

	a := newApp(s)
	a.objs, a.file, a.suggest = objs, file, suggest
	a.say("Drag to draw. Click to type. Drag a shape to move it.")
	if note != "" {
		a.say(note)
	}

	for {
		a.render()
		switch ev := s.PollEvent().(type) {
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventKey:
			if a.key(ev) {
				return
			}
		case *tcell.EventMouse:
			a.mouse(ev)
		}
	}
}
