package exportpdf

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/ispacewang/quesora-native/internal/mathtext"
)

const leftEdge = 18.0
const lineWidth = 174.0
const bottomEdge = 279.0
const mmPerDIP = 25.4 / 96

type pageLayout struct {
	pdf    *fpdf.Fpdf
	raster func(string, int) (image.Image, error)
	images int
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
	scale := min(120/float64(b.Dx()), 67/float64(b.Dy()))
	w, h := float64(b.Dx())*scale, float64(b.Dy())*scale
	l.ensureSpace(h + 6)
	y := l.pdf.GetY() + 2
	l.drawImage(img, leftEdge+(lineWidth-w)/2, y, w, h)
	l.pdf.SetXY(leftEdge, y+h+4)
}

// text lays out TeX images as baseline-aligned objects between selectable
// Chinese text runs. Explicit display math gets its own centered line.
func (l *pageLayout) text(prefix, value string, lineHeight float64) {
	parts := mathtext.Split(value)
	if len(parts) == 0 || l.raster == nil {
		l.pdf.MultiCell(0, lineHeight, prefix+value, "", "L", false)
		return
	}
	var line []lineItem
	width := 0.0
	flush := func(center, force bool) {
		if len(line) == 0 && !force {
			return
		}
		above, below := lineHeight*.7, lineHeight*.3
		for _, item := range line {
			above = max(above, item.height+.5)
			below = max(below, item.depth+.5)
		}
		h := above + below
		l.ensureSpace(h)
		y := l.pdf.GetY()
		x := leftEdge + 1
		if center {
			x = leftEdge + (lineWidth-width)/2
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
		line = nil
		width = 0
	}
	addText := func(s string) {
		for _, ch := range s {
			if ch == '\r' {
				continue
			}
			if ch == '\n' {
				flush(false, true)
				continue
			}
			v := string(ch)
			w := l.pdf.GetStringWidth(v)
			if width+w > lineWidth-2 && len(line) > 0 {
				flush(false, false)
			}
			line = append(line, lineItem{text: v, width: w})
			width += w
		}
	}
	addText(prefix)
	afterDisplay := false
	for _, p := range parts {
		if p.Kind == mathtext.Plain {
			v := p.Source
			if afterDisplay {
				v = strings.TrimPrefix(strings.TrimPrefix(v, "\r"), "\n")
			}
			addText(v)
			afterDisplay = false
			continue
		}
		display := p.Kind == mathtext.Display
		size := 15
		if display {
			size = 18
		}
		vector, err := mathtext.Render(p.TeX, size, display)
		if err != nil {
			addText(p.Source)
			afterDisplay = false
			continue
		}
		limit := max(32, int(max(vector.Width, vector.Ascent+vector.Descent)*4))
		img, err := l.raster(vector.SVG, min(2400, limit))
		if err != nil || img == nil {
			addText(p.Source)
			afterDisplay = false
			continue
		}
		scale := min(mmPerDIP, (lineWidth-2)/float64(vector.Width), 55/float64(vector.Ascent+vector.Descent))
		w := float64(vector.Width) * scale
		if display || width+w > lineWidth-2 {
			flush(false, false)
		}
		line = append(line, lineItem{image: img, width: w, height: float64(vector.Ascent) * scale, depth: float64(vector.Descent) * scale})
		width += w
		if display {
			flush(true, false)
		}
		afterDisplay = display
	}
	flush(false, false)
}
