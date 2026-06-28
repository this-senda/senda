// Package termimg renders ANSI terminal output (the kind lipgloss/Bubble Tea
// emit) into images, and encodes a sequence of frames into an animated GIF.
//
// It exists so the senda terminal-UI documentation screenshots and the walkthrough GIF
// can be generated headlessly from the real tuiModel.render() output — no PTY,
// no ffmpeg, no external capture tools. Everything is pure Go: an SGR parser
// builds a cell grid, a cached glyph rasterizer paints it with a monospace
// font, and box-drawing runes are stroked procedurally so panel borders connect
// seamlessly regardless of font metrics.
package termimg

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"

	"github.com/mattn/go-runewidth"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Options configures a Renderer. The zero value is not usable; call Defaults
// and override what you need.
type Options struct {
	CellW, CellH int        // cell size in pixels
	Pad          int        // padding around the grid, in pixels
	FontSize     float64    // glyph point size (DPI is fixed at 72 → points == px)
	DefaultFg    color.RGBA // foreground for unstyled cells
	DefaultBg    color.RGBA // background for unstyled cells and padding

	// Antialias controls glyph edge smoothing. Leave it on (true) for both PNG
	// stills and GIFs — combined with Supersample it gives crisp, smooth glyphs.
	// Turning it off thresholds glyphs to 1-bit coverage (pure fg/bg, no blends),
	// which keeps a GIF's 256-colour palette vivid but renders chunky at 1×.
	Antialias bool

	// SoftHinting picks font.HintingVertical instead of the default HintingFull.
	// Full hinting snaps stems to the pixel grid — crisp at 1× but chunky when
	// supersampled; soften it for the hi-res supersampled pass.
	SoftHinting bool

	// Supersample renders the grid at this integer factor, then downscales the
	// result back to 1× with a Catmull-Rom filter — crisp, grayscale-AA glyphs at
	// the normal output size without shipping an oversized image. 0/1 = off.
	// Set Antialias=true alongside it so the downscale has real coverage to blend.
	Supersample int

	// Font search paths. The first readable file in each list wins. Fallback
	// faces are consulted, in order, only for runes the primary face lacks
	// (e.g. the 🔒 secret marker, which DejaVu Sans Mono does not carry).
	RegularPaths  []string
	BoldPaths     []string
	FallbackPaths []string
}

// Defaults returns the options used for the senda terminal-UI docs: a Tokyo-Night-ish
// dark ground matching the TUI palette, DejaVu Sans Mono primary, FreeMono
// fallback.
func Defaults() Options {
	return Options{
		CellW:     9,
		CellH:     19,
		Pad:       14,
		FontSize:  14.7,
		Antialias: true,
		DefaultFg: color.RGBA{0xa9, 0xb1, 0xd6, 0xff}, // colFg
		DefaultBg: color.RGBA{0x15, 0x16, 0x1a, 0xff}, // bgApp
		RegularPaths: envPaths("SENDA_TUI_FONT",
			"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", // Debian/Ubuntu
			"/usr/share/fonts/dejavu/DejaVuSansMono.ttf",          // Fedora
			"/usr/share/fonts/TTF/DejaVuSansMono.ttf",             // Arch
			"/Library/Fonts/DejaVuSansMono.ttf",                   // macOS
		),
		BoldPaths: envPaths("SENDA_TUI_FONT_BOLD",
			"/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf",
			"/usr/share/fonts/dejavu/DejaVuSansMono-Bold.ttf",
			"/usr/share/fonts/TTF/DejaVuSansMono-Bold.ttf", // Arch
			"/Library/Fonts/DejaVuSansMono-Bold.ttf",
		),
		FallbackPaths: envPaths("SENDA_TUI_FONT_FALLBACK",
			"/usr/share/fonts/truetype/freefont/FreeMono.ttf",
			"/usr/share/fonts/freefont/FreeMono.ttf",
			"/usr/share/fonts/gnu-free/FreeMono.otf", // Arch (gnu-free-fonts)
		),
	}
}

func envPaths(env string, defaults ...string) []string {
	if v := os.Getenv(env); v != "" {
		return append([]string{v}, defaults...)
	}
	return defaults
}

type loadedFont struct {
	sf   *sfnt.Font
	face font.Face
}

// Renderer turns ANSI strings into images. Construct one with New and reuse it;
// glyph masks are cached across calls.
type Renderer struct {
	opt      Options
	regular  loadedFont
	bold     loadedFont
	fallback []loadedFont
	baseline int
	ss       int // supersample factor (≥1)

	masks map[maskKey]*image.Alpha
}

type maskKey struct {
	r    rune
	face int8 // 0 regular, 1 bold, 2+ fallback index+2
}

// New loads the fonts and prepares a Renderer.
func New(opt Options) (*Renderer, error) {
	ss := opt.Supersample
	if ss < 1 {
		ss = 1
	}
	// Render everything at ss×; Image downscales the finished frame back to 1×.
	opt.CellW *= ss
	opt.CellH *= ss
	opt.Pad *= ss
	opt.FontSize *= float64(ss)
	r := &Renderer{opt: opt, ss: ss, masks: map[maskKey]*image.Alpha{}}
	var err error
	if r.regular, err = loadFace(opt.RegularPaths, opt.FontSize, opt.SoftHinting); err != nil {
		return nil, fmt.Errorf("regular font: %w", err)
	}
	// Bold is optional; fall back to regular if absent.
	if r.bold, err = loadFace(opt.BoldPaths, opt.FontSize, opt.SoftHinting); err != nil {
		r.bold = r.regular
	}
	for _, p := range opt.FallbackPaths {
		if lf, e := loadFace([]string{p}, opt.FontSize, opt.SoftHinting); e == nil {
			r.fallback = append(r.fallback, lf)
		}
	}
	m := r.regular.face.Metrics()
	content := (m.Ascent + m.Descent).Round()
	r.baseline = m.Ascent.Round() + (opt.CellH-content)/2
	return r, nil
}

func loadFace(paths []string, size float64, soft bool) (loadedFont, error) {
	hint := font.HintingFull
	if soft {
		hint = font.HintingVertical
	}
	var lastErr error
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			lastErr = err
			continue
		}
		sf, err := sfnt.Parse(b)
		if err != nil {
			lastErr = err
			continue
		}
		face, err := opentype.NewFace(sf, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: hint})
		if err != nil {
			lastErr = err
			continue
		}
		return loadedFont{sf: sf, face: face}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no font path provided")
	}
	return loadedFont{}, lastErr
}

// Image parses ansi and renders it to an RGBA image.
func (r *Renderer) Image(ansi string) *image.RGBA {
	g := r.parse(ansi)
	w := r.opt.Pad*2 + g.cols*r.opt.CellW
	h := r.opt.Pad*2 + g.rows*r.opt.CellH
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{r.opt.DefaultBg}, image.Point{}, draw.Src)
	for y := 0; y < g.rows; y++ {
		for x := 0; x < g.cols; x++ {
			r.drawCell(img, x, y, g.at(x, y))
		}
	}
	if r.ss > 1 {
		dst := image.NewRGBA(image.Rect(0, 0, w/r.ss, h/r.ss))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
		return dst
	}
	return img
}

// PNG renders ansi and writes it as PNG.
func (r *Renderer) PNG(w io.Writer, ansi string) error {
	return png.Encode(w, r.Image(ansi))
}

func (r *Renderer) drawCell(img *image.RGBA, col, row int, c cell) {
	fg, bg := c.fg, c.bg
	if c.reverse {
		fg, bg = bg, fg
	}
	if c.faint {
		fg = blend(bg, fg, 0.55)
	}
	x0 := r.opt.Pad + col*r.opt.CellW
	y0 := r.opt.Pad + row*r.opt.CellH
	cw := r.opt.CellW * runewidthOr1(c.r)
	rect := image.Rect(x0, y0, x0+cw, y0+r.opt.CellH)
	draw.Draw(img, rect, &image.Uniform{bg}, image.Point{}, draw.Src)

	if c.r != 0 && c.r != ' ' {
		if box := r.drawBox(img, x0, y0, fg, c.r); !box {
			r.drawGlyph(img, x0, y0, fg, c.r, c.bold)
		}
	}
	if c.underline {
		uy := y0 + r.opt.CellH - 2
		ul := image.Rect(x0, uy, x0+cw, uy+1)
		draw.Draw(img, ul, &image.Uniform{fg}, image.Point{}, draw.Src)
	}
}

// drawGlyph paints rune ch in colour fg with its top-left cell origin at (x0,y0)
// using a cached alpha mask.
func (r *Renderer) drawGlyph(img *image.RGBA, x0, y0 int, fg color.RGBA, ch rune, bold bool) {
	lf, faceID := r.faceFor(ch, bold)
	mask := r.mask(ch, faceID, lf)
	if mask == nil {
		return
	}
	b := mask.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		py := y0 + y
		for x := b.Min.X; x < b.Max.X; x++ {
			a := mask.AlphaAt(x, y).A
			if a == 0 {
				continue
			}
			px := x0 + x
			base := img.RGBAAt(px, py)
			img.SetRGBA(px, py, over(base, fg, a))
		}
	}
}

// mask rasterises a glyph into an alpha tile, caching by rune+face.
func (r *Renderer) mask(ch rune, faceID int8, lf loadedFont) *image.Alpha {
	key := maskKey{ch, faceID}
	if m, ok := r.masks[key]; ok {
		return m
	}
	w := r.opt.CellW * runewidthOr1(ch)
	tile := image.NewAlpha(image.Rect(0, 0, w, r.opt.CellH))
	d := &font.Drawer{Dst: tile, Src: image.Opaque, Face: lf.face, Dot: fixed.P(0, r.baseline)}
	d.DrawString(string(ch))
	if !r.opt.Antialias {
		// Threshold to 1-bit coverage: pixels are pure fg or pure bg, no blends.
		for i, a := range tile.Pix {
			if a >= 128 {
				tile.Pix[i] = 0xff
			} else {
				tile.Pix[i] = 0
			}
		}
	}
	r.masks[key] = tile
	return tile
}

// faceFor picks the face that actually carries ch, falling back when the
// primary (bold or regular) lacks the glyph.
func (r *Renderer) faceFor(ch rune, bold bool) (loadedFont, int8) {
	if bold && hasGlyph(r.bold.sf, ch) {
		return r.bold, 1
	}
	if hasGlyph(r.regular.sf, ch) {
		return r.regular, 0
	}
	for i, fb := range r.fallback {
		if hasGlyph(fb.sf, ch) {
			return fb, int8(2 + i)
		}
	}
	if bold {
		return r.bold, 1
	}
	return r.regular, 0
}

func hasGlyph(sf *sfnt.Font, ch rune) bool {
	if sf == nil {
		return false
	}
	var buf sfnt.Buffer
	idx, err := sf.GlyphIndex(&buf, ch)
	return err == nil && idx != 0
}

func runewidthOr1(r rune) int {
	if w := runewidth.RuneWidth(r); w > 0 {
		return w
	}
	return 1
}

// over alpha-composites src (with coverage a/255) onto an opaque dst.
func over(dst, src color.RGBA, a uint8) color.RGBA {
	af := float64(a) / 255
	return color.RGBA{
		R: uint8(float64(dst.R)*(1-af) + float64(src.R)*af),
		G: uint8(float64(dst.G)*(1-af) + float64(src.G)*af),
		B: uint8(float64(dst.B)*(1-af) + float64(src.B)*af),
		A: 0xff,
	}
}

func blend(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 0xff,
	}
}
