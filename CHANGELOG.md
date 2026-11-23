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
