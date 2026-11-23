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
//	asciidraw export [flags] <file> <output>   write a picture: .md, .png or .jpg
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
		fmt.Fprintln(stderr, "usage: asciidraw export [flags] <file> <output>")
		fmt.Fprintln(stderr, "\nThe output's extension picks the format: .md, .png, .jpg, .svg or .html.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return 2
	}
	in, out := fs.Arg(0), fs.Arg(1)
	format, ok := formatOf(out)
	if !ok {
		fmt.Fprintf(stderr, "asciidraw: %s: the output must end in .md, .png, .jpg, .svg or .html\n", out)
		return 2
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
	e.Formats = []string{format}
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
	base := strings.TrimSuffix(out, filepath.Ext(out))
	for _, f := range planExports(drawing, e, base) {
		if !e.Split || len(trimPages(drawing)) == 1 {
			f.path = out // keep the extension as typed, such as .jpeg
		}
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
