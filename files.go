package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// A drawing is saved as YAML, one entry per object, so it can be reopened
// and edited. ^E exports the picture itself as Markdown.

type fileDoc struct {
	Version int          `yaml:"version"`
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
var hasValue = map[string]bool{"checkbox": true, "radio": true, "tabs": true, "progress": true, "scrollbar": true}

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
	doc := fileDoc{Version: 1, Objects: []fileObject{}}
	for _, o := range objs {
		f := fileObject{Kind: o.Kind, Group: o.Group, ID: o.ID, From: toFileAnchor(o.From), To: toFileAnchor(o.To)}
		if o.Kind == "text" {
			f.X0, f.Y0, f.Text = ptr(o.X0), ptr(o.Y0), o.Text
		} else {
			f.Style = styles[min(max(o.Style, 0), len(styles)-1)].name
			f.X0, f.Y0, f.X1, f.Y1 = ptr(o.X0), ptr(o.Y0), ptr(o.X1), ptr(o.Y1)
			if isWidget(o.Kind) || hasLabel(o.Kind) {
				f.Text = o.Text
			}
			if hasValue[o.Kind] {
				f.Value = ptr(o.Value)
			}
		}
		doc.Objects = append(doc.Objects, f)
	}
	var buf bytes.Buffer
	buf.WriteString("# asciidraw drawing. Open with: asciidraw <file>. Press ^E in it to export Markdown.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	enc.Close()
	return buf.Bytes(), nil
}

func decodeYAML(data []byte) ([]object, error) {
	var doc fileDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var objs []object
	for i, f := range doc.Objects {
		o := object{Kind: f.Kind, Group: f.Group, ID: f.ID}
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
			if f.Kind != "box" && f.Kind != "oval" && f.Kind != "line" && f.Kind != "arrow" && !isWidget(f.Kind) {
				return nil, fmt.Errorf("object %d: unknown kind %q", i+1, f.Kind)
			}
			st, ok := styleIndex(f.Style)
			if !ok {
				return nil, fmt.Errorf("object %d: unknown style %q", i+1, f.Style)
			}
			o.Style = st
			o.X0, o.Y0, o.X1, o.Y1 = val(f.X0), val(f.Y0), val(f.X1), val(f.Y1)
			if isWidget(f.Kind) || hasLabel(f.Kind) {
				o.Text, o.Value = f.Text, val(f.Value)
			}
			o.normalize()
		}
		objs = append(objs, o)
	}
	return objs, nil
}

func isYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// withExt swaps or adds an extension.
func withExt(path, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

func saveYAML(path string, objs []object) error {
	data, err := encodeYAML(objs)
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

// loadDrawing opens a .yaml drawing, or imports a .txt or .md picture as one
// text object. saveTo is where ^S should write without asking: the file
// itself for YAML, nothing for an import.
func loadDrawing(path string) (objs []object, saveTo, note string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", "", err
	}
	if isYAML(path) {
		objs, err = decodeYAML(data)
		if err != nil {
			return nil, "", "", fmt.Errorf("%s: %w", path, err)
		}
		return objs, path, "", nil
	}
	text := strings.ReplaceAll(string(data), "\r", "")
	if strings.EqualFold(filepath.Ext(path), ".md") {
		text = firstFence(text)
	}
	text = strings.TrimRight(strings.ReplaceAll(text, "\t", "    "), "\n")
	if text != "" {
		objs = []object{{Kind: "text", Text: strings.Split(text, "\n")}}
	}
	return objs, "", fmt.Sprintf("Imported %s as one text object. ^S saves it as YAML.", path), nil
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
