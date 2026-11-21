# asciidraw todo

Roughly in order of payoff for drawing terminal UI mockups.

## 1. Housekeeping

- [x] Make it a git repo: Gitea (tig.koszek.com), mirrored to GitHub.
- [x] Add `.github/repo.diaml`.
- [x] Write a README: what it is, install, keys, file formats.
- [ ] Second outside review of the README with `agy` (its quota ran out on
      2026-11-20; a cold-read agent stood in).
- [ ] Pick a license.

## 2. UI widget palette

Stamp ready-made pieces instead of building them from boxes and text. Each
is one object you can move, resize and relabel. Put them in the ^T menu
under a third heading, "Widgets".

- [x] Button `[ OK ]`
- [x] Checkbox `[x]` and radio button `(•)`
- [x] Input field `[__________]`
- [x] Dropdown `[ Choose  v ]`
- [x] Tabs
- [x] Scrollbar
- [x] Progress bar `[████░░░░]`
- [x] Window with a title bar

## 3. Select several shapes

- [x] Drag a rectangle around shapes to select them.
- [x] Shift-click to add a shape to the selection.
- [x] Move, copy, cut and delete the whole selection.
- [x] Group, so a dialog and its buttons move as one piece.

## 4. Arrows that stay attached

- [x] An arrow that starts or ends on a box's edge stays attached when the
      box moves.

## 5. Labels that belong to boxes

- [x] Text typed inside a box moves with the box.
- [x] Center a label in its box.
- [x] Grow a box to fit its label.

## 6. Clean crossings

- [ ] Where lines meet, draw `┼ ├ ┬` (or `+` in ASCII) instead of letting
      one shape overwrite the other.

## 7. Smaller things

- [ ] Bring to front and send to back, maybe `[` and `]`.
- [ ] Reorder shapes by dragging rows in the panel.
- [ ] Duplicate in place, with an offset, in one key.
- [ ] Real text editing: arrow keys, and typing in the middle of a line.
- [ ] A canvas bigger than the window, with scrolling.
- [ ] `?` opens a help screen with every key.
- [ ] Fill patterns `░▒▓` for shaded areas.
- [ ] Autosave, so a crash or a closed window loses nothing.

## 8. Drawings inside docs

- [ ] ^E updates a drawing inside an existing Markdown file, between
      marker comments, instead of writing a separate `.md`.
