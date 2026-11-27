package mathtext

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"sync"

	texmath "github.com/go-tex/math"
)

const maxFormulaBytes = 1024

// Image holds a self-contained path-only SVG and its dimensions relative to the baseline.
type Image struct {
	SVG                    string
	Width, Ascent, Descent float32
}

var renderer = sync.OnceValues(func() (*texmath.Renderer, error) {
	return texmath.New(texmath.DefaultFont())
})

var invalidFormula = errors.New("unsupported or oversized math expression")
var controlSequence = regexp.MustCompile(`\\[a-zA-Z]+|\\.`)

// Only math-mode commands may reach the renderer. In particular, imported text
// cannot define macros, switch tokenization rules, or request external resources.
var formulaCommands = strings.Fields(`
frac dfrac tfrac binom dbinom tbinom sqrt sum prod coprod int iint iiint oint lim limsup liminf
min max sin cos tan sec csc cot arcsin arccos arctan sinh cosh tanh log ln exp det gcd
left right middle big Big bigg Bigg bigl bigr Bigl Bigr biggl biggr Biggl Biggr
langle rangle lfloor rfloor lceil rceil vert Vert lvert rvert lVert rVert
begin end text textrm operatorname mathrm mathit mathbf mathcal mathbb mathsf mathtt mathfrak boldsymbol
hat widehat bar overline underline vec tilde widetilde dot ddot check breve acute grave
displaystyle textstyle scriptstyle scriptscriptstyle limits nolimits overbrace underbrace overset underset
quad qquad thinspace medspace thickspace enspace negthinspace pmod bmod mod
alpha beta gamma delta epsilon varepsilon zeta eta theta vartheta iota kappa lambda mu nu xi pi varpi rho varrho sigma varsigma tau upsilon phi varphi chi psi omega
Gamma Delta Theta Lambda Xi Pi Sigma Upsilon Phi Psi Omega
pm mp times div cdot ast star circ bullet oplus otimes cup cap setminus
le leq ge geq ne neq approx equiv sim simeq cong propto ll gg subset supset subseteq supseteq in ni notin
emptyset varnothing forall exists neg land lor wedge vee perp parallel mid angle triangle
rightarrow leftarrow leftrightarrow Rightarrow Leftarrow Leftrightarrow to mapsto longrightarrow longleftarrow
infty partial nabla ell hbar imath jmath Re Im aleph
ldots cdots vdots ddots dots colon degree
, : ; ! { } | % _ $ # & \\
`)
var allowedCommand = func() map[string]bool {
	result := make(map[string]bool, len(formulaCommands))
	for _, name := range formulaCommands {
		result[name] = true
	}
	result[" "] = true
	result["\\"] = true
	return result
}()

// Render lays out common TeX math on demand. Limits cap work for question banks and live previews.
func Render(source string, size int, display bool) (Image, error) {
	if len(source) == 0 || len(source) > maxFormulaBytes || size < 8 || size > 32 {
		return Image{}, invalidFormula
	}
	for _, command := range controlSequence.FindAllString(source, -1) {
		if !allowedCommand[command[1:]] {
			return Image{}, invalidFormula
		}
	}
	// Escaped braces are literal delimiters and do not count as nesting.
	depth := 0
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\\':
			if i+1 < len(source) {
				i++
			}
		case '{':
			depth++
			if depth > 24 {
				return Image{}, invalidFormula
			}
		case '}':
			depth--
			if depth < 0 {
				return Image{}, invalidFormula
			}
		}
	}
	if depth != 0 {
		return Image{}, invalidFormula
	}
	r, err := renderer()
	if err != nil {
		return Image{}, err
	}
	render := r.RenderSVGMetrics
	if display {
		render = r.RenderDisplaySVGMetrics
	}
	svg, m, err := render(source, size)
	if err != nil {
		return Image{}, err
	}
	total := m.Height + m.Depth
	if !positive(m.Width) || !positive(total) || m.Height < 0 || m.Depth < 0 || m.Width > 2048 || total > 320 || len(svg) > 256<<10 {
		return Image{}, invalidFormula
	}
	return Image{SVG: svg, Width: float32(m.Width), Ascent: float32(m.Height), Descent: float32(m.Depth)}, nil
}

func positive(v float64) bool { return v > 0 && !math.IsInf(v, 0) }
