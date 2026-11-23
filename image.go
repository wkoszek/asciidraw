package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// PNG and JPEG export. Each character cell becomes a block of pixels, sized
// from a font size: 12px, the reference, makes cells 7×15 pixels. Letters
// come from Go Mono, which is built in, so a picture looks the same on
// every machine. Lines, corners, shades and the shapes widgets use are
// drawn as pixels instead, so they meet across cells with no gaps.
//
// To keep files small, edges blend in only a few steps between a color
// and its background. That keeps a picture to a handful of colors, which
// lets the PNG be stored with a small palette.

// cellSize is the size of one character cell at a font size in pixels.
func cellSize(px int) (w, h int) {
	return int(math.Round(float64(px) * 0.6)), int(math.Round(float64(px) * 1.25))
}

// alphaSteps is how many steps an edge blends in, from background to color.
const alphaSteps = 4

type theme struct{ bg, fg, rule color.RGBA }

var themes = map[string]theme{
	"light": {rgb(0xffffff), rgb(0x222222), rgb(0xd0d0d0)},
	"dark":  {rgb(0x1e1e1e), rgb(0xd4d4d4), rgb(0x3c3c3c)},
}

func rgb(v uint32) color.RGBA { return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff} }

// namedRGB gives the terminal palette names fixed colors, since a picture
// has no terminal theme to follow.
var namedRGB = map[string]color.RGBA{
	"black": rgb(0x000000), "red": rgb(0xcd3131), "green": rgb(0x0dbc79),
	"yellow": rgb(0xe5e510), "blue": rgb(0x2472c8), "magenta": rgb(0xbc3fbc),
	"cyan": rgb(0x11a8cd), "white": rgb(0xe5e5e5), "gray": rgb(0x666666),
}

// rgbOf turns a stored color name into a color; "" gives def.
func rgbOf(name string, def color.RGBA) color.RGBA {
	if c, ok := namedRGB[name]; ok {
		return c
	}
	if strings.HasPrefix(name, "#") {
		if v, err := strconv.ParseUint(name[1:], 16, 32); err == nil {
			return rgb(uint32(v))
		}
	}
	return def
}

// contrast picks near-black or near-white text, whichever shows better
// on a background.
func contrast(bg color.RGBA) color.RGBA {
	if 299*int(bg.R)+587*int(bg.G)+114*int(bg.B) > 128*1000 {
		return themes["light"].fg
	}
	return themes["dark"].fg
}

var (
	monoOnce sync.Once
	monoFont *opentype.Font
)

func faceFor(px int) font.Face {
	monoOnce.Do(func() { monoFont, _ = opentype.Parse(gomono.TTF) })
	face, _ := opentype.NewFace(monoFont, &opentype.FaceOptions{Size: float64(px), DPI: 72, Hinting: font.HintingFull})
	return face
}

// picCell is one cell of a picture, with its colors worked out.
type picCell struct {
	r      rune
	fg, bg color.RGBA
}

// picture lays out one page as cells, trimmed to what was drawn.
func picture(objs []object, th theme) [][]picCell {
	c := compose(objs)
	rows := 0
	cols := 0
	for y := range c.rows {
		for x := range c.rows[y] {
			if c.rows[y][x] != ' ' || c.bgc[y][x] != "" {
				rows = y + 1
				cols = max(cols, x+1)
			}
		}
	}
	out := make([][]picCell, rows)
	for y := range out {
		out[y] = make([]picCell, cols)
		for x := range out[y] {
			bg := rgbOf(c.bgAt(x, y), th.bg)
			fg := th.fg
			if bg != th.bg {
				fg = contrast(bg) // the theme's text color may not show on it
			}
			if i := c.owner(x, y); i >= 0 && i < len(objs) {
				fg = rgbOf(objs[i].FG, fg)
			}
			out[y][x] = picCell{c.get(x, y), fg, bg}
		}
	}
	return out
}

// painter draws cells into an image.
type painter struct {
	img       *image.RGBA
	face      font.Face
	cw, ch    int
	ascent    int
	line, dbl int // line thickness, and how far apart a double line's two lines are
	rast      *vector.Rasterizer
	mask      *image.Alpha
}

func newPainter(img *image.RGBA, px int) *painter {
	cw, ch := cellSize(px)
	face := faceFor(px)
	m := face.Metrics()
	asc, desc := m.Ascent.Round(), m.Descent.Round()
	t := max(1, int(math.Round(float64(px)/12)))
	return &painter{
		img: img, face: face, cw: cw, ch: ch,
		ascent: (ch-(asc+desc))/2 + asc,
		line:   t, dbl: t,
		rast: vector.NewRasterizer(cw, ch),
		mask: image.NewAlpha(image.Rect(0, 0, cw, ch)),
	}
}

// blend mixes c over the pixel at (x, y) by alpha a, 0 to 255, in a few
// steps only.
func (p *painter) blend(x, y int, c color.RGBA, a uint8) {
	q := (int(a)*alphaSteps + 127) / 255
	if q == 0 || !(image.Point{x, y}.In(p.img.Rect)) {
		return
	}
	i := p.img.PixOffset(x, y)
	px := p.img.Pix[i : i+4 : i+4]
	mix := func(d, s uint8) uint8 { return uint8((int(d)*(alphaSteps-q) + int(s)*q) / alphaSteps) }
	px[0], px[1], px[2], px[3] = mix(px[0], c.R), mix(px[1], c.G), mix(px[2], c.B), 0xff
}

func (p *painter) rect(x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			p.blend(x, y, c, 0xff)
		}
	}
}

// fillMask paints the rasterizer's shape, placed at cell (ox, oy).
func (p *painter) fillMask(ox, oy int, c color.RGBA) {
	clear(p.mask.Pix)
	p.rast.Draw(p.mask, p.mask.Rect, image.Opaque, image.Point{})
	for y := 0; y < p.ch; y++ {
		for x := 0; x < p.cw; x++ {
			if a := p.mask.Pix[y*p.mask.Stride+x]; a > 0 {
				p.blend(ox+x, oy+y, c, a)
			}
		}
	}
	p.rast.Reset(p.cw, p.ch)
}

func (p *painter) poly(pts ...[2]float32) {
	p.rast.MoveTo(pts[0][0], pts[0][1])
	for _, q := range pts[1:] {
		p.rast.LineTo(q[0], q[1])
	}
	p.rast.ClosePath()
}

// Which way each line character runs.
var lightArms = map[rune]uint8{
	'─': armL | armR, '│': armU | armD, '┌': armR | armD, '┐': armL | armD,
	'└': armU | armR, '┘': armU | armL, '├': armU | armD | armR, '┤': armU | armD | armL,
	'┬': armL | armR | armD, '┴': armL | armR | armU, '┼': armU | armD | armL | armR,
}

var doubleArms = map[rune]uint8{
	'═': armL | armR, '║': armU | armD, '╔': armR | armD, '╗': armL | armD,
	'╚': armU | armR, '╝': armU | armL, '╠': armU | armD | armR, '╣': armU | armD | armL,
	'╦': armL | armR | armD, '╩': armL | armR | armU, '╬': armU | armD | armL | armR,
}

// cell draws one character at pixel (ox, oy). The background is already
// there.
func (p *painter) cell(ox, oy int, r rune, fg color.RGBA) {
	w, h, t := p.cw, p.ch, p.line
	cx, cy := w/2, h/2
	// hline and vline draw a line t pixels thick, centered on y or x.
	hline := func(x0, x1, y int) { p.rect(ox+x0, oy+y-t/2, ox+x1, oy+y-t/2+t, fg) }
	vline := func(y0, y1, x int) { p.rect(ox+x-t/2, oy+y0, ox+x-t/2+t, oy+y1, fg) }
	if arms, ok := lightArms[r]; ok {
		if arms&armL != 0 {
			hline(0, cx+t-t/2, cy)
		}
		if arms&armR != 0 {
			hline(cx-t/2, w, cy)
		}
		if arms&armU != 0 {
			vline(0, cy+t-t/2, cx)
		}
		if arms&armD != 0 {
			vline(cy-t/2, h, cx)
		}
		return
	}
	if arms, ok := doubleArms[r]; ok {
		p.double(ox, oy, arms, fg)
		return
	}
	f := func(v int) float32 { return float32(v) }
	switch r {
	case ' ':
	case '╭', '╮', '╯', '╰':
		p.corner(ox, oy, r, fg)
	case '╱':
		p.poly([2]float32{f(w), 0}, [2]float32{f(w), f(t)}, [2]float32{f(t), f(h)}, [2]float32{0, f(h)}, [2]float32{0, f(h - t)}, [2]float32{f(w - t), 0})
		p.fillMask(ox, oy, fg)
	case '╲':
		p.poly([2]float32{0, 0}, [2]float32{f(t), 0}, [2]float32{f(w), f(h - t)}, [2]float32{f(w), f(h)}, [2]float32{f(w - t), f(h)}, [2]float32{0, f(t)})
		p.fillMask(ox, oy, fg)
	case '█':
		p.rect(ox, oy, ox+w, oy+h, fg)
	case '░', '▒', '▓':
		a := map[rune]uint8{'░': 64, '▒': 128, '▓': 192}[r]
		for y := oy; y < oy+h; y++ {
			for x := ox; x < ox+w; x++ {
				p.blend(x, y, fg, a)
			}
		}
	case '■':
		s := w * 3 / 4
		p.rect(ox+(w-s)/2, oy+(h-s)/2, ox+(w-s)/2+s, oy+(h-s)/2+s, fg)
	case '▲', '▼', '▶', '◀':
		m := float32(w) * 0.1
		top, bot := f(h)/2-f(w)/2, f(h)/2+f(w)/2
		switch r {
		case '▲':
			p.poly([2]float32{f(w) / 2, top}, [2]float32{f(w) - m, bot}, [2]float32{m, bot})
		case '▼':
			p.poly([2]float32{m, top}, [2]float32{f(w) - m, top}, [2]float32{f(w) / 2, bot})
		case '▶':
			p.poly([2]float32{m, top}, [2]float32{f(w) - m, f(h) / 2}, [2]float32{m, bot})
		case '◀':
			p.poly([2]float32{f(w) - m, top}, [2]float32{f(w) - m, bot}, [2]float32{m, f(h) / 2})
		}
		p.fillMask(ox, oy, fg)
	case '●':
		p.circle(f(w)/2, f(h)/2, f(w)*0.38)
		p.fillMask(ox, oy, fg)
	default:
		p.glyph(ox, oy, r, fg)
	}
}

// double draws a double line. Each arm is two lines, d pixels either side
// of the middle; where arms meet, the inner and outer lines stop at each
// other, the way ╔ and ╬ look in a terminal font.
func (p *painter) double(ox, oy int, arms uint8, fg color.RGBA) {
	w, h, t, d := p.cw, p.ch, p.line, p.dbl
	cx, cy := w/2, h/2
	U, D, L, R := arms&armU != 0, arms&armD != 0, arms&armL != 0, arms&armR != 0
	pick := func(a bool, va int, b bool, vb, vc int) int {
		switch {
		case a:
			return va
		case b:
			return vb
		}
		return vc
	}
	hline := func(x0, x1, y int) { p.rect(ox+x0-t/2, oy+y-t/2, ox+x1-t/2+t, oy+y-t/2+t, fg) }
	vline := func(y0, y1, x int) { p.rect(ox+x-t/2, oy+y0-t/2, ox+x-t/2+t, oy+y1-t/2+t, fg) }
	if R {
		hline(pick(U, cx+d, L, cx, cx-d), w+t, cy-d)
		hline(pick(D, cx+d, L, cx, cx-d), w+t, cy+d)
	}
	if L {
		hline(-t, pick(U, cx-d, R, cx, cx+d), cy-d)
		hline(-t, pick(D, cx-d, R, cx, cx+d), cy+d)
	}
	if U {
		vline(-t, pick(L, cy-d, D, cy, cy+d), cx-d)
		vline(-t, pick(R, cy-d, D, cy, cy+d), cx+d)
	}
	if D {
		vline(pick(L, cy+d, U, cy, cy-d), h+t, cx-d)
		vline(pick(R, cy+d, U, cy, cy-d), h+t, cx+d)
	}
}

// corner draws a rounded corner: a quarter circle, with straight lines out
// to the cell's edges.
func (p *painter) corner(ox, oy int, r rune, fg color.RGBA) {
	w, h, t := p.cw, p.ch, p.line
	cx, cy := w/2, h/2
	rad := float64(min(cx, cy))
	// The arc's center, and which way it turns, for each corner.
	sx, sy := 1.0, 1.0 // ╭: the arc runs right and down from its center
	switch r {
	case '╮':
		sx = -1
	case '╯':
		sx, sy = -1, -1
	case '╰':
		sy = -1
	}
	ccx, ccy := float64(cx)+sx*rad+0.5, float64(cy)+sy*rad+0.5
	in, out := rad-float64(t)/2, rad+float64(t)/2
	const n = 12
	var pts [][2]float32
	for i := 0; i <= n; i++ {
		a := float64(i) / n * math.Pi / 2
		pts = append(pts, [2]float32{float32(ccx - sx*out*math.Cos(a)), float32(ccy - sy*out*math.Sin(a))})
	}
	for i := n; i >= 0; i-- {
		a := float64(i) / n * math.Pi / 2
		pts = append(pts, [2]float32{float32(ccx - sx*in*math.Cos(a)), float32(ccy - sy*in*math.Sin(a))})
	}
	p.poly(pts...)
	p.fillMask(ox, oy, fg)
	// Straight on from the arc's ends to the edges of the cell.
	ex, ey := int(ccx), int(ccy)
	if sx > 0 {
		p.rect(ox+ex, oy+cy-t/2, ox+w, oy+cy-t/2+t, fg)
	} else {
		p.rect(ox, oy+cy-t/2, ox+ex, oy+cy-t/2+t, fg)
	}
	if sy > 0 {
		p.rect(ox+cx-t/2, oy+ey, ox+cx-t/2+t, oy+h, fg)
	} else {
		p.rect(ox+cx-t/2, oy, ox+cx-t/2+t, oy+ey, fg)
	}
}

func (p *painter) circle(cx, cy, r float32) {
	const n = 24
	var pts [][2]float32
	for i := 0; i < n; i++ {
		a := float64(i) / n * 2 * math.Pi
		pts = append(pts, [2]float32{cx + r*float32(math.Cos(a)), cy + r*float32(math.Sin(a))})
	}
	p.poly(pts...)
}

// glyph draws a letter from the font, centered in its cell. A character
// the font lacks is drawn as an empty box.
func (p *painter) glyph(ox, oy int, r rune, fg color.RGBA) {
	adv, ok := p.face.GlyphAdvance(r)
	if !ok {
		t := p.line
		p.rect(ox+1, oy+2, ox+p.cw-1, oy+2+t, fg)
		p.rect(ox+1, oy+p.ch-2-t, ox+p.cw-1, oy+p.ch-2, fg)
		p.rect(ox+1, oy+2, ox+1+t, oy+p.ch-2, fg)
		p.rect(ox+p.cw-1-t, oy+2, ox+p.cw-1, oy+p.ch-2, fg)
		return
	}
	dot := fixed.Point26_6{X: fixed.I(ox) + (fixed.I(p.cw)-adv)/2, Y: fixed.I(oy + p.ascent)}
	dr, mask, mp, _, ok := p.face.Glyph(dot, r)
	if !ok {
		return
	}
	for y := dr.Min.Y; y < dr.Max.Y; y++ {
		for x := dr.Min.X; x < dr.Max.X; x++ {
			_, _, _, a := mask.At(mp.X+x-dr.Min.X, mp.Y+y-dr.Min.Y).RGBA()
			p.blend(x, y, fg, uint8(a>>8))
		}
	}
}

// layout measures pages for renderPages: each page as cells, and the
// image's size in cells. Pages sit one under another with a row between
// them for a rule, and a margin of one cell all around.
func layout(pages [][]object, th theme) (pics [][][]picCell, cols, rows int) {
	cols = 1
	for i, objs := range pages {
		pic := picture(objs, th)
		pics = append(pics, pic)
		if i > 0 {
			rows++
		}
		rows += max(len(pic), 1)
		for _, row := range pic {
			cols = max(cols, len(row))
		}
	}
	return pics, cols + 2, rows + 2
}

// pixelSize is how big renderPages makes an image, without drawing it.
func pixelSize(pages [][]object, px int) (w, h int) {
	_, cols, rows := layout(pages, themes["light"])
	cw, ch := cellSize(px)
	return cols * cw, rows * ch
}

// renderPages draws pages one under another, with a thin rule between them.
func renderPages(pages [][]object, px int, th theme) *image.RGBA {
	cw, ch := cellSize(px)
	pics, cols, rows := layout(pages, th)
	W, H := cols*cw, rows*ch
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	p := newPainter(img, px)
	p.rect(0, 0, W, H, th.bg)
	y := 1
	for i, pic := range pics {
		if i > 0 {
			p.rect(cw/2, y*ch+ch/2, W-cw/2, y*ch+ch/2+p.line, th.rule)
			y++
		}
		for r, row := range pic {
			for c, cell := range row {
				if cell.bg != th.bg {
					p.rect((c+1)*cw, (y+r)*ch, (c+2)*cw, (y+r+1)*ch, cell.bg)
				}
			}
		}
		for r, row := range pic {
			for c, cell := range row {
				p.cell((c+1)*cw, (y+r)*ch, cell.r, cell.fg)
			}
		}
		y += max(len(pic), 1)
	}
	return img
}

// encodePNG stores the image with a palette when it has 256 colors or
// fewer, which it nearly always does, and at the best compression.
func encodePNG(img *image.RGBA) ([]byte, error) {
	var out image.Image = img
	if pal := paletteOf(img); pal != nil {
		out = pal
	}
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	err := enc.Encode(&b, out)
	return b.Bytes(), err
}

func paletteOf(img *image.RGBA) *image.Paletted {
	index := map[color.RGBA]uint8{}
	var pal color.Palette
	pix := img.Pix
	idx := make([]uint8, len(pix)/4)
	for i := 0; i < len(pix); i += 4 {
		c := color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
		k, ok := index[c]
		if !ok {
			if len(pal) == 256 {
				return nil
			}
			k = uint8(len(pal))
			index[c] = k
			pal = append(pal, c)
		}
		idx[i/4] = k
	}
	return &image.Paletted{Pix: idx, Stride: img.Rect.Dx(), Rect: img.Rect, Palette: pal}
}

// encodeJPEG stores a picture with no color in it as grayscale, which
// makes a smaller file.
func encodeJPEG(img *image.RGBA) ([]byte, error) {
	var out image.Image = img
	if isGray(img) {
		g := image.NewGray(img.Rect)
		for i := 0; i < len(img.Pix); i += 4 {
			g.Pix[i/4] = img.Pix[i]
		}
		out = g
	}
	var b bytes.Buffer
	err := jpeg.Encode(&b, out, &jpeg.Options{Quality: 80})
	return b.Bytes(), err
}

func isGray(img *image.RGBA) bool {
	p := img.Pix
	for i := 0; i < len(p); i += 4 {
		if p[i] != p[i+1] || p[i] != p[i+2] {
			return false
		}
	}
	return true
}
