package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Commands that work without opening the drawing on screen:
//
//	asciidraw export [flags] <file> [output]   write pictures, as ^E does
//	asciidraw convert <file> <output>          write a drawing: .adraw or .yaml
//
// Both read anything asciidraw opens: a drawing, or a Markdown or text file.

// runCommand runs a command if args names one. It reports whether it did,
// and the exit status.
func runCommand(args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	switch args[0] {
	case "export":
		return true, cmdExport(args[1:], stdout, stderr)
	case "convert":
		return true, cmdConvert(args[1:], stdout, stderr)
	}
	return false, 0
}

// formatOf gives the export format for an output name, from its extension.
func formatOf(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		return "md", true
	case ".png":
		return "png", true
	case ".jpg", ".jpeg":
		return "jpg", true
	case ".svg":
		return "svg", true
	case ".html", ".htm":
		return "html", true
	}
	return "", false
}

func cmdExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	font := fs.Int("font", 0, "font size in pixels for .png, .jpg, .svg and .html (default: the drawing's, or 12)")
	theme := fs.String("theme", "", "light or dark (default: the drawing's, or light)")
	pages := fs.String("pages", "", "together, or split for one file per page (default: the drawing's)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: asciidraw export [flags] <file> [output]")
		fmt.Fprintln(stderr, "\nWith no output, it writes each format saved in the drawing, in its folder.")
		fmt.Fprintln(stderr, "An output writes just one format; its extension picks .md, .png, .jpg,")
		fmt.Fprintln(stderr, ".svg or .html.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 && fs.NArg() != 2 {
		fs.Usage()
		return 2
	}
	in, out := fs.Arg(0), fs.Arg(1) // out is "" when not given
	format := ""
	if out != "" {
		var ok bool
		if format, ok = formatOf(out); !ok {
			fmt.Fprintf(stderr, "asciidraw: %s: the output must end in .md, .png, .jpg, .svg or .html\n", out)
			return 2
		}
	}
	drawing, meta, _, _, err := loadDrawing(in)
	if err != nil {
		fmt.Fprintln(stderr, "asciidraw:", err)
		return 1
	}
	for i := range drawing { // hidden layers aren't exported
		if i < len(meta.Layers) {
			drawing[i] = visible(drawing[i], meta.Layers[i].hidden)
		}
	}
	e := meta.Export
	if format != "" {
		e.Formats = []string{format}
	}
	if *font != 0 {
		if *font < 6 || *font > 72 {
			fmt.Fprintln(stderr, "asciidraw: --font must be 6 to 72")
			return 2
		}
		e.Font = *font
	}
	if *theme != "" {
		if !slices.Contains(themeNames, *theme) {
			fmt.Fprintln(stderr, "asciidraw: --theme must be light or dark")
			return 2
		}
		e.Theme = *theme
	}
	switch *pages {
	case "":
	case "together":
		e.Split = false
	case "split":
		e.Split = true
	default:
		fmt.Fprintln(stderr, "asciidraw: --pages must be together or split")
		return 2
	}
	// The output names the file; for split pages, it is the base of each name.
	// With no output, the files go next to the drawing, named after it.
	base := exportBase(in)
	if out != "" {
		base = strings.TrimSuffix(out, filepath.Ext(out))
	}
	plan := planExports(drawing, e, base)
	for k := range plan {
		if out != "" && (!e.Split || len(trimPages(drawing)) == 1) {
			plan[k].path = out // keep the extension as typed, such as .jpeg
		}
		if sameFile(plan[k].path, in) {
			fmt.Fprintf(stderr, "asciidraw: %s would replace the drawing itself; name another output\n", plan[k].path)
			return 2
		}
	}
	for _, f := range plan {
		data, err := exportBytes(f, e)
		if err == nil {
			err = os.WriteFile(f.path, data, 0o644)
		}
		if err != nil {
			fmt.Fprintln(stderr, "asciidraw:", err)
			return 1
		}
		fmt.Fprintln(stdout, f.path)
	}
	return 0
}

func cmdConvert(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stderr, "usage: asciidraw convert <file> <output>")
		fmt.Fprintln(stderr, "\nWrites a drawing (.adraw or .yaml) from a drawing, or from a Markdown or")
		fmt.Fprintln(stderr, "text file. To make a picture, use asciidraw export.")
		return 2
	}
	in, out := args[0], args[1]
	if !isYAML(out) {
		hint := ""
		if _, ok := formatOf(out); ok {
			hint = "; for a picture, use asciidraw export"
		}
		fmt.Fprintf(stderr, "asciidraw: %s: the output must end in .adraw or .yaml%s\n", out, hint)
		return 2
	}
	drawing, meta, _, _, err := loadDrawing(in)
	if err == nil {
		err = saveYAML(out, drawing, meta)
	}
	if err != nil {
		fmt.Fprintln(stderr, "asciidraw:", err)
		return 1
	}
	fmt.Fprintln(stdout, out)
	return 0
}

// sameFile says whether two paths name one file, so export never writes
// over the Markdown file it read.
func sameFile(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	x, err1 := os.Stat(a)
	y, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(x, y)
}
