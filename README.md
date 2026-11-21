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
  typing. With the Erase tool, a click deletes instead.
- **Right-drag** a rectangle to delete every shape it touches. This works
  with any tool. The Erase tool does the same with a left drag.
- **^T** opens the menu. Pick a tool (Box, Oval, Line, Arrow, Erase, Select) and a
  style (ascii, line, round, double). Use the up and down arrows to move,
  Space to pick, Enter to pick and close, and Esc to close. You can also
  click a row. The status bar shows a sample of each style before you pick
  it.
- **Shift** while you drag a box or an oval makes it square or round.

A terminal cell is about twice as tall as it is wide. So a circle 10 rows
tall is about 20 columns wide.

A line goes sideways from where you press, turns once, then goes up or down
to where you let go. Drag straight across or straight down for a line with
no turn. An arrow is the same line with a head on the end.

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

- **Click** a shape to select it. Click selected text again to add to it.
  Typing only adds and removes at the end of the text. To change the
  middle, delete the text and type it again.
- **Drag** a shape's edge to move it.
- **Drag** a box corner, or either end of a line, to resize it. An oval has
  no corners you can see, so a selected oval shows `+` where they are.
- **Tab** and **Shift-Tab** step through the shapes. The arrow keys nudge
  the selected one.

Arrows don't stay attached to boxes yet. If you move a box, move its arrows
too.

The panel on the right lists every shape. Hover over a row to light up its
shape. Click a row to select its shape. Shift-click rows to add or remove shapes. **^O** hides or shows the panel. It also
hides when the window is narrower than 72 columns.

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
| ^Q | quit |

If you have unsaved changes, ^N and ^Q warn you. Press the key again to go
ahead.

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
  - kind: text
    x0: 5
    y0: 3
    text:
      - Login
```

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
