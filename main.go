// asciidraw: draw boxes, lines, arrows and text in the terminal with the
// mouse, move them around as objects, and save the result as plain text.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

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
	dPanel         // holding a row in the object panel, or a Shift-click
	dSelect        // dragging a selection box with the Select tool
)

type drag struct {
	mode           int
	x0, y0, x1, y1 int
	obj            int            // object being moved or resized
	orig           object         // that object when the drag began
	h              handle         // which point a resize moves
	wasSel         bool           // object was already selected at press
	before         []object       // all objects at press, for undo
	origs          map[int]object // every shape a move carries, as it was
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

	sel      int          // primary selected object, or -1
	selset   map[int]bool // every selected object; see select.go
	selFor   int          // the a.sel that selset was built for
	lastTool int          // the tool v switches back to
	hover    int          // object under the mouse, on the canvas or in the panel
	pressed  int          // object under a held mouse button

	typing bool // text cursor is out
	edit   int  // text object being typed into; -1 until the first key
	cx, cy int  // where a new text object will start
	typed  bool // undo snapshot taken for this run of typing
	tl, tc int  // the typing cursor: line and column in the text
	vx, vy int  // scroll: the canvas cell shown at the window's top left

	msg       string
	msgErr    bool
	confirm   string   // action waiting for a second press: "new" or "quit"
	mx, my    int      // last mouse position, for the readout
	swallow   bool     // ignore mouse events until the button is released
	shift     bool     // Shift held during the last mouse event
	snap      int      // shape a dragged line end would attach to, or -1
	clip      []object // what ^C copied, moved to 0,0
	clipText  string   // its picture, as put on the system clipboard
	showPanel bool
	help      bool   // the ? screen is open
	dropAt    int    // panel row a dragged row would land on, or -1
	panelTop  int    // first object shown in the panel
	suggest   string // name offered by the first save of an imported file
	exported  string // where the last ^E export went
}

func newApp(s tcell.Screen) *app {
	return &app{s: s, sel: -1, selFor: -1, selset: map[int]bool{}, hover: -1, pressed: -1, edit: -1, snap: -1, dropAt: -1, showPanel: true}
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
	a.reattach(nil) // attached line ends follow whatever just changed
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
	clearRecovery(a.file) // saved: no recovery copy needed, old name or new
	clearRecovery(path)
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
	if a.help {
		a.help = false
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
		clearRecovery(a.file) // quitting on purpose throws the changes away
		return true
	case tcell.KeyCtrlN:
		if a.dirty && pending != "new" {
			a.confirm = "new"
			a.sayErr("Unsaved changes. ^N again to discard, ^S to save.")
			return false
		}
		clearRecovery(a.file)
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
	case tcell.KeyCtrlG:
		a.stopTyping()
		a.toggleGroup()
	case tcell.KeyCtrlD:
		a.duplicate()
	case tcell.KeyCtrlA:
		a.stopTyping()
		a.selectAll()
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
	case tcell.KeyDelete:
		a.deleteForward()
	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown, tcell.KeyHome, tcell.KeyEnd:
		a.moveCursor(ev.Key())
	case tcell.KeyRune:
		a.typeRune(ev.Rune())
	}
}

func (a *app) newline() {
	if a.edit < 0 {
		a.cy++
		return
	}
	a.typeSnapshot()
	o := &a.objs[a.edit]
	if len(o.Text) == 0 {
		o.Text = []string{""}
	}
	// Split the line at the cursor.
	line := []rune(o.Text[a.tl])
	left, right := string(line[:a.tc]), string(line[a.tc:])
	o.Text = append(o.Text[:a.tl], append([]string{left, right}, o.Text[a.tl+1:]...)...)
	a.tl, a.tc = a.tl+1, 0
	a.refit()
}

// refit grows a box or oval to fit the label being typed, and moves the
// lines attached to it.
func (a *app) refit() {
	if a.edit >= 0 && hasLabel(a.objs[a.edit].Kind) {
		a.objs[a.edit].fitLabel()
		a.reattach(nil)
	}
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
		a.edit, a.tl, a.tc = len(a.objs)-1, 0, 0
	}
	a.sel = a.edit
	o := &a.objs[a.edit]
	if len(o.Text) == 0 { // a box's first letter
		o.Text = []string{""}
	}
	line := []rune(o.Text[a.tl])
	o.Text[a.tl] = string(line[:a.tc]) + string(r) + string(line[a.tc:])
	a.tc++
	a.refit()
}

func (a *app) backspace() {
	if a.edit < 0 {
		return
	}
	o := &a.objs[a.edit]
	if len(o.Text) == 0 || (a.tl == 0 && a.tc == 0) {
		return // nothing before the cursor
	}
	a.typeSnapshot()
	if a.tc > 0 {
		line := []rune(o.Text[a.tl])
		o.Text[a.tl] = string(line[:a.tc-1]) + string(line[a.tc:])
		a.tc--
	} else { // at the start of a line: join it to the one above
		a.tc = runeLen(o.Text[a.tl-1])
		o.Text[a.tl-1] += o.Text[a.tl]
		o.Text = append(o.Text[:a.tl], o.Text[a.tl+1:]...)
		a.tl--
	}
	a.dropIfEmpty()
}

// deleteForward is the Delete key while typing: it removes the letter
// under the cursor, or joins the next line on at the end of one.
func (a *app) deleteForward() {
	if a.edit < 0 {
		return
	}
	o := &a.objs[a.edit]
	if len(o.Text) == 0 {
		return
	}
	line := []rune(o.Text[a.tl])
	switch {
	case a.tc < len(line):
		a.typeSnapshot()
		o.Text[a.tl] = string(line[:a.tc]) + string(line[a.tc+1:])
	case a.tl+1 < len(o.Text):
		a.typeSnapshot()
		o.Text[a.tl] += o.Text[a.tl+1]
		o.Text = append(o.Text[:a.tl+1], o.Text[a.tl+2:]...)
	default:
		return
	}
	a.dropIfEmpty()
}

// dropIfEmpty handles text with nothing left in it: a label is cleared and
// its shape stays; loose text goes away, and the cursor stays where it was.
func (a *app) dropIfEmpty() {
	o := &a.objs[a.edit]
	if len(o.Text) != 1 || o.Text[0] != "" {
		return
	}
	if hasLabel(o.Kind) {
		o.Text = nil
		a.tl, a.tc = 0, 0
		return
	}
	a.cx, a.cy = o.X0, o.Y0
	a.remove(a.edit)
	a.edit, a.sel = -1, -1
}

// moveCursor moves the typing cursor. Before the first letter it moves the
// spot where new text will start.
func (a *app) moveCursor(k tcell.Key) {
	if a.edit < 0 {
		switch k {
		case tcell.KeyLeft:
			a.cx = max(a.cx-1, 0)
		case tcell.KeyRight:
			a.cx++
		case tcell.KeyUp:
			a.cy = max(a.cy-1, 0)
		case tcell.KeyDown:
			a.cy++
		}
		return
	}
	t := a.objs[a.edit].Text
	if len(t) == 0 {
		return
	}
	n := func(l int) int { return runeLen(t[l]) }
	switch k {
	case tcell.KeyLeft:
		if a.tc > 0 {
			a.tc--
		} else if a.tl > 0 {
			a.tl, a.tc = a.tl-1, n(a.tl-1)
		}
	case tcell.KeyRight:
		if a.tc < n(a.tl) {
			a.tc++
		} else if a.tl+1 < len(t) {
			a.tl, a.tc = a.tl+1, 0
		}
	case tcell.KeyUp:
		if a.tl > 0 {
			a.tl--
			a.tc = min(a.tc, n(a.tl))
		}
	case tcell.KeyDown:
		if a.tl+1 < len(t) {
			a.tl++
			a.tc = min(a.tc, n(a.tl))
		}
	case tcell.KeyHome:
		a.tc = 0
	case tcell.KeyEnd:
		a.tc = n(a.tl)
	}
}

func (a *app) stopTyping() {
	if a.edit >= 0 && a.edit < len(a.objs) {
		o := &a.objs[a.edit]
		for len(o.Text) > 1 && o.Text[len(o.Text)-1] == "" {
			o.Text = o.Text[:len(o.Text)-1]
		}
		if hasLabel(o.Kind) && len(o.Text) == 1 && o.Text[0] == "" {
			o.Text = nil
		}
	}
	a.typing, a.edit = false, -1
}

func (a *app) startEditing(i int) {
	a.typing, a.edit, a.typed = true, i, false
	a.setSelection([]int{i}, i)
	// The cursor starts at the end.
	t := a.objs[i].Text
	a.tl, a.tc = max(len(t)-1, 0), 0
	if len(t) > 0 {
		a.tc = runeLen(t[a.tl])
	}
}

// cursor is where the next typed character goes.
func (a *app) cursor() (int, int) {
	if a.edit >= 0 && a.edit < len(a.objs) {
		o := a.objs[a.edit]
		if len(o.Text) == 0 {
			return o.labelCursor()
		}
		if hasLabel(o.Kind) {
			x, y := o.labelStart(a.tl)
			return x + a.tc, y
		}
		return o.X0 + a.tc, o.Y0 + a.tl
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
	case tcell.KeyHome: // back to the top left of the drawing
		a.vx, a.vy = 0, 0
		return
	case tcell.KeyPgUp, tcell.KeyPgDn:
		_, h := a.canvasSize()
		step := max(h/2, 1)
		if ev.Key() == tcell.KeyPgUp {
			step = -step
		}
		a.vy = max(a.vy+step, 0)
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
		case 'g':
			a.toggleGroup()
		case 'v':
			a.toggleSelectTool()
		case 't':
			a.cycleStyle()
		case ']':
			a.restack(true)
		case '[':
			a.restack(false)
		case '?':
			a.help = true
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
		if len(a.selected()) != 1 {
			a.sayErr("Select just one shape to edit it.")
			return
		}
		switch k := a.objs[a.sel].Kind; {
		case k == "text" || hasLabel(k):
			a.startEditing(a.sel)
		case isWidget(k):
			a.editWidget()
		}
	}
}

// deleteSel deletes the selected shapes and leaves the clipboard alone.
func (a *app) deleteSel() {
	sel := a.selected()
	if len(sel) == 0 {
		a.sayErr("Select a shape first: click it, or press Tab.")
		return
	}
	before := cloneObjs(a.objs)
	a.say("Deleted " + a.describe(sel) + ".")
	for k := len(sel) - 1; k >= 0; k-- {
		a.remove(sel[k])
	}
	a.sel, a.hover = -1, -1
	a.commit(before)
}

// describe names a selection for a status message: "Box" or "3 shapes".
func (a *app) describe(idx []int) string {
	if len(idx) == 1 {
		return kindNames[a.objs[idx[0]].Kind]
	}
	return fmt.Sprintf("%d shapes", len(idx))
}

func (a *app) nudge(dx, dy int) {
	sel := a.selected()
	before := cloneObjs(a.objs)
	x, y, _, _ := selBounds(a.objs, sel)
	dx, dy = max(dx, -x), max(dy, -y)
	for _, i := range sel {
		a.objs[i].move(dx, dy)
	}
	// Like a drag: a nudged line checks again what its ends touch.
	for _, i := range sel {
		if isLine(a.objs[i].Kind) {
			a.detect(i)
		}
	}
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
	if a.help {
		if held {
			a.help, a.swallow = false, true
		}
		return
	}
	cw, h := a.canvasSize()
	if a.drag != nil && a.drag.mode != dPanel {
		// Dragging against the window's edge scrolls the canvas along.
		switch {
		case sx >= cw-1:
			a.vx++
		case sx <= 0 && a.vx > 0:
			a.vx--
		}
		switch {
		case sy >= h-1:
			a.vy++
		case sy <= 0 && a.vy > 0:
			a.vy--
		}
	}
	// Screen to canvas: the canvas can be bigger than the window.
	x, y := min(max(sx, 0), cw-1)+a.vx, min(max(sy, 0), h-1)+a.vy

	if a.drag != nil {
		if a.drag.mode == dPanel && a.drag.obj >= 0 {
			a.dropAt = a.panelDropAt(sx, sy)
		}
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
			a.wheel(b)
		}
		return
	}
	// A button just went down.
	switch {
	case a.inPanel(sx, sy):
		a.stopTyping()
		a.drag = &drag{mode: dPanel, obj: a.panelItemAt(sx, sy)}
		a.dropAt = -1
		if i := a.panelItemAt(sx, sy); i >= 0 {
			if a.shift {
				a.toggleSel(i)
			} else {
				a.setSelection(a.groupOf(i), i)
			}
			if a.isSel(i) {
				a.pressed = i
			}
		}
	case sy >= h: // status bar or legend
		a.swallow = true
	default:
		a.mx, a.my = x, y
		a.press(x, y, b&tcell.Button2 != 0)
	}
}

// wheel scrolls the canvas: up and down, or sideways with Shift or a
// sideways wheel.
func (a *app) wheel(b tcell.ButtonMask) {
	const step = 3
	up, down := b&tcell.WheelUp != 0, b&tcell.WheelDown != 0
	left, right := b&tcell.WheelLeft != 0, b&tcell.WheelRight != 0
	if a.shift {
		left, right, up, down = left || up, right || down, false, false
	}
	switch {
	case up:
		a.vy = max(a.vy-step, 0)
	case down:
		a.vy += step
	case left:
		a.vx = max(a.vx-step, 0)
	case right:
		a.vx += step
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
	// A lone selected shape's handles come first. An oval's corners are
	// empty cells, so this is the only way to grab them.
	if sel := a.selected(); len(sel) == 1 {
		i := sel[0]
		if hd := a.objs[i].handleAt(x, y); hd.ok {
			d.mode, d.obj, d.orig, d.h, d.wasSel = dResize, i, a.objs[i].clone(), hd, true
			a.pressed = i
			return
		}
	}
	i := compose(a.objs).owner(x, y)
	if i >= 0 && a.objs[i].Kind == "fill" && boxAt(a.objs, x, y) > i {
		i = -1 // the inside of a box drawn over a fill counts as empty
	}
	if i >= 0 {
		if a.shift { // Shift-click adds or removes; it never drags
			d.mode = dPanel
			a.toggleSel(i)
			return
		}
		d.wasSel = a.isSel(i)
		if !d.wasSel {
			a.sel = i // selects the shape, and its group
		}
		a.syncSel()
		a.sel, a.selFor = i, i // i is the primary; keep the rest selected
		d.mode, d.obj, d.orig = dMove, i, a.objs[i].clone()
		if sel := a.selected(); len(sel) == 1 {
			if hd := a.objs[i].handleAt(x, y); hd.ok {
				d.mode, d.h = dResize, hd
			}
		}
		d.origs = map[int]object{}
		for _, j := range a.selected() {
			d.origs[j] = a.objs[j].clone()
		}
		a.pressed = i
		return
	}
	if a.tool == toolSelect {
		d.mode = dSelect
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
		dx, dy := x-d.x0, y-d.y0
		for _, o := range d.origs { // nothing may cross the top or left edge
			bx, by, _, _ := o.bounds()
			dx, dy = max(dx, -bx), max(dy, -by)
		}
		for j, o := range d.origs {
			o = o.clone()
			o.move(dx, dy)
			a.objs[j] = o
		}
		// Lines being carried keep their shape; lines left behind follow
		// the shapes they are attached to.
		skip := map[int]bool{}
		for j := range d.origs {
			skip[j] = true
		}
		a.reattach(skip)
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
		a.snap = -1
		if isLine(o.Kind) { // the end being dragged attaches as it goes
			a.detect(d.obj)
			ex, ey := a.objs[d.obj].X0, a.objs[d.obj].Y0
			if d.h.x1 {
				ex, ey = a.objs[d.obj].X1, a.objs[d.obj].Y1
			}
			a.snap, _, _ = anchorAt(a.objs, d.obj, ex, ey)
		}
		a.reattach(map[int]bool{d.obj: true})
	case dCreate:
		a.snap = -1
		if isLine(tools[a.tool].kind) {
			a.snap, _, _ = anchorAt(a.objs, -1, x, y)
		}
	}
}

// newShape makes the object a drag (or, for a widget, a click) creates.
func (a *app) newShape(d *drag) object {
	o := object{Kind: tools[a.tool].kind, X0: d.x0, Y0: d.y0, X1: d.x1, Y1: d.y1, Style: a.style}
	if a.shift && o.squarable() {
		o.X1, o.Y1 = symmetric(d.x0, d.y0, d.x1, d.y1)
	}
	if isLine(o.Kind) {
		a.ensureIDs()
		_, o.From, _ = anchorAt(a.objs, -1, o.X0, o.Y0)
		_, o.To, _ = anchorAt(a.objs, -1, o.X1, o.Y1)
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
	a.drag, a.pressed, a.snap = nil, -1, -1
	drop := a.dropAt
	a.dropAt = -1
	switch d.mode {
	case dPanel:
		// A row dragged to another row moves its shapes there in the
		// stacking order: below that row going down, above it going up.
		if d.obj >= 0 && drop >= 0 && drop != d.obj {
			if drop > d.obj {
				drop++
			}
			a.moveSelTo(drop)
			a.say("Moved " + a.describe(a.selected()) + " in the stacking order.")
		}
	case dMove, dResize:
		a.objs[d.obj].normalize()
		// A line that was moved on its own may now touch other shapes, or
		// none. One carried along with its shapes stays attached.
		for j := range d.origs {
			if isLine(a.objs[j].Kind) {
				a.detect(j)
			}
		}
		if a.commit(d.before) || !d.wasSel {
			return
		}
		// A click, with no drag, on something already selected: out of a
		// group or a bigger selection it picks just that shape; on lone
		// text it goes back to typing.
		switch {
		case len(a.selected()) > 1:
			a.setSelection([]int{d.obj}, d.obj)
		case a.objs[d.obj].Kind == "text":
			a.startEditing(d.obj)
		case hasLabel(a.objs[d.obj].Kind) && a.objs[d.obj].inside(d.x0, d.y0):
			a.startEditing(d.obj) // a second click on a box's label
		}
	case dSelect:
		if d.moved() {
			a.selectIn(d.x0, d.y0, d.x1, d.y1, a.shift)
		} else if !a.shift {
			a.sel = -1
		}
	case dCreate:
		if !d.moved() && !a.widgetTool() && !a.shift {
			// A click inside a box types its label. Shift-click there for
			// loose text instead.
			if i := boxAt(a.objs, d.x0, d.y0); i >= 0 {
				a.startEditing(i)
				return
			}
		}
		if !d.moved() && !a.widgetTool() {
			// A click on empty space puts out the text cursor.
			a.typing, a.edit, a.typed = true, -1, false
			a.cx, a.cy = d.x0, d.y0
			return
		}
		a.objs = append(a.objs, a.newShape(d))
		a.sel = len(a.objs) - 1
		if n := len(a.objs) - 1; a.objs[n].Kind == "fill" { // shading belongs behind things
			a.objs = append([]object{a.objs[n]}, a.objs[:n]...)
			a.sel = 0
		}
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
		case dMove:
			x, y, w, h := selBounds(a.objs, a.selected())
			return fmt.Sprintf("x=%d y=%d w=%d h=%d", x, y, w, h)
		case dResize:
			x, y, w, h := a.objs[d.obj].bounds()
			return fmt.Sprintf("x=%d y=%d w=%d h=%d", x, y, w, h) + a.shiftMark(a.objs[d.obj])
		case dCreate:
			if d.moved() {
				o := a.newShape(d)
				x, y, w, h := o.bounds()
				return fmt.Sprintf("x=%d y=%d w=%d h=%d", x, y, w, h) + a.shiftMark(o)
			}
			fallthrough
		case dErase, dSelect:
			x0, x1 := order(d.x0, d.x1)
			y0, y1 := order(d.y0, d.y1)
			return fmt.Sprintf("x=%d y=%d w=%d h=%d", x0, y0, x1-x0+1, y1-y0+1)
		}
	}
	pos := fmt.Sprintf("x=%d y=%d", a.mx, a.my)
	if a.vx != 0 || a.vy != 0 {
		pos = fmt.Sprintf("view %d,%d  ", a.vx, a.vy) + pos
	}
	return pos
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
	case a.pressed >= 0 && a.isSel(i):
		st = st.Foreground(colPressed).Bold(true)
	case a.isSel(i):
		st = st.Foreground(colSel).Bold(true)
	case a.hovered(i) || i == a.snap:
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
	if a.typing && a.menu == nil { // keep the typing cursor in view
		cx, cy := a.cursor()
		a.vx = min(a.vx, cx)
		a.vx = max(a.vx, cx-cw+1)
		a.vy = min(a.vy, cy)
		a.vy = max(a.vy, cy-h+1)
	}
	for sy := 0; sy < h; sy++ {
		for sx := 0; sx < cw; sx++ {
			x, y := sx+a.vx, sy+a.vy
			st := a.objStyle(scene.owner(x, y), doomed)
			if d != nil && (d.mode == dErase || d.mode == dSelect) {
				x0, x1 := order(d.x0, d.x1)
				y0, y1 := order(d.y0, d.y1)
				if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
					if d.mode == dErase {
						st = st.Background(tcell.ColorMaroon)
					} else {
						st = st.Background(tcell.PaletteColor(24)) // selection box: blue
					}
				}
			}
			s.SetContent(sx, sy, scene.get(x, y), nil, st)
		}
	}
	// A selected oval shows its corners, since that is where it resizes.
	if sel := a.selected(); len(sel) == 1 && a.objs[sel[0]].Kind == "oval" {
		i := sel[0]
		o := a.objs[i]
		for _, p := range [][2]int{{o.X0, o.Y0}, {o.X1, o.Y0}, {o.X0, o.Y1}, {o.X1, o.Y1}} {
			px, py := p[0]-a.vx, p[1]-a.vy
			if px >= 0 && py >= 0 && px < cw && py < h && scene.get(p[0], p[1]) == ' ' {
				s.SetContent(px, py, '+', nil, a.objStyle(i, doomed))
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
		// Position readout, right-aligned. The message gets the room between.
		pos := a.readout()
		posX := max(sw-runeLen(pos)-1, 0)
		if room := posX - (x + 3) - 2; a.msg != "" && room > 1 {
			ms := bar.Foreground(tcell.ColorLightGreen).Bold(true)
			if a.msgErr {
				ms = bar.Foreground(tcell.ColorYellow).Bold(true)
			}
			msg := a.msg
			if runeLen(msg) > room {
				msg = string([]rune(msg)[:room-1]) + "…"
			}
			a.text(x+3, sh-2, msg, ms)
		}
		a.text(posX, sh-2, pos, bar.Foreground(tcell.ColorAqua).Bold(true))
		if a.typing && a.menu == nil {
			cx, cy := a.cursor()
			s.ShowCursor(cx-a.vx, cy-a.vy)
		}
	}
	if a.menu != nil {
		a.renderMenu()
	}
	if a.help {
		a.renderHelp()
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
	a.say("Drag to draw. Click to type. Press ? for every key.")
	if note != "" {
		a.say(note)
	}
	a.recover()

	// Autosave on a timer, and right away if the window closes.
	go func() {
		for range time.Tick(autosaveEvery) {
			s.PostEvent(tcell.NewEventInterrupt("autosave"))
		}
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGTERM)
	go func() {
		<-sigs
		s.PostEvent(tcell.NewEventInterrupt("quit"))
	}()

	for {
		a.render()
		switch ev := s.PollEvent().(type) {
		case *tcell.EventInterrupt:
			a.autosave()
			if ev.Data() == "quit" {
				return
			}
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
