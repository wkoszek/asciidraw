# asciidraw

Draw ASCII mockups in the terminal with your mouse.

You drag to make boxes, ovals, lines and arrows. You click to type. You can
also add widgets, such as buttons, checkboxes, input fields and tabs. Each
shape stays separate, so you can move it, resize it, copy it or delete it
later. Save the drawing as YAML and export the picture as Markdown.

```
╭──────────────────╮                ╭────────────────╮
│  Login           │───────────────>│  Dashboard     │
│  [ user      ]   │                ╰────────────────╯
╰──────────────────╯
```

You need a terminal that passes mouse clicks and drags to programs. asciidraw
is built and used on macOS in Ghostty. On other systems everything works
except the system clipboard: copy and paste stay inside asciidraw.

## Install

You need Go 1.27.3 or newer.

```sh
git clone https://tig.koszek.com/wkoszek/asciidraw.git
cd asciidraw
make install
```

`make install` puts `asciidraw` in `$GOBIN`, or in `$(go env GOPATH)/bin`
if `GOBIN` isn't set. That is usually `~/go/bin`. Make sure that folder is on
your PATH.

## Run

```sh
asciidraw              # a new, untitled drawing
asciidraw mock.yaml    # open a drawing, or start a new one under that name
asciidraw notes.md     # bring in a picture from Markdown or plain text
```

In this README, `^T` means Ctrl+T.

## Drawing

- **Drag** on empty space to draw with the current tool.
- **Click** on empty space to type. Enter starts a new line. Esc stops
  typing. With the Erase tool, a click deletes instead. Inside a box or an
  oval, a click starts typing that shape's label; see Labels below.
- **Right-drag** a rectangle to delete every shape it touches. This works
  with any tool. The Erase tool does the same with a left drag.
- **^T** opens the menu. Pick a tool (Box, Oval, Line, Arrow, Fill, Erase, Select) and a
  style (ascii, line, round, double). Use the up and down arrows to move,
  Space to pick, Enter to pick and close, and Esc to close. You can also
  click a row. The status bar shows a sample of each style before you pick
  it.
- **Shift** while you drag a box or an oval makes it square or round.

A terminal cell is about twice as tall as it is wide. So a circle 10 rows
tall is about 20 columns wide.

A line goes sideways from where you press, turns once, then goes up or down
to where you let go. Drag straight across or straight down for a line with
no turn. An arrow is the same line with a head on the end. A line can also
attach to shapes and follow them when they move. An attached line picks its
own turns; see Attached lines below.

Where lines and shape edges meet, they join instead of one covering the
other. The shapes stay separate: move one away, and the join goes away.

```
┌─────┐             ┌─────┐
│     │             │     │
│  ┌──┼──┐    ──────┤     │
│  │  │  │          │     │
└──┼──┘  │          └──┬──┘
   │     │             │
   └─────┘             │
```

Lines that cross make a cross: `┼` in the line and round styles, `╬` in
double, and `+` in ascii. A line that ends square on a box's edge, away from
its corners, makes a T, such as `┤` or `┬`. To get one, start the line away
from the box and end it on the edge, or drag one of its ends onto the edge.
Box edges, window borders and tall button borders all join this way. When
two styles meet, the shape drawn last picks the style of the join.

Arrowheads, text, labels and the curves of ovals don't join. Each covers
whatever was drawn before it, and anything drawn later covers it. The
Markdown export shows the joins just as the screen does.

## Widgets

Widgets are ready-made UI parts. There are nine: button, checkbox, radio
button, input field, dropdown, tabs, progress bar, scrollbar and window.

```
button   checkbox           radio        dropdown
[ OK ]   [x] Remember me    (•) Option   [ Choose     ▼ ]

input field                 progress bar
[user______________]        [█████░░░░░░░░]  40%

tabs
│ Home │ Settings │ About │
│      └──────────┴───────┘
```

The ^T menu lists the widgets in a second column, to the right of the
tools. Pick one and close the menu with Esc. Then click empty space to place
the widget at its default size, or drag to set its size. Drag a widget's
corner later to resize it.

Some widgets limit their size. A checkbox or a radio button is always one
row, as wide as its label. A button is one row, unless you drag it 3 or more
rows tall; then it becomes a box with the label in the middle.

To change a widget, click it to select it:

- **Enter** lets you edit its label. You type in the status bar, Enter
  saves, and Esc cancels. For tabs, type the names with commas between
  them. For a progress bar or a scrollbar, type a number from 0 to 100. On
  a scrollbar it sets where the handle sits.
- **Space** checks or unchecks a checkbox or radio button, or makes the next
  tab the active one. On a progress bar or scrollbar it adds 10, and after
  100 it goes back to 0.

Radio buttons don't know about each other. Checking one leaves the others as
they are.

A window's inside is empty space. To draw in it, switch to a drawing tool
such as Box first; with the Window tool still picked, a click places another
window. Shapes inside a window don't move with it unless you group them
with the window (see Groups below).

A new widget uses the style picked in the menu, and keeps it. In the ascii
style it uses only plain characters, such as `#` and `.` for a progress bar.
Widgets export to Markdown like everything else.

## Changing shapes

- **Click** a shape to select it. Click selected text again to type in it.
- **Enter** on a selected box, oval or text also starts typing in it. The
  cursor starts after the last letter.
- **While you type,** the arrow keys move the cursor, not the shape. Home
  and End jump to the start and end of the line. Letters go in at the
  cursor. Backspace deletes the letter before it, and Delete the letter
  after it. Enter in the middle of a line splits it in two.
- **Drag** a shape's edge to move it.
- **Drag** a box corner, or either end of a line, to resize it. An oval has
  no corners you can see, so a selected oval shows `+` where they are.
- **Tab** and **Shift-Tab** step through the shapes. The arrow keys nudge
  the selected one.

### Labels

A box or an oval can have a label. With Box, Oval, Line or Arrow picked,
click the empty space inside the shape and type. Enter starts a new line
and Esc stops typing. The label sits in the middle of the shape. Move, copy
or delete the shape, and the label goes with it.

```
+-----------+
|           |
|    OK     |
|           |
+-----------+
```

If the label outgrows the shape, the shape grows right and down to fit.
You can't drag a shape smaller than its label. Deleting letters doesn't
shrink it back; drag a corner for that.

To change a label, click the shape to select it, then click a letter of its
label. If the shape is in a group, click it once more first. You can also
select it and press Enter. Either way, the cursor starts after the last
letter, and you edit as with any text (see Changing shapes).
Backspace over every letter, and the label is gone; the shape stays.

The panel shows each label next to its shape. Widgets have labels too, but
you edit those in the status bar; see Widgets above.

To add text that isn't part of the shape, Shift-click the empty space
inside it. That text is a shape of its own, and it doesn't move with the box
unless you group them (see Groups below).

### Attached lines

Pick Line or Arrow. Start in the empty cell just outside a shape, and end
on another shape's edge or just outside it. Each end attaches to the shape
it touches. While you drag, that shape lights up. Lines attach to boxes and
widgets, and to the rectangle around an oval. Text and other lines don't
take attachments.

Don't start on the edge itself: pressing on a shape grabs the shape and
moves it.

```
╭─────────╮
│  Login  │────────────╮             ╭─────────╮
╰─────────╯            │             │  Home   │
                       ╰────────────>│         │
                                     ╰─────────╯
```

Move or resize the shape, and the end follows. It stays on the same side, at
the same spot along it. If the side gets too short for that spot, the end
moves to the end of the side.

The line picks its turns from where its ends are attached:

- If both ends are on left or right sides, it turns twice, halfway between
  them, as above.
- If both ends are on tops or bottoms, it turns twice, halfway between them.
- Otherwise it turns once.

To change what a line is attached to:

- Drag one of its ends onto a shape's edge, or just outside it, to attach
  that end. If the line is in a group, click it twice first, so that it is
  selected alone.
- Drag an end away from its shape to detach it. Moving the whole line by
  itself, with the mouse or the arrow keys, checks both ends again where
  the line lands.
- Select a line and its shapes and move them together, and the line stays
  attached.
- Deleting a shape detaches only that end. The line stays, and may turn
  differently.
- A pasted line stays attached only to shapes pasted along with it.

Attachments are saved with the drawing, and undo puts them back.

### The panel

The panel on the right lists every shape. Hover over a row to light up its
shape. Click a row to select its shape. Shift-click rows to add or remove
shapes. Drag a row up or down to move its shape in the stacking order (see
Stacking order below). **^O** hides or shows the panel. It also hides when
the window is narrower than 72 columns.

### Stacking order

Shapes are drawn in the order of the panel's list, top to bottom, so a
shape lower in the list covers the ones above it.

- **]** moves the selected shapes to the front, which is the bottom of the
  list. **[** moves them to the back, the top of the list. A grouped shape
  moves with its group.
- Drag a row down in the panel to put its shape just below the row you drop
  it on, or up to put it just above.

### Fill

The Fill tool shades a rectangle: drag one, as you would a box. A new fill
goes to the back, behind everything else. To change it, click a part of it
that shows, or its row in the panel. Then press **Space** or `t` for the
next shade. Each press steps through `░ ▒ ▓ █`, then the plain `. : % #`,
then back to `░`. These are standard characters, so no special font is
needed. A fill never blocks clicks meant for a box on top of it: a click
inside the box still types the box's label.

### Colors

Select one or more shapes and press **C** (Shift-c). A small panel opens
with two rows of colors: **Text** for a shape's lines and letters, and
**Back** for its background. Each shape takes on a color as you move to it.

- The left and right arrow keys move between colors, and the up and down
  arrow keys switch between the two rows. You can also click a color.
- Enter keeps the colors. Esc puts the old ones back. A click outside the
  color panel also keeps them.
- Every selected shape gets the colors, including every shape in a
  selected group. Changing only the Text row leaves each shape's
  background as it was, and the other way around.

There are ten choices. The first, shown as `··`, is the default: your
terminal's normal text and background. Pick it to take a color off. The
others are black, red, green, yellow, blue, magenta, cyan, white and gray.
They come from your terminal's theme, so red in one theme can look
different in another.

A background fills the inside of a box or a window, and the inside of an
oval's curve. For white letters on a blue box, set Text to white and Back
to blue. On lines, text, fills and widgets, the background goes only behind
the characters themselves.

A selected shape shows its lines and letters in the selection color, but
its background stays. To see a shape's real text color, click away, or
open the color panel, which shows the real colors while it is open. The
panel on the right shows a small swatch next to each colored shape.

Colors work in every style, ascii included. Undo takes them off in one
step, and copy, paste and ^D keep them. They are saved in the YAML by name,
such as `fg: red` and `bg: blue`. The Markdown export keeps the characters
and drops the colors.

### Duplicate

**^D** copies the selected shapes and puts the copy two columns right and
one row down. Don't confuse it with `d`, which cuts. The copies are
selected, so you can drag them where they go. Copied groups and lines
attached among the copies stay that way. Duplicating doesn't touch the
clipboard, so what you copied with `y` is still there.

## Selecting several shapes

- **Shift-click** a shape to add it to the selection. Shift-click it again
  to take it out. This doesn't work with the Erase tool, where a click
  deletes.
- **v** switches to the Select tool. Start on empty space and drag a
  rectangle around the shapes you want. Every shape fully inside it gets
  selected. This replaces the selection, unless you hold Shift to add to
  it. Press **v** again to go back to your last tool.
- **^A** selects everything. **Esc** clears the selection. If you are
  typing, the first Esc stops typing and the second clears.

Dragging any selected shape moves all of them, and the arrow keys nudge all
of them. `y` copies them, `d` cuts them, and `x` deletes them. `p` pastes
what you copied at the mouse. To resize a shape, select it alone.

### Groups

Select two or more shapes and press **g** to group them. After that, the
group acts as one piece:

- A click on any shape in the group selects the whole group, so a window
  and its buttons move together.
- Shift-click adds or removes the whole group.
- The panel tags each shape in a group, such as `g1`.

To change one shape in a group, click it once to select the group, then
click it again. Now only that shape is selected. It stays in the group, but
you can move, resize or edit it alone.

To ungroup, select one whole group and press **g**. If the selection mixes
a group with other shapes, **g** makes one new group of all of them. Groups
don't nest. Groups are saved with the drawing, and a pasted copy of a group
is a new group.

## Keys

When you are not typing, these letters work as commands:

| Key | Does | Same as |
|---|---|---|
| `y` | copy | ^C |
| `d` | cut | ^X |
| `p` | paste where the mouse is | ^V |
| `x` | delete, and leave the clipboard alone | Del |
| `g` | group, or ungroup | ^G |
| `v` | switch to the Select tool; press again to go back | |
| `t` | change the style of the selected shapes: ascii, line, round, double | |
| `]` / `[` | bring to the front / send to the back | |
| `C` (Shift-c) | set the text and background colors of the selected shapes | |

They come from vim. Press Esc first if you are typing.

Copying also puts the shape's picture on the macOS clipboard as plain text,
so you can paste it into other apps. Paste uses whatever you copied last. If
that was text from another app, it lands at the mouse as a new text shape.
If you are typing, it goes into the text instead.

| Key | Does |
|---|---|
| ^S | save |
| ^E | export Markdown |
| ^N | new drawing |
| ^Z / ^Y | undo / redo |
| ^T | tools and styles |
| ^A | select everything |
| ^O | show or hide the panel |
| ^D | duplicate the selected shapes |
| ^Q | quit |
| `?` | show the keys on one screen |

If you have unsaved changes, ^N and ^Q warn you. Press the key again to go
ahead.

## Scrolling

The drawing can be bigger than the window, and has no size limit.

- Over the drawing, the mouse wheel scrolls up and down, and Shift with the
  wheel scrolls sideways. Over the panel, the wheel scrolls its list.
- When you aren't typing, PgUp and PgDn scroll by half a window, and Home
  goes back to the top left.
- When you drag to the window's edge, the view scrolls with you, so you can
  keep drawing past it. While you type, the view follows the cursor.

When the view isn't at the top left, the status bar shows the column and row
at the window's corner, such as `view 3,6`.

## Autosave

Every 30 seconds, asciidraw writes any unsaved changes to a recovery copy.
It also writes one when the terminal tells it the window is closing. If it
crashes, you lose at most the last 30 seconds of work. Your drawing's own
file only changes when you press ^S.

The next time you open the same drawing, asciidraw finds the copy, loads
it, and says so. Press ^S to keep the changes. Saving deletes the copy.

Quitting with ^Q twice, or starting over with ^N twice, throws the unsaved
changes away for good and deletes the copy too.

On macOS the copies live in `~/Library/Caches/asciidraw/recover/`, and on
Linux in `~/.cache/asciidraw/recover/`. An untitled drawing has one copy,
`untitled.yaml`, which comes back the next time you start asciidraw with no
file. Two untitled sessions at once share that copy, and the last one to
write it wins.

## Files

**^S** saves the drawing as YAML, one entry per shape. Coordinates are
columns and rows, counted from 0 at the top left. Styles are stored by name,
so the file is easy to edit by hand:

```yaml
version: 1
objects:
  - kind: box
    style: round
    x0: 2
    y0: 1
    x1: 24
    y1: 6
    text:
      - Login
    id: 1
  - kind: text
    x0: 4
    y0: 8
    text:
      - Forgot password?
    id: 2
```

The box's `text` is its label. The second entry is text on its own. The
`id` names a shape, so that lines can say what they are attached to.

**^E** writes the picture inside a code block. For `mock.yaml` it writes
`mock.md` in the same folder. If the drawing has no name yet, it asks for
one.

If `mock.md` already exists, ^E asks before replacing it. After that, later
exports in the same session replace it without asking. Make your changes in
the drawing, not in the export. Exporting does not save the drawing; press
^S for that.

Opening a `.txt` or `.md` file brings the picture in as one text shape. For
Markdown, it takes the first code block fenced with backticks, or the whole
file if there is none. The first save then offers the same name with
`.yaml`.

## Cmd keys in Ghostty

asciidraw doesn't see the Cmd key. To make Cmd+S and Cmd+N send ^S
and ^N to asciidraw, add this to `~/.config/ghostty/config`:

```
keybind = super+s=text:\x13
keybind = super+n=text:\x0e
```

Ghostty uses Cmd+N to open a new window. With the second line, Cmd+N stops
doing that, in every program.

asciidraw asks the terminal to pass Shift+drag through to it. If Shift+drag
still selects text in Ghostty, add `mouse-shift-capture = true` to the same
config file.

## Development

```sh
make check     # gofmt, go vet, tests
make run       # build and start; make run FILE=mock.yaml opens a file
make help      # every target
```

The tests drive the program through a fake terminal screen. They click,
drag and type, then check the shapes and the saved files.
