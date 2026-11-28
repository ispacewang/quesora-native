package mathtext

import "testing"

func TestLiteralMathGlyphsRetainTheirShape(t *testing.T) {
	for _, ch := range "π√≤≥∞₀₁ₙ⅓𝑥ℝ∫∠" {
		glyph, ok := LiteralGlyph(ch, 16)
		if !ok || glyph.Width < 1 || glyph.Width > 40 || glyph.Ascent+glyph.Descent > 45 {
			t.Fatalf("%c: %#v", ch, glyph)
		}
	}
}
