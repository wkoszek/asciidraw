package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestMain keeps every test away from the real recovery folder, so a test
// that saves, quits or starts a new drawing can't delete real autosaves.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "asciidraw-test-recover")
	if err != nil {
		panic(err)
	}
	recoverDir = func() string { return dir }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

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
	click(a, 4, 2) // inside the box: types the box's label
	typeStr(a, "Hi")
	if len(a.objs) != 1 || !reflect.DeepEqual(a.objs[0].Text, []string{"Hi"}) {
		t.Fatalf("want a box labeled Hi, got %+v", a.objs)
	}

	dir := t.TempDir()
	ctrl(a, tcell.KeyCtrlS)
	if a.prompt == nil {
		t.Fatal("expected a save-as prompt for an untitled file")
	}
	typeStr(a, filepath.Join(dir, "mock"))
	ctrl(a, tcell.KeyEnter)
	a.render()
	if a.file != filepath.Join(dir, "mock.adraw") || a.dirty {
		t.Fatalf("a new drawing is saved as .adraw: file=%q dirty=%v", a.file, a.dirty)
	}

	// The YAML brings the objects back.
	pages, _, saveTo, note, err := loadDrawing(a.file)
	if err != nil || note != "" || saveTo != a.file || len(pages) != 1 || !reflect.DeepEqual(pages[0], a.objs) {
		t.Fatalf("reload: err=%v note=%q pages=%+v", err, note, pages)
	}

	// ^E writes mock.md next to it, no questions asked.
	ctrl(a, tcell.KeyCtrlE)
	got, err := os.ReadFile(filepath.Join(dir, "mock.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "```text\n\n  +-------+\n  |  Hi   |\n  |       |\n  +-------+\n```\n"
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
	pages, _, saveTo, note, err := loadDrawing(md)
	objs := pages[0]
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
	ctrl(a, tcell.KeyDown) // Fill
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

func TestCycleStyle(t *testing.T) {
	a := threeBoxes(t)
	typeStr(a, "t")
	if a.style != 1 || a.objs[0].Style != 0 {
		t.Fatal("with nothing selected, t changes the style for new shapes")
	}
	click(a, 0, 0)
	shiftClick(a, 10, 0)
	typeStr(a, "t")
	if a.objs[0].Style != 1 || a.objs[1].Style != 1 || a.objs[2].Style != 0 {
		t.Fatalf("t restyles the selection: %+v", a.objs)
	}
	for range 3 {
		typeStr(a, "t")
	}
	if a.objs[0].Style != 0 {
		t.Fatal("after double comes ascii again")
	}
	if !strings.HasPrefix(a.ascii(), "+---+") {
		t.Fatalf("drawn in ascii again: %q", a.ascii())
	}
	ctrl(a, tcell.KeyCtrlZ)
	if a.objs[0].Style != 3 {
		t.Fatal("each t is one undo step")
	}
}

func TestBoxLabel(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 12, 4)
	click(a, 3, 2)
	typeStr(a, "OK")
	ctrl(a, tcell.KeyEscape)
	if got, want := a.ascii(), "+-----------+\n|           |\n|    OK     |\n|           |\n+-----------+\n"; got != want {
		t.Fatalf("centered:\n%s\nwant:\n%s", got, want)
	}
	// The label moves with the box.
	drag2(a, 5, 0, 5, 3)
	if a.objs[0].Y0 != 3 || len(a.objs) != 1 || !strings.Contains(a.ascii(), "|    OK     |") {
		t.Fatalf("move:\n%s", a.ascii())
	}
	// A second click on the label goes back to typing in it.
	ctrl(a, tcell.KeyEscape)
	click(a, 5, 5) // selects the box (the click lands on the label's O)
	click(a, 5, 5)
	if !a.typing || a.edit != 0 {
		t.Fatalf("second click on the label should edit: typing=%v edit=%d", a.typing, a.edit)
	}
	typeStr(a, " then a much longer label")
	ctrl(a, tcell.KeyEscape)
	o := a.objs[0]
	if w := o.X1 - o.X0 + 1; w != runeLen(o.Text[0])+4 {
		t.Fatalf("the box should grow to fit: width %d for %q", w, o.Text[0])
	}
	// It can't be dragged smaller than its label.
	click(a, o.X1, o.Y1)
	drag2(a, o.X1, o.Y1, 5, 4)
	if a.objs[0].X1 != o.X1 || a.objs[0].Y1 != o.Y0+2 {
		t.Fatalf("shrink below the label: %+v", a.objs[0])
	}
	// Backspacing everything clears the label and keeps the box.
	ctrl(a, tcell.KeyEnter)
	for range 40 {
		ctrl(a, tcell.KeyBackspace2)
	}
	ctrl(a, tcell.KeyEscape)
	if len(a.objs) != 1 || a.objs[0].Text != nil {
		t.Fatalf("empty label: %+v", a.objs)
	}
}

func TestShiftClickInBoxIsLooseText(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 12, 4)
	shiftClick(a, 2, 1)
	typeStr(a, "note")
	if len(a.objs) != 2 || a.objs[1].Kind != "text" || a.objs[1].X0 != 2 || a.objs[0].Text != nil {
		t.Fatalf("shift-click inside a box should type loose text: %+v", a.objs)
	}
}

func TestLabelGrowMovesArrows(t *testing.T) {
	a := joined(t)
	click(a, 5, 2) // inside A
	typeStr(a, "a label that is wider than A")
	ctrl(a, tcell.KeyEscape)
	o, ar := a.objs[0], a.objs[2]
	if ar.X0 != o.X1+1 || ar.From.ID != o.ID {
		t.Fatalf("the arrow should follow A's right side as it grows: A %+v arrow %+v", o, ar)
	}
	back, err := decodeYAML(must(encodeYAML(a.objs)))
	if err != nil || !reflect.DeepEqual(back, a.objs) {
		t.Fatalf("yaml: %v %+v", err, back)
	}
	if !strings.Contains(string(must(encodeYAML(a.objs))), "a label that is wider than A") {
		t.Fatal("the label should be in the yaml")
	}
}

func TestCleanCrossings(t *testing.T) {
	st := 1 // line style
	objs := []object{
		{Kind: "box", X0: 0, Y0: 0, X1: 6, Y1: 4, Style: st},
		{Kind: "box", X0: 3, Y0: 2, X1: 9, Y1: 6, Style: st},
	}
	got := compose(objs).String()
	want := "┌─────┐\n│     │\n│  ┌──┼──┐\n│  │  │  │\n└──┼──┘  │\n   │     │\n   └─────┘\n"
	if got != want {
		t.Fatalf("overlapping boxes:\n%s\nwant:\n%s", got, want)
	}
	// A line that stops on a box edge makes a tee; one that leaves it, too.
	objs = []object{
		{Kind: "box", X0: 4, Y0: 0, X1: 10, Y1: 4, Style: st},
		{Kind: "line", X0: 0, Y0: 2, X1: 4, Y1: 2, Style: st},
		{Kind: "line", X0: 7, Y0: 4, X1: 7, Y1: 6, Style: st},
	}
	got = compose(objs).String()
	want = "    ┌─────┐\n    │     │\n────┤     │\n    │     │\n    └──┬──┘\n       │\n       │\n"
	if got != want {
		t.Fatalf("tees:\n%s\nwant:\n%s", got, want)
	}
	// An arrowhead sits on top; it doesn't join.
	objs = []object{
		{Kind: "box", X0: 4, Y0: 0, X1: 10, Y1: 4, Style: st},
		{Kind: "arrow", X0: 0, Y0: 2, X1: 4, Y1: 2, Style: st},
	}
	if got := compose(objs).String(); !strings.Contains(got, "───>     │") {
		t.Fatalf("arrowhead:\n%s", got)
	}
	// ASCII joins with +.
	objs = []object{
		{Kind: "line", X0: 0, Y0: 1, X1: 4, Y1: 1},
		{Kind: "line", X0: 2, Y0: 0, X1: 2, Y1: 2},
	}
	if got := compose(objs).String(); got != "  |\n--+--\n  |\n" {
		t.Fatalf("ascii cross: %q", got)
	}
}

func TestStackingOrder(t *testing.T) {
	a := threeBoxes(t)
	ids := func() []int {
		var out []int
		for _, o := range a.objs {
			out = append(out, o.ID)
		}
		return out
	}
	start := ids()
	click(a, 0, 0)
	typeStr(a, "]")
	if got := ids(); got[2] != start[0] || a.sel != 2 || !a.isSel(2) {
		t.Fatalf("] front: %v sel=%d", got, a.sel)
	}
	typeStr(a, "[")
	if got := ids(); !reflect.DeepEqual(got, start) || a.sel != 0 {
		t.Fatalf("[ back: %v", got)
	}
	ctrl(a, tcell.KeyCtrlZ)
	if got := ids(); got[2] != start[0] {
		t.Fatalf("undo [: %v", got)
	}
}

func TestDuplicate(t *testing.T) {
	board := fakeClipboard(t)
	a := joined(t)
	ctrl(a, tcell.KeyCtrlA)
	ctrl(a, tcell.KeyCtrlD)
	if len(a.objs) != 6 || len(a.selected()) != 3 || *board != "" {
		t.Fatalf("duplicate: %d objects, %d selected, clipboard %q", len(a.objs), len(a.selected()), *board)
	}
	b, ar := a.objs[4], a.objs[5]
	if b.X0 != a.objs[1].X0+2 || b.Y0 != a.objs[1].Y0+1 || ar.To.ID != b.ID || ar.From.ID != a.objs[3].ID {
		t.Fatalf("the copy should be offset and wired to the copied boxes: %+v %+v", b, ar)
	}
	ctrl(a, tcell.KeyCtrlZ)
	if len(a.objs) != 3 {
		t.Fatal("undo the duplicate")
	}
}

func TestFill(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 2, 1, 8, 3)
	a.tool = toolFill
	drag2(a, 0, 0, 10, 4)
	if a.objs[0].Kind != "fill" || a.sel != 0 {
		t.Fatalf("a new fill goes to the back: %+v", a.objs)
	}
	if got := a.ascii(); !strings.HasPrefix(got, "...........\n..+-----+..\n..|.....|..") {
		t.Fatalf("fill behind the box:\n%s", got)
	}
	typeStr(a, " ")
	if a.objs[0].fillRune() != ':' {
		t.Fatalf("Space steps from . to the next shade, : : got %c", a.objs[0].fillRune())
	}
	ctrl(a, tcell.KeyCtrlZ)
	ctrl(a, tcell.KeyCtrlZ)
	if len(a.objs) != 1 {
		t.Fatalf("one undo for the shade, one for the fill: %d left", len(a.objs))
	}
	ctrl(a, tcell.KeyCtrlY)
	// Inside the box, a click types the box's label, not grabs the fill.
	a.tool = toolBox
	click(a, 5, 2)
	typeStr(a, "OK")
	if a.objs[1].Kind != "box" || len(a.objs[1].Text) == 0 {
		t.Fatalf("label over a fill: %+v", a.objs)
	}
	back, err := decodeYAML(must(encodeYAML(a.objs)))
	if err != nil || !reflect.DeepEqual(back, a.objs) {
		t.Fatalf("yaml: %v %+v", err, back)
	}
}

func TestHelpScreen(t *testing.T) {
	a := testAppSized(t, 100, 30)
	typeStr(a, "?")
	if !a.help {
		t.Fatal("? opens help")
	}
	a.render()
	typeStr(a, "x")
	if a.help || len(a.objs) != 0 {
		t.Fatal("any key closes help, and does nothing else")
	}
}

func TestPanelDragReorders(t *testing.T) {
	a := threeBoxes(t)
	px := 100 - panelW + 3
	first := a.objs[0].ID
	press(a, px, 1, tcell.Button1) // row 1: the first box
	press(a, px, 3, tcell.Button1) // over row 3
	if a.dropAt != 2 {
		t.Fatalf("drop target: %d", a.dropAt)
	}
	a.render()
	press(a, px, 3, tcell.ButtonNone)
	if a.objs[2].ID != first || a.sel != 2 {
		t.Fatalf("dragged to the bottom row: %+v sel=%d", a.objs, a.sel)
	}
	press(a, px, 3, tcell.Button1)
	press(a, px, 1, tcell.Button1)
	press(a, px, 1, tcell.ButtonNone)
	if a.objs[0].ID != first {
		t.Fatal("dragged back to the top row")
	}
}

func TestTextEditing(t *testing.T) {
	a := testAppSized(t, 100, 30)
	click(a, 2, 2)
	typeStr(a, "helo")
	ctrl(a, tcell.KeyLeft)
	typeStr(a, "l") // hel|o -> hell|o
	if got := a.objs[0].Text[0]; got != "hello" {
		t.Fatalf("insert in the middle: %q", got)
	}
	ctrl(a, tcell.KeyHome)
	ctrl(a, tcell.KeyDelete) // drop the h
	ctrl(a, tcell.KeyEnd)
	ctrl(a, tcell.KeyEnter)
	typeStr(a, "world")
	if got := a.objs[0].Text; !reflect.DeepEqual(got, []string{"ello", "world"}) {
		t.Fatalf("delete and new line: %q", got)
	}
	// Enter in the middle splits a line; Backspace at a line's start joins.
	ctrl(a, tcell.KeyHome)
	ctrl(a, tcell.KeyRight)
	ctrl(a, tcell.KeyRight)
	ctrl(a, tcell.KeyEnter) // wo | rld
	if got := a.objs[0].Text; !reflect.DeepEqual(got, []string{"ello", "wo", "rld"}) {
		t.Fatalf("split: %q", got)
	}
	ctrl(a, tcell.KeyBackspace2)
	if got := a.objs[0].Text; !reflect.DeepEqual(got, []string{"ello", "world"}) {
		t.Fatalf("join: %q", got)
	}
	if x, y := a.cursor(); x != 4 || y != 3 {
		t.Fatalf("cursor after the join should sit between o and r: %d,%d", x, y)
	}
	// Up keeps the column where it can.
	ctrl(a, tcell.KeyUp)
	typeStr(a, "!")
	if got := a.objs[0].Text[0]; got != "el!lo" {
		t.Fatalf("up: %q", got)
	}
	// One undo takes the whole run of typing back.
	ctrl(a, tcell.KeyEscape)
	ctrl(a, tcell.KeyCtrlZ)
	if len(a.objs) != 0 {
		t.Fatalf("undo: %+v", a.objs)
	}
}

func TestLabelEditingInTheMiddle(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 14, 4)
	click(a, 3, 2)
	typeStr(a, "Lgin")
	for range 3 {
		ctrl(a, tcell.KeyLeft)
	}
	typeStr(a, "o")
	ctrl(a, tcell.KeyEscape)
	if got := a.objs[0].Text[0]; got != "Login" {
		t.Fatalf("label: %q", got)
	}
}

func TestScrolling(t *testing.T) {
	a := testAppSized(t, 60, 14) // no panel; canvas 60x12
	a.mouse(tcell.NewEventMouse(5, 5, tcell.WheelDown, 0))
	a.mouse(tcell.NewEventMouse(5, 5, tcell.WheelDown, 0))
	if a.vy != 6 {
		t.Fatalf("wheel down: vy=%d", a.vy)
	}
	// Drawing now lands 6 rows further down the canvas.
	drag2(a, 0, 0, 4, 2)
	if o := a.objs[0]; o.Y0 != 6 || o.Y1 != 8 {
		t.Fatalf("draw while scrolled: %+v", o)
	}
	a.mouse(tcell.NewEventMouse(5, 5, tcell.WheelDown, tcell.ModShift))
	if a.vx != 3 {
		t.Fatalf("shift+wheel goes sideways: vx=%d", a.vx)
	}
	a.render()
	if got := a.readout(); !strings.HasPrefix(got, "view 3,6") {
		t.Fatalf("readout shows the view: %q", got)
	}
	ctrl(a, tcell.KeyHome)
	if a.vx != 0 || a.vy != 0 {
		t.Fatalf("back to the top left: %d,%d", a.vx, a.vy)
	}
	// Typing past the bottom keeps the cursor on screen.
	click(a, 1, 11)
	for range 5 {
		typeStr(a, "x")
		ctrl(a, tcell.KeyEnter)
	}
	a.render()
	if _, cy := a.cursor(); cy-a.vy > 11 || a.vy == 0 {
		t.Fatalf("cursor off screen: cy=%d vy=%d", cy, a.vy)
	}
	// Dragging against the right edge scrolls along.
	ctrl(a, tcell.KeyEscape)
	a.vx, a.vy = 0, 0
	press(a, 50, 1, tcell.Button1)
	for range 5 {
		press(a, 59, 1, tcell.Button1)
	}
	press(a, 59, 1, tcell.ButtonNone)
	if o := a.objs[len(a.objs)-1]; o.X1 <= 59 {
		t.Fatalf("drag past the edge: %+v vx=%d", o, a.vx)
	}
}

func TestAutosaveAndRecover(t *testing.T) {
	dir := t.TempDir()
	old := recoverDir
	recoverDir = func() string { return filepath.Join(dir, "recover") }
	t.Cleanup(func() { recoverDir = old })

	file := filepath.Join(dir, "ui.yaml")
	a := testAppSized(t, 100, 30)
	a.file = file
	a.autosave() // nothing changed: nothing written
	if _, err := os.Stat(recoverPath(file)); err == nil {
		t.Fatal("no changes, no recovery copy")
	}
	drag2(a, 0, 0, 6, 3)
	a.autosave()
	if _, err := os.Stat(file); err == nil {
		t.Fatal("autosave must never write the real file")
	}

	// A new session on the same file brings the changes back.
	b := testAppSized(t, 100, 30)
	b.file = file
	b.recover()
	if len(b.objs) != 1 || !b.dirty || !strings.HasPrefix(b.msg, "Recovered") {
		t.Fatalf("recover: %+v dirty=%v msg=%q", b.objs, b.dirty, b.msg)
	}
	// Saving clears the copy.
	ctrl(b, tcell.KeyCtrlS)
	if _, err := os.Stat(recoverPath(file)); err == nil {
		t.Fatal("saving should delete the recovery copy")
	}
	// Quitting on purpose throws an unsaved copy away too.
	drag2(b, 10, 0, 16, 3)
	b.autosave()
	ctrl(b, tcell.KeyCtrlQ)
	if quit := ctrl(b, tcell.KeyCtrlQ); !quit {
		t.Fatal("second ^Q quits")
	}
	if _, err := os.Stat(recoverPath(file)); err == nil {
		t.Fatal("^Q twice should delete the recovery copy")
	}
	// Untitled drawings have one copy of their own.
	if filepath.Base(recoverPath("")) != "untitled.yaml" {
		t.Fatalf("untitled copy: %s", recoverPath(""))
	}
}

func TestHelpFits80x24(t *testing.T) {
	a := testAppSized(t, 80, 24)
	a.help = true
	a.render()
	sim := a.s.(tcell.SimulationScreen)
	cells, w, _ := sim.GetContents()
	row := func(y int) string {
		var b strings.Builder
		for x := 0; x < w; x++ {
			if r := cells[y*w+x].Runes; len(r) > 0 {
				b.WriteRune(r[0])
			}
		}
		return b.String()
	}
	var top, bottom int = -1, -1
	for y := 0; y < 24; y++ {
		if strings.Contains(row(y), "? Keys") {
			top = y
		}
		if strings.Contains(row(y), "Any key closes this.") {
			bottom = y
		}
	}
	if top < 0 || bottom < 0 || !strings.Contains(row(top), "╮") {
		t.Fatalf("help should fit whole on 80x24: top=%d bottom=%d\n%s", top, bottom, row(top))
	}
}

func TestEveryShadePressShows(t *testing.T) {
	a := testAppSized(t, 100, 30)
	a.style = 1
	a.tool = toolFill
	drag2(a, 0, 0, 5, 2)
	seen := map[rune]bool{a.objs[0].fillRune(): true}
	for k := 0; k < 7; k++ {
		key := "t"
		if k%2 == 1 {
			key = " "
		}
		before := a.objs[0].fillRune()
		typeStr(a, key)
		if got := a.objs[0].fillRune(); got == before {
			t.Fatalf("press %d (%q) left the fill as %c", k, key, got)
		}
		seen[a.objs[0].fillRune()] = true
	}
	if len(seen) != 8 {
		t.Fatalf("t and Space should reach all 8 shades: %v", seen)
	}
}

func cellStyle(a *app, x, y int) (rune, tcell.Color, tcell.Color) {
	r, _, st, _ := a.s.(tcell.SimulationScreen).GetContent(x, y)
	fg, bg, _ := st.Decompose()
	return r, fg, bg
}

func TestColors(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 10, 4)
	click(a, 3, 2)
	typeStr(a, "Hi")
	ctrl(a, tcell.KeyEscape) // stop typing; the box stays selected
	typeStr(a, "C")
	if a.colors == nil {
		t.Fatal("C should open the color picker")
	}
	ctrl(a, tcell.KeyRight) // text: black
	ctrl(a, tcell.KeyRight) // text: red
	ctrl(a, tcell.KeyDown)
	for range 5 {
		ctrl(a, tcell.KeyRight) // back: blue
	}
	if a.objs[0].FG != "red" || a.objs[0].BG != "blue" {
		t.Fatalf("live preview: %+v", a.objs[0])
	}
	a.render()
	if _, fg, bg := cellStyle(a, 0, 0); fg != colorOf("red") || bg != colorOf("blue") {
		t.Fatalf("while picking, the border shows its real colors: %v %v", fg, bg)
	}
	ctrl(a, tcell.KeyEnter)
	ctrl(a, tcell.KeyEscape) // deselect, to see the colors without highlight
	a.render()
	if _, _, bg := cellStyle(a, 2, 1); bg != colorOf("blue") {
		t.Fatalf("a box's background fills its inside: %v", bg)
	}
	if r, fg, _ := cellStyle(a, 4, 2); r != 'H' || fg != colorOf("red") {
		t.Fatalf("the label takes the text color: %q %v", r, fg)
	}
	if _, _, bg := cellStyle(a, 20, 2); bg == colorOf("blue") {
		t.Fatal("the background stays inside the box")
	}
	// Saved by name, ignored by the export.
	data := string(must(encodeYAML(a.objs)))
	if !strings.Contains(data, "fg: red") || !strings.Contains(data, "bg: blue") {
		t.Fatalf("yaml:\n%s", data)
	}
	back, err := decodeYAML([]byte(data))
	if err != nil || !reflect.DeepEqual(back, a.objs) {
		t.Fatalf("round trip: %v", err)
	}
	if strings.ContainsAny(markdown(a.ascii()), "\x1b") || !strings.Contains(markdown(a.ascii()), "Hi") {
		t.Fatal("the export is plain text")
	}
	if _, err := decodeYAML([]byte("objects:\n  - kind: box\n    fg: chartreuse\n")); err == nil {
		t.Fatal("an unknown color should be an error")
	}
	// Esc in the picker puts the old colors back; Enter's change undoes in one step.
	click(a, 0, 0)
	typeStr(a, "C")
	ctrl(a, tcell.KeyRight)
	ctrl(a, tcell.KeyEscape)
	if a.objs[0].FG != "red" || a.colors != nil {
		t.Fatalf("Esc should undo the picking: %+v", a.objs[0])
	}
	ctrl(a, tcell.KeyCtrlZ)
	if a.objs[0].FG != "" || a.objs[0].BG != "" {
		t.Fatalf("one undo takes the colors off: %+v", a.objs[0])
	}
}

func TestPickerMouseAndMany(t *testing.T) {
	a := threeBoxes(t)
	click(a, 0, 0)
	shiftClick(a, 10, 0)
	typeStr(a, "C")
	x0, y0, _, _ := a.pickerBox()
	click(a, x0+8+3*4, y0+1) // the green swatch in the text row
	if a.objs[0].FG != "green" || a.objs[1].FG != "green" || a.objs[2].FG != "" {
		t.Fatalf("a click colors every selected shape: %+v", a.objs)
	}
	click(a, 1, 25) // outside: keeps the colors
	if a.colors != nil || a.objs[0].FG != "green" {
		t.Fatal("a click outside the picker keeps what was picked")
	}
	if len(a.objs) != 3 {
		t.Fatal("clicks in the picker must not draw")
	}
	ctrl(a, tcell.KeyEscape) // nothing selected now
	typeStr(a, "C")
	if !a.msgErr || a.colors != nil {
		t.Fatal("C with nothing selected should explain")
	}
}

func TestOvalBackgroundStaysInside(t *testing.T) {
	o := object{Kind: "oval", X0: 0, Y0: 0, X1: 20, Y1: 6, BG: "blue"}
	c := compose([]object{o})
	if c.bgAt(10, 3) != "blue" {
		t.Fatal("the middle of the oval should be blue")
	}
	if c.bgAt(1, 1) != "" || c.bgAt(19, 5) != "" {
		t.Fatal("the corners outside the curve should not be painted")
	}
}

func TestHexColors(t *testing.T) {
	for in, want := range map[string]string{"#FF8800": "#ff8800", "f80": "#ff8800", "#abc": "#aabbcc"} {
		if got, ok := parseHex(in); !ok || got != want {
			t.Errorf("parseHex(%q) = %q, %v", in, got, ok)
		}
	}
	for _, bad := range []string{"#ff88", "#gggggg", "orange", ""} {
		if _, ok := parseHex(bad); ok {
			t.Errorf("parseHex(%q) should fail", bad)
		}
	}

	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 10, 4)
	typeStr(a, "C")
	typeStr(a, "#")
	if a.prompt == nil {
		t.Fatal("# in the picker asks for a hex color")
	}
	ctrl(a, tcell.KeyBackspace2)
	typeStr(a, "nope")
	ctrl(a, tcell.KeyEnter)
	if !a.msgErr || a.objs[0].FG != "" {
		t.Fatal("a bad hex color is refused")
	}
	typeStr(a, "#")
	ctrl(a, tcell.KeyBackspace2)
	typeStr(a, "#F80")
	ctrl(a, tcell.KeyEnter)
	if a.objs[0].FG != "#ff8800" || a.colors == nil {
		t.Fatalf("hex text color, picker still open: %+v", a.objs[0])
	}
	// The hex slot stays in the row: step away and back to it.
	ctrl(a, tcell.KeyRight) // wraps to the default
	ctrl(a, tcell.KeyLeft)  // back to the hex slot
	if a.objs[0].FG != "#ff8800" {
		t.Fatalf("arrow back to the hex color: %q", a.objs[0].FG)
	}
	ctrl(a, tcell.KeyEnter)
	ctrl(a, tcell.KeyEscape)
	a.render()
	if _, fg, _ := cellStyle(a, 0, 0); fg != tcell.NewHexColor(0xff8800) {
		t.Fatalf("drawn in the hex color: %v", fg)
	}
	data := string(must(encodeYAML(a.objs)))
	if !strings.Contains(data, "fg: '#ff8800'") && !strings.Contains(data, `fg: "#ff8800"`) {
		t.Fatalf("yaml:\n%s", data)
	}
	back, err := decodeYAML([]byte(data))
	if err != nil || back[0].FG != "#ff8800" {
		t.Fatalf("round trip: %v %+v", err, back)
	}
	if _, err := decodeYAML([]byte("objects:\n  - kind: box\n    bg: \"#12\"\n")); err == nil {
		t.Fatal("a broken hex color in a file is an error")
	}
	// Reopening the picker on a hex-colored shape starts on its hex slot.
	click(a, 0, 0)
	typeStr(a, "C")
	if a.colors.cur[0] != hexSlot || a.colors.hex[0] != "#ff8800" {
		t.Fatalf("reopen: %+v", a.colors)
	}
}

func TestHexInFilesIsForgiving(t *testing.T) {
	objs, err := decodeYAML([]byte("objects:\n  - kind: box\n    fg: \"#FF8800\"\n    bg: \"#0af\"\n"))
	if err != nil || objs[0].FG != "#ff8800" || objs[0].BG != "#00aaff" {
		t.Fatalf("hand-typed hex in a file: %v %+v", err, objs)
	}
	if _, err := decodeYAML([]byte("objects:\n  - kind: box\n    fg: ff8800\n")); err == nil {
		t.Fatal("hex without # in a file is still an error")
	}
}

func TestPalette(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 10, 4)
	typeStr(a, "C")
	// Up from Text wraps to the palette, which then colors the text.
	ctrl(a, tcell.KeyUp)
	ctrl(a, tcell.KeyRight)
	if a.objs[0].FG != defaultPal[1] {
		t.Fatalf("palette slot 2 on the text: %q", a.objs[0].FG)
	}
	// Space switches it to the background.
	typeStr(a, " ")
	ctrl(a, tcell.KeyRight)
	if a.objs[0].BG != defaultPal[2] || a.objs[0].FG != defaultPal[1] {
		t.Fatalf("palette on the background: %+v", a.objs[0])
	}
	// Coming down from Back, it colors the background too.
	ctrl(a, tcell.KeyUp)
	ctrl(a, tcell.KeyDown)
	if a.colors.row != palRow || a.colors.target != 1 {
		t.Fatalf("down from Back: %+v", a.colors)
	}

	// p opens the grid on the slot; moving tries colors, Esc puts it back.
	typeStr(a, "p")
	if a.colors.grid == nil {
		t.Fatal("p opens the preset grid")
	}
	a.render()
	ctrl(a, tcell.KeyRight)
	if a.pal[2] == defaultPal[2] || a.objs[0].BG != a.pal[2] {
		t.Fatalf("the grid tries colors live: pal=%q bg=%q", a.pal[2], a.objs[0].BG)
	}
	ctrl(a, tcell.KeyEscape)
	if a.colors.grid != nil || a.pal[2] != defaultPal[2] || a.objs[0].BG != defaultPal[2] {
		t.Fatalf("Esc in the grid: pal=%q bg=%q", a.pal[2], a.objs[0].BG)
	}

	// Click the button, then a swatch: blue 500 goes into the slot.
	x0, y0, w, _ := a.pickerBox()
	click(a, x0+w-4, y0+4)
	if a.colors.grid == nil {
		t.Fatal("the Palette button opens the grid")
	}
	gx, gy, _, _ := a.gridBox()
	col, _, _ := presetAt("#3b82f6")
	click(a, gx+2+col*3, gy+1+2)
	if a.colors.grid != nil || a.pal[2] != "#3b82f6" || a.objs[0].BG != "#3b82f6" {
		t.Fatalf("a click in the grid picks: pal=%q bg=%q", a.pal[2], a.objs[0].BG)
	}
	// A click on a palette slot uses it.
	click(a, x0+8+4*3, y0+3)
	if a.colors.slot != 4 || a.objs[0].BG != defaultPal[4] {
		t.Fatalf("click a slot: %d %q", a.colors.slot, a.objs[0].BG)
	}
	ctrl(a, tcell.KeyEnter)
	if !a.dirty || a.pal[2] != "#3b82f6" {
		t.Fatal("Enter keeps the palette and marks the drawing changed")
	}

	// Saved with the drawing, and only when it isn't the default.
	data := string(must(encodeDoc([][]object{a.objs}, a.meta())))
	if !strings.Contains(data, "palette:") || !strings.Contains(data, "#3b82f6") {
		t.Fatalf("yaml:\n%s", data)
	}
	_, m, err := decodeDoc([]byte(data))
	pal := m.Pal
	if err != nil || !slices.Equal(pal, a.pal) {
		t.Fatalf("round trip: %v %v", err, pal)
	}
	if strings.Contains(string(must(encodeDoc([][]object{a.objs}, defaultMeta()))), "palette") {
		t.Fatal("the default palette isn't written")
	}
	_, m, err = decodeDoc([]byte("palette: ['#F80', red]\nobjects: []\n"))
	pal = m.Pal
	if err != nil || pal[0] != "#ff8800" || pal[1] != "red" || pal[2] != defaultPal[2] {
		t.Fatalf("a short palette fills up from the default: %v %v", err, pal)
	}
	if _, _, err := decodeDoc([]byte("palette: [nope]\nobjects: []\n")); err == nil {
		t.Fatal("a bad palette color is an error")
	}

	// Esc on the whole picker undoes palette changes too.
	click(a, 0, 0)
	typeStr(a, "C")
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyDown)
	typeStr(a, "p")
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyEnter)  // keep in the grid
	ctrl(a, tcell.KeyEscape) // but not in the picker
	if a.pal[2] != "#3b82f6" {
		t.Fatalf("Esc in the picker puts the palette back: %v", a.pal)
	}

	// A new drawing keeps the palette.
	a.dirty = false
	ctrl(a, tcell.KeyCtrlN)
	if a.pal[2] != "#3b82f6" {
		t.Fatal("^N keeps the palette")
	}
}

// boxesAt makes boxes from {x, y, w, h}, selects them all, and gives the app.
func boxesAt(t *testing.T, r ...[4]int) *app {
	a := testAppSized(t, 120, 40)
	for k, b := range r {
		a.objs = append(a.objs, object{Kind: "box", X0: b[0], Y0: b[1], X1: b[0] + b[2] - 1, Y1: b[1] + b[3] - 1, ID: k + 1})
	}
	a.selectAll()
	return a
}

func alignWith(a *app, keys string) {
	for _, r := range keys {
		typeStr(a, string(r))
	}
}

func TestAlign(t *testing.T) {
	r := [][4]int{{4, 2, 6, 3}, {20, 7, 10, 5}, {40, 4, 4, 9}}
	for _, tc := range []struct {
		key  string
		want func(o object) int
		val  int
	}{
		{"h", func(o object) int { return o.X0 }, 4},
		{"l", func(o object) int { return o.X1 }, 43},
		{"k", func(o object) int { return o.Y0 }, 2},
		{"j", func(o object) int { return o.Y1 }, 12},
		{"c", func(o object) int { x, _, w, _ := o.bounds(); return x + (w-1)/2 }, 23},
		{"m", func(o object) int { _, y, _, h := o.bounds(); return y + (h-1)/2 }, 7},
	} {
		a := boxesAt(t, r...)
		alignWith(a, "a"+tc.key)
		for i, o := range a.objs {
			if got := tc.want(o); got != tc.val {
				t.Errorf("a%s: shape %d at %d, want %d", tc.key, i, got, tc.val)
			}
		}
		if a.msgErr || a.aligning {
			t.Errorf("a%s: %q", tc.key, a.msg)
		}
		ctrl(a, tcell.KeyCtrlZ)
		if a.objs[1].X0 != 20 || a.objs[1].Y0 != 7 {
			t.Errorf("a%s: one undo puts it back: %+v", tc.key, a.objs[1])
		}
	}

	// Arrow keys work too, and only the other axis stays put.
	a := boxesAt(t, r...)
	typeStr(a, "a")
	a.render()
	if !a.aligning {
		t.Fatal("a waits for a second key")
	}
	ctrl(a, tcell.KeyLeft)
	if a.objs[1].X0 != 4 || a.objs[1].Y0 != 7 {
		t.Fatalf("a← lines up left edges: %+v", a.objs[1])
	}
	// Doing it again changes nothing.
	alignWith(a, "ah")
	if a.msg != "Already lined up." {
		t.Fatalf("again: %q", a.msg)
	}
	// Any other key cancels, and so does a click.
	alignWith(a, "aq")
	if a.aligning || a.objs[1].X0 != 4 {
		t.Fatal("a then q cancels")
	}
	typeStr(a, "a")
	n := len(a.objs)
	if x0, _, _, _ := a.alignBox(); x0 < 2 {
		t.Fatal("the test needs room left of the window")
	}
	drag2(a, 1, 1, 5, 3)
	if a.aligning || len(a.objs) != n {
		t.Fatal("a click outside the window cancels, and draws nothing")
	}
	// One shape isn't enough; nothing selected explains.
	a.setSelection([]int{0}, 0)
	alignWith(a, "ah")
	if !a.msgErr {
		t.Fatal("aligning one shape should explain")
	}
	a.sel = -1
	typeStr(a, "a")
	if !a.msgErr || a.aligning {
		t.Fatal("a with nothing selected should explain")
	}
}

// The window's keys draw what they do: [ | ] line up left edges, centers
// and right edges; ^ - _ top edges, middles and bottom edges.
func TestAlignWindowKeys(t *testing.T) {
	r := [][4]int{{4, 2, 6, 3}, {20, 7, 10, 5}, {40, 4, 4, 9}}
	for key, vim := range map[string]string{"[": "h", "|": "c", "]": "l", "^": "k", "-": "m", "_": "j"} {
		a, b := boxesAt(t, r...), boxesAt(t, r...)
		alignWith(a, "a"+key)
		alignWith(b, "a"+vim)
		for i := range a.objs {
			if a.objs[i].X0 != b.objs[i].X0 || a.objs[i].Y0 != b.objs[i].Y0 {
				t.Errorf("a%s should do what a%s does", key, vim)
			}
		}
	}

	// The window shows while it waits, and a click on a choice runs it.
	a := boxesAt(t, r...)
	typeStr(a, "a")
	a.render()
	if !strings.Contains(screenText(a), "centers evenly spaced, across") {
		t.Fatal("a opens the window")
	}
	x0, y0, _, _ := a.alignBox()
	click(a, x0+2+2, y0+2+1) // the [ on "left edges"
	if a.aligning || a.objs[1].X0 != 4 || a.objs[2].X0 != 4 {
		t.Fatalf("clicking left edges lines them up: %q", a.msg)
	}
	// The words "y  equal gaps, down" run y, and the Y words run Y.
	for _, tc := range []struct {
		row  int
		want string
	}{{9, "with even gaps, top to bottom"}, {10, "centers of 3 evenly, top to bottom"}} {
		b := boxesAt(t, [4]int{0, 0, 3, 3}, [4]int{0, 5, 3, 3}, [4]int{0, 20, 3, 3})
		typeStr(b, "a")
		bx, by, _, _ := b.alignBox()
		click(b, bx+2+30, by+2+tc.row)
		if !strings.Contains(b.msg, tc.want) {
			t.Errorf("row %d: %q, want %q", tc.row, b.msg, tc.want)
		}
	}
	// A click on the window's empty space keeps it open.
	typeStr(a, "a")
	click(a, x0+2+40, y0+2+4)
	if !a.aligning {
		t.Fatal("a click on empty space in the window does nothing")
	}
	ctrl(a, tcell.KeyEscape)
	if a.aligning {
		t.Fatal("Esc closes the window")
	}
}

func TestSpread(t *testing.T) {
	// Out of order on purpose: spreading goes by position, not drawing order.
	a := boxesAt(t, [4]int{30, 0, 5, 3}, [4]int{0, 5, 4, 3}, [4]int{8, 2, 6, 3}, [4]int{50, 9, 3, 3})
	alignWith(a, "ax")
	// Span 0..52, widths 4+6+5+3 = 18, so 35 cells of gaps in 3: 11, 12, 12.
	var xs []int
	for _, i := range []int{1, 2, 0, 3} {
		xs = append(xs, a.objs[i].X0)
	}
	if !slices.Equal(xs, []int{0, 15, 33, 50}) {
		t.Fatalf("spread across: %v", xs)
	}
	if a.objs[0].Y0 != 0 || a.objs[1].Y0 != 5 {
		t.Fatal("spreading across leaves y alone")
	}
	alignWith(a, "ay")
	// Tops 0, 2, 5, 9 and heights 3: 12 rows, no room left for gaps, so they stack.
	var ys []int
	for _, i := range []int{0, 2, 1, 3} {
		ys = append(ys, a.objs[i].Y0)
	}
	if !slices.Equal(ys, []int{0, 3, 6, 9}) {
		t.Fatalf("spread down: %v", ys)
	}
	// X spreads by centers: 1 and 51 stay, and the others go to 17 and 34,
	// steps of 16, 17 and 17 across the 50 cells between.
	c := boxesAt(t, [4]int{0, 0, 3, 3}, [4]int{5, 0, 9, 3}, [4]int{20, 0, 5, 3}, [4]int{50, 0, 3, 3})
	alignWith(c, "aX")
	var mids []int
	for _, o := range c.objs {
		x, _, w, _ := o.bounds()
		mids = append(mids, x+(w-1)/2)
	}
	if !slices.Equal(mids, []int{1, 17, 34, 51}) {
		t.Fatalf("spread by centers: %v", mids)
	}
	alignWith(c, "aY") // all on one row already: nothing moves
	if c.msg != "Already evenly spaced." {
		t.Fatalf("aY on a row: %q", c.msg)
	}
	// Two shapes are not enough.
	b := boxesAt(t, [4]int{0, 0, 3, 3}, [4]int{9, 0, 3, 3})
	alignWith(b, "ax")
	if !b.msgErr {
		t.Fatal("spreading two shapes should explain")
	}
}

func TestAlignGroupsAndArrows(t *testing.T) {
	// Boxes 1 and 2 are a group; box 3 is alone; an arrow joins 2 and 3.
	a := boxesAt(t, [4]int{0, 0, 5, 3}, [4]int{10, 4, 5, 3}, [4]int{30, 8, 5, 3})
	a.objs[0].Group, a.objs[1].Group = 1, 1
	a.objs = append(a.objs, object{Kind: "arrow", X0: 15, Y0: 5, X1: 29, Y1: 9, ID: 4,
		From: anchor{ID: 2, Side: 'r', At: 1, Gap: 1}, To: anchor{ID: 3, Side: 'l', At: 1, Gap: 1}})
	a.reattach(nil)
	a.selectAll()
	alignWith(a, "ak")
	// The group moves as one: its top goes to 0, which it already is. Box 3's top goes to 0.
	if a.objs[0].Y0 != 0 || a.objs[1].Y0 != 4 || a.objs[2].Y0 != 0 {
		t.Fatalf("group stays one piece: %+v %+v %+v", a.objs[0], a.objs[1], a.objs[2])
	}
	// The arrow follows its boxes instead of being aligned itself.
	ar := a.objs[3]
	if ar.From.ID != 2 || ar.To.ID != 3 {
		t.Fatalf("the arrow stays attached: %+v", ar)
	}
	ex, ey := anchorPoint(a.objs[2], ar.To)
	if ar.X1 != ex || ar.Y1 != ey {
		t.Fatalf("the arrow's end follows box 3: (%d,%d) want (%d,%d)", ar.X1, ar.Y1, ex, ey)
	}
}

func TestLabelPlace(t *testing.T) {
	a := testAppSized(t, 100, 30)
	a.objs = []object{
		{Kind: "box", X0: 0, Y0: 0, X1: 19, Y1: 6, Text: []string{"Hi"}, ID: 1},
		{Kind: "oval", X0: 30, Y0: 0, X1: 55, Y1: 8, Text: []string{"Oval"}, ID: 2},
		{Kind: "text", X0: 0, Y0: 20, Text: []string{"loose"}, ID: 3},
	}
	a.selectAll()
	typeStr(a, "T")
	if a.placer == nil {
		t.Fatal("T opens the label dialog")
	}
	a.render()
	// The arrows move the labels right away.
	ctrl(a, tcell.KeyUp)
	ctrl(a, tcell.KeyLeft)
	if x, y := a.objs[0].labelStart(0); x != 2 || y != 1 {
		t.Fatalf("box label top left at (%d,%d)", x, y)
	}
	ctrl(a, tcell.KeyRight)
	ctrl(a, tcell.KeyRight)
	ctrl(a, tcell.KeyRight) // stops at the edge
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyDown)
	if x, y := a.objs[0].labelStart(0); x != 16 || y != 5 {
		t.Fatalf("box label bottom right at (%d,%d)", x, y)
	}
	// The oval's label stays inside the curve.
	c := compose(a.objs)
	x, y := a.objs[1].labelStart(0)
	for i := range 4 {
		if c.get(x+i, y) != []rune("Oval")[i] {
			t.Fatalf("oval label drawn at (%d,%d):\n%s", x, y, a.ascii())
		}
	}
	for xx := x + 4; xx < 56; xx++ {
		if c.get(xx, y) != ' ' {
			break
		}
		if xx == 55 {
			t.Fatal("the curve should be to the right of the oval's label")
		}
	}
	// Enter keeps it, in one undo step; loose text is left alone.
	ctrl(a, tcell.KeyEnter)
	if a.placer != nil || a.objs[0].HAlign != 1 || a.objs[0].VAlign != 1 || a.objs[1].HAlign != 1 {
		t.Fatalf("kept: %+v %+v", a.objs[0], a.objs[1])
	}
	if a.objs[2].HAlign != 0 {
		t.Fatal("loose text has no label position")
	}
	// Saved by name.
	data := string(must(encodeYAML(a.objs)))
	if !strings.Contains(data, "place: bottom right") {
		t.Fatalf("yaml:\n%s", data)
	}
	back, err := decodeYAML([]byte(data))
	if err != nil || !reflect.DeepEqual(back, a.objs) {
		t.Fatalf("round trip: %v", err)
	}
	ctrl(a, tcell.KeyCtrlZ)
	if a.objs[0].HAlign != 0 || a.objs[0].VAlign != 0 {
		t.Fatal("one undo centers it again")
	}
	// A number key picks right away, keypad style; Esc puts it back.
	a.selectAll()
	typeStr(a, "T7")
	if a.placer != nil || a.objs[0].HAlign != -1 || a.objs[0].VAlign != -1 {
		t.Fatalf("7 is top left: %+v", a.objs[0])
	}
	a.selectAll()
	typeStr(a, "T")
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyEscape)
	if a.objs[0].VAlign != -1 {
		t.Fatal("Esc leaves the label where it was")
	}
	// A click on a button picks it.
	a.selectAll()
	typeStr(a, "T")
	x0, y0, _, _ := a.placerBox()
	h, v, ok := a.placeButtonAt(x0+2+placeColW(-1)+1+2, y0+3)
	if !ok || h != 0 || v != 1 {
		t.Fatalf("button at bottom center: %d %d %v", h, v, ok)
	}
	click(a, x0+2+placeColW(-1)+1+2, y0+3)
	if a.placer != nil || a.objs[0].HAlign != 0 || a.objs[0].VAlign != 1 {
		t.Fatalf("click bottom center: %+v", a.objs[0])
	}
	// Centered output is unchanged from before, and a file can say it many ways.
	if strings.Contains(string(must(encodeYAML([]object{{Kind: "box", X1: 9, Y1: 4, Text: []string{"x"}}}))), "place") {
		t.Fatal("a centered label writes no place")
	}
	for _, in := range []string{"top-left", "Left Top", "middle right", "center"} {
		if _, _, ok := parsePlace(in); !ok {
			t.Errorf("parsePlace(%q)", in)
		}
	}
	if _, err := decodeYAML([]byte("objects:\n  - kind: box\n    place: sideways\n")); err == nil {
		t.Fatal("an unknown place is an error")
	}
	// Nothing with a label selected explains.
	a.setSelection([]int{2}, 2)
	typeStr(a, "T")
	if a.placer != nil || !a.msgErr {
		t.Fatal("T on loose text should explain")
	}
}

func TestLabelPlaceTooBigStaysCentered(t *testing.T) {
	// The label fills the box, so bottom right comes out where center does.
	o := object{Kind: "box", X0: 0, Y0: 0, X1: 5, Y1: 2, Text: []string{"ab"}, HAlign: 1, VAlign: 1}
	if x, y := o.labelStart(0); x != 2 || y != 1 {
		t.Fatalf("(%d,%d)", x, y)
	}
	// A snug oval: the label is too wide to move sideways, so it doesn't
	// move up either, where the curve would cut through it.
	ov := object{Kind: "oval", X0: 0, Y0: 0, X1: 13, Y1: 8, Text: []string{"abcdefgh"}, VAlign: -1}
	cv := ov
	cv.VAlign = 0
	ox, oy := ov.labelStart(0)
	if cx, cy := cv.labelStart(0); ox != cx || oy != cy {
		t.Fatalf("snug oval label moved to (%d,%d) from (%d,%d)", ox, oy, cx, cy)
	}
}

// The ruler in the bottom right names each page with one character and
// marks the one on screen. A click on a name goes to that page.
func TestPager(t *testing.T) {
	a := testApp(t)
	sw, sh := a.s.Size()
	lastRow := func() string { a.render(); return strings.Split(screenText(a), "\n")[sh-1] }
	ruler := func() string { return strings.TrimRight(lastRow(), " ") }
	// The ruler is always five names wide; pages not made yet are dim.
	if !strings.HasSuffix(ruler(), "Pages:01234") {
		t.Fatalf("a new drawing: %q", ruler())
	}
	x0, _ := a.pager()
	style := func(i int) tcell.Style { _, _, st, _ := a.s.GetContent(x0+i, sh-1); return st }
	if style(0) != style(0).Background(tcell.ColorYellow) || style(1) != style(1).Dim(true) {
		t.Fatal("page 0 is marked, page 1 doesn't exist yet")
	}
	drag2(a, 0, 0, 10, 4)
	ctrl(a, tcell.KeyEscape)
	typeStr(a, "+>") // copy to page 1, then > adds an empty page 2
	if a.page != 2 || len(a.pages) != 3 || len(a.objs) != 0 {
		t.Fatalf("+ then >: page=%d pages=%d objs=%d", a.page, len(a.pages), len(a.objs))
	}
	if !strings.HasSuffix(ruler(), "Pages:01234") || style(2) != style(2).Background(tcell.ColorYellow) || style(1).Dim(true) == style(1) {
		t.Fatalf("three pages, on page 2: %q", ruler())
	}
	if strings.Contains(lastRow(), "^O") && sw < 50 {
		t.Fatal("legend items make room for the ruler")
	}
	press(a, x0+4, sh-1, tcell.Button1) // page 4 doesn't exist
	press(a, x0+4, sh-1, 0)
	if a.page != 2 {
		t.Fatal("a click on a page not made yet does nothing")
	}
	press(a, x0, sh-1, tcell.Button1)
	press(a, x0, sh-1, 0)
	if a.page != 0 {
		t.Fatalf("click on 0 goes to page 0, got %d", a.page)
	}

	// 62 pages, 0 to Z, and no more. The names slide; the width stays.
	pp := make([][]object, maxPages)
	for i := range pp {
		pp[i] = cloneObjs(a.objs)
	}
	a.setPages(pp)
	a.gotoPage(20)
	if !strings.HasSuffix(ruler(), "Pages:ijklm") {
		t.Fatalf("page k sits in the middle: %q", ruler())
	}
	a.gotoPage(maxPages - 1)
	if !strings.HasSuffix(ruler(), "Pages:VWXYZ") {
		t.Fatalf("last page: %q", ruler())
	}
	typeStr(a, ".")
	if !a.msgErr || len(a.pages) != maxPages {
		t.Fatal(". stops at 62 pages")
	}
	typeStr(a, "+")
	if !a.msgErr || len(a.pages) != maxPages {
		t.Fatal("+ stops at 62 pages")
	}
	if a.pageName() != "page Z" {
		t.Fatalf("last page is Z: %s", a.pageName())
	}
	if _, _, err := decodeDoc([]byte("version: 3\n")); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatal("a file from a newer asciidraw is refused")
	}
	big := "version: 2\npages:\n" + strings.Repeat("  - objects: []\n", maxPages+1)
	if _, _, err := decodeDoc([]byte(big)); err == nil {
		t.Fatal("a file with 63 pages is refused")
	}
}

func TestPages(t *testing.T) {
	a := testAppSized(t, 100, 30)
	typeStr(a, ".")
	if !a.msgErr || len(a.pages) != 1 {
		t.Fatal("no new page after an empty one")
	}
	typeStr(a, ",")
	if !a.msgErr {
		t.Fatal(", on the first page explains")
	}
	drag2(a, 0, 0, 10, 4) // page 1: a box
	ctrl(a, tcell.KeyEscape)
	typeStr(a, ".")
	if a.page != 1 || len(a.pages) != 2 || len(a.objs) != 0 {
		t.Fatalf("., starts an empty page 2: page=%d pages=%d objs=%d", a.page, len(a.pages), len(a.objs))
	}
	a.render()
	if !strings.Contains(screenText(a), "Pages:01") {
		t.Fatal("the ruler shows the pages")
	}
	// Going back from an untouched empty page drops it.
	typeStr(a, ",")
	if a.page != 0 || len(a.pages) != 1 || len(a.objs) != 1 {
		t.Fatalf("back: page=%d pages=%d", a.page, len(a.pages))
	}
	// One you drew on and undid keeps its redo.
	typeStr(a, ".")
	drag2(a, 20, 0, 30, 4)
	ctrl(a, tcell.KeyCtrlZ)
	typeStr(a, ",.")
	ctrl(a, tcell.KeyCtrlY)
	if a.page != 1 || len(a.objs) != 1 {
		t.Fatalf("redo survives leaving the page: page=%d objs=%d", a.page, len(a.objs))
	}
	ctrl(a, tcell.KeyCtrlZ)
	a.redo = nil
	typeStr(a, ",")
	typeStr(a, ".")
	drag2(a, 20, 0, 30, 4) // page 2: a box of its own
	drag2(a, 40, 0, 50, 4)
	ctrl(a, tcell.KeyEscape)
	// Undo belongs to the page: on page 1 it can't touch page 2.
	typeStr(a, ",")
	ctrl(a, tcell.KeyCtrlZ)
	if len(a.objs) != 0 || len(a.pages[1].objs) != 2 {
		t.Fatalf("undo on page 1: %d here, %d on page 2", len(a.objs), len(a.pages[1].objs))
	}
	ctrl(a, tcell.KeyCtrlY)
	typeStr(a, ".")
	ctrl(a, tcell.KeyCtrlZ)
	if len(a.objs) != 1 {
		t.Fatalf("page 2 keeps its own undo: %d", len(a.objs))
	}
	ctrl(a, tcell.KeyCtrlY)

	// + copies page 2 to a new page 3, and the copy is separate.
	typeStr(a, "+")
	if a.page != 2 || len(a.pages) != 3 || len(a.objs) != 2 || !a.dirty {
		t.Fatalf("clone: page=%d pages=%d objs=%d", a.page, len(a.pages), len(a.objs))
	}
	a.objs[0].move(1, 0)
	if a.pages[1].objs[0].X0 != 20 {
		t.Fatal("changing the copy leaves the original alone")
	}
	// < goes back like ,.
	typeStr(a, "<")
	if a.page != 1 {
		t.Fatal("< goes back")
	}
	// > goes forward like .
	typeStr(a, ">")
	if a.page != 2 || len(a.pages) != 3 {
		t.Fatal("> goes to the next page")
	}
	typeStr(a, "<")
	// + in the middle inserts right after, and the rest move down.
	typeStr(a, ",+")
	if a.page != 1 || len(a.pages) != 4 || len(a.objs) != 1 || len(a.pages[2].objs) != 2 {
		t.Fatalf("clone page 1: page=%d pages=%d", a.page, len(a.pages))
	}

	// Saved as a list of pages; an empty page at the end is left out.
	a.pages = append(a.pages, page{})
	data := string(must(encodeDoc(a.allPages(), a.meta())))
	if !strings.Contains(data, "pages:") || strings.Count(data, "- objects:") != 4 || !strings.Contains(data, "version: 2") {
		t.Fatalf("yaml:\n%s", data)
	}
	pages, _, err := decodeDoc([]byte(data))
	if err != nil || len(pages) != 4 || !reflect.DeepEqual(pages[3], a.allPages()[3]) {
		t.Fatalf("round trip: %v %d", err, len(pages))
	}
	// A version 1 file, with a bare objects list, is one page.
	pages, _, err = decodeDoc([]byte("version: 1\nobjects:\n  - kind: box\n    x1: 4\n    y1: 2\n"))
	if err != nil || len(pages) != 1 || len(pages[0]) != 1 {
		t.Fatalf("old file: %v %v", err, pages)
	}
	if _, _, err := decodeDoc([]byte("objects:\n  - kind: box\npages:\n  - objects: []\n")); err == nil {
		t.Fatal("both pages and objects is an error")
	}
	if _, _, err := decodeDoc([]byte("pages:\n  - objects: []\n  - objects:\n      - kind: blob\n")); err == nil || !strings.Contains(err.Error(), "page 1:") {
		t.Fatalf("an error names its page: %v", err)
	}
	// A new drawing has one page, and saving it writes pages[0].
	data = string(must(encodeYAML([]object{{Kind: "box", X1: 4, Y1: 2}})))
	if !strings.Contains(data, "pages:\n  - objects:\n      - kind: box") {
		t.Fatalf("one page:\n%s", data)
	}

	// The export has every page, each in its own block.
	md := markdownPages(a.pictures())
	if strings.Count(md, "```text") != 4 || !strings.Contains(md, "Page 3\n") {
		t.Fatalf("export:\n%s", md)
	}
	if strings.Contains(markdownPages([]string{"x\n"}), "Page") {
		t.Fatal("a single page exports without a Page line")
	}

	// Save and reopen: all pages come back, starting on the first.
	dir := t.TempDir()
	path := filepath.Join(dir, "p.yaml")
	a.write(path)
	b := testAppSized(t, 100, 30)
	pp, _, _, _, err := loadDrawing(path)
	if err != nil {
		t.Fatal(err)
	}
	b.setPages(pp)
	if len(b.pages) != 4 || b.page != 0 || len(b.objs) != 1 {
		t.Fatalf("reopen: %d pages, page %d, %d objs", len(b.pages), b.page, len(b.objs))
	}
}

func screenText(a *app) string {
	s := a.s.(tcell.SimulationScreen)
	cells, w, h := s.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if r := cells[y*w+x].Runes; len(r) > 0 {
				b.WriteRune(r[0])
			} else {
				b.WriteRune(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// S opens the export settings. ^E writes every ticked format; pages go in
// one file or one file per page; the settings are saved with the drawing.
func TestExportSettings(t *testing.T) {
	a := testAppSized(t, 80, 24)
	dir := t.TempDir()
	a.file = filepath.Join(dir, "mock.yaml")
	drag2(a, 0, 0, 10, 4)
	ctrl(a, tcell.KeyEscape)

	typeStr(a, "S")
	if a.settings == nil {
		t.Fatal("S opens the settings")
	}
	// Write row: untick the only format is refused, then tick PNG and JPEG.
	typeStr(a, " ")
	if !a.msgErr || !a.exp.has("md") {
		t.Fatal("at least one format stays ticked")
	}
	ctrl(a, tcell.KeyRight)
	typeStr(a, " ")
	ctrl(a, tcell.KeyRight)
	typeStr(a, " ")
	if !slices.Equal(a.exp.Formats, []string{"md", "png", "jpg"}) {
		t.Fatalf("formats: %v", a.exp.Formats)
	}
	// Font row follows the cursor: 12 is the default, Right picks 14.
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyRight)
	if a.exp.Font != 14 {
		t.Fatalf("font: %d", a.exp.Font)
	}
	a.render()
	if !strings.Contains(screenText(a), "mock.md, mock.png, mock.jpg") {
		t.Fatal("the dialog says what ^E writes")
	}
	ctrl(a, tcell.KeyEscape)
	if !a.exp.isDefault() || a.dirty && a.settings != nil {
		t.Fatal("Esc puts the settings back")
	}
	typeStr(a, "S")
	ctrl(a, tcell.KeyRight)
	typeStr(a, " ")
	ctrl(a, tcell.KeyEnter)
	if a.settings != nil || !a.exp.has("png") {
		t.Fatal("Enter keeps the settings")
	}

	// One page: mock.md and mock.png.
	ctrl(a, tcell.KeyCtrlE)
	for _, f := range []string{"mock.md", "mock.png"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("one page writes %s: %v", f, err)
		}
	}
	data := must(os.ReadFile(filepath.Join(dir, "mock.png")))
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	cw, ch := cellSize(12)
	if img.Bounds().Dx() != 13*cw || img.Bounds().Dy() != 7*ch {
		t.Fatalf("an 11×5 box plus a cell of margin: %v", img.Bounds())
	}
	if _, ok := img.(*image.Paletted); !ok || len(data) > 2000 {
		t.Fatalf("a small paletted PNG: %T, %d bytes", img, len(data))
	}

	// Two pages, a file per page: mock_page0.png, mock_page1.png.
	typeStr(a, "+")
	a.exp.Split = true
	a.exported = ""
	ctrl(a, tcell.KeyCtrlE) // new names, so nothing to ask about
	for _, f := range []string{"mock_page0.md", "mock_page1.md", "mock_page0.png", "mock_page1.png"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("split pages write %s: %v", f, err)
		}
	}
	// Together, two pages make one taller image. mock.png is there from
	// before; in a new session, it asks first.
	a.exp.Split = false
	a.exported, a.wrote = "", nil
	ctrl(a, tcell.KeyCtrlE)
	if a.prompt == nil || !strings.Contains(a.prompt.label, "1 more exist") {
		t.Fatal("asks before replacing mock.md and mock.png")
	}
	typeStr(a, "y")
	ctrl(a, tcell.KeyEnter)
	img, err = png.Decode(bytes.NewReader(must(os.ReadFile(filepath.Join(dir, "mock.png")))))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dy() != (5+1+5+2)*ch {
		t.Fatalf("two pages stacked with a gap: %v", img.Bounds())
	}

	// A format ticked later still asks about a file that was already there.
	os.WriteFile(filepath.Join(dir, "mock.jpg"), []byte("old"), 0o644)
	a.exp.Formats = []string{"md", "png", "jpg"}
	ctrl(a, tcell.KeyCtrlE)
	if a.prompt == nil || !strings.Contains(a.prompt.label, "mock.jpg exists") {
		t.Fatal("asks about mock.jpg, not the files it wrote")
	}
	ctrl(a, tcell.KeyEscape)

	// Saved with the drawing, and back on open.
	a.exp.Theme, a.exp.Font = "dark", 20
	a.write(a.file)
	yml := string(must(os.ReadFile(a.file)))
	if !strings.Contains(yml, "export:") || !strings.Contains(yml, "theme: dark") {
		t.Fatalf("yaml:\n%s", yml)
	}
	_, m, _, _, err := loadDrawing(a.file)
	if err != nil || m.Export.Theme != "dark" || m.Export.Font != 20 || !m.Export.has("png") {
		t.Fatalf("reopen: %v %+v", err, m.Export)
	}
	if strings.Contains(string(must(encodeYAML(a.objs))), "export:") {
		t.Fatal("default settings aren't written")
	}
	if _, _, err := decodeDoc([]byte("export:\n  formats: [gif]\n")); err == nil {
		t.Fatal("an unknown format is an error")
	}

	// JPEG works too, and a picture with no color is stored as grayscale.
	j := must(encodeJPEG(renderPages([][]object{a.objs}, 12, themes["light"])))
	if g, err := jpeg.Decode(bytes.NewReader(j)); err != nil || g.ColorModel() != color.GrayModel {
		t.Fatalf("gray jpeg: %v", err)
	}
}

// Every line character meets its neighbor at the cell edge, with no gap.
func TestImageLinesMeet(t *testing.T) {
	for _, st := range []int{1, 2, 3} {
		objs := []object{{Kind: "box", X0: 0, Y0: 0, X1: 4, Y1: 3, Style: st}}
		img := renderPages([][]object{objs}, 12, themes["light"])
		cw, ch := cellSize(12)
		bg := themes["light"].bg
		// Along the top edge, the middle row of the cells is drawn all the way.
		y := ch + ch/2
		if st == 3 {
			y -= 1 // a double line's outer line
		}
		for x := cw + cw/2 + 2; x < 5*cw; x++ {
			if img.RGBAAt(x, y) == bg {
				t.Fatalf("style %d: gap at x=%d", st, x)
			}
		}
	}
}

// L opens the library: built-in pieces and the user's own from
// ~/.config/asciidraw/library. Enter adds the piece at the mouse.
func TestLibrary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	b := builtinItems()
	var names []string
	for _, it := range b {
		if it.err != nil || len(it.objs) == 0 {
			t.Fatalf("builtin %s: %v", it.name, it.err)
		}
		names = append(names, it.name)
	}
	want := []string{"login dialog", "register dialog", "browser window", "webapp 4 panes", "open file dialog"}
	if !slices.Equal(names, want) {
		t.Fatalf("builtin pieces: %q", names)
	}

	a := testAppSized(t, 100, 30)
	a.mx, a.my = 5, 3
	typeStr(a, "L")
	if a.library == nil {
		t.Fatal("L opens the library")
	}
	a.render()
	if !strings.Contains(screenText(a), "[ builtin | custom ]") || !strings.Contains(screenText(a), "Sign in") {
		t.Fatal("tabs and a preview of the login dialog")
	}
	ctrl(a, tcell.KeyEnter)
	if a.library != nil || len(a.objs) != len(b[0].objs) {
		t.Fatalf("Enter adds the piece: %d objects", len(a.objs))
	}
	x, y, _, _ := selBounds(a.objs, a.selected())
	if x != 5 || y != 3 || len(a.selected()) != len(a.objs) || a.objs[0].Group == 0 {
		t.Fatalf("at the mouse, selected, grouped: %d,%d", x, y)
	}
	ctrl(a, tcell.KeyCtrlZ)
	if len(a.objs) != 0 {
		t.Fatal("one undo takes it back out")
	}
	// The mouse: the first click picks a name, even the one already
	// picked; a second click on it adds it.
	typeStr(a, "L")
	if _, err := os.Stat(filepath.Join(dir, "asciidraw", "library")); err != nil {
		t.Fatal("opening the library makes the custom folder")
	}
	lx, ly, _, _ := a.libraryBox()
	press(a, lx+4, ly+3, tcell.Button1)
	press(a, lx+4, ly+3, 0)
	if a.library == nil || len(a.objs) != 0 {
		t.Fatal("one click only picks")
	}
	press(a, lx+4, ly+4, tcell.Button1)
	press(a, lx+4, ly+4, 0)
	press(a, lx+4, ly+4, tcell.Button1)
	press(a, lx+4, ly+4, 0)
	if a.library != nil || len(a.objs) != len(b[1].objs) {
		t.Fatalf("two clicks on register add it: %d", len(a.objs))
	}
	ctrl(a, tcell.KeyCtrlZ)

	// Custom: an empty folder, then a drawing and a Markdown file.
	typeStr(a, "L")
	ctrl(a, tcell.KeyRight)
	a.render()
	if !strings.Contains(screenText(a), "No pieces yet") {
		t.Fatal("an empty custom tab says where pieces go")
	}
	ctrl(a, tcell.KeyEscape)
	lib := filepath.Join(dir, "asciidraw", "library")
	os.MkdirAll(lib, 0o755)
	os.WriteFile(filepath.Join(lib, "logo.md"), []byte("# Logo\n\n```\n/\\_/\\\n( o.o )\n```\n"), 0o644)
	os.WriteFile(filepath.Join(lib, "card.adraw"), must(encodeYAML([]object{{Kind: "box", X0: 4, Y0: 4, X1: 10, Y1: 7}})), 0o644)
	os.WriteFile(filepath.Join(lib, "broken.adraw"), []byte("version: 9\n"), 0o644)
	os.WriteFile(filepath.Join(lib, "notes.pdf"), []byte("x"), 0o644)
	typeStr(a, "L")
	ctrl(a, tcell.KeyRight)
	var got []string
	for _, it := range a.library.items[libCustom] {
		got = append(got, it.name)
	}
	if !slices.Equal(got, []string{"broken", "card", "logo"}) {
		t.Fatalf("custom pieces: %q", got)
	}
	if a.library.items[libCustom][0].err == nil {
		t.Fatal("a file it can't read shows why")
	}
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyEnter)
	if len(a.objs) != 1 || a.objs[0].Kind != "text" || a.objs[0].Text[1] != "( o.o )" || a.objs[0].X0 != 5 {
		t.Fatalf("a Markdown piece is one text shape: %+v", a.objs)
	}
	// An .adraw file opens as a drawing too.
	pp, _, saveTo, _, err := loadDrawing(filepath.Join(lib, "card.adraw"))
	if err != nil || len(pp[0]) != 1 || saveTo == "" {
		t.Fatalf("open .adraw: %v", err)
	}
	if exportBase("/x/card.adraw") != "/x/card" {
		t.Fatal("an .adraw drawing exports as card.md")
	}
}

// asciidraw export and convert work without a screen.
func TestCommands(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "n.md")
	os.WriteFile(in, []byte("# x\n\n```\n+--+\n|hi|\n+--+\n```\n"), 0o644)
	run := func(args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		ran, code := runCommand(args, &out, &errb)
		if !ran {
			t.Fatalf("%v is a command", args)
		}
		return code, out.String(), errb.String()
	}
	if ran, _ := runCommand([]string{"mock.yaml"}, io.Discard, io.Discard); ran {
		t.Fatal("a file name is not a command")
	}

	// convert: Markdown in, drawing out.
	ad := filepath.Join(dir, "n.adraw")
	if code, out, e := run("convert", in, ad); code != 0 || !strings.Contains(out, "n.adraw") {
		t.Fatalf("convert: %d %s", code, e)
	}
	pp, _, _, _, err := loadDrawing(ad)
	if err != nil || len(pp[0]) != 1 || pp[0][0].Text[1] != "|hi|" {
		t.Fatalf("converted drawing: %v %+v", err, pp)
	}
	if code, _, e := run("convert", in, filepath.Join(dir, "n.png")); code != 2 || !strings.Contains(e, "asciidraw export") {
		t.Fatalf("convert to a picture points to export: %d %s", code, e)
	}

	// export: the extension picks the format; flags override the drawing.
	png1 := filepath.Join(dir, "n.png")
	if code, _, e := run("export", ad, png1); code != 0 {
		t.Fatalf("export png: %s", e)
	}
	img, err := png.Decode(bytes.NewReader(must(os.ReadFile(png1))))
	cw, ch := cellSize(12)
	if err != nil || img.Bounds().Dx() != 6*cw || img.Bounds().Dy() != 5*ch {
		t.Fatalf("png at 12px: %v %v", err, img.Bounds())
	}
	jp := filepath.Join(dir, "big.jpeg")
	if code, _, e := run("export", "--font", "24", "--theme", "dark", ad, jp); code != 0 {
		t.Fatalf("export jpeg: %s", e)
	}
	if j, err := jpeg.Decode(bytes.NewReader(must(os.ReadFile(jp)))); err != nil || j.Bounds().Dx() != 6*14 {
		t.Fatalf("jpeg at 24px: %v", err)
	}
	md := filepath.Join(dir, "out.md")
	run("export", ad, md)
	if got := string(must(os.ReadFile(md))); got != "```text\n+--+\n|hi|\n+--+\n```\n" {
		t.Fatalf("md: %q", got)
	}
	for _, bad := range [][]string{
		{"export", ad, filepath.Join(dir, "x.gif")},
		{"export", "--theme", "blue", ad, png1},
		{"export", "--font", "200", ad, png1},
		{"convert", ad},
	} {
		if code, _, _ := run(bad...); code != 2 {
			t.Fatalf("%v: exit %d, want 2", bad, code)
		}
	}
	if code, _, _ := run("export", filepath.Join(dir, "none.yaml"), png1); code != 1 {
		t.Fatal("a missing input exits 1")
	}

	// No output: what ^E writes, the formats saved with the drawing.
	saved := filepath.Join(dir, "saved.adraw")
	m := defaultMeta()
	m.Export.Formats = []string{"md", "svg"}
	os.WriteFile(saved, must(encodeDoc([][]object{{{Kind: "box", X1: 4, Y1: 2}}}, m)), 0o644)
	if code, out, e := run("export", saved); code != 0 || out != filepath.Join(dir, "saved.md")+"\n"+filepath.Join(dir, "saved.svg")+"\n" {
		t.Fatalf("export with saved settings: %d %q %s", code, out, e)
	}
	// It never writes over the Markdown file it read.
	srcMD := filepath.Join(dir, "page.md")
	os.WriteFile(srcMD, []byte("```\n+--+\n+--+\n```\n"), 0o644)
	if code, _, e := run("export", srcMD); code != 2 || !strings.Contains(e, "replace the drawing") {
		t.Fatalf("export over its own input: %d %s", code, e)
	}
	srcHTM := filepath.Join(dir, "page.htm")
	os.WriteFile(srcHTM, []byte("+--+\n+--+\n"), 0o644)
	if code, _, e := run("export", srcHTM, srcHTM); code != 2 || !strings.Contains(e, "replace the drawing") {
		t.Fatalf("export to its own input, named as typed: %d %s", code, e)
	}

	// Split pages: out_page0.png, out_page1.png.
	two := filepath.Join(dir, "two.adraw")
	box := []object{{Kind: "box", X1: 4, Y1: 2}}
	os.WriteFile(two, must(encodeDoc([][]object{box, box}, defaultMeta())), 0o644)
	if code, out, e := run("export", "--pages", "split", two, filepath.Join(dir, "out.png")); code != 0 || !strings.Contains(out, "out_page1.png") {
		t.Fatalf("split: %d %s %s", code, out, e)
	}
}

// The S dialog ends in Save and Cancel. Save writes the drawing, settings
// and all; Cancel, Esc and a click outside put the settings back.
func TestSettingsButtons(t *testing.T) {
	a := testAppSized(t, 80, 24)
	a.file = filepath.Join(t.TempDir(), "mock.adraw")
	drag2(a, 0, 0, 10, 4)
	ctrl(a, tcell.KeyEscape)

	// Pick the dark theme, walk down to the buttons, and choose Cancel.
	typeStr(a, "S")
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyRight)
	if a.exp.Theme != "dark" {
		t.Fatalf("theme: %s", a.exp.Theme)
	}
	a.render()
	if !strings.Contains(screenText(a), "[ Save ]   [ Cancel ]") {
		t.Fatal("the dialog shows Save and Cancel")
	}
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyRight)
	ctrl(a, tcell.KeyEnter)
	if a.settings != nil || a.exp.Theme != "light" {
		t.Fatal("Cancel puts the settings back")
	}

	// A click outside cancels too.
	typeStr(a, "S")
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyRight)
	click(a, 0, 0)
	if a.settings != nil || a.exp.Theme != "light" {
		t.Fatal("a click outside cancels")
	}

	// Save writes the file now, with the settings in it.
	typeStr(a, "S")
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyDown)
	ctrl(a, tcell.KeyRight)
	ctrl(a, tcell.KeyEnter)
	if a.settings != nil || a.dirty || !strings.HasPrefix(a.msg, "Saved ") {
		t.Fatalf("Save saves the drawing: %q", a.msg)
	}
	_, m, _, _, err := loadDrawing(a.file)
	if err != nil || m.Export.Theme != "dark" {
		t.Fatalf("the file holds the theme: %v %+v", err, m.Export)
	}

	// Clicking the Save button works the same.
	typeStr(a, "S")
	x0, _, _, _ := a.settingsBox()
	a.msg = ""
	click(a, x0+2+setLabelW+1, a.rowY(setButtons))
	if a.settings != nil || !strings.HasPrefix(a.msg, "Saved ") {
		t.Fatalf("clicking Save saves: %q", a.msg)
	}
}

// SVG draws a box's sides as four joined lines, letters as text, and
// escapes them; HTML keeps the picture and its colors.
func TestSVGAndHTML(t *testing.T) {
	objs := []object{
		{Kind: "box", X0: 0, Y0: 0, X1: 9, Y1: 3, Style: 1, Text: []string{"a<b"}},
		{Kind: "text", X0: 0, Y0: 5, Text: []string{"red & bold"}, FG: "red"},
	}
	svg := string(renderSVG([][]object{objs}, 12, themes["light"]))
	if !strings.HasPrefix(svg, "<svg ") || !strings.Contains(svg, ">a&lt;b</text>") || !strings.Contains(svg, ">red &amp; bold</text>") {
		t.Fatalf("svg text:\n%s", svg)
	}
	d := svg[strings.Index(svg, `<path d="`)+9:]
	d = d[:strings.Index(d, `"`)]
	if strings.Count(svg, "<path ") != 1 || strings.Count(d, "M") != 4 {
		t.Fatalf("one path, a line per side:\n%s", svg)
	}
	if !strings.Contains(svg, `fill="#cd3131"`) {
		t.Fatal("named colors get fixed shades")
	}
	cw, ch := cellSize(12)
	if !strings.Contains(svg, fmt.Sprintf(`width="%d" height="%d"`, 12*cw, 8*ch)) {
		t.Fatal("same size as the PNG")
	}

	page := string(renderHTML([][]object{objs, objs}, 12, themes["dark"], "x<y"))
	for _, want := range []string{"<title>x&lt;y</title>", "background:#1e1e1e", "a&lt;b", `<span style="color:#cd3131">red &amp; bold</span>`, "<p>Page 1</p>", "┌────────┐"} {
		if !strings.Contains(page, want) {
			t.Fatalf("html lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "  \n") {
		t.Fatal("no spaces at the ends of rows")
	}

	// Both come out of export and the S settings.
	dir := t.TempDir()
	in := filepath.Join(dir, "d.adraw")
	os.WriteFile(in, must(encodeYAML(objs)), 0o644)
	for _, out := range []string{"d.svg", "d.html"} {
		var o, e bytes.Buffer
		if _, code := runCommand([]string{"export", in, filepath.Join(dir, out)}, &o, &e); code != 0 {
			t.Fatalf("export %s: %s", out, e.String())
		}
	}
	if _, err := fromFileExport(&fileExport{Formats: []string{"svg", "html"}}); err != nil {
		t.Fatal(err)
	}
}

// Save as keeps a .yaml name as typed; other names get .adraw.
func TestSaveAsExtension(t *testing.T) {
	dir := t.TempDir()
	for typed, want := range map[string]string{"a": "a.adraw", "b.yaml": "b.yaml", "c.yml": "c.yml", "d.adraw": "d.adraw", "e.txt": "e.adraw"} {
		a := testApp(t)
		ctrl(a, tcell.KeyCtrlS)
		typeStr(a, filepath.Join(dir, typed))
		ctrl(a, tcell.KeyEnter)
		if a.file != filepath.Join(dir, want) {
			t.Fatalf("%s saved as %s, want %s", typed, filepath.Base(a.file), want)
		}
	}
}

// "asciidraw mock" opens mock.adraw or mock.yaml if there is one.
func TestDrawingPath(t *testing.T) {
	dir := t.TempDir()
	at := func(n string) string { return filepath.Join(dir, n) }
	if drawingPath(at("mock")) != at("mock") {
		t.Fatal("nothing there: the name as given")
	}
	os.WriteFile(at("old.yaml"), []byte("version: 2\n"), 0o644)
	if drawingPath(at("old")) != at("old.yaml") {
		t.Fatal("finds old.yaml")
	}
	os.WriteFile(at("old.adraw"), []byte("version: 2\n"), 0o644)
	if drawingPath(at("old")) != at("old.adraw") {
		t.Fatal(".adraw comes first")
	}
	os.WriteFile(at("old"), []byte("x"), 0o644)
	if drawingPath(at("old")) != at("old") {
		t.Fatal("a file with the very name wins")
	}
	if drawingPath(at("new.yaml")) != at("new.yaml") {
		t.Fatal("a drawing name stays")
	}
}

// Layers: l moves shapes, H hides, K locks. Higher layers draw on top;
// hidden ones aren't drawn, picked or exported; locked ones are drawn and
// exported, but clicks go through them.
func TestLayers(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 10, 4) // box A, layer 0
	drag2(a, 5, 2, 15, 6) // box B, layer 0, on top of A
	ctrl(a, tcell.KeyEscape)
	a.sel = 0 // A
	typeStr(a, "l2")
	if a.objs[1].Layer != 2 || a.objs[1].X0 != 0 || a.objs[0].X0 != 5 || a.lay.active != 2 {
		t.Fatalf("A on layer 2, now after B: %+v", a.objs)
	}
	if !a.isSel(1) {
		t.Fatal("A stays selected at its new place")
	}
	a.render()
	if !strings.Contains(screenText(a), "layer 2") {
		t.Fatal("the status bar shows the layer")
	}
	// A new shape goes on the layer new shapes go on: 2.
	ctrl(a, tcell.KeyEscape)
	drag2(a, 30, 0, 40, 4)
	if a.objs[2].Layer != 2 {
		t.Fatalf("new shape on layer 2: %+v", a.objs[2])
	}
	// Moving is undoable.
	ctrl(a, tcell.KeyCtrlZ)
	ctrl(a, tcell.KeyCtrlZ)
	if a.objs[0].Layer != 0 || a.objs[1].Layer != 0 || a.objs[0].X0 != 0 {
		t.Fatalf("undo puts A back on layer 0: %+v", a.objs)
	}
	ctrl(a, tcell.KeyCtrlY)
	ctrl(a, tcell.KeyCtrlY)

	// Hide layer 2: A and the new box vanish, can't be picked, aren't exported.
	ctrl(a, tcell.KeyEscape)
	undos := len(a.undo)
	typeStr(a, "H2")
	if len(a.undo) != undos || !a.dirty {
		t.Fatal("hiding isn't an undo step, but it is a change to save")
	}
	a.render()
	if a.scene().get(0, 0) != ' ' {
		t.Fatal("A isn't drawn")
	}
	if a.pickScene().owner(0, 0) != -1 {
		t.Fatal("A can't be clicked")
	}
	ctrl(a, tcell.KeyCtrlA)
	if len(a.selected()) != 1 {
		t.Fatalf("^A skips hidden shapes: %v", a.selected())
	}
	ctrl(a, tcell.KeyEscape)
	for i := 0; i < 3; i++ {
		ctrl(a, tcell.KeyTab)
		if !a.pickable(a.sel) {
			t.Fatal("Tab skips hidden shapes")
		}
	}
	pages := a.visiblePages()
	if len(pages[0]) != 1 {
		t.Fatalf("export leaves out layer 2: %d shapes", len(pages[0]))
	}
	typeStr(a, "H2")

	// Lock layer 0: B is drawn but clicks go through it.
	typeStr(a, "K0")
	if a.pickScene().owner(6, 6) != -1 || a.scene().owner(6, 6) != 0 {
		t.Fatal("B is drawn, but not pickable")
	}
	if len(a.visiblePages()[0]) != 3 {
		t.Fatal("locked layers export")
	}
	a.render()
	if !strings.Contains(screenText(a), "lock:0") {
		t.Fatal("the status bar shows the lock")
	}

	// Saved and read back: layers, hidden, locked, the active layer.
	typeStr(a, "H5")
	path := filepath.Join(t.TempDir(), "l.adraw")
	a.write(path)
	yml := string(must(os.ReadFile(path)))
	for _, want := range []string{"layer: \"2\"", "hidden: \"5\"", "locked: \"0\"", "active: \"2\""} {
		if !strings.Contains(yml, want) {
			t.Fatalf("yaml lacks %s:\n%s", want, yml)
		}
	}
	pp, m, _, _, err := loadDrawing(path)
	if err != nil || len(m.Layers) != 1 || m.Layers[0] != a.lay || pp[0][1].Layer != 2 {
		t.Fatalf("reload: %v %+v", err, m.Layers)
	}
	// Shapes come back in layer order even if the file mixes them up.
	objs, _, err := decodeDoc([]byte("pages:\n  - objects:\n      - kind: box\n        layer: \"3\"\n      - kind: text\n        text: [hi]\n"))
	if err != nil || objs[0][0].Kind != "text" || objs[0][1].Layer != 3 {
		t.Fatalf("layer order on load: %v %+v", err, objs)
	}
	for _, bad := range []string{
		"pages:\n  - objects:\n      - kind: box\n        layer: \"?\"\n",
		"pages:\n  - hidden: \"2!\"\n    objects: []\n",
	} {
		if _, _, err := decodeDoc([]byte(bad)); err == nil {
			t.Fatalf("bad layer accepted: %q", bad)
		}
	}

	// Layers are per page: a new page starts with none hidden or locked.
	typeStr(a, ".")
	if a.lay != (layerState{}) {
		t.Fatalf("new page: %+v", a.lay)
	}
	typeStr(a, ",")
	if a.lay.locked != layerBit(0) {
		t.Fatal("the first page keeps its lock")
	}

	// A key that isn't a layer name cancels.
	typeStr(a, "l")
	ctrl(a, tcell.KeyEscape)
	if a.layerCmd != 0 {
		t.Fatal("Esc cancels")
	}
}

// A locked layer's box can't be typed into, and its group-mates on other
// layers don't drag it into the selection.
func TestLockedLayerHoles(t *testing.T) {
	a := testAppSized(t, 100, 30)
	drag2(a, 0, 0, 10, 4)  // box on layer 0
	drag2(a, 20, 0, 30, 4) // box on layer 0
	ctrl(a, tcell.KeyEscape)
	a.setSelection([]int{0, 1}, 1)
	typeStr(a, "g") // one group
	a.sel = 0
	a.setSelection([]int{0}, 0)
	typeStr(a, "l1") // first box to layer 1; its group-mate stays on 0
	ctrl(a, tcell.KeyEscape)
	typeStr(a, "K0")
	click(a, 24, 2) // inside the locked box
	if a.typing && a.edit >= 0 && a.objs[a.edit].Kind == "box" {
		t.Fatal("a click in a locked box doesn't type its label")
	}
	ctrl(a, tcell.KeyEscape)
	ctrl(a, tcell.KeyTab)
	for _, i := range a.selected() {
		if !a.pickable(i) {
			t.Fatalf("Tab brought in a locked group-mate: %v", a.selected())
		}
	}
}

func TestHideCurrentLayerWarns(t *testing.T) {
	a := testApp(t)
	typeStr(a, "H0")
	if !strings.Contains(a.msg, "New shapes still go on it") {
		t.Fatalf("hiding the current layer says so: %q", a.msg)
	}
	typeStr(a, "H0")
	typeStr(a, "K3")
	if strings.Contains(a.msg, "New shapes") {
		t.Fatalf("locking another layer doesn't: %q", a.msg)
	}
}
