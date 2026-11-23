package main

import "runtime/debug"

// version is set at build time by GoReleaser and make:
//
//	go build -ldflags "-X main.version=v0.1.0"
//
// A plain `go install github.com/wkoszek/asciidraw@v0.1.0` sets nothing, so
// fall back to the module version Go records in the binary.
var version = ""

func getVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

const usage = `usage: asciidraw [file]
       asciidraw export [flags] <file> <output>
       asciidraw convert <file> <output>

  asciidraw              a new, untitled drawing
  asciidraw mock.yaml    open a drawing, or start a new one under that name
  asciidraw notes.md     bring in a picture from Markdown or plain text

  asciidraw export mock.yaml mock.png
                         write a picture without opening the drawing; the
                         output's extension picks .md, .png, .jpg, .svg or
                         .html. Flags:
                         --font N, --theme light|dark, --pages together|split
  asciidraw convert notes.md notes.adraw
                         write a drawing (.adraw or .yaml) from a drawing,
                         Markdown or text

  -v, --version          print the version and exit
  -h, --help             print this help and exit

Press ? inside asciidraw for the keys.
`
