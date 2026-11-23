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

You need a terminal that passes mouse clicks and drags to programs. I build and
use asciidraw on macOS in Ghostty. It also runs on Linux, Windows and the
BSDs.

## Install

On macOS or Linux, with Homebrew:

```sh
brew install wkoszek/tap/asciidraw
```

On Debian, Ubuntu, Fedora, Alpine or Arch, download the `.deb`, `.rpm`,
`.apk` or `.pkg.tar.zst` file from the
[releases page](https://github.com/wkoszek/asciidraw/releases) and install it
with your package manager. The same page has a `.tar.gz` archive for macOS,
Linux and the BSDs, and a `.zip` for Windows. Each holds the binary.

With Go 1.24 or newer:

```sh
go install github.com/wkoszek/asciidraw@latest
```

Go puts `asciidraw` in `$GOBIN` if you set it, and in `$(go env GOPATH)/bin`
otherwise. That is usually `~/go/bin`. Make sure that folder is on your PATH.

`asciidraw --version` prints the version. `man asciidraw` shows the manual.
`go install` installs only the binary, without the manual.

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

A box or an oval can have a label. Pick the Box, Oval, Line or Arrow tool,
then click the empty space inside a box or an oval and type. Enter starts a
new line and Esc stops typing. The label sits in the middle of the shape,
unless you move it with **T** (Shift-t; see Label position below). Move,
copy or delete the shape, and the label goes with it.

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

The shape list on the right shows each label next to its shape. Widgets
have labels too, but you edit those in the status bar; see Widgets above.

To add text that isn't part of the shape, Shift-click the empty space
inside it. That text is a shape of its own, and it doesn't move with the box
unless you group them (see Groups below).

#### Label position

Select a box or an oval and press **T** (Shift-t) to choose where its
label sits. Press Esc first if you're typing. A small window opens with
nine buttons:

```
[ top left    ] [ top center    ] [ top right    ]
[ center left ] [ center        ] [ center right ]
[ bottom left ] [ bottom center ] [ bottom right ]
```

The arrow keys move between the buttons, and the label moves with them, so
you see each spot on the shape. Enter keeps the new spot. Esc puts the
label back where it was before you pressed T. A click on a button picks
it and closes the window. A click outside the window keeps the spot the
arrows reached.

The number keys work like a keypad: `7` is top left, `5` is center and `3`
is bottom right. One press picks the spot and closes the window.

A label on the left or right keeps one blank column from the border. In an
oval, the label stays inside the curve, so it sits a little farther in. If
the label fills the shape, it has no room to move and stays centered; give
the shape more room and it moves to the spot you picked. Every selected box
and oval gets the same spot. Lines, widgets and loose text in the selection
stay as they are. One ^Z undoes the move.

The YAML saves the spot as `place`, such as `place: top left`. When you
pick center, the file has no `place`.

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

Select one or more shapes and press **C** (Shift-c). If you're typing a
label, press Esc first. A small panel opens with three rows, from top to
bottom: **Text** for a shape's lines and letters, **Back** for its
background, and **Pal**, a palette of 16 colors of your own.

- The up and down arrow keys move between rows. The left and right arrow
  keys move along a row, and the selected shapes take on each color as you
  go. You can also click a color.
- Enter keeps the colors. Esc puts the old ones back. A click outside the
  color panel also keeps them.
- Every selected shape gets the colors, including every shape in a
  selected group. Changing only the Text row leaves each shape's
  background as it was, and the other way around.

The Text and Back rows each have ten choices. The first, shown as `··`, is
the default: your terminal's normal text and background. Pick it to take a
color off. The others are black, red, green, yellow, blue, magenta, cyan,
white and gray. They come from your terminal's theme, so red in one theme
can look different in another.

For any other color, press `#` in the color panel. The status bar asks for
a hex color, with the `#` already typed. Type three or six hex digits
(0-9, a-f), such as `ff8800`, or `f80` for short, and press Enter. If the
row already has a hex color, its code is typed in for you; Backspace erases
it. Esc cancels, and a code that isn't a color is refused.

On the Text and Back rows, the hex color goes in an extra swatch at the end
of the row. The arrow keys can reach it again. Each row has one such
swatch, so a second hex color replaces the first.

Ghostty, which supports 24-bit color, shows hex colors exactly. Terminals
without it show the nearest color they have.

A background fills the inside of a box or a window, and the inside of an
oval's curve. For white letters on a blue box, set Text to white and Back
to blue. On lines, text, fills and widgets, the background goes only behind
the characters themselves.

A selected shape shows its lines and letters in the selection color, but
its background stays. To see a shape's real text color, click away, or
open the color panel, which shows the real colors while it is open. The
panel on the right shows a small swatch next to each colored shape.

Colors work in every style, ascii included. Undo puts back the colors a
shape had before, in one step, and copy, paste and ^D keep them. The
Markdown export keeps the characters and drops the colors.

The YAML stores them as `fg` for the text color and `bg` for the
background: named colors by name, such as `fg: red`, and hex colors in
quotes, such as `fg: "#ff8800"`. Keep the quotes, since YAML reads a bare
`#` as the start of a comment. When you edit a file by hand, `#FF8800` and
`#f80` work too; asciidraw stores them as `#ff8800`.

#### Your palette

The Pal row holds 16 colors. It starts with four grays and twelve bright
colors.

A palette color goes on the text or the background. Which one depends on
how you got to the Pal row. Press down from Back, and it colors the
background. Press up from Text, which wraps around to the bottom, and it
colors the text. The line under the palette says which one it is. On the
Pal row, Space switches to the other one and puts the color there. With
the mouse, click the word Text or Back, then click a palette color.

To change a palette color, move to it and press `p`, or click
**[ p Palette ]**. A grid of ready-made colors opens above the panel. It
has 19 columns. The first runs from white through gray to black, and the
other 18 are hues, each in six shades from light at the top to dark at the
bottom. The arrow keys move through the grid, and the shapes show each
color as you go.

- Enter, Space or a click on a color puts it in the palette and closes the
  grid. Press Enter again to keep it on the shapes.
- Esc, or a click anywhere off the grid, closes the grid and leaves the
  palette color as it was.
- On the Pal row, `#` lets you type a hex code for that palette color.

A shape keeps its own copy of a color. Changing a palette color later
doesn't recolor shapes you already colored with it.

The palette is saved with the drawing, as a list called `palette` in the
YAML. The file has no `palette` list until you change a color. The list
takes up to 16 colors, by name or as quoted hex codes, and any it leaves
out keep their starting colors.

^N (new drawing) keeps the palette, so the next drawing starts with the
same colors. Esc on the color panel puts back the palette along with the
shapes' colors. Undo doesn't change the palette.

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

### Align and spread out

Select two or more shapes and press **a**. The status bar lists the
choices. Press one more key:

| Key | Does |
|---|---|
| `h` or ← | lines up the left edges |
| `l` or → | lines up the right edges |
| `k` or ↑ | lines up the top edges |
| `j` or ↓ | lines up the bottom edges |
| `c` | lines up the centers from left to right, as in a column |
| `m` | lines up the centers from top to bottom, as in a row |
| `x` | spreads the shapes out evenly, left to right |
| `y` | spreads the shapes out evenly, top to bottom |

Esc, any other key, or a click cancels. A cancelling click does nothing
else.

Each command moves the shapes one way only. Lining up left edges moves
shapes sideways, to the leftmost edge in the selection. Lining up top
edges moves them up or down. `c` and `m` line the shapes up on the center
of the whole selection.

Spreading needs three or more shapes. For `x`, the leftmost and rightmost
shapes stay put, and the ones between move so the gaps are all the same.
For `y`, the topmost and bottommost stay put. If the space doesn't divide
evenly, some gaps are one cell wider. If the shapes are too wide for the
space, they overlap by the same amount.

A selected group counts as one shape. Its shapes move together and keep
their places inside it. If you picked one shape out of a group, that shape
moves alone. Lines attached to shapes don't count. Their attached ends
move with their shapes.

One ^Z undoes the whole move.

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
| `T` (Shift-t) | choose where the label sits in a box or an oval | |
| `a` | align or spread out the selected shapes, then press a second key (see Align and spread out) | |
| `>` / `<` | go to the next page / the page before (see Pages) | `.` / `,` |
| `+` | copy this page to a new page after it, and go there | |

They come from vim. Press Esc first if you are typing.

Copying also puts a picture of your selection on the system clipboard as
plain text, for other apps. See [The clipboard](#the-clipboard) for how that
works on each system. Paste checks the system clipboard first. If it holds
text from another app, that text becomes a new text shape at the mouse, or
goes in at the cursor if you are typing. Otherwise paste uses the shapes you
last copied in asciidraw.

| Key | Does |
|---|---|
| ^S | save |
| ^E | export (see Export) |
| S (Shift-s) | export settings: formats, font size, theme, pages |
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

## Pages

A drawing can have several pages, such as one screen of an app per page.
A new drawing has one page.

- **`>`** goes to the next page. On the last page, it adds a new, empty
  page. If the last page is already empty, nothing happens.
- **`<`** goes back one page.
- **`+`** copies the page you're on to a new page right after it, and takes
  you there. Use it to draw the next state of a screen without starting
  over. ^Z can't undo `+`.

`.` and `,` do the same as `>` and `<`, so you don't need Shift. These
keys work with or without a selection. If you're typing in a shape, press
Esc first.

Each page has a one-character name. The first ten are `0` to `9`, the
next 26 are `a` to `z`, and the last 26 are `A` to `Z`. That makes 62
pages at most. Once you have 62, `>` and `+` can't add another; the
status line says so.

The bottom right corner shows five names at a time, as in `Pages:01234`.
The page you're on is yellow, and names of pages you haven't made yet are
dim. As you move, the names slide along and the page you're on stays in
view. Click a bright name to go to that page.

Each page has its own shapes and its own undo: ^Z only undoes changes on
the page you're on. To copy shapes to another page, select them, press
`y`, go to that page, and press `p`.

When you leave an empty last page for an earlier one, asciidraw drops it.
It keeps the page if you drew there and erased, so ^Z and ^Y still work.
Saving leaves out empty pages at the end too.

You can't delete a page in the middle yet. To clear a page, press ^A, then
`x`. The empty page stays.

^E exports all the pages. See Export for how they are split into files.

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

**^S** saves the drawing as YAML: a list of pages, each with one entry per
shape. Coordinates are columns and rows, counted from 0 at the top left.
Styles are stored by name, so the file is easy to edit by hand:

```yaml
version: 2
pages:
  - objects:
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
  - objects:
      - kind: box
        x0: 2
        y0: 1
        x1: 24
        y1: 6
        id: 1
```

This drawing has two pages. On the first page, the box's `text` is its
label, and the second entry is text on its own. The second page has a box
with no `style`, so it uses the first style, ascii. The `id` names a
shape, so that lines can say what they are attached to. Ids only need to
be different within one page, so two pages can use the same ids.

Files saved before pages existed say `version: 1` and have an `objects`
list and no `pages`. They open as one page. The next save turns them into
`version: 2`.

The export settings from **S** are saved in the file too; see Export.

Opening a `.txt` or `.md` file brings the picture in as one text shape. For
Markdown, it takes the first code block fenced with backticks, or the whole
file if there is none. The first save then offers the same name with
`.yaml`.

## Export

**^E** exports the drawing in each format ticked in the export settings:
Markdown, PNG and JPEG. Only Markdown is ticked at first. For `mock.yaml`
it writes `mock.md`, `mock.png` and `mock.jpg` in the same folder. If the
drawing has no name yet, it asks for one.

If a file it would write is already there, ^E asks before replacing it.
Files that ^E wrote earlier in the same session are replaced without
asking. The next export overwrites the file, so make your changes in the
drawing, not in the export. Exporting does not save the drawing; press
^S for that.

**S** (Shift-s) opens the export settings. If you're typing text, press
Esc first. Up and down move between rows, and left and right move along
a row. Enter keeps your changes and Esc throws them away. You can also
click.

- **Write** picks the formats. Press Space to tick or untick the one under
  the cursor. You can't untick the last one.
- **Font** is the font size for PNG and JPEG: 10, 12, 14, 16, 20 or 24
  pixels. 12 is the default. At 12, each character takes 7 by 15 pixels.
  The window shows the image size of the page you're on.
- **Theme** is light (dark text on white) or dark (light text on near
  black).
- **Pages** says what to do with a drawing of several pages. "One file"
  writes one file per format, such as `mock.md` and `mock.png`, with all
  the pages in it. "A file per page" adds the page's name to each file:
  `mock_page0.png`, `mock_page1.png` and so on. Page names start at `0`,
  as in the corner of the screen. A drawing with one page always gets
  plain names such as `mock.png`.

The settings are saved in the drawing file, under `export`, once you
change them from the defaults. Changing them marks the drawing as
changed, so press ^S to keep them.

When a Markdown file holds several pages, each page gets its own code
block, with a line such as `Page 2` above it. When an image holds
several pages, they sit one under another with a thin line between them.
Empty pages at the end are left out.

PNG is the format to pick for small files. A drawing that fills the
screen usually makes a PNG of a few kilobytes. A JPEG of the same
drawing is usually several times bigger, because JPEG is made for
photos, not lines and letters. A JPEG with no color in it is saved in
grays, which makes it a little smaller.

Letters use Go Mono, a font built into asciidraw, so an image looks the
same on every computer. Box lines, corners, shades and arrowheads are
drawn as shapes, so lines meet with no gaps. The ascii style's `-`, `|`
and `+` are letters, like the rest of the text. Named colors like red
get one fixed shade in images.

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

## The clipboard

On macOS, asciidraw uses `pbcopy` and `pbpaste`. On Windows and in WSL, it
uses PowerShell. On Linux and the BSDs, it uses `wl-copy` and `wl-paste` under
Wayland, or `xclip` or `xsel` under X11. Copy works without them, but you
need one to paste text from other apps.

Over SSH, without those programs, or when one of them fails, asciidraw sets
the clipboard with an escape code called OSC 52. Ghostty, iTerm2, kitty,
WezTerm, Windows Terminal and recent xterm all honor it. In tmux, add
`set -g set-clipboard on` to `~/.tmux.conf`. asciidraw only writes through
OSC 52, so paste then uses the shapes you last copied in asciidraw.

## Development

```sh
make check     # gofmt, go vet, tests
make run       # build and start; make run FILE=mock.yaml opens a file
make snapshot  # build every release package into dist/, publish nothing
make help      # every target
```

To release, use a Mac with GoReleaser and `opc`. It needs the Developer ID
Application certificate in the keychain and a notarytool profile;
`scripts/macos-sign.sh` shows how to make the profile. It also needs a GitHub
token in 1Password at `op://infra/github-pat-wkoszek/token`. The token
must be able to publish releases on `wkoszek/asciidraw` and push to
`wkoszek/homebrew-tap`. Last, it needs a tig token at
`op://infra/gitea-pat-wkoszek/token`, to tell tig to update the GitHub
mirror.

Commit subjects follow [Conventional Commits](https://www.conventionalcommits.org).
The commits since the last tag set the next version, and the biggest bump
wins:

| Subject | Version | Example |
| --- | --- | --- |
| `fix:` or `perf:` | patch | v1.2.3 to v1.2.4 |
| `feat:` | minor | v1.2.3 to v1.3.0 |
| any type with `!`, as in `feat!:` | major | v1.2.3 to v2.0.0 |

A line in the message body that starts with `BREAKING CHANGE:` also makes a
major bump. Before v1.0.0, a breaking change bumps the minor version instead.
The other types (`docs`, `test`, `refactor`, `build`, `ci`, `chore`, `style`
and `revert`) make no release unless they break something. A scope is
optional, as in `fix(clipboard): keep tabs when pasting`. Keep subjects to 72
characters or fewer.

Run `make hooks` once in each clone. Then git rejects a commit whose subject
breaks these rules. It lets through the subjects git writes itself, the ones
that start with `Merge`, `Revert "`, `fixup!`, `squash!` or `amend!`. CI checks
the commits in each push or pull request to `main` the same way. When it
can't find where a push started, it checks only the newest commit.

To release, run `make rel`. It drafts the new version's section of
`CHANGELOG.md` from the commits and opens it in `$EDITOR`. Turn the draft
into plain sentences and save. `make rel` commits the section, pushes `main`
to tig, and runs `make release`. `make next` shows the version ahead of time.

The same steps, one at a time:

```sh
make next       # the next version, and the commits behind it
make changelog  # draft its CHANGELOG.md section from those commits
                # edit the draft into plain sentences, then commit it
git push tig main
make release
```

`make release` first runs `make check`. Then it tags HEAD with the next
version and pushes the tag to tig. It asks tig to update the GitHub mirror,
and waits up to two minutes for the tag to show up there. Before it tags, it
stops unless:

- you're on `main` and have no uncommitted changes,
- HEAD is the same commit as tig's `main`, and
- `CHANGELOG.md` has a section for the new version.

When some commits since the last tag don't follow the format, `make rel`
can't pick the version. It lists those commits and prints the three commands
to choose from, such as `make rel BUMP=minor`, with the version each one
makes. Pass `BUMP` again if you rerun before the tag exists. If a release
fails after the tag is made, run `make rel` again. It reuses the tag on
HEAD.

Next, `make release` copies that version's section of `CHANGELOG.md` into
`build/notes.md` and hands off to GoReleaser. GoReleaser
builds the archives and Linux packages. It signs the macOS binary and has
Apple notarize it. Last, it publishes the GitHub release and updates the cask
in the Homebrew tap. The settings are in `.goreleaser.yaml`.

Three more channels stay off unless you ask for them. `PUBLISH_SCOOP=1`
pushes to `wkoszek/scoop-bucket`. `PUBLISH_WINGET=1` pushes to the fork
`wkoszek/winget-pkgs` and opens a pull request on `microsoft/winget-pkgs`.
For both, the token must reach those repos too. `AUR_KEY` holds the SSH
private key for your AUR account, and setting it uploads `asciidraw-bin`:

```sh
PUBLISH_SCOOP=1 PUBLISH_WINGET=1 AUR_KEY="$(cat ~/.ssh/aur)" make release
```

`make snapshot` builds every package unsigned into `dist/` and publishes
nothing.

The tests drive the program through a fake terminal screen. They click,
drag and type, then check the shapes and the saved files.

## License

BSD 2-Clause. See [LICENSE](LICENSE).
