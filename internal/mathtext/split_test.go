package mathtext

import "testing"

func TestSplitPreservesMixedQuestionAndLiteralCurrency(t *testing.T) {
	source := "若 $x^2=4$，求根。\n$$\\frac{1}{2}$$ 后记"
	parts := Split(source)
	if len(parts) != 5 || parts[1].Kind != Inline || parts[1].TeX != "x^2=4" || parts[3].Kind != Display || parts[3].TeX != "\\frac{1}{2}" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
	var rebuilt string
	for _, p := range parts {
		rebuilt += p.Source
	}
	if rebuilt != source {
		t.Fatal("formula splitter lost surrounding text")
	}
	for _, plain := range []string{"价格从 $5 到 $10", "file_name", "未闭合 $x^2", "金额 \\$5", "定义 \\sqrt{x} 的含义"} {
		if got := Split(plain); len(got) != 0 {
			t.Fatalf("plain text %q interpreted as formula: %#v", plain, got)
		}
	}
}

func TestSplitSupportsBareMathAndAlternativeDelimiters(t *testing.T) {
	for _, test := range []struct {
		input, tex string
		kind       Kind
	}{
		{"x^2+2x+1", "x^2+2x+1", Inline},
		{"\\(x+1\\)", "x+1", Inline},
		{"\\[\\sum_{i=1}^n i\\]", "\\sum_{i=1}^n i", Display},
	} {
		got := Split(test.input)
		if len(got) != 1 || got[0].Kind != test.kind || got[0].TeX != test.tex {
			t.Fatalf("%q: %#v", test.input, got)
		}
	}
}

func TestMixedUnicodeFormulaKeepsFullWidthPunctuation(t *testing.T) {
	if parts := Split("x₁≤√2，x₂=π"); len(parts) != 0 {
		t.Fatalf("CJK punctuation should stay in ordinary text: %#v", parts)
	}
}
