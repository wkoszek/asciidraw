# Changelog

Each release gets a section here, newest first. `make release` copies the
section for the tag being released into the GitHub release notes, so write it
before you tag.

## Unreleased

- `a` lines up the selected shapes by their edges or centers, or spaces
  them out evenly.
- `T` places the label in a box or an oval in one of nine spots, such as
  top left or bottom right. The drawing file stores any spot but center as
  `place`.
- Pages. `>` and `<` move between them, and so do `.` and `,`. `>` on
  the last page starts a new one, unless the last page is empty. `+`
  copies a page to a new page after it. Each page has its own undo.
- Pages are named `0`-`9`, `a`-`z`, `A`-`Z`, so a drawing holds up to 62.
  The bottom right corner shows five names at a time and marks the current
  one. Click a name to go to that page.
- ^E exports each page in its own code block, leaving out empty pages at
  the end.
- `L` opens a library of ready-made pieces, in two tabs. The builtin tab
  has a login dialog, a register dialog, a browser window, a four-pane web
  app and an open file dialog. The custom tab reads `.adraw`, `.yaml`,
  `.md` and `.txt` files from `~/.config/asciidraw/library/`, or from
  `$XDG_CONFIG_HOME/asciidraw/library/` if that is set.
- Drawings are now `.adraw` files. ^S adds `.adraw` to a name typed with
  no extension, and `asciidraw mock` opens `mock.adraw`, or starts it.
  They hold the same YAML as before, and `.yaml` and `.yml` files still
  open and save under their own names.
- `asciidraw export <file> <output>` saves a drawing as Markdown, PNG or
  JPEG without opening it. The output's extension sets the format.
  `--font`, `--theme` and `--pages` override the saved export settings
  for this run.
- `asciidraw convert <file> <output>` writes a drawing (`.adraw` or
  `.yaml`) from a drawing, a Markdown file or a text file.
- More Linux builds: 32-bit x86 (386), RISC-V and LoongArch.
- Layers. Each page has up to 62 layers, named `0`-`9`, `a`-`z`,
  `A`-`Z`. Press `l`, then a layer name, to move the selection there. `H`
  hides or shows a layer, and `K` locks or unlocks it. Hidden layers
  aren't drawn or exported; clicks go through locked ones. The panel's
  `L` column shows each shape's layer.
- SVG and HTML export, in the `S` settings and in `asciidraw export`.
  SVG draws lines as shapes, so they stay sharp at any zoom. HTML puts
  the picture in a `<pre>` block, with each shape in its own color.
- PNG and JPEG export. `S` opens the export settings. There you pick the
  formats ^E writes and the font size for images, 12px by default.
- The settings also pick a light or dark theme for images, and one file
  for all pages or a file per page, such as `mock_page0.png`. They are
  saved in the drawing file under `export`.
- PNGs are stored with a small list of colors when the picture allows,
  so a screen-sized drawing usually makes a PNG of a few kilobytes.
- The drawing file is now a list of pages (`version: 2`). Older files open
  as one page. v0.1.0 doesn't check the version, so it opens a version 2
  file as an empty drawing; don't save over the file from v0.1.0. From now
  on, asciidraw refuses a file newer than the version it knows.

## v0.1.0 (2026-11-22)

The first release.

### Drawing

- Drag to draw boxes, ovals, lines and arrows. Click to type text.
- Four styles: ascii, line, round and double. `t` switches the selected
  shapes between them.
- Widgets: button, checkbox, radio button, input field, dropdown, tabs,
  scrollbar, progress bar, and a window with a title bar.
- Boxes and ovals hold a label that moves with them.
- Lines and arrows attach to boxes, ovals and widgets, and follow them when
  they move or change size.
- Where lines and box edges meet, they join with `┼`, `┤`, `┬` and the
  like, in the drawing's style.
- Fill areas with eight shades, `░ ▒ ▓ █ . : % #`. Space steps through them.
- Text and background colors: the terminal's own colors, any hex color, and
  a palette of 16 colors saved with the drawing.

### Editing

- Select several shapes with Shift-click or a rectangle, and group them.
- Copy, cut, paste, duplicate, undo and redo.
- Bring shapes to the front or send them to the back, from the keys or by
  dragging rows in the panel.
- The canvas is bigger than the window and scrolls.
- `?` shows a summary of the keys.

### Files

- `^S` saves the drawing as YAML that is easy to edit by hand.
- `^E` exports the picture as Markdown.
- Open a `.txt` or `.md` file to bring in its picture.
- Every 30 seconds, unsaved changes go to a recovery copy. Open the same
  drawing after a crash, and the copy comes back.

### Systems and packages

- Runs on macOS, Linux, Windows, FreeBSD, OpenBSD and NetBSD.
- Copy and paste use the system clipboard on each of them. Over SSH, copy
  uses OSC 52, and paste uses what you copied in asciidraw.
- Packages: a Homebrew cask, `.deb`, `.rpm`, `.apk` and Arch packages, and
  archives for every system. The macOS binary is signed and notarized.
- `--version`, `--help`, and a man page.
- BSD 2-Clause license.
