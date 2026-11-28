package exportpdf

import (
	"bytes"
	"fmt"
	"golang.org/x/image/font/sfnt"
	"image"
	"image/png"
	"strings"
	"unicode"

	"github.com/go-pdf/fpdf"
	"github.com/ispacewang/quesora-native/internal/mathtext"
)

const leftEdge = 21.0
const lineWidth = 168.0
const bottomEdge = 273.0
const mmPerDIP = 25.4 / 96

type pageLayout struct {
	pdf        *fpdf.Fpdf
	raster     func(string, int) (image.Image, error)
	images     int
	coverage   *sfnt.Font
	fontBuffer sfnt.Buffer
	glyphs     map[glyphKey]lineItem
}
type glyphKey struct {
	ch rune
	px int
}

type lineItem struct {
	text                 string
	image                image.Image
	width, height, depth float64
}

func (l *pageLayout) ensureSpace(h float64) {
	if l.pdf.GetY()+h > bottomEdge {
		l.pdf.AddPage()
	}
}
func (l *pageLayout) drawImage(img image.Image, x, y, w, h float64) {
	var data bytes.Buffer
	if png.Encode(&data, img) != nil {
		return
	}
	l.images++
	name := fmt.Sprintf("question-media-%d", l.images)
	opt := fpdf.ImageOptions{ImageType: "PNG"}
	l.pdf.RegisterImageOptionsReader(name, opt, &data)
	l.pdf.ImageOptions(name, x, y, w, h, false, opt, 0, "")
}
func (l *pageLayout) figure(svg string) {
	if l.raster == nil {
		return
	}
	img, err := l.raster(svg, 1080)
	if err != nil || img == nil || img.Bounds().Empty() {
		return
	}
	b := img.Bounds()
	scale := min(116/float64(b.Dx()), 62/float64(b.Dy()))
	w, h := float64(b.Dx())*scale, float64(b.Dy())*scale
	l.ensureSpace(h + 10)
	y := l.pdf.GetY() + 4
	l.drawImage(img, leftEdge+(lineWidth-w)/2, y, w, h)
	l.pdf.SetXY(leftEdge, y+h+6)
}

func (l *pageLayout) text(prefix, value string, lineHeight float64) {
	l.textAt(prefix, value, lineHeight, 0)
}

// textAt lays out ordinary text, fallback glyphs and TeX on the same baseline.
// Content that fits the embedded Chinese font remains selectable.
func (l *pageLayout) textAt(prefix, value string, lineHeight, indent float64) {
	parts := mathtext.Split(value)
	if len(parts) == 0 {
		parts = []mathtext.Part{{Kind: mathtext.Plain, Source: value}}
	}
	capacity := lineWidth - indent
	var line []lineItem
	width := 0.0
	flush := func(center, force bool) {
		if len(line) == 0 && !force {
			return
		}
		above, below := lineHeight*.68, lineHeight*.32
		for _, item := range line {
			above = max(above, item.height+.55)
			below = max(below, item.depth+.55)
		}
		h := above + below
		l.ensureSpace(h)
		y := l.pdf.GetY()
		x := leftEdge + indent
		if center {
			x += (capacity - width) / 2
		}
		for i := 0; i < len(line); {
			item := line[i]
			if item.image != nil {
				l.drawImage(item.image, x, y+above-item.height, item.width, item.height+item.depth)
				x += item.width
				i++
				continue
			}
			var run strings.Builder
			start := x
			for i < len(line) && line[i].image == nil {
				run.WriteString(line[i].text)
				x += line[i].width
				i++
			}
			l.pdf.Text(start, y+above, run.String())
		}
		l.pdf.SetXY(leftEdge, y+h)
		line = line[:0]
		width = 0
	}
	appendItem := func(item lineItem) {
		if width+item.width > capacity && len(line) > 0 {
			flush(false, false)
		}
		line = append(line, item)
		width += item.width
	}
	pt, _ := l.pdf.GetFontSize()
	px := min(32, max(8, int(pt*96/72+.5)))
	addText := func(s string) {
		for _, ch := range s {
			switch ch {
			case '\r':
				continue
			case '\n':
				flush(false, true)
				continue
			case '\t':
				ch = ' '
			}
			if unicode.IsControl(ch) || ch == '\u200b' || ch == '\ufeff' {
				continue
			}
			if unicode.IsSpace(ch) {
				ch = ' '
			}
			if ch == ' ' && len(line) == 0 {
				continue
			}
			if !l.canPrint(ch) {
				if item, ok := l.symbol(ch, px); ok {
					appendItem(item)
					continue
				}
				// An uncommon character absent from both embedded fonts must remain
				// identifiable rather than showing a missing-glyph box.
				label := fmt.Sprintf("[U+%04X]", ch)
				appendItem(lineItem{text: label, width: l.pdf.GetStringWidth(label)})
				continue
			}
			v := string(ch)
			appendItem(lineItem{text: v, width: l.pdf.GetStringWidth(v)})
		}
	}

	addText(prefix)
	afterDisplay := false
	for _, part := range parts {
		if part.Kind == mathtext.Plain {
			text := part.Source
			if afterDisplay {
				text = strings.TrimPrefix(strings.TrimPrefix(text, "\r"), "\n")
			}
			addText(text)
			afterDisplay = false
			continue
		}
		display := part.Kind == mathtext.Display
		size := px
		if display {
			size = min(32, px+5)
		}
		vector, err := mathtext.Render(part.TeX, size, display)
		item, ok := l.vectorItem(vector, capacity)
		if err != nil || !ok {
			addText(part.Source)
			afterDisplay = false
			continue
		}
		if display {
			flush(false, false)
			l.ensureSpace(item.height + item.depth + 6)
			l.pdf.Ln(2)
		}
		appendItem(item)
		if display {
			flush(true, false)
			l.pdf.Ln(2)
		}
		afterDisplay = display
	}
	flush(false, false)
}

func (l *pageLayout) canPrint(ch rune) bool {
	if ch >= 0xffff {
		return false
	}
	glyph, err := l.coverage.GlyphIndex(&l.fontBuffer, ch)
	return err == nil && glyph != 0
}
func (l *pageLayout) symbol(ch rune, size int) (lineItem, bool) {
	if l.raster == nil {
		return lineItem{}, false
	}
	if l.glyphs == nil {
		l.glyphs = make(map[glyphKey]lineItem)
	}
	key := glyphKey{ch, size}
	if item, exists := l.glyphs[key]; exists {
		return item, item.image != nil
	}
	vector, ok := mathtext.LiteralGlyph(ch, size)
	if !ok {
		l.glyphs[key] = lineItem{}
		return lineItem{}, false
	}
	item, ok := l.vectorItem(vector, lineWidth)
	l.glyphs[key] = item
	return item, ok
}
func (l *pageLayout) vectorItem(vector mathtext.Image, capacity float64) (lineItem, bool) {
	if l.raster == nil || vector.Width <= 0 || vector.Ascent+vector.Descent <= 0 {
		return lineItem{}, false
	}
	dimension := min(2400, max(42, int(max(vector.Width, vector.Ascent+vector.Descent)*4)))
	img, err := l.raster(vector.SVG, dimension)
	if err != nil || img == nil || img.Bounds().Empty() {
		return lineItem{}, false
	}
	scale := min(mmPerDIP, capacity/float64(vector.Width), 55/float64(vector.Ascent+vector.Descent))
	return lineItem{image: img, width: float64(vector.Width) * scale, height: float64(vector.Ascent) * scale, depth: float64(vector.Descent) * scale}, true
}
