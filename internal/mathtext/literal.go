package mathtext

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-opentype/opentype"
	texmath "github.com/go-tex/math"
	"golang.org/x/text/unicode/norm"
)

var literalFont = sync.OnceValues(func() (*opentype.Font, error) { return opentype.Parse(texmath.DefaultFont()) })

// LiteralGlyph draws a Unicode symbol as written. It does not interpret the
// rune as TeX, so subscript and superscript glyphs keep their original height.
func LiteralGlyph(ch rune, size int) (Image, bool) {
	font, err := literalFont()
	if err != nil || size < 8 || size > 32 {
		return Image{}, false
	}
	gid, ok := font.GlyphIndex(ch)
	glyphSize, shift := size, 0.0
	if !ok || gid == 0 {
		plain := []rune(norm.NFKC.String(string(ch)))
		if len(plain) != 1 || plain[0] == ch {
			return Image{}, false
		}
		switch {
		case strings.ContainsRune("₀₁₂₃₄₅₆₇₈₉₊₋₌₍₎ₐₑₒₓₔₕₖₗₘₙₚₛₜᵢᵣᵤᵥᵦᵧᵨᵩᵪ", ch):
			shift = float64(size) * .23
		case strings.ContainsRune("⁰¹²³⁴⁵⁶⁷⁸⁹⁺⁻⁼⁽⁾ⁿᵃᵇᶜᵈᵉᶠᵍʰⁱʲᵏˡᵐᵒᵖʳˢᵗᵘᵛʷˣʸᶻ", ch):
			shift = -float64(size) * .38
		default:
			return Image{}, false
		}
		gid, ok = font.GlyphIndex(plain[0])
		if !ok || gid == 0 {
			return Image{}, false
		}
		glyphSize = max(6, size*2/3)
	}
	face := font.NewFace(glyphSize)
	path, ok := face.GlyphSVGPath(gid)
	if !ok || path == "" {
		return Image{}, false
	}
	left, bottom, right, top, ok := face.GlyphControlBox(gid)
	if !ok {
		return Image{}, false
	}
	scale := float64(glyphSize) / float64(font.UnitsPerEm())
	left, bottom, right, top = left*scale, bottom*scale, right*scale, top*scale
	left = min(0, left) - .35
	width := max(float64(face.AdvanceIndex(gid)), right) + .35 - left
	ascent, descent := max(0, top-shift)+.35, max(0, shift-bottom)+.35
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.3f" height="%.3f" viewBox="%.3f %.3f %.3f %.3f"><path fill="currentColor" transform="translate(0 %.3f)" d="%s"/></svg>`, width, ascent+descent, left, -ascent, width, ascent+descent, shift, path)
	return Image{SVG: svg, Width: float32(width), Ascent: float32(ascent), Descent: float32(descent)}, true
}
