package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// A drawing is saved as YAML: a list of pages, each a list of objects, so it
// can be reopened and edited. ^E exports the pictures as Markdown.

// fileVersion is the drawing format this asciidraw writes, and the newest
// it reads. Version 2 added pages.
const fileVersion = 2

type fileDoc struct {
	Version int          `yaml:"version"`
	Palette []string     `yaml:"palette,omitempty"` // the 16 slots in the color panel
	Export  *fileExport  `yaml:"export,omitempty"`  // the S settings; none means the defaults
	Pages   []filePage   `yaml:"pages,omitempty"`
	Objects []fileObject `yaml:"objects,omitempty"` // version 1 files: one page, no pages list
}

// docMeta is what a drawing keeps besides its pages' shapes.
type docMeta struct {
	Pal    []string
	Export exportSettings
	Layers []layerState // one per page; missing means nothing hidden or locked
}

func defaultMeta() docMeta { return docMeta{Pal: slices.Clone(defaultPal), Export: defaultExport()} }

type filePage struct {
	Hidden  string       `yaml:"hidden,omitempty"` // hidden layers, such as "2a"
	Locked  string       `yaml:"locked,omitempty"` // locked layers
	Active  string       `yaml:"active,omitempty"` // the layer new shapes go on, if not 0
	Objects []fileObject `yaml:"objects"`
}

// fileObject is how an object looks on disk. Text sits at x0, y0; shapes
// run from x0, y0 to x1, y1. Pointers keep zeros from being dropped while
// leaving out fields a kind doesn't use.
type fileObject struct {
	Kind  string      `yaml:"kind"`
	Style string      `yaml:"style,omitempty"`
	X0    *int        `yaml:"x0,omitempty"`
	Y0    *int        `yaml:"y0,omitempty"`
	X1    *int        `yaml:"x1,omitempty"`
	Y1    *int        `yaml:"y1,omitempty"`
	Text  []string    `yaml:"text,omitempty"`
	Value *int        `yaml:"value,omitempty"`
	Group int         `yaml:"group,omitempty"`
	ID    int         `yaml:"id,omitempty"`
	From  *fileAnchor `yaml:"from,omitempty"`
	To    *fileAnchor `yaml:"to,omitempty"`
	FG    string      `yaml:"fg,omitempty"`
	BG    string      `yaml:"bg,omitempty"`
	Place string      `yaml:"place,omitempty"` // a label's position, like "top left"; none means centered
	Layer string      `yaml:"layer,omitempty"` // 0-9, a-z or A-Z; none means 0
}

// fileAnchor is an attached line end: the shape's id, the side (left,
// right, top or bottom), how far along it, and 1 if it sits just outside.
type fileAnchor struct {
	ID   int    `yaml:"id"`
	Side string `yaml:"side"`
	At   int    `yaml:"at"`
	Gap  int    `yaml:"gap,omitempty"`
}

var sideNames = map[byte]string{'l': "left", 'r': "right", 't': "top", 'b': "bottom"}

func toFileAnchor(an anchor) *fileAnchor {
	if an.ID == 0 {
		return nil
	}
	return &fileAnchor{an.ID, sideNames[an.Side], an.At, an.Gap}
}

func fromFileAnchor(f *fileAnchor) (anchor, error) {
	if f == nil {
		return anchor{}, nil
	}
	for b, n := range sideNames {
		if n == f.Side {
			return anchor{f.ID, b, f.At, f.Gap}, nil
		}
	}
	return anchor{}, fmt.Errorf("unknown side %q", f.Side)
}

// hasValue lists the widgets whose state is worth writing down.
var hasValue = map[string]bool{"checkbox": true, "radio": true, "tabs": true, "progress": true, "scrollbar": true, "fill": true}

func ptr(n int) *int { return &n }

func val(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func styleIndex(name string) (int, bool) {
	if name == "" {
		return 0, true
	}
	for i, st := range styles {
		if st.name == name {
			return i, true
		}
	}
	return 0, false
}

func encodeYAML(objs []object) ([]byte, error) {
	return encodeDoc([][]object{objs}, docMeta{Export: defaultExport()})
}

// encodeDoc writes the pages, the palette and the export settings. Empty
// pages at the end are left out, and so are a palette and settings still
// at their defaults.
func encodeDoc(pages [][]object, m docMeta) ([]byte, error) {
	doc := fileDoc{Version: fileVersion, Export: toFileExport(m.Export)}
	if m.Pal != nil && !slices.Equal(m.Pal, defaultPal) {
		doc.Palette = m.Pal
	}
	for i, objs := range trimPages(pages) {
		fp := filePage{Objects: encodeObjects(objs)}
		if i < len(m.Layers) {
			l := m.Layers[i]
			fp.Hidden, fp.Locked = layerNames(l.hidden), layerNames(l.locked)
			if l.active != 0 {
				fp.Active = pageChar(l.active)
			}
		}
		doc.Pages = append(doc.Pages, fp)
	}
	var buf bytes.Buffer
	buf.WriteString("# asciidraw drawing. Open with: asciidraw <file>. Press ^E in it to export.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	enc.Close()
	return buf.Bytes(), nil
}

func encodeObjects(objs []object) []fileObject {
	out := []fileObject{}
	for _, o := range objs {
		f := fileObject{Kind: o.Kind, Group: o.Group, ID: o.ID, From: toFileAnchor(o.From), To: toFileAnchor(o.To), FG: o.FG, BG: o.BG}
		if o.Layer > 0 && o.Layer < maxLayers {
			f.Layer = pageChar(o.Layer)
		}
		if o.Kind == "text" {
			f.X0, f.Y0, f.Text = ptr(o.X0), ptr(o.Y0), o.Text
		} else {
			f.Style = styles[min(max(o.Style, 0), len(styles)-1)].name
			f.X0, f.Y0, f.X1, f.Y1 = ptr(o.X0), ptr(o.Y0), ptr(o.X1), ptr(o.Y1)
			if isWidget(o.Kind) || hasLabel(o.Kind) {
				f.Text = o.Text
			}
			if hasLabel(o.Kind) && (o.HAlign != 0 || o.VAlign != 0) {
				f.Place = placeName(o.HAlign, o.VAlign)
			}
			if hasValue[o.Kind] {
				f.Value = ptr(o.Value)
			}
		}
		out = append(out, f)
	}
	return out
}

// decodeYAML reads the first page.
func decodeYAML(data []byte) ([]object, error) {
	pages, _, err := decodeDoc(data)
	if err != nil {
		return nil, err
	}
	return pages[0], nil
}

// decodeDoc reads the pages, the palette and the export settings. A
// version 1 file, with a bare objects list, is one page. A file without a
// palette gets the default one; a short one is filled up from it.
func decodeDoc(data []byte) ([][]object, docMeta, error) {
	pages, pal, ex, err := decodeParts(data)
	if err != nil {
		return nil, docMeta{}, err
	}
	m := docMeta{Pal: pal, Export: ex}
	var doc fileDoc
	yaml.Unmarshal(data, &doc) // read already, without error
	for n, p := range doc.Pages {
		var l layerState
		var e1, e2 error
		l.hidden, e1 = parseLayerNames(p.Hidden)
		l.locked, e2 = parseLayerNames(p.Locked)
		if err := errors.Join(e1, e2); err != nil {
			return nil, docMeta{}, fmt.Errorf("page %s: %w", pageChar(n), err)
		}
		if p.Active != "" {
			a, ok := layerOf([]rune(p.Active)[0])
			if !ok || len([]rune(p.Active)) != 1 {
				return nil, docMeta{}, fmt.Errorf("page %s: active layer %q; layers are 0-9, a-z and A-Z", pageChar(n), p.Active)
			}
			l.active = a
		}
		m.Layers = append(m.Layers, l)
	}
	return pages, m, nil
}

func decodeParts(data []byte) ([][]object, []string, exportSettings, error) {
	pages, pal, doc, err := decodePages(data)
	if err != nil {
		return nil, nil, exportSettings{}, err
	}
	ex, err := fromFileExport(doc.Export)
	if err != nil {
		return nil, nil, exportSettings{}, err
	}
	return pages, pal, ex, nil
}

func decodePages(data []byte) ([][]object, []string, fileDoc, error) {
	var doc fileDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, doc, err
	}
	if len(doc.Palette) > palSlots {
		return nil, nil, doc, fmt.Errorf("the palette has %d colors; it holds %d", len(doc.Palette), palSlots)
	}
	pal := slices.Clone(defaultPal)
	for i, c := range doc.Palette {
		name, ok := cleanColor(c)
		if !ok || name == "" {
			return nil, nil, doc, fmt.Errorf("palette color %d: unknown color %q", i+1, c)
		}
		pal[i] = name
	}
	if doc.Version > fileVersion {
		return nil, nil, doc, fmt.Errorf("the file is version %d, newer than this asciidraw reads (%d); update asciidraw", doc.Version, fileVersion)
	}
	if len(doc.Pages) > 0 && len(doc.Objects) > 0 {
		return nil, nil, doc, fmt.Errorf("the file has both pages and a bare objects list; put the objects in a page")
	}
	if len(doc.Pages) == 0 {
		doc.Pages = []filePage{{Objects: doc.Objects}}
	}
	if len(doc.Pages) > maxPages {
		return nil, nil, doc, fmt.Errorf("the file has %d pages; a drawing holds at most %d", len(doc.Pages), maxPages)
	}
	var pages [][]object
	for n, p := range doc.Pages {
		objs, err := decodeObjects(p.Objects)
		if err != nil {
			if len(doc.Pages) > 1 {
				err = fmt.Errorf("page %s: %w", pageChar(n), err)
			}
			return nil, nil, doc, err
		}
		// Shapes keep to layer order, whatever order the file lists them in.
		slices.SortStableFunc(objs, func(x, y object) int { return x.Layer - y.Layer })
		pages = append(pages, objs)
	}
	return pages, pal, doc, nil
}

func decodeObjects(fs []fileObject) ([]object, error) {
	var objs []object
	for i, f := range fs {
		o := object{Kind: f.Kind, Group: f.Group, ID: f.ID}
		for _, c := range []*string{&f.FG, &f.BG} {
			name, ok := cleanColor(*c)
			if !ok {
				return nil, fmt.Errorf("object %d: unknown color %q", i+1, *c)
			}
			*c = name
		}
		o.FG, o.BG = f.FG, f.BG
		if f.Layer != "" {
			l, ok := layerOf([]rune(f.Layer)[0])
			if !ok || len([]rune(f.Layer)) != 1 {
				return nil, fmt.Errorf("object %d: layer %q; layers are 0-9, a-z and A-Z", i+1, f.Layer)
			}
			o.Layer = l
		}
		var err error
		if o.From, err = fromFileAnchor(f.From); err == nil {
			o.To, err = fromFileAnchor(f.To)
		}
		if err != nil {
			return nil, fmt.Errorf("object %d: %w", i+1, err)
		}
		switch f.Kind {
		case "text":
			o.X0, o.Y0, o.Text = val(f.X0), val(f.Y0), f.Text
			if len(o.Text) == 0 {
				continue // nothing to show
			}
		default:
			if f.Kind != "box" && f.Kind != "oval" && f.Kind != "fill" && f.Kind != "line" && f.Kind != "arrow" && !isWidget(f.Kind) {
				return nil, fmt.Errorf("object %d: unknown kind %q", i+1, f.Kind)
			}
			st, ok := styleIndex(f.Style)
			if !ok {
				return nil, fmt.Errorf("object %d: unknown style %q", i+1, f.Style)
			}
			o.Style = st
			o.X0, o.Y0, o.X1, o.Y1 = val(f.X0), val(f.Y0), val(f.X1), val(f.Y1)
			if isWidget(f.Kind) || hasLabel(f.Kind) {
				o.Text = f.Text
			}
			if f.Place != "" {
				h, v, ok := parsePlace(f.Place)
				if !ok {
					return nil, fmt.Errorf("object %d: unknown place %q", i+1, f.Place)
				}
				if hasLabel(f.Kind) {
					o.HAlign, o.VAlign = h, v
				}
			}
			o.Value = val(f.Value)
			o.normalize()
		}
		objs = append(objs, o)
	}
	return objs, nil
}

func isYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml" || ext == ".adraw"
}

// drawingPath finds the file a name on the command line means. A name
// with no file of its own opens mock.adraw, or mock.yaml, if there is
// one, so "asciidraw mock" never starts over a drawing that exists.
func drawingPath(arg string) string {
	if _, err := os.Stat(arg); err == nil || isYAML(arg) {
		return arg
	}
	for _, ext := range []string{".adraw", ".yaml", ".yml"} {
		if p := withExt(arg, ext); p != arg {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return arg
}

// withExt swaps or adds an extension.
func withExt(path, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

func saveYAML(path string, pages [][]object, m docMeta) error {
	data, err := encodeDoc(pages, m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// markdown wraps the picture in a code fence, longer than any run of
// backticks inside it.
func markdown(picture string) string {
	fence := "```"
	for strings.Contains(picture, fence) {
		fence += "`"
	}
	return fence + "text\n" + picture + fence + "\n"
}

// markdownPages exports every page: one code block for a single page, and
// a "Page N" line above each block when there are more, N being the
// page's one-character name.
func markdownPages(pictures []string) string {
	if len(pictures) == 1 {
		return markdown(pictures[0])
	}
	var b strings.Builder
	for i, p := range pictures {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Page %s\n\n%s", pageChar(i), markdown(p))
	}
	return b.String()
}

// loadDrawing opens a .yaml drawing, or imports a .txt or .md picture as one
// text object. saveTo is where ^S should write without asking: the file
// itself for YAML, nothing for an import.
func loadDrawing(path string) (pages [][]object, meta docMeta, saveTo, note string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, docMeta{}, "", "", err
	}
	if isYAML(path) {
		pages, meta, err = decodeDoc(data)
		if err != nil {
			return nil, docMeta{}, "", "", fmt.Errorf("%s: %w", path, err)
		}
		return pages, meta, path, "", nil
	}
	var objs []object
	text := strings.ReplaceAll(string(data), "\r", "")
	if strings.EqualFold(filepath.Ext(path), ".md") {
		text = firstFence(text)
	}
	text = strings.TrimRight(strings.ReplaceAll(text, "\t", "    "), "\n")
	if text != "" {
		objs = []object{{Kind: "text", Text: strings.Split(text, "\n")}}
	}
	return [][]object{objs}, defaultMeta(), "", fmt.Sprintf("Imported %s as one text object. ^S saves it as YAML.", path), nil
}

// firstFence returns the inside of the first fenced code block, or all of
// the text if there is none.
func firstFence(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "```") {
			continue
		}
		fence := t[:len(t)-len(strings.TrimLeft(t, "`"))]
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == fence {
				return strings.Join(lines[i+1:j], "\n")
			}
		}
		return strings.Join(lines[i+1:], "\n")
	}
	return text
}
