package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func testAppSized(t *testing.T, w, h int) *app {
	t.Helper()
	s := tcell.NewSimulationScreen("")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(w, h)
	t.Cleanup(s.Fini)
	return newApp(s)
}

// testApp is too narrow for the panel, so the canvas is the whole width.
func testApp(t *testing.T) *app { return testAppSized(t, 40, 12) }

func press(a *app, x, y int, b tcell.ButtonMask) { a.mouse(tcell.NewEventMouse(x, y, b, 0)) }
func ctrl(a *app, k tcell.Key) bool              { return a.key(tcell.NewEventKey(k, 0, 0)) }
func typeStr(a *app, s string) {
	for _, r := range s {
		a.key(tcell.NewEventKey(tcell.KeyRune, r, 0))
	}
}

func drag2(a *app, x0, y0, x1, y1 int) {
	press(a, x0, y0, tcell.Button1)
	press(a, x1, y1, tcell.Button1)
	press(a, x1, y1, tcell.ButtonNone)
}

func click(a *app, x, y int) {
	press(a, x, y, tcell.Button1)
	press(a, x, y, tcell.ButtonNone)
}

func TestDrawTypeSave(t *testing.T) {
	a := testApp(t)
	press(a, 2, 1, tcell.Button1)
	press(a, 6, 3, tcell.Button1)
	press(a, 10, 4, tcell.Button1)
	a.render() // preview must not panic
	press(a, 10, 4, tcell.ButtonNone)
	click(a, 4, 2) // inside the box is empty space: starts typing
	typeStr(a, "Hi")
	if len(a.objs) != 2 || a.objs[1].Kind != "text" {
		t.Fatalf("want box + text, got %+v", a.objs)
	}

	dir := t.TempDir()
	ctrl(a, tcell.KeyCtrlS)
	if a.prompt == nil {
		t.Fatal("expected a save-as prompt for an untitled file")
	}
	typeStr(a, filepath.Join(dir, "mock"))
	ctrl(a, tcell.KeyEnter)
	a.render()
	if a.file != filepath.Join(dir, "mock.yaml") || a.dirty {
		t.Fatalf("file=%q dirty=%v", a.file, a.dirty)
	}

	// The YAML brings the objects back.
	objs, saveTo, note, err := loadDrawing(a.file)
	if err != nil || note != "" || saveTo != a.file || !reflect.DeepEqual(objs, a.objs) {
		t.Fatalf("reload: err=%v note=%q objs=%+v", err, note, objs)
	}

	// ^E writes mock.md next to it, no questions asked.
	ctrl(a, tcell.KeyCtrlE)
	got, err := os.ReadFile(filepath.Join(dir, "mock.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "```text\n\n  +-------+\n  | Hi    |\n  |       |\n  +-------+\n```\n"
	if string(got) != want {
		t.Fatalf("md got:\n%q\nwant:\n%q", got, want)
	}
	if a.prompt != nil || a.msg == "" {
		t.Fatalf("export should not prompt and should confirm: %q", a.msg)
	}

	// A second ^S saves to the same file without asking.
	drag2(a, 20, 1, 25, 1)
	ctrl(a, tcell.KeyCtrlS)
	if a.prompt != nil || a.dirty {
		t.Fatal("second save should not prompt")
	}
}

func TestYAMLFormat(t *testing.T) {
	objs := []object{
		{Kind: "box", X0: 0, Y0: 0, X1: 9, Y1: 3, Style: 2},
		{Kind: "arrow", X0: 10, Y0: 1, X1: 20, Y1: 4, Style: 1},
		{Kind: "text", X0: 2, Y0: 1, Text: []string{"  Hi: there", "#2"}},
	}
	data, err := encodeYAML(objs)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"kind: box", "style: round", "x0: 0", "kind: text", "x0: 2", "  - kind: box"} {
		if !strings.Contains(string(data), s) {
			t.Errorf("yaml lacks %q:\n%s", s, data)
		}
	}
	back, err := decodeYAML(data)
	if err != nil || !reflect.DeepEqual(back, objs) {
		t.Fatalf("round trip: %v\n%+v", err, back)
	}
	if _, err := decodeYAML([]byte("objects:\n  - kind: blob\n")); err == nil {
		t.Fatal("unknown kind should fail")
	}
}

func TestImport(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "ui.md")
	os.WriteFile(md, []byte("# Title\n\n````text\n+--+\n|ok|\n+--+\n````\nafter\n"), 0o644)
	objs, saveTo, note, err := loadDrawing(md)
	if err != nil || saveTo != "" || note == "" {
		t.Fatalf("err=%v saveTo=%q note=%q", err, saveTo, note)
	}
	if got := compose(objs).String(); got != "+--+\n|ok|\n+--+\n" {
		t.Fatalf("md import: %q", got)
	}
	// Fences inside the picture get a longer fence on export.
	if got := markdown("```\n"); got != "````text\n```\n````\n" {
		t.Fatalf("fence: %q", got)
	}
}

func TestTyping(t *testing.T) {
	a := testApp(t)
	click(a, 3, 3)
	typeStr(a, "ab")
	ctrl(a, tcell.KeyEnter)
	typeStr(a, "c")
	if got := a.objs[0].Text; !reflect.DeepEqual(got, []string{"ab", "c"}) {
		t.Fatalf("text: %q", got)
	}
	ctrl(a, tcell.KeyBackspace2)
	ctrl(a, tcell.KeyBackspace2)
	if got := a.objs[0].Text; !reflect.DeepEqual(got, []string{"ab"}) {
		t.Fatalf("after backspace: %q", got)
	}
	ctrl(a, tcell.KeyBackspace2)
	ctrl(a, tcell.KeyBackspace2)
	if len(a.objs) != 0 {
		t.Fatal("erasing all the text should drop the object")
	}
	typeStr(a, "x")
	if a.objs[0].X0 != 3 || a.objs[0].Y0 != 3 {
		t.Fatalf("new text should start where the old one did: %+v", a.objs[0])
	}
	ctrl(a, tcell.KeyEscape) // stop typing
	ctrl(a, tcell.KeyEscape) // deselect
	// One click selects the text, a second click goes back to typing in it.
	click(a, 3, 3)
	if a.typing || a.sel != 0 {
		t.Fatalf("first click: typing=%v sel=%d", a.typing, a.sel)
	}
	click(a, 3, 3)
	typeStr(a, "y")
	if got := a.objs[0].Text[0]; got != "xy" {
		t.Fatalf("second click should edit: %q", got)
	}
}

func TestMoveResizeUndo(t *testing.T) {
	a := testApp(t)
	drag2(a, 2, 1, 10, 4)
	box := a.objs[0]

	// Grab the top edge and move the box right 3, down 2.
	press(a, 5, 1, tcell.Button1)
	if a.pressed != 0 || a.sel != 0 {
		t.Fatalf("press on edge: pressed=%d sel=%d", a.pressed, a.sel)
	}
	press(a, 8, 3, tcell.Button1)
	if got := a.readout(); got != "x=5 y=3 w=9 h=4" {
		t.Fatalf("readout while moving: %q", got)
	}
	a.render()
	press(a, 8, 3, tcell.ButtonNone)
	if a.pressed != -1 || a.sel != 0 {
		t.Fatalf("release: pressed=%d sel=%d", a.pressed, a.sel)
	}
	if o := a.objs[0]; o.X0 != 5 || o.Y0 != 3 || o.X1 != 13 || o.Y1 != 6 {
		t.Fatalf("moved: %+v", o)
	}

	// Drag the bottom-right corner past the top-left: the box flips and stays tidy.
	drag2(a, 13, 6, 1, 0)
	if o := a.objs[0]; o.X0 != 1 || o.Y0 != 0 || o.X1 != 5 || o.Y1 != 3 {
		t.Fatalf("resized: %+v", o)
	}

	ctrl(a, tcell.KeyCtrlZ)
	ctrl(a, tcell.KeyCtrlZ)
	if !reflect.DeepEqual(a.objs[0], box) {
		t.Fatalf("two undos should restore the box: %+v", a.objs[0])
	}

	// Moving can't push a shape off the top-left.
	drag2(a, 2, 1, 0, 0)
	drag2(a, 2, 1, -5, -5) // clamped to the canvas anyway
	if x, y, _, _ := a.objs[0].bounds(); x < 0 || y < 0 {
		t.Fatalf("off canvas: %+v", a.objs[0])
	}
}

func TestKeysOnSelection(t *testing.T) {
	a := testApp(t)
	drag2(a, 0, 0, 4, 2)
	drag2(a, 6, 0, 9, 0)
	ctrl(a, tcell.KeyEscape)
	ctrl(a, tcell.KeyTab)
	if a.sel != 0 {
		t.Fatalf("Tab: sel=%d", a.sel)
	}
	ctrl(a, tcell.KeyRight)
	ctrl(a, tcell.KeyDown)
	if o := a.objs[0]; o.X0 != 1 || o.Y0 != 1 {
		t.Fatalf("nudge: %+v", o)
	}
	ctrl(a, tcell.KeyDelete)
	if len(a.objs) != 1 || a.objs[0].Kind != "box" || a.objs[0].Y1 != 0 {
		t.Fatalf("delete: %+v", a.objs)
	}
}

func TestPanel(t *testing.T) {
	a := testAppSized(t, 100, 24)
	if w, _ := a.canvasSize(); w != 100-panelW {
		t.Fatalf("canvas width %d", w)
	}
	drag2(a, 2, 1, 10, 4)
	drag2(a, 12, 1, 20, 1)
	ctrl(a, tcell.KeyEscape)
	px := 100 - panelW + 3

	// Hover row 2 (the line).
	press(a, px, 2, tcell.ButtonNone)
	if a.hover != 1 {
		t.Fatalf("hover row: %d", a.hover)
	}
	// Hover the box on the canvas: the panel row lights too.
	press(a, 2, 2, tcell.ButtonNone)
	if a.hover != 0 {
		t.Fatalf("hover canvas: %d", a.hover)
	}
	// Hold row 1: selected and pressed. Release: still selected, not pressed.
	press(a, px, 1, tcell.Button1)
	if a.sel != 0 || a.pressed != 0 {
		t.Fatalf("hold row: sel=%d pressed=%d", a.sel, a.pressed)
	}
	a.render()
	press(a, px, 1, tcell.ButtonNone)
	if a.sel != 0 || a.pressed != -1 {
		t.Fatalf("release row: sel=%d pressed=%d", a.sel, a.pressed)
	}
	if len(a.objs) != 2 {
		t.Fatal("clicks in the panel must not draw")
	}
	// ^O hides the panel and gives the space back.
	ctrl(a, tcell.KeyCtrlO)
	if w, _ := a.canvasSize(); w != 100 {
		t.Fatalf("hidden panel: width %d", w)
	}
}

func TestMenuPicksToolAndStyle(t *testing.T) {
	a := testApp(t)
	ctrl(a, tcell.KeyCtrlT)
	if a.menu == nil {
		t.Fatal("^T should open the menu")
	}
	a.render()
	ctrl(a, tcell.KeyDown) // Oval
	ctrl(a, tcell.KeyDown) // Line
	ctrl(a, tcell.KeyDown) // Arrow
	a.key(tcell.NewEventKey(tcell.KeyRune, ' ', 0))
	if a.tool != toolArrow || a.menu == nil {
		t.Fatalf("Space should pick and stay open: tool=%d", a.tool)
	}
	ctrl(a, tcell.KeyDown) // Erase
	ctrl(a, tcell.KeyDown) // Select
	ctrl(a, tcell.KeyDown) // ascii
	ctrl(a, tcell.KeyDown) // line
	if a.previewStyle() != 1 || a.style != 0 {
		t.Fatalf("preview=%d style=%d", a.previewStyle(), a.style)
	}
	a.render()
	ctrl(a, tcell.KeyEnter)
	if a.menu != nil || a.style != 1 || a.tool != toolArrow {
		t.Fatalf("Enter should pick and close: style=%d tool=%d", a.style, a.tool)
	}
	drag2(a, 1, 0, 5, 2)
	if got, want := a.ascii(), " ────┐\n     │\n     v\n"; got != want {
		t.Fatalf("arrow:\n%q\nwant\n%q", got, want)
	}
	// Drag the arrow's head to a new spot.
	drag2(a, 5, 2, 8, 0)
	if o := a.objs[0]; o.X1 != 8 || o.Y1 != 0 || o.X0 != 1 {
		t.Fatalf("arrow end: %+v", o)
	}
}

func TestLineDirections(t *testing.T) {
	st := styles[0]
	cases := []struct {
		x0, y0, x1, y1 int
		arrow          bool
		want           string
	}{
		{0, 0, 3, 0, true, "--->\n"},
		{3, 0, 0, 0, true, "<---\n"},
		{0, 0, 0, 2, false, "|\n|\n|\n"},
		{0, 2, 0, 0, true, "^\n|\n|\n"},
		{3, 0, 0, 2, false, "+---\n|\n|\n"},
	}
	for _, c := range cases {
		cv := &canvas{}
		drawLine(cv, c.x0, c.y0, c.x1, c.y1, st, c.arrow)
		if got := cv.String(); got != c.want {
			t.Errorf("%v: got %q want %q", c, got, c.want)
		}
	}
}

func TestMenuClickDoesNotDraw(t *testing.T) {
	a := testApp(t)
	ctrl(a, tcell.KeyCtrlT)
	press(a, 0, 0, tcell.Button1)
	press(a, 5, 3, tcell.Button1)
	press(a, 5, 3, tcell.ButtonNone)
	if a.menu != nil || len(a.objs) != 0 {
		t.Fatalf("menu=%v objs=%+v", a.menu, a.objs)
	}
	ctrl(a, tcell.KeyCtrlT)
	mx, my := a.menuItemPos(itemOfTool(toolLine))
	click(a, mx, my)
	if a.tool != toolLine {
		t.Fatalf("click should pick Line, got %d", a.tool)
	}
}

func TestErase(t *testing.T) {
	a := testApp(t)
	drag2(a, 0, 0, 4, 2)
	drag2(a, 10, 0, 14, 2)
	// Right-drag across the first box's edge deletes it, not the second.
	press(a, 3, 1, tcell.Button2)
	press(a, 4, 1, tcell.Button2)
	a.render()
	press(a, 4, 1, tcell.ButtonNone)
	if len(a.objs) != 1 || a.objs[0].X0 != 10 {
		t.Fatalf("right-drag erase: %+v", a.objs)
	}
	// Erase tool: a click deletes, and never starts typing.
	a.tool = toolErase
	click(a, 10, 0)
	if a.typing || len(a.objs) != 0 {
		t.Fatalf("typing=%v objs=%+v", a.typing, a.objs)
	}
}

func TestUndoAndNewConfirm(t *testing.T) {
	a := testApp(t)
	drag2(a, 1, 1, 5, 1)
	if a.ascii() != "\n -----\n" {
		t.Fatalf("line: %q", a.ascii())
	}
	ctrl(a, tcell.KeyCtrlZ)
	if a.ascii() != "" {
		t.Fatalf("undo: %q", a.ascii())
	}
	ctrl(a, tcell.KeyCtrlY)
	ctrl(a, tcell.KeyCtrlN)
	if a.ascii() == "" {
		t.Fatal("first ^N on a dirty file should only warn")
	}
	ctrl(a, tcell.KeyCtrlN)
	if a.ascii() != "" || a.file != "" || a.sel != -1 {
		t.Fatal("second ^N should clear")
	}
}

func TestOverwritePrompt(t *testing.T) {
	a := testApp(t)
	drag2(a, 0, 0, 4, 2)
	path := filepath.Join(t.TempDir(), "x.yaml")
	os.WriteFile(path, []byte("old\n"), 0o644)
	ctrl(a, tcell.KeyCtrlS)
	typeStr(a, path)
	ctrl(a, tcell.KeyEnter)
	if a.prompt == nil {
		t.Fatal("expected overwrite prompt")
	}
	typeStr(a, "n")
	ctrl(a, tcell.KeyEnter)
	if b, _ := os.ReadFile(path); string(b) != "old\n" {
		t.Fatal("answering n must not overwrite")
	}
}

func TestReadout(t *testing.T) {
	a := testApp(t)
	press(a, 7, 3, tcell.ButtonNone)
	if got := a.readout(); got != "x=7 y=3" {
		t.Fatalf("hover: %q", got)
	}
	press(a, 10, 5, tcell.Button1)
	press(a, 2, 1, tcell.Button1)
	if got := a.readout(); got != "x=2 y=1 w=9 h=5" {
		t.Fatalf("drag: %q", got)
	}
	a.render()
	press(a, 2, 1, tcell.ButtonNone)
	if got := a.readout(); got != "x=2 y=1" {
		t.Fatalf("after release: %q", got)
	}
}

func TestOvalSymmetric(t *testing.T) {
	for _, r := range [][4]int{{0, 0, 20, 6}, {0, 0, 9, 3}, {0, 0, 24, 12}, {0, 0, 7, 2}} {
		c := &canvas{}
		drawOval(c, r[0], r[1], r[2], r[3], styles[0])
		mirror := map[rune]rune{'/': '\\', '\\': '/', '-': '-', '|': '|', ' ': ' '}
		for y := r[1]; y <= r[3]; y++ {
			for x := r[0]; x <= r[2]; x++ {
				// Left-right mirror swaps / and \; top-bottom mirror does too.
				if got, want := c.get(r[2]-x, y), mirror[c.get(x, y)]; got != want {
					t.Fatalf("%v: not mirrored at %d,%d:\n%s", r, x, y, c.String())
				}
				if got, want := c.get(x, r[3]-(y-r[1])), mirror[c.get(x, y)]; got != want {
					t.Fatalf("%v: not flipped at %d,%d:\n%s", r, x, y, c.String())
				}
			}
		}
	}
}

func TestShiftCircleAndSquare(t *testing.T) {
	a := testAppSized(t, 100, 30)
	a.tool = toolOval
	press(a, 2, 2, tcell.Button1)
	a.mouse(tcell.NewEventMouse(15, 4, tcell.Button1, tcell.ModShift))
	if got := a.readout(); got != "x=2 y=2 w=15 h=8 ⇧" {
		t.Fatalf("readout: %q", got)
	}
	a.mouse(tcell.NewEventMouse(15, 4, tcell.ButtonNone, tcell.ModShift))
	if o := a.objs[0]; o.Kind != "oval" || o.X1-o.X0 != 2*(o.Y1-o.Y0) {
		t.Fatalf("circle should be twice as wide as tall: %+v", o)
	}
	// Up and to the left, near the edge: stays on the canvas.
	a.tool = toolBox
	press(a, 30, 3, tcell.Button1)
	a.mouse(tcell.NewEventMouse(10, 0, tcell.Button1, tcell.ModShift))
	a.mouse(tcell.NewEventMouse(10, 0, tcell.ButtonNone, tcell.ModShift))
	if o := a.objs[1]; o.X0 < 0 || o.Y0 < 0 || o.X1-o.X0 != 2*(o.Y1-o.Y0) {
		t.Fatalf("square: %+v", o)
	}
	// Without Shift the shape follows the mouse.
	press(a, 40, 10, tcell.Button1)
	press(a, 60, 12, tcell.Button1)
	press(a, 60, 12, tcell.ButtonNone)
	if o := a.objs[2]; o.X1 != 60 || o.Y1 != 12 {
		t.Fatalf("free box: %+v", o)
	}
}

func TestOvalCornerResize(t *testing.T) {
	a := testAppSized(t, 100, 30)
	a.tool = toolOval
	drag2(a, 2, 2, 22, 8)
	if a.sel != 0 {
		t.Fatal("new oval should be selected")
	}
	a.render() // draws the corner handles
	if r, _, _, _ := a.s.(tcell.SimulationScreen).GetContent(22, 8); r != '+' {
		t.Fatalf("corner handle not drawn: %q", r)
	}
	// Its bottom-right corner is an empty cell, but it is a handle.
	drag2(a, 22, 8, 30, 12)
	if o := a.objs[0]; len(a.objs) != 1 || o.X1 != 30 || o.Y1 != 12 {
		t.Fatalf("resize: %+v (%d objects)", o, len(a.objs))
	}
	// And it saves.
	back, err := decodeYAML(must(encodeYAML(a.objs)))
	if err != nil || !reflect.DeepEqual(back, a.objs) {
		t.Fatalf("yaml: %v %+v", err, back)
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

// fakeClipboard stands in for pbcopy/pbpaste during a test.
func fakeClipboard(t *testing.T) *string {
	t.Helper()
	var board string
	oldW, oldR := clipWrite, clipRead
	clipWrite = func(s string) error { board = s; return nil }
	clipRead = func() (string, error) { return board, nil }
	t.Cleanup(func() { clipWrite, clipRead = oldW, oldR })
	return &board
}

func TestCopyPaste(t *testing.T) {
	board := fakeClipboard(t)
	a := testApp(t)
	ctrl(a, tcell.KeyCtrlC)
	if !a.msgErr {
		t.Fatal("^C with nothing selected should say so")
	}
	drag2(a, 2, 1, 6, 3) // selected after drawing
	ctrl(a, tcell.KeyCtrlC)
	if *board != "+---+\n|   |\n+---+\n" {
		t.Fatalf("system clipboard: %q", *board)
	}
	// Paste at the mouse.
	press(a, 20, 4, tcell.ButtonNone)
	ctrl(a, tcell.KeyCtrlV)
	if len(a.objs) != 2 || a.sel != 1 {
		t.Fatalf("paste: %+v sel=%d", a.objs, a.sel)
	}
	if o := a.objs[1]; o.X0 != 20 || o.Y0 != 4 || o.X1 != 24 || o.Y1 != 6 {
		t.Fatalf("pasted at the wrong place: %+v", o)
	}
	ctrl(a, tcell.KeyCtrlZ)
	if len(a.objs) != 1 {
		t.Fatal("undo should remove the paste")
	}
	// Cut: gone from the canvas, still pasteable.
	ctrl(a, tcell.KeyTab)
	ctrl(a, tcell.KeyCtrlX)
	if len(a.objs) != 0 {
		t.Fatal("cut should remove the shape")
	}
	ctrl(a, tcell.KeyCtrlV)
	if len(a.objs) != 1 || a.objs[0].Kind != "box" {
		t.Fatalf("paste after cut: %+v", a.objs)
	}
}

func TestPasteOutsideText(t *testing.T) {
	board := fakeClipboard(t)
	a := testApp(t)
	drag2(a, 0, 0, 4, 2)
	ctrl(a, tcell.KeyCtrlC)
	// Something else lands on the clipboard: ^V pastes it as text.
	*board = "hello\r\n\tworld\n"
	press(a, 8, 1, tcell.ButtonNone)
	ctrl(a, tcell.KeyCtrlV)
	o := a.objs[len(a.objs)-1]
	if o.Kind != "text" || o.X0 != 8 || o.Y0 != 1 || !reflect.DeepEqual(o.Text, []string{"hello", "    world"}) || a.typing {
		t.Fatalf("text paste: %+v typing=%v", o, a.typing)
	}
	// While typing, it goes into the text.
	click(a, 20, 5)
	typeStr(a, "a")
	*board = "bc"
	ctrl(a, tcell.KeyCtrlV)
	if got := a.objs[len(a.objs)-1].Text; !reflect.DeepEqual(got, []string{"abc"}) || !a.typing {
		t.Fatalf("paste while typing: %q", got)
	}
}

func TestYankPut(t *testing.T) {
	fakeClipboard(t)
	a := testApp(t)
	drag2(a, 2, 1, 6, 3)
	typeStr(a, "y")
	press(a, 20, 4, tcell.ButtonNone)
	typeStr(a, "p")
	if len(a.objs) != 2 || a.objs[1].X0 != 20 || a.objs[1].Kind != "box" {
		t.Fatalf("y/p: %+v", a.objs)
	}
	// While typing, y and p are just letters.
	click(a, 30, 6)
	typeStr(a, "yp")
	if o := a.objs[len(a.objs)-1]; o.Kind != "text" || o.Text[0] != "yp" || len(a.objs) != 3 {
		t.Fatalf("typing yp: %+v", a.objs)
	}
}

func TestCutAndDeleteKeys(t *testing.T) {
	board := fakeClipboard(t)
	a := testApp(t)
	drag2(a, 0, 0, 4, 2)
	drag2(a, 10, 0, 14, 2)
	typeStr(a, "d") // cut the second box
	if len(a.objs) != 1 || *board == "" {
		t.Fatalf("d: %+v board=%q", a.objs, *board)
	}
	ctrl(a, tcell.KeyTab)
	typeStr(a, "x") // delete the first, clipboard untouched
	if len(a.objs) != 0 || a.clip[0].X1 != 4 {
		t.Fatalf("x: %+v clip=%+v", a.objs, a.clip)
	}
	press(a, 3, 3, tcell.ButtonNone)
	typeStr(a, "p") // the cut box comes back
	if len(a.objs) != 1 || a.objs[0].X0 != 3 {
		t.Fatalf("p after d and x: %+v", a.objs)
	}
	ctrl(a, tcell.KeyEscape)
	typeStr(a, "x")
	if !a.msgErr {
		t.Fatal("x with nothing selected should say so")
	}
}

func TestExportAsksBeforeOverwrite(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "ui.md")
	os.WriteFile(md, []byte("my notes\n"), 0o644)
	a := testApp(t)
	a.file = filepath.Join(dir, "ui.yaml")
	drag2(a, 0, 0, 4, 2)

	ctrl(a, tcell.KeyCtrlE)
	if a.prompt == nil {
		t.Fatal("^E should ask before replacing ui.md")
	}
	typeStr(a, "n")
	ctrl(a, tcell.KeyEnter)
	if b, _ := os.ReadFile(md); string(b) != "my notes\n" {
		t.Fatal("answering n must leave ui.md alone")
	}

	ctrl(a, tcell.KeyCtrlE)
	typeStr(a, "y")
	ctrl(a, tcell.KeyEnter)
	if b, _ := os.ReadFile(md); !strings.Contains(string(b), "+---+") {
		t.Fatalf("answering y should export: %q", b)
	}

	// Now it is this drawing's export: the next ^E doesn't ask.
	drag2(a, 10, 0, 14, 2)
	ctrl(a, tcell.KeyCtrlE)
	if a.prompt != nil {
		t.Fatal("second export to the same file should not ask")
	}
	// Saved under a new name, ^E follows it.
	ctrl(a, tcell.KeyCtrlS) // saves to ui.yaml
	a.file = ""
	ctrl(a, tcell.KeyCtrlS)
	typeStr(a, filepath.Join(dir, "other"))
	ctrl(a, tcell.KeyEnter)
	ctrl(a, tcell.KeyCtrlE)
	if _, err := os.Stat(filepath.Join(dir, "other.md")); err != nil {
		t.Fatalf("export should follow the new name: %v", err)
	}
}

func useTool(a *app, kind string) {
	for i, t := range tools {
		if t.kind == kind {
			a.tool = i
			return
		}
	}
	panic("no tool " + kind)
}

func TestWidgetsStampAndDraw(t *testing.T) {
	a := testAppSized(t, 100, 30)
	useTool(a, "button")
	click(a, 0, 0)
	useTool(a, "checkbox")
	click(a, 10, 0)
	useTool(a, "input")
	drag2(a, 0, 2, 11, 5) // only the width counts for an input
	useTool(a, "progress")
	click(a, 0, 4)
	if got, want := a.ascii(), "[ OK ]    [x] Remember me\n\n[__________]\n\n[#####........]  40%\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if a.typing {
		t.Fatal("a click with a widget tool stamps a widget, it doesn't start typing")
	}
}

func TestTabsLine(t *testing.T) {
	c := &canvas{}
	o := object{Kind: "tabs", Text: []string{"A", "BB", "C"}, Value: 1}
	o.normalize()
	drawWidget(c, o, styles[1])
	if got, want := c.String(), "│ A │ BB │ C │\n└───┘    └───┘\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	o.Value = 0
	c = &canvas{}
	drawWidget(c, o, styles[1])
	if got, want := c.String(), "│ A │ BB │ C │\n│   └────┴───┘\n"; got != want {
		t.Fatalf("active first:\n%s\nwant:\n%s", got, want)
	}
}

func TestWidgetEditAndToggle(t *testing.T) {
	a := testAppSized(t, 100, 30)
	useTool(a, "button")
	click(a, 0, 0)
	ctrl(a, tcell.KeyEnter) // selected after stamping: edit its label
	if a.prompt == nil || string(a.prompt.buf) != "OK" {
		t.Fatalf("expected a label prompt with OK: %+v", a.prompt)
	}
	for range 2 {
		ctrl(a, tcell.KeyBackspace2)
	}
	typeStr(a, "Sign in")
	ctrl(a, tcell.KeyEnter)
	if got := a.ascii(); got != "[ Sign in ]\n" {
		t.Fatalf("relabel should widen the button: %q", got)
	}

	useTool(a, "checkbox")
	click(a, 0, 2)
	typeStr(a, " ")
	if a.objs[1].Value != 0 {
		t.Fatal("Space should uncheck")
	}

	useTool(a, "tabs")
	click(a, 0, 4)
	typeStr(a, " ")
	if a.objs[2].Value != 1 {
		t.Fatal("Space should move to the next tab")
	}
	ctrl(a, tcell.KeyEnter)
	for range len(a.prompt.buf) {
		ctrl(a, tcell.KeyBackspace2)
	}
	typeStr(a, "One, Two")
	ctrl(a, tcell.KeyEnter)
	if o := a.objs[2]; !reflect.DeepEqual(o.Text, []string{"One", "Two"}) || o.Value != 1 {
		t.Fatalf("tabs: %+v", o)
	}

	useTool(a, "progress")
	click(a, 0, 7)
	ctrl(a, tcell.KeyEnter)
	for range len(a.prompt.buf) {
		ctrl(a, tcell.KeyBackspace2)
	}
	typeStr(a, "lots")
	ctrl(a, tcell.KeyEnter)
	if !a.msgErr || a.objs[3].Value != 40 {
		t.Fatal("a bad percent should be refused")
	}
	ctrl(a, tcell.KeyEnter)
	ctrl(a, tcell.KeyBackspace2)
	ctrl(a, tcell.KeyBackspace2)
	typeStr(a, "250%")
	ctrl(a, tcell.KeyEnter)
	if a.objs[3].Value != 100 {
		t.Fatalf("percent should clamp to 100: %d", a.objs[3].Value)
	}
	ctrl(a, tcell.KeyCtrlZ)
	if a.objs[3].Value != 40 {
		t.Fatal("undo should restore the percent")
	}
}

func TestWidgetShapesAndFiles(t *testing.T) {
	a := testAppSized(t, 100, 30)
	// A checkbox keeps its size however you drag it.
	useTool(a, "checkbox")
	drag2(a, 0, 0, 40, 6)
	if o := a.objs[0]; o.Y1 != 0 || o.X1 != 3+runeLen("Remember me") {
		t.Fatalf("checkbox: %+v", o)
	}
	// A scrollbar dragged wide turns horizontal.
	useTool(a, "scrollbar")
	drag2(a, 0, 2, 20, 3)
	if o := a.objs[1]; o.Y0 != o.Y1 || o.X1 != 20 {
		t.Fatalf("scrollbar: %+v", o)
	}
	// A window's body is see-through: a click inside it starts typing.
	useTool(a, "window")
	click(a, 0, 5)
	a.tool = toolBox
	click(a, 5, 9)
	if !a.typing {
		t.Fatal("clicking inside a window should type, not select it")
	}
	ctrl(a, tcell.KeyEscape)
	for _, k := range []string{"button", "radio", "input", "dropdown", "tabs", "progress"} {
		useTool(a, k)
		click(a, 40, 2*len(a.objs))
	}
	back, err := decodeYAML(must(encodeYAML(a.objs)))
	if err != nil || !reflect.DeepEqual(back, a.objs) {
		t.Fatalf("yaml round trip: %v\n%+v\n%+v", err, back, a.objs)
	}
	if !strings.Contains(string(must(encodeYAML(a.objs))), "kind: checkbox") {
		t.Fatal("yaml should name the widget kind")
	}
}

func TestMenuWidgetsColumn(t *testing.T) {
	a := testAppSized(t, 100, 30)
	ctrl(a, tcell.KeyCtrlT)
	ctrl(a, tcell.KeyRight)
	if a.previewTool() != shapeTools {
		t.Fatalf("Right should jump to the first widget: %d", a.previewTool())
	}
	ctrl(a, tcell.KeyLeft)
	if a.previewTool() != 0 {
		t.Fatal("Left should jump back to the first tool")
	}
	x, y := a.menuItemPos(itemOfTool(shapeTools + 2))
	click(a, x, y)
	if tools[a.tool].kind != "radio" {
		t.Fatalf("clicking the Radio row should pick it: %q", tools[a.tool].kind)
	}
	a.render()
}

func shiftClick(a *app, x, y int) {
	a.mouse(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModShift))
	a.mouse(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModShift))
}

// threeBoxes draws boxes at x=0, 10 and 20 and leaves nothing selected.
func threeBoxes(t *testing.T) *app {
	a := testAppSized(t, 100, 30)
	for _, x := range []int{0, 10, 20} {
		drag2(a, x, 0, x+4, 2)
	}
	ctrl(a, tcell.KeyEscape)
	return a
}

func TestShiftClickAndMoveMany(t *testing.T) {
	a := threeBoxes(t)
	click(a, 0, 0)
	shiftClick(a, 10, 0)
	if got := a.selected(); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("shift-click: %v", got)
	}
	shiftClick(a, 10, 0) // and out again
	shiftClick(a, 10, 0)

	// Drag one of them: both move, the third stays.
	drag2(a, 2, 0, 2, 5)
	if a.objs[0].Y0 != 5 || a.objs[1].Y0 != 5 || a.objs[2].Y0 != 0 {
		t.Fatalf("move many: %+v", a.objs)
	}
	ctrl(a, tcell.KeyLeft) // box 0 is at the left edge, so nothing moves
	if a.objs[1].X0 != 10 {
		t.Fatal("nudge must not push the selection off the left edge")
	}
	ctrl(a, tcell.KeyRight)
	if a.objs[0].X0 != 1 || a.objs[1].X0 != 11 {
		t.Fatalf("nudge many: %+v", a.objs)
	}
	ctrl(a, tcell.KeyEnter)
	if !a.msgErr {
		t.Fatal("Enter with two shapes selected should say to pick one")
	}
	typeStr(a, "x")
	if len(a.objs) != 1 || a.objs[0].X0 != 20 {
		t.Fatalf("x should delete both: %+v", a.objs)
	}
	ctrl(a, tcell.KeyCtrlZ)
	if len(a.objs) != 3 {
		t.Fatal("one undo should bring both back")
	}
}

func TestSelectTool(t *testing.T) {
	a := threeBoxes(t)
	typeStr(a, "v")
	if a.tool != toolSelect {
		t.Fatalf("v should pick Select: %d", a.tool)
	}
	// A box around the first two, and partly over the third. It starts on
	// empty space: starting on a shape would grab that shape instead.
	drag2(a, 23, 3, 0, 0)
	if got := a.selected(); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("select box: %v", got)
	}
	// Shift adds; a plain click on empty space clears.
	a.mouse(tcell.NewEventMouse(19, 0, tcell.Button1, tcell.ModShift))
	a.mouse(tcell.NewEventMouse(30, 5, tcell.Button1, tcell.ModShift))
	a.mouse(tcell.NewEventMouse(30, 5, tcell.ButtonNone, tcell.ModShift))
	if len(a.selected()) != 3 {
		t.Fatalf("shift select box: %v", a.selected())
	}
	click(a, 50, 20)
	if len(a.selected()) != 0 || a.typing || len(a.objs) != 3 {
		t.Fatal("a click on empty space with Select clears, and draws nothing")
	}
	typeStr(a, "v")
	if a.tool != toolBox {
		t.Fatal("v again goes back to the last tool")
	}
	ctrl(a, tcell.KeyCtrlA)
	if len(a.selected()) != 3 {
		t.Fatal("^A selects everything")
	}
}

func TestGroups(t *testing.T) {
	a := threeBoxes(t)
	typeStr(a, "g")
	if !a.msgErr {
		t.Fatal("g with nothing selected should explain")
	}
	click(a, 0, 0)
	shiftClick(a, 10, 0)
	typeStr(a, "g")
	if a.objs[0].Group == 0 || a.objs[0].Group != a.objs[1].Group || a.objs[2].Group != 0 {
		t.Fatalf("group: %+v", a.objs)
	}
	ctrl(a, tcell.KeyEscape)

	// One click on a member selects the group; a second picks just it.
	click(a, 10, 0)
	if got := a.selected(); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("click a member: %v", got)
	}
	click(a, 10, 0)
	if got := a.selected(); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("second click: %v", got)
	}
	// Dragging a member moves the group.
	ctrl(a, tcell.KeyEscape)
	drag2(a, 12, 0, 12, 3)
	if a.objs[0].Y0 != 3 || a.objs[1].Y0 != 3 {
		t.Fatalf("move group: %+v", a.objs)
	}

	// Copy and paste: the copy is a new group of its own.
	fakeClipboard(t)
	typeStr(a, "y")
	press(a, 40, 10, tcell.ButtonNone)
	typeStr(a, "p")
	g0, g1 := a.objs[0].Group, a.objs[3].Group
	if len(a.objs) != 5 || g1 == 0 || g1 == g0 || a.objs[4].Group != g1 || a.objs[3].X0 != 40 {
		t.Fatalf("paste group: %+v", a.objs)
	}
	// It survives a save.
	back, err := decodeYAML(must(encodeYAML(a.objs)))
	if err != nil || !reflect.DeepEqual(back, a.objs) {
		t.Fatalf("yaml: %v %+v", err, back)
	}

	// g on a whole group ungroups it.
	click(a, 0, 3)
	typeStr(a, "g")
	if a.objs[0].Group != 0 || a.objs[1].Group != 0 {
		t.Fatalf("ungroup: %+v", a.objs)
	}
	a.render()
}

func TestPanelShiftClick(t *testing.T) {
	a := threeBoxes(t)
	px := 100 - panelW + 3
	click(a, px, 1)
	a.mouse(tcell.NewEventMouse(px, 3, tcell.Button1, tcell.ModShift))
	if a.pressed != 2 {
		t.Fatalf("held row: %d", a.pressed)
	}
	a.render()
	a.mouse(tcell.NewEventMouse(px, 3, tcell.ButtonNone, tcell.ModShift))
	if got := a.selected(); !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("panel shift-click: %v", got)
	}
}

func TestStatusMessageFits(t *testing.T) {
	a := testAppSized(t, 60, 12)
	a.say(strings.Repeat("long message ", 10))
	a.render()
	s := a.s.(tcell.SimulationScreen)
	cells, w, h := s.GetContents()
	var b strings.Builder
	for x := 0; x < w; x++ {
		if r := cells[(h-2)*w+x].Runes; len(r) > 0 {
			b.WriteRune(r[0])
		}
	}
	line := strings.TrimRight(b.String(), " ")
	if !strings.HasSuffix(line, "x=0 y=0") || !strings.Contains(line, "…") {
		t.Fatalf("status line: %q", line)
	}
}

func TestHoverLightsGroup(t *testing.T) {
	a := threeBoxes(t)
	click(a, 0, 0)
	shiftClick(a, 10, 0)
	typeStr(a, "g")
	ctrl(a, tcell.KeyEscape)
	press(a, 10, 1, tcell.ButtonNone) // hover the second box's edge
	a.render()
	sim := a.s.(tcell.SimulationScreen)
	color := func(x, y int) tcell.Color {
		_, _, st, _ := sim.GetContent(x, y)
		fg, _, _ := st.Decompose()
		return fg
	}
	if color(0, 1) != colHover || color(10, 1) != colHover {
		t.Fatalf("both grouped boxes should light: %v %v", color(0, 1), color(10, 1))
	}
	if color(20, 1) == colHover {
		t.Fatal("the box outside the group should not light")
	}
}

// joined draws box A (0,0)-(10,4), box B (30,0)-(40,4) and an arrow from
// just right of A to just left of B.
func joined(t *testing.T) *app {
	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 10, 4)
	drag2(a, 30, 0, 40, 4)
	a.tool = toolArrow
	drag2(a, 11, 2, 29, 2)
	a.tool = toolBox
	ctrl(a, tcell.KeyEscape)
	return a
}

func TestArrowAttaches(t *testing.T) {
	a := joined(t)
	ar := a.objs[2]
	if ar.From.ID != a.objs[0].ID || ar.From.Side != 'r' || ar.To.ID != a.objs[1].ID || ar.To.Side != 'l' {
		t.Fatalf("anchors: %+v", ar)
	}
	// Move B down 4: the arrow's head follows, and bends halfway.
	drag2(a, 35, 0, 35, 4)
	if o := a.objs[2]; o.X1 != 29 || o.Y1 != 6 || o.X0 != 11 || o.Y0 != 2 {
		t.Fatalf("after move: %+v", o)
	}
	if got := a.objs[2].route(); !reflect.DeepEqual(got, [][2]int{{11, 2}, {20, 2}, {20, 6}, {29, 6}}) {
		t.Fatalf("route: %v", got)
	}
	// One undo puts both back.
	ctrl(a, tcell.KeyCtrlZ)
	if a.objs[1].Y0 != 0 || a.objs[2].Y1 != 2 {
		t.Fatalf("undo: %+v %+v", a.objs[1], a.objs[2])
	}
	// Nudging A moves the arrow's tail too.
	click(a, 5, 0)
	ctrl(a, tcell.KeyDown)
	if a.objs[2].Y0 != 3 {
		t.Fatalf("nudge: %+v", a.objs[2])
	}
}

func TestArrowAttachResizeAndDelete(t *testing.T) {
	a := joined(t)
	// Shrink B from the top to rows 3-4. The anchor sat 2 rows down its
	// left side, which no longer fits, so the end moves to the lowest row
	// still on the side.
	click(a, 35, 0)
	drag2(a, 30, 0, 30, 3)
	if o := a.objs[2]; o.X1 != 29 || o.Y1 != 4 {
		t.Fatalf("after resize: %+v", o)
	}
	// Delete A: the arrow stays and lets go of A.
	click(a, 5, 0)
	typeStr(a, "x")
	ar := a.objs[1]
	if len(a.objs) != 2 || ar.Kind != "arrow" || ar.From.ID != 0 || ar.To.ID == 0 || ar.X0 != 11 {
		t.Fatalf("after delete: %+v", a.objs)
	}
}

func TestArrowLetsGoWhenMovedAlone(t *testing.T) {
	a := joined(t)
	drag2(a, 20, 2, 20, 10) // grab the arrow's middle and move it down
	if o := a.objs[2]; o.From.ID != 0 || o.To.ID != 0 || o.Y0 != 10 {
		t.Fatalf("moved alone should let go: %+v", o)
	}
	ctrl(a, tcell.KeyCtrlZ)
	// Moved with both its boxes, it stays attached.
	ctrl(a, tcell.KeyCtrlA)
	drag2(a, 5, 0, 5, 6)
	if o := a.objs[2]; o.From.ID == 0 || o.To.ID == 0 || o.Y0 != 8 {
		t.Fatalf("moved together should stay attached: %+v", o)
	}
}

func TestDragLineEndToAttach(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 30, 0, 40, 4)
	a.tool = toolLine
	drag2(a, 0, 10, 10, 10) // a loose line
	if a.objs[1].To.ID != 0 {
		t.Fatal("a line away from shapes is not attached")
	}
	// Drag its end to just under the box: the box lights up, then it attaches.
	press(a, 10, 10, tcell.Button1)
	press(a, 35, 5, tcell.Button1)
	if a.snap != 0 {
		t.Fatalf("the box should light up as a target: %d", a.snap)
	}
	press(a, 35, 5, tcell.ButtonNone)
	if o := a.objs[1]; o.To.ID != a.objs[0].ID || o.To.Side != 'b' || a.snap != -1 {
		t.Fatalf("attach by dragging: %+v snap=%d", o, a.snap)
	}
}

func TestAttachmentsSaveAndPaste(t *testing.T) {
	fakeClipboard(t)
	a := joined(t)
	back, err := decodeYAML(must(encodeYAML(a.objs)))
	if err != nil || !reflect.DeepEqual(back, a.objs) {
		t.Fatalf("yaml: %v\n%+v\n%+v", err, back, a.objs)
	}
	if !strings.Contains(string(must(encodeYAML(a.objs))), "side: right") {
		t.Fatal("yaml should name the side")
	}
	// Copy B and the arrow: the pasted arrow keeps its end on the pasted B
	// and lets go of A, which wasn't copied.
	click(a, 35, 0)
	shiftClick(a, 20, 2)
	typeStr(a, "y")
	press(a, 50, 10, tcell.ButtonNone)
	typeStr(a, "p")
	nb, na := a.objs[3], a.objs[4]
	if nb.Kind != "box" || na.Kind != "arrow" {
		nb, na = na, nb
	}
	if na.From.ID != 0 || na.To.ID != nb.ID || nb.ID == a.objs[1].ID {
		t.Fatalf("paste: box %+v arrow %+v", nb, na)
	}
}

func TestNudgeAttachedLine(t *testing.T) {
	a := joined(t)
	click(a, 20, 2) // the arrow alone
	ctrl(a, tcell.KeyDown)
	if o := a.objs[2]; o.Y0 != 3 || o.Y1 != 3 || o.From.ID == 0 || o.To.ID == 0 {
		t.Fatalf("nudged along the sides, it moves and stays attached: %+v", o)
	}
	ctrl(a, tcell.KeyRight)
	if o := a.objs[2]; o.X0 != 12 || o.From.ID != 0 || o.To.ID == 0 {
		t.Fatalf("nudged away from A, its tail lets go: %+v", o)
	}
}
