package diagram

import (
	"strings"
	"testing"
)

func TestLegacyCoordinateGraphAndSliderUpdate(t *testing.T) {
	cfg, err := Parse([]byte(`{"boundingbox":[-4,4,4,-4],"grid":true,"elements":[{"type":"point","attrs":[1,1],"opts":{"name":"A"}},{"type":"segment","attrs":["A",[3,2]]},{"type":"slider","attrs":[[-3,-3],[0,-3],[1,2,5]],"opts":{"name":"a"}},{"type":"functiongraph","attrs":["a*x^2-1/x"],"opts":{"name":"f"}},{"type":"glider","attrs":[1,0,"f"],"opts":{"name":"B"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	before, err := cfg.SVG()
	after, err2 := cfg.SVGWith(map[string]float64{"a": 4})
	if err != nil || err2 != nil || before == after || !strings.Contains(after, "a = 4") || !strings.Contains(after, "<path") || !strings.Contains(after, ">B<") {
		t.Fatal(err, err2)
	}
}

func TestUntrustedSVGAcceptsLocalShapesAndRejectsActiveContent(t *testing.T) {
	clean, err := SanitizeSVG(`<svg viewBox="0 0 100 100" onload="doSomething()"><path d="M0 0L40 40" stroke="url(https://example.invalid/p)"/><text x="5" y="20">角 A</text></svg>`)
	if err != nil || strings.Contains(clean, "onload") || strings.Contains(clean, "https:") || !strings.Contains(clean, "角 A") {
		t.Fatal(clean, err)
	}
	for _, markup := range []string{`<svg><script>bad</script></svg>`, `<svg><image href="remote.png"/></svg>`, `<!DOCTYPE svg><svg><rect/></svg>`} {
		if _, err := SanitizeSVG(markup); err == nil {
			t.Fatal("accepted unsupported element")
		}
	}
}

func TestFunctionMathAndInvalidSyntax(t *testing.T) {
	f, err := parseFunction("-2^2+sin(pi/2)+3*x", nil)
	if err != nil || f(2) != 3 {
		t.Fatal(err)
	}
	for _, expr := range []string{"fetch(x)", "x;", "2(", "x**", "x[0]"} {
		if _, err := parseFunction(expr, nil); err == nil {
			t.Fatal(expr)
		}
	}
}
