package main

import (
	"fmt"
	"html"
	"image/color"
	"slices"
	"strings"
)

// SVG and HTML export. Both lay out cells the way the PNG does (see
// image.go): the font size sets the cell, 7×15 at 12px.
//
// The SVG draws line characters as real lines, joined where they touch,
// so they stay crisp at any zoom and the file stays small. Other
// characters are text, each run stretched to its cells so the columns
// line up whatever font the viewer has.
//
// The HTML is one <pre> per page, with spans for colors. The browser's
// own monospace font draws everything.

func hexOf(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// seg is a straight line in cell units, from (x0, y0) to (x1, y1).
type seg struct {
	x0, y0, x1, y1 float64
	double         bool // drawn as two lines; only for joining up
}

// svgPainter collects shapes, then writes them out with the lines joined.
type svgPainter struct {
	cw, ch float64 // a cell in pixels
	lineW  float64
	lines  map[color.RGBA][]seg
	body   strings.Builder
}

// cellSegs gives the lines a light line character draws in its cell, in
// pixels, from the middle out to the edges it touches.
func (p *svgPainter) lightSegs(ox, oy float64, arms uint8) []seg {
	cx, cy := ox+p.cw/2, oy+p.ch/2
	var out []seg
	if arms&armL != 0 {
		out = append(out, seg{ox, cy, cx, cy, false})
	}
	if arms&armR != 0 {
		out = append(out, seg{cx, cy, ox + p.cw, cy, false})
	}
	if arms&armU != 0 {
		out = append(out, seg{cx, oy, cx, cy, false})
	}
	if arms&armD != 0 {
		out = append(out, seg{cx, cy, cx, oy + p.ch, false})
	}
	return out
}

// doubleSegs is the SVG twin of painter.double.
func (p *svgPainter) doubleSegs(ox, oy float64, arms uint8) []seg {
	w, h := p.cw, p.ch
	cx, cy, d := ox+w/2, oy+h/2, w/6
	U, D, L, R := arms&armU != 0, arms&armD != 0, arms&armL != 0, arms&armR != 0
	pick := func(a bool, va float64, b bool, vb, vc float64) float64 {
		switch {
		case a:
			return va
		case b:
			return vb
		}
		return vc
	}
	var out []seg
	if R {
		out = append(out, seg{pick(U, cx+d, L, cx, cx-d), cy - d, ox + w, cy - d, true})
		out = append(out, seg{pick(D, cx+d, L, cx, cx-d), cy + d, ox + w, cy + d, true})
	}
	if L {
		out = append(out, seg{ox, cy - d, pick(U, cx-d, R, cx, cx+d), cy - d, true})
		out = append(out, seg{ox, cy + d, pick(D, cx-d, R, cx, cx+d), cy + d, true})
	}
	if U {
		out = append(out, seg{cx - d, oy, cx - d, pick(L, cy-d, D, cy, cy+d), true})
		out = append(out, seg{cx + d, oy, cx + d, pick(R, cy-d, D, cy, cy+d), true})
	}
	if D {
		out = append(out, seg{cx - d, pick(L, cy+d, U, cy, cy-d), cx - d, oy + h, true})
		out = append(out, seg{cx + d, pick(R, cy+d, U, cy, cy-d), cx + d, oy + h, true})
	}
	return out
}

// joinSegs merges lines that lie on one row or column and touch, so a box
// side is one line, not one per cell.
func joinSegs(segs []seg) []seg {
	var hs, vs, other []seg
	for _, s := range segs {
		switch {
		case s.y0 == s.y1:
			if s.x0 > s.x1 {
				s.x0, s.x1 = s.x1, s.x0
			}
			hs = append(hs, s)
		case s.x0 == s.x1:
			if s.y0 > s.y1 {
				s.y0, s.y1 = s.y1, s.y0
			}
			vs = append(vs, s)
		default:
			other = append(other, s)
		}
	}
	merge := func(ss []seg, key func(seg) (float64, float64, float64)) []seg {
		slices.SortFunc(ss, func(a, b seg) int {
			ka, sa, _ := key(a)
			kb, sb, _ := key(b)
			if ka != kb {
				return cmpF(ka, kb)
			}
			return cmpF(sa, sb)
		})
		var out []seg
		for _, s := range ss {
			if n := len(out); n > 0 {
				k0, _, e0 := key(out[n-1])
				k1, s1, e1 := key(s)
				if k0 == k1 && s1 <= e0+0.01 {
					if e1 > e0 {
						if out[n-1].y0 == out[n-1].y1 {
							out[n-1].x1 = s.x1
						} else {
							out[n-1].y1 = s.y1
						}
					}
					continue
				}
			}
			out = append(out, s)
		}
		return out
	}
	hs = merge(hs, func(s seg) (float64, float64, float64) { return s.y0, s.x0, s.x1 })
	vs = merge(vs, func(s seg) (float64, float64, float64) { return s.x0, s.y0, s.y1 })
	return append(append(hs, vs...), other...)
}

func cmpF(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func f1(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

// shape writes one character that isn't a line or a letter: corners,
// diagonals, blocks and the marks widgets use. It reports whether it knew
// the character.
func blockRune(r rune) bool { return r == '█' || r == '░' || r == '▒' || r == '▓' }

// block fills a run of one block character, w pixels wide. A shade is the
// color mixed with the background, solid, so rows that touch show no seam.
func (p *svgPainter) block(ox, oy, w float64, r rune, fg, bg color.RGBA) {
	k := map[rune]int{'█': 4, '░': 1, '▒': 2, '▓': 3}[r]
	mix := func(a, b uint8) uint8 { return uint8((int(a)*k + int(b)*(4-k)) / 4) }
	c := color.RGBA{mix(fg.R, bg.R), mix(fg.G, bg.G), mix(fg.B, bg.B), 0xff}
	fmt.Fprintf(&p.body, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n", f1(ox), f1(oy), f1(w+0.2), f1(p.ch+0.2), hexOf(c))
}

func (p *svgPainter) shape(ox, oy float64, r rune, fg color.RGBA) bool {
	w, h := p.cw, p.ch
	cx, cy := ox+w/2, oy+h/2
	fill := hexOf(fg)
	b := &p.body
	switch r {
	case '╭', '╮', '╯', '╰':
		rad := min(w, h) / 2
		// From the edge, along the line, round the corner, out to the other edge.
		var d string
		switch r {
		case '╭':
			d = fmt.Sprintf("M%s %sV%sA%s %s 0 0 1 %s %sH%s", f1(cx), f1(oy+h), f1(cy+rad), f1(rad), f1(rad), f1(cx+rad), f1(cy), f1(ox+w))
		case '╮':
			d = fmt.Sprintf("M%s %sH%sA%s %s 0 0 1 %s %sV%s", f1(ox), f1(cy), f1(cx-rad), f1(rad), f1(rad), f1(cx), f1(cy+rad), f1(oy+h))
		case '╯':
			d = fmt.Sprintf("M%s %sV%sA%s %s 0 0 1 %s %sH%s", f1(cx), f1(oy), f1(cy-rad), f1(rad), f1(rad), f1(cx-rad), f1(cy), f1(ox))
		case '╰':
			d = fmt.Sprintf("M%s %sH%sA%s %s 0 0 1 %s %sV%s", f1(ox+w), f1(cy), f1(cx+rad), f1(rad), f1(rad), f1(cx), f1(cy-rad), f1(oy))
		}
		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%s"/>`+"\n", d, fill, f1(p.lineW))
	case '╱':
		p.lines[fg] = append(p.lines[fg], seg{ox + w, oy, ox, oy + h, false})
	case '╲':
		p.lines[fg] = append(p.lines[fg], seg{ox, oy, ox + w, oy + h, false})
	case '■':
		s := w * 0.75
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n", f1(cx-s/2), f1(cy-s/2), f1(s), f1(s), fill)
	case '▲', '▼', '▶', '◀':
		m, top, bot := w*0.1, cy-w/2, cy+w/2
		var pts [][2]float64
		switch r {
		case '▲':
			pts = [][2]float64{{cx, top}, {ox + w - m, bot}, {ox + m, bot}}
		case '▼':
			pts = [][2]float64{{ox + m, top}, {ox + w - m, top}, {cx, bot}}
		case '▶':
			pts = [][2]float64{{ox + m, top}, {ox + w - m, cy}, {ox + m, bot}}
		case '◀':
			pts = [][2]float64{{ox + w - m, top}, {ox + w - m, bot}, {ox + m, cy}}
		}
		var ps []string
		for _, q := range pts {
			ps = append(ps, f1(q[0])+","+f1(q[1]))
		}
		fmt.Fprintf(b, `<polygon points="%s" fill="%s"/>`+"\n", strings.Join(ps, " "), fill)
	case '●':
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", f1(cx), f1(cy), f1(w*0.38), fill)
	default:
		return false
	}
	return true
}

// renderSVG draws pages one under another, like renderPages.
func renderSVG(pages [][]object, px int, th theme) []byte {
	cwi, chi := cellSize(px)
	cw, ch := float64(cwi), float64(chi)
	pics, cols, rows := layout(pages, th)
	W, H := float64(cols)*cw, float64(rows)*ch
	p := &svgPainter{cw: cw, ch: ch, lineW: max(1, float64(px)/12), lines: map[color.RGBA][]seg{}}
	b := &p.body
	fmt.Fprintf(b, `<rect width="%s" height="%s" fill="%s"/>`+"\n", f1(W), f1(H), hexOf(th.bg))
	var texts strings.Builder
	y := 1
	for i, pic := range pics {
		if i > 0 {
			ry := float64(y)*ch + ch/2
			fmt.Fprintf(b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="%s"/>`+"\n", f1(cw/2), f1(ry), f1(W-cw/2), f1(ry), hexOf(th.rule), f1(p.lineW))
			y++
		}
		for r, row := range pic {
			oy := float64(y+r) * ch
			// Backgrounds, a run of one color at a time.
			for c := 0; c < len(row); {
				e := c
				for e < len(row) && row[e].bg == row[c].bg {
					e++
				}
				if row[c].bg != th.bg {
					fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n", f1(float64(c+1)*cw), f1(oy), f1(float64(e-c)*cw), f1(ch), hexOf(row[c].bg))
				}
				c = e
			}
			// Lines and shapes; letters gather into runs of one color.
			run, runX, runC := []rune{}, 0, color.RGBA{}
			flush := func() {
				if strings.TrimSpace(string(run)) == "" {
					run = run[:0]
					return
				}
				s := strings.TrimRight(string(run), " ")
				n := runeLen(s)
				fmt.Fprintf(&texts, `<text x="%s" y="%s" fill="%s" textLength="%s">%s</text>`+"\n",
					f1(float64(runX+1)*cw), f1(oy+ch*0.75), hexOf(runC), f1(float64(n)*cw), html.EscapeString(s))
				run = run[:0]
			}
			for c, cell := range row {
				ox := float64(c+1) * cw
				if arms, ok := lightArms[cell.r]; ok {
					flush()
					p.lines[cell.fg] = append(p.lines[cell.fg], p.lightSegs(ox, oy, arms)...)
					continue
				}
				if arms, ok := doubleArms[cell.r]; ok {
					flush()
					p.lines[cell.fg] = append(p.lines[cell.fg], p.doubleSegs(ox, oy, arms)...)
					continue
				}
				if blockRune(cell.r) {
					flush()
					if c > 0 && row[c-1].r == cell.r && row[c-1].fg == cell.fg && row[c-1].bg == cell.bg {
						continue // drawn with the first cell of the run
					}
					e := c
					for e < len(row) && row[e].r == cell.r && row[e].fg == cell.fg && row[e].bg == cell.bg {
						e++
					}
					p.block(ox, oy, float64(e-c)*cw, cell.r, cell.fg, cell.bg)
					continue
				}
				if cell.r != ' ' && p.shape(ox, oy, cell.r, cell.fg) {
					flush()
					continue
				}
				// A new color, or a gap of two spaces, starts a new run, so
				// stretching a run to its cells can't pull letters apart.
				if len(run) > 0 && cell.r != ' ' && (cell.fg != runC || strings.HasSuffix(string(run), "  ")) {
					flush()
				}
				if len(run) == 0 {
					if cell.r == ' ' {
						continue
					}
					runX, runC = c, cell.fg
				}
				run = append(run, cell.r)
			}
			flush()
		}
		y += max(len(pic), 1)
	}
	// Lines, one path per color.
	keys := make([]color.RGBA, 0, len(p.lines))
	for c := range p.lines {
		keys = append(keys, c)
	}
	slices.SortFunc(keys, func(a, b color.RGBA) int { return strings.Compare(hexOf(a), hexOf(b)) })
	for _, c := range keys {
		var d strings.Builder
		for _, s := range joinSegs(p.lines[c]) {
			fmt.Fprintf(&d, "M%s %sL%s %s", f1(s.x0), f1(s.y0), f1(s.x1), f1(s.y1))
		}
		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linecap="square"/>`+"\n", d.String(), hexOf(c), f1(p.lineW))
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`+"\n<style>rect{shape-rendering:crispEdges}</style>\n", f1(W), f1(H), f1(W), f1(H))
	out.WriteString(b.String())
	if texts.Len() > 0 {
		fmt.Fprintf(&out, `<g font-family="ui-monospace,Menlo,Consolas,'DejaVu Sans Mono',monospace" font-size="%d" xml:space="preserve">`+"\n", px)
		out.WriteString(texts.String())
		out.WriteString("</g>\n")
	}
	out.WriteString("</svg>\n")
	return []byte(out.String())
}

// renderHTML writes a page that shows the drawing: one <pre> per page,
// with a "Page N" line above each when there are several, like the
// Markdown export.
func renderHTML(pages [][]object, px int, th theme, title string) []byte {
	_, ch := cellSize(px)
	var b strings.Builder
	fmt.Fprintf(&b, "<!doctype html>\n<meta charset=\"utf-8\">\n<title>%s</title>\n", html.EscapeString(title))
	fmt.Fprintf(&b, "<style>body{margin:0;padding:%dpx;background:%s;color:%s;font:%dpx sans-serif}"+
		"pre{margin:0 0 %dpx;font:%dpx/%dpx ui-monospace,Menlo,Consolas,'DejaVu Sans Mono',monospace}</style>\n",
		ch, hexOf(th.bg), hexOf(th.fg), px, ch, px, ch)
	for i, objs := range pages {
		if len(pages) > 1 {
			fmt.Fprintf(&b, "<p>Page %s</p>\n", pageChar(i))
		}
		b.WriteString("<pre>")
		for _, row := range picture(objs, th) {
			// Spaces at the end of a row, in the page's own colors, add nothing.
			for len(row) > 0 && row[len(row)-1].r == ' ' && row[len(row)-1].bg == th.bg {
				row = row[:len(row)-1]
			}
			// Spans for runs of one color; the theme's colors need none.
			for c := 0; c < len(row); {
				e := c
				// A space shows no text color, so it joins whatever run it is in.
				for e < len(row) && row[e].bg == row[c].bg && (row[e].fg == row[c].fg || row[e].r == ' ') {
					e++
				}
				for e > c+1 && row[e-1].r == ' ' && row[e-1].fg != row[c].fg {
					e-- // but a run doesn't end in borrowed spaces
				}
				var s strings.Builder
				for _, cell := range row[c:e] {
					s.WriteRune(cell.r)
				}
				text := html.EscapeString(s.String())
				var style []string
				if row[c].fg != th.fg {
					style = append(style, "color:"+hexOf(row[c].fg))
				}
				if row[c].bg != th.bg {
					style = append(style, "background:"+hexOf(row[c].bg))
				}
				if len(style) > 0 {
					fmt.Fprintf(&b, `<span style="%s">%s</span>`, strings.Join(style, ";"), text)
				} else {
					b.WriteString(text)
				}
				c = e
			}
			b.WriteString("\n")
		}
		b.WriteString("</pre>\n")
	}
	return []byte(b.String())
}
