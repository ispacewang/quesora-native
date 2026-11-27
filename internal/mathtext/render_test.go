package mathtext

import (
	"strings"
	"testing"
)

func TestRenderHasVectorPathsAndBaseline(t *testing.T) {
	for _, tex := range []string{`x^2 + \sqrt{y}`, `\frac{1}{x}`, `\sum_{i=1}^{n}i`, `\begin{pmatrix}1&2\\3&4\end{pmatrix}`} {
		icon, err := Render(tex, 16, true)
		if err != nil || icon.Width <= 0 || icon.Ascent <= 0 || !strings.Contains(icon.SVG, "<path") {
			t.Fatalf("%q: width=%v ascent=%v err=%v", tex, icon.Width, icon.Ascent, err)
		}
	}
	inline, err := Render(`\frac{1}{x}`, 16, false)
	if err != nil || inline.Descent <= 0 {
		t.Fatalf("inline denominator must extend below the baseline: %v", err)
	}
}

func TestRenderRejectsUnsupportedOrExcessiveInput(t *testing.T) {
	for _, tex := range []string{`\input{question.txt}`, `\def\a{a}\a`, strings.Repeat("{", 25) + "x" + strings.Repeat("}", 25), strings.Repeat("x", 1025)} {
		if _, err := Render(tex, 16, false); err == nil {
			t.Fatalf("accepted unsupported or excessive formula %q", tex)
		}
	}
}
