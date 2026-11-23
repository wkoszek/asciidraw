package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Export settings, and the S dialog that changes them. ^E writes every
// format that is ticked. They are saved with the drawing.

type exportSettings struct {
	Formats []string // "md", "png", "jpg", in that order
	Font    int      // pixels; sets the size of a PNG or JPEG
	Theme   string   // "light" or "dark", for images
	Split   bool     // a drawing with several pages: one file per page
}

var exportFormats = []struct{ key, ext, name string }{
	{"md", ".md", "Markdown"}, {"png", ".png", "PNG"}, {"jpg", ".jpg", "JPEG"},
}

var fontSizes = []int{10, 12, 14, 16, 20, 24}

const refFont = 12 // the reference size, and the default

var themeNames = []string{"light", "dark"}

func defaultExport() exportSettings {
	return exportSettings{Formats: []string{"md"}, Font: refFont, Theme: "light"}
}

func (e exportSettings) isDefault() bool {
	d := defaultExport()
	return slices.Equal(e.Formats, d.Formats) && e.Font == d.Font && e.Theme == d.Theme && e.Split == d.Split
}

func (e exportSettings) has(key string) bool { return slices.Contains(e.Formats, key) }

// fileExport is how the settings look in the drawing file.
type fileExport struct {
	Formats []string `yaml:"formats,omitempty"`
	Font    int      `yaml:"font,omitempty"`
	Theme   string   `yaml:"theme,omitempty"`
	Pages   string   `yaml:"pages,omitempty"` // "split" for one file per page
}

func toFileExport(e exportSettings) *fileExport {
	if e.isDefault() {
		return nil
	}
	f := &fileExport{Formats: e.Formats, Font: e.Font, Theme: e.Theme}
	if e.Split {
		f.Pages = "split"
	}
	return f
}

func fromFileExport(f *fileExport) (exportSettings, error) {
	e := defaultExport()
	if f == nil {
		return e, nil
	}
	if len(f.Formats) > 0 {
		e.Formats = nil
		for _, ff := range exportFormats {
			if slices.Contains(f.Formats, ff.key) {
				e.Formats = append(e.Formats, ff.key)
			}
		}
		for _, k := range f.Formats {
			if !slices.ContainsFunc(exportFormats, func(ff struct{ key, ext, name string }) bool { return ff.key == k }) {
				return e, fmt.Errorf("export: unknown format %q; use md, png or jpg", k)
			}
		}
	}
	if f.Font != 0 {
		if f.Font < 6 || f.Font > 72 {
			return e, fmt.Errorf("export: font %d is out of range (6 to 72)", f.Font)
		}
		e.Font = f.Font
	}
	if f.Theme != "" {
		if !slices.Contains(themeNames, f.Theme) {
			return e, fmt.Errorf("export: unknown theme %q; use light or dark", f.Theme)
		}
		e.Theme = f.Theme
	}
	switch f.Pages {
	case "", "together":
	case "split":
		e.Split = true
	default:
		return e, fmt.Errorf("export: pages %q; use together or split", f.Pages)
	}
	return e, nil
}

// ---- what ^E writes ----

// exportBase is a name with any export or drawing extension taken off.
func exportBase(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".png", ".jpg", ".jpeg", ".yaml", ".yml", ".adraw", ".txt":
		return strings.TrimSuffix(name, filepath.Ext(name))
	}
	return name
}

// exportFile is one file ^E writes: its path, and the pages that go in it.
type exportFile struct {
	path, format string
	pages        [][]object
	first        int // the first page's index, for its name
}

// exportPlan lists the files ^E writes for a base name such as "mock".
// One page, or pages kept together, gives mock.md; split pages give
// mock_page0.md, mock_page1.md and so on.
func (a *app) exportPlan(base string) []exportFile {
	pages := trimPages(a.allPages())
	var out []exportFile
	for _, f := range exportFormats {
		if !a.exp.has(f.key) {
			continue
		}
		if !a.exp.Split || len(pages) == 1 {
			out = append(out, exportFile{base + f.ext, f.key, pages, 0})
			continue
		}
		for i, objs := range pages {
			out = append(out, exportFile{base + "_page" + pageChar(i) + f.ext, f.key, [][]object{objs}, i})
		}
	}
	return out
}

// export writes the pictures: next to the YAML file if there is one, or
// under a name it asks for. It asks before replacing files, except the
// ones this session already exported.
func (a *app) export() {
	switch {
	case a.exported != "":
		a.unlessExported(a.exported)
	case a.file != "":
		a.unlessExported(exportBase(a.file))
	default:
		a.ask("Export as: ", "", func(name string) {
			name = expandHome(strings.TrimSpace(name))
			if name == "" {
				a.say("Cancelled.")
				return
			}
			a.unlessExported(exportBase(name))
		})
	}
}

// unlessExported writes the exports right away if none of the files is
// there yet, or this session wrote them, and otherwise asks first.
func (a *app) unlessExported(base string) {
	var there []string
	for _, f := range a.exportPlan(base) {
		if _, err := os.Stat(f.path); err == nil && !a.wrote[f.path] {
			there = append(there, f.path)
		}
	}
	if len(there) == 0 {
		a.writeExports(base)
		return
	}
	q := there[0] + " exists"
	if len(there) > 1 {
		q = fmt.Sprintf("%s and %d more exist", there[0], len(there)-1)
	}
	a.ask(q+". Overwrite? (y/n): ", "", func(ans string) {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ans)), "y") {
			a.writeExports(base)
		} else {
			a.say("Cancelled.")
		}
	})
}

func (a *app) writeExports(base string) {
	a.stopTyping()
	plan := a.exportPlan(base)
	var names []string
	for _, f := range plan {
		data, err := a.exportData(f)
		if err == nil {
			err = os.WriteFile(f.path, data, 0o644)
		}
		if err != nil {
			a.sayErr("Export failed: " + err.Error())
			return
		}
		names = append(names, filepath.Base(f.path))
		if a.wrote == nil {
			a.wrote = map[string]bool{}
		}
		a.wrote[f.path] = true
	}
	a.exported = base
	msg := "Exported " + strings.Join(names, ", ")
	if len(names) > 3 {
		msg = fmt.Sprintf("Exported %d files, %s to %s", len(names), names[0], names[len(names)-1])
	}
	if n := len(trimPages(a.allPages())); n > 1 && !a.exp.Split {
		msg += fmt.Sprintf(" (%d pages)", n)
	}
	a.say(msg)
}

func (a *app) exportData(f exportFile) ([]byte, error) {
	if f.format == "md" {
		var pics []string
		for _, objs := range f.pages {
			pics = append(pics, compose(objs).String())
		}
		return []byte(markdownPages(pics)), nil
	}
	img := renderPages(f.pages, a.exp.Font, themes[a.exp.Theme])
	if f.format == "png" {
		return encodePNG(img)
	}
	return encodeJPEG(img)
}

// ---- the S dialog ----

const (
	setFormats = iota
	setFont
	setTheme
	setPages
	setRows
)

var setLabels = [setRows]string{"Write", "Font", "Theme", "Pages"}

type settings struct {
	row    int
	col    [setRows]int // the cursor in each row
	before exportSettings
}

func (a *app) openSettings() {
	a.stopTyping()
	e := a.exp
	st := &settings{before: e}
	st.col[setFont] = max(slices.Index(fontSizes, e.Font), 0)
	st.col[setTheme] = max(slices.Index(themeNames, e.Theme), 0)
	if e.Split {
		st.col[setPages] = 1
	}
	a.settings = st
}

// choices are the words in a row, as the dialog shows them.
func (a *app) choices(row int) []string {
	switch row {
	case setFormats:
		var out []string
		for _, f := range exportFormats {
			box := "[ ]"
			if a.exp.has(f.key) {
				box = "[x]"
			}
			out = append(out, box+" "+f.name)
		}
		return out
	case setFont:
		var out []string
		for _, n := range fontSizes {
			out = append(out, fmt.Sprint(n))
		}
		return out
	case setTheme:
		return themeNames
	}
	return []string{"one file", "a file per page"}
}

// chosen says whether a word in a row is the one in effect.
func (a *app) chosen(row, col int) bool {
	switch row {
	case setFormats:
		return a.exp.has(exportFormats[col].key)
	case setFont:
		return a.exp.Font == fontSizes[col]
	case setTheme:
		return a.exp.Theme == themeNames[col]
	}
	return a.exp.Split == (col == 1)
}

// setPick makes the word under the cursor take effect. In the Write row it
// ticks or unticks a format; at least one stays ticked.
func (a *app) setPick(row, col int) {
	e := &a.exp
	switch row {
	case setFormats:
		key := exportFormats[col].key
		if e.has(key) {
			if len(e.Formats) == 1 {
				a.sayErr("Keep at least one format ticked.")
				return
			}
			e.Formats = slices.DeleteFunc(slices.Clone(e.Formats), func(k string) bool { return k == key })
		} else {
			var f []string
			for _, ff := range exportFormats {
				if e.has(ff.key) || ff.key == key {
					f = append(f, ff.key)
				}
			}
			e.Formats = f
		}
	case setFont:
		e.Font = fontSizes[col]
	case setTheme:
		e.Theme = themeNames[col]
	case setPages:
		e.Split = col == 1
	}
}

func (a *app) closeSettings(keep bool) {
	st := a.settings
	a.settings = nil
	if !keep {
		a.exp = st.before
		a.say("Export settings left as they were.")
		return
	}
	if !a.exportSame(st.before) {
		a.dirty = true // the settings are saved with the drawing
	}
	a.say("^E writes " + a.planNames() + ".")
}

func (a *app) exportSame(e exportSettings) bool {
	return slices.Equal(a.exp.Formats, e.Formats) && a.exp.Font == e.Font && a.exp.Theme == e.Theme && a.exp.Split == e.Split
}

// planNames is what ^E would write, for the dialog and the message.
func (a *app) planNames() string {
	base := "drawing"
	switch {
	case a.exported != "":
		base = a.exported
	case a.file != "":
		base = exportBase(a.file)
	}
	var names []string
	for _, f := range a.exportPlan(base) {
		names = append(names, filepath.Base(f.path))
	}
	if len(names) > 3 {
		return fmt.Sprintf("%d files, %s to %s", len(names), names[0], names[len(names)-1])
	}
	return strings.Join(names, ", ")
}

func (a *app) settingsKey(ev *tcell.EventKey) {
	st := a.settings
	move := func(d int) {
		n := len(a.choices(st.row))
		st.col[st.row] = (st.col[st.row] + d + n) % n
		if st.row != setFormats {
			a.setPick(st.row, st.col[st.row]) // radio rows follow the cursor
		}
	}
	switch ev.Key() {
	case tcell.KeyUp:
		st.row = (st.row + setRows - 1) % setRows
	case tcell.KeyDown, tcell.KeyTab:
		st.row = (st.row + 1) % setRows
	case tcell.KeyLeft:
		move(-1)
	case tcell.KeyRight:
		move(1)
	case tcell.KeyEnter:
		a.closeSettings(true)
	case tcell.KeyEscape:
		a.closeSettings(false)
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'k':
			st.row = (st.row + setRows - 1) % setRows
		case 'j':
			st.row = (st.row + 1) % setRows
		case 'h':
			move(-1)
		case 'l':
			move(1)
		case ' ', 'x':
			a.setPick(st.row, st.col[st.row])
		case 'S', 'q':
			a.closeSettings(true)
		}
	}
}

const setLabelW = 8 // "  Write " and so on, before the choices

func (a *app) settingsBox() (x0, y0, w, h int) {
	w, h = 60, 12
	sw, sh := a.s.Size()
	return max((sw-w)/2, 0), max((sh-2-h)/2, 0), w, h
}

// settingsAt finds the word at screen cell (x, y).
func (a *app) settingsAt(x, y int) (row, col int, ok bool) {
	x0, y0, _, _ := a.settingsBox()
	row = y - y0 - 2
	if row < 0 || row >= setRows {
		return 0, 0, false
	}
	cx := x0 + 2 + setLabelW
	for c, word := range a.choices(row) {
		if x >= cx && x < cx+runeLen(word)+2 {
			return row, c, true
		}
		cx += runeLen(word) + 3
	}
	return 0, 0, false
}

func (a *app) settingsMouse(x, y int, b tcell.ButtonMask) {
	if b&tcell.Button1 == 0 {
		return
	}
	if row, col, ok := a.settingsAt(x, y); ok {
		st := a.settings
		st.row, st.col[row] = row, col
		a.setPick(row, col)
		return
	}
	x0, y0, w, h := a.settingsBox()
	if x < x0 || x >= x0+w || y < y0 || y >= y0+h {
		a.closeSettings(true)
	}
}

func (a *app) renderSettings() {
	st := a.settings
	x0, y0, w, h := a.settingsBox()
	a.frame(x0, y0, w, h, " S Export settings ")
	dim := tcell.StyleDefault.Dim(true)
	for row := 0; row < setRows; row++ {
		y := y0 + 2 + row
		ls := tcell.StyleDefault
		if row == st.row {
			ls = ls.Bold(true).Foreground(tcell.ColorAqua)
		}
		a.text(x0+2, y, setLabels[row], ls)
		cx := x0 + 2 + setLabelW
		for c, word := range a.choices(row) {
			ws := tcell.StyleDefault
			if a.chosen(row, c) {
				ws = ws.Bold(true).Foreground(tcell.ColorYellow)
			}
			if row == st.row && c == st.col[row] {
				ws = ws.Reverse(true)
			}
			a.text(cx, y, " "+word+" ", ws)
			cx += runeLen(word) + 3
		}
		if row == setFont {
			a.text(cx, y, "px", dim)
		}
	}
	pw, ph := pixelSize([][]object{a.objs}, a.exp.Font)
	a.text(x0+2, y0+7, fmt.Sprintf("This page: %d×%d px. 12px is the reference size.", pw, ph), dim)
	a.text(x0+2, y0+8, "^E writes "+a.planNames(), tcell.StyleDefault)
	a.text(x0+2, y0+h-2, "↑↓ row  ←→ choose  Space tick  Enter keep  Esc undo", dim)
}
