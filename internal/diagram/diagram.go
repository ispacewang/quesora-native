// Package diagram implements the declarative, local subset of Quesora question figures.
package diagram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const MaxBytes = 64 << 10

type Point struct{ X, Y float64 }
type Element struct {
	Type  string            `json:"type"`
	Attrs []json.RawMessage `json:"attrs"`
	Opts  struct {
		Name  string  `json:"name"`
		ID    string  `json:"id"`
		Color string  `json:"strokeColor"`
		Fill  string  `json:"fillColor"`
		Width float64 `json:"strokeWidth"`
	} `json:"opts"`
}
type Config struct {
	Bounds   []float64 `json:"boundingbox"`
	Axis     *bool     `json:"axis"`
	Grid     bool      `json:"grid"`
	Elements []Element `json:"elements"`
}

func Parse(raw []byte) (Config, error) {
	var cfg Config
	if len(raw) > MaxBytes || json.Unmarshal(raw, &cfg) != nil || len(cfg.Elements) == 0 || len(cfg.Elements) > 32 {
		return cfg, errors.New("配图配置无效或超过限制")
	}
	if len(cfg.Bounds) == 0 {
		cfg.Bounds = []float64{-5, 5, 5, -5}
	}
	b := cfg.Bounds
	if len(b) != 4 || b[2]-b[0] < .001 || b[1]-b[3] < .001 {
		return cfg, errors.New("坐标范围无效")
	}
	for _, v := range b {
		if !finite(v) || math.Abs(v) > 1e6 {
			return cfg, errors.New("坐标范围超出限制")
		}
	}
	return cfg, nil
}

func finite(v float64) bool          { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func str(raw json.RawMessage) string { var s string; _ = json.Unmarshal(raw, &s); return s }
func num(raw json.RawMessage) (float64, bool) {
	var v float64
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, false
	}
	err := json.Unmarshal(raw, &v)
	return v, err == nil && finite(v) && math.Abs(v) <= 1e6
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3})?$`)

func colorValue(input, fallback string) string {
	if hexColor.MatchString(input) {
		return input
	}
	return fallback
}
func xy(a []json.RawMessage) (Point, bool) {
	if len(a) < 2 {
		return Point{}, false
	}
	x, ok := num(a[0])
	y, ok2 := num(a[1])
	return Point{x, y}, ok && ok2
}
func resolve(raw json.RawMessage, points map[string]Point) (Point, bool) {
	if name := str(raw); name != "" {
		p, ok := points[name]
		return p, ok
	}
	var a []json.RawMessage
	if json.Unmarshal(raw, &a) != nil {
		return Point{}, false
	}
	return xy(a)
}
func escape(s string) string {
	var out bytes.Buffer
	for _, ch := range s {
		switch ch {
		case '&':
			out.WriteString("&amp;")
		case '<':
			out.WriteString("&lt;")
		case '>':
			out.WriteString("&gt;")
		case '"':
			out.WriteString("&quot;")
		default:
			if ch >= 32 {
				out.WriteRune(ch)
			}
		}
	}
	return out.String()
}

type plotter struct {
	s             strings.Builder
	b             []float64
	scale, x0, y0 float64
	points        map[string]Point
	functions     map[string]function
	variables     map[string]float64
}

// SVG draws the supported graph elements in the original coordinate system.
// Limits on element counts and sampling keep imported figures inexpensive.
func (cfg Config) SVG() (string, error) { return cfg.SVGWith(nil) }

// SVGWith overrides slider values for the interactive viewport. Exports use SVG.
func (cfg Config) SVGWith(values map[string]float64) (string, error) {
	b := cfg.Bounds
	if len(b) != 4 {
		return "", errors.New("坐标范围无效")
	}
	p := plotter{b: b, points: map[string]Point{}, functions: map[string]function{}, variables: map[string]float64{}}
	for _, param := range cfg.Parameters() {
		v, ok := values[param.Name]
		if !ok || !finite(v) {
			v = param.Value
		}
		p.variables[param.Name] = min(param.Max, max(param.Min, v))
	}
	for _, e := range cfg.Elements {
		if e.Type != "functiongraph" || len(e.Attrs) == 0 {
			continue
		}
		if f, err := parseFunction(str(e.Attrs[0]), p.variables); err == nil {
			if e.Opts.Name != "" {
				p.functions[e.Opts.Name] = f
			}
			if e.Opts.ID != "" {
				p.functions[e.Opts.ID] = f
			}
		}
	}
	p.scale = min(600/(b[2]-b[0]), 340/(b[1]-b[3]))
	p.x0 = (640 - (b[2]-b[0])*p.scale) / 2
	p.y0 = (380 - (b[1]-b[3])*p.scale) / 2
	p.s.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="380" viewBox="0 0 640 380"><defs><clipPath id="plot"><rect width="640" height="380"/></clipPath></defs><g clip-path="url(#plot)">`)
	p.axes(cfg.Grid, cfg.Axis == nil || *cfg.Axis)
	for _, e := range cfg.Elements {
		if e.Type != "point" && e.Type != "glider" {
			continue
		}
		if pt, ok := p.point(e); ok {
			if e.Opts.Name != "" {
				p.points[e.Opts.Name] = pt
			}
			if e.Opts.ID != "" {
				p.points[e.Opts.ID] = pt
			}
		}
	}
	shown := 0
	for _, e := range cfg.Elements {
		if p.draw(e) {
			shown++
		}
	}
	p.s.WriteString("</g></svg>")
	if shown == 0 {
		return "", errors.New("配图没有可显示的元素")
	}
	return p.s.String(), nil
}

func (p *plotter) pix(a Point) Point {
	return Point{p.x0 + (a.X-p.b[0])*p.scale, p.y0 + (p.b[1]-a.Y)*p.scale}
}
func (p *plotter) line(a, b Point, color string, stroke float64) {
	a, b = p.pix(a), p.pix(b)
	fmt.Fprintf(&p.s, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.2f"/>`, a.X, a.Y, b.X, b.Y, color, stroke)
}
func (p *plotter) label(a Point, label string) {
	if label == "" || len(label) > 240 {
		return
	}
	a = p.pix(a)
	fmt.Fprintf(&p.s, `<text x="%.2f" y="%.2f" font-size="14" fill="currentColor">%s</text>`, a.X+7, a.Y-6, escape(label))
}
func (p *plotter) axes(grid, axis bool) {
	b := p.b
	if grid {
		step := math.Pow(10, math.Floor(math.Log10(max(b[2]-b[0], b[1]-b[3])/6)))
		for x := math.Ceil(b[0]/step) * step; x <= b[2]; x += step {
			p.line(Point{x, b[1]}, Point{x, b[3]}, "#aab5c4", .5)
		}
		for y := math.Ceil(b[3]/step) * step; y <= b[1]; y += step {
			p.line(Point{b[0], y}, Point{b[2], y}, "#aab5c4", .5)
		}
	}
	if axis {
		if b[3] <= 0 && b[1] >= 0 {
			p.line(Point{b[0], 0}, Point{b[2], 0}, "currentColor", 1)
			p.label(Point{b[2], 0}, "x")
		}
		if b[0] <= 0 && b[2] >= 0 {
			p.line(Point{0, b[3]}, Point{0, b[1]}, "currentColor", 1)
			p.label(Point{0, b[1]}, "y")
		}
	}
}
func (p *plotter) draw(e Element) bool {
	a := e.Attrs
	stroke := colorValue(e.Opts.Color, "#4b83c8")
	fill := colorValue(e.Opts.Fill, "none")
	w := e.Opts.Width
	if w <= 0 {
		w = 2
	}
	w = min(w, 6)
	switch e.Type {
	case "functiongraph":
		if len(a) == 0 {
			return false
		}
		f, err := parseFunction(str(a[0]), p.variables)
		if err != nil {
			return false
		}
		from, to := p.b[0], p.b[2]
		if len(a) == 3 {
			if x, ok := num(a[1]); ok {
				from = max(from, x)
			}
			if x, ok := num(a[2]); ok {
				to = min(to, x)
			}
		}
		return p.curve(f, from, to, stroke, w)
	case "point", "text", "glider":
		pt, ok := p.point(e)
		if !ok {
			return false
		}
		if e.Type == "text" {
			if len(a) < 3 || str(a[2]) == "" {
				return false
			}
			p.label(pt, str(a[2]))
			return true
		}
		v := p.pix(pt)
		fmt.Fprintf(&p.s, `<circle cx="%.2f" cy="%.2f" r="4" fill="%s"/>`, v.X, v.Y, stroke)
		p.label(pt, e.Opts.Name)
	case "line", "segment", "arrow":
		if len(a) < 2 {
			return false
		}
		u, ok := resolve(a[0], p.points)
		v, ok2 := resolve(a[1], p.points)
		if !ok || !ok2 || u == v {
			return false
		}
		if e.Type == "line" {
			dx, dy := v.X-u.X, v.Y-u.Y
			t0, t1 := math.Inf(-1), math.Inf(1)
			for _, d := range [][4]float64{{u.X, dx, p.b[0], p.b[2]}, {u.Y, dy, p.b[3], p.b[1]}} {
				if d[1] == 0 {
					if d[0] < d[2] || d[0] > d[3] {
						return false
					}
					continue
				}
				l, h := (d[2]-d[0])/d[1], (d[3]-d[0])/d[1]
				t0, t1 = max(t0, min(l, h)), min(t1, max(l, h))
			}
			if t0 > t1 {
				return false
			}
			u, v = Point{u.X + t0*dx, u.Y + t0*dy}, Point{u.X + t1*dx, u.Y + t1*dy}
		}
		p.line(u, v, stroke, w)
		if e.Type == "arrow" {
			end, start := p.pix(v), p.pix(u)
			angle := math.Atan2(end.Y-start.Y, end.X-start.X)
			fmt.Fprintf(&p.s, `<path d="M%.2f %.2fL%.2f %.2fL%.2f %.2fZ" fill="%s"/>`, end.X, end.Y, end.X-11*math.Cos(angle-.45), end.Y-11*math.Sin(angle-.45), end.X-11*math.Cos(angle+.45), end.Y-11*math.Sin(angle+.45), stroke)
		}
	case "circle":
		if len(a) < 2 {
			return false
		}
		u, ok := resolve(a[0], p.points)
		if !ok {
			return false
		}
		r, ok := num(a[1])
		if !ok {
			v, found := resolve(a[1], p.points)
			if !found {
				return false
			}
			r = math.Hypot(v.X-u.X, v.Y-u.Y)
		}
		if r <= 0 {
			return false
		}
		u = p.pix(u)
		fmt.Fprintf(&p.s, `<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s" fill-opacity=".18" stroke="%s" stroke-width="%.2f"/>`, u.X, u.Y, r*p.scale, fill, stroke, w)
	case "angle", "sector":
		if len(a) < 3 {
			return false
		}
		first, ok1 := resolve(a[0], p.points)
		center, ok2 := resolve(a[1], p.points)
		last, ok3 := resolve(a[2], p.points)
		if e.Type == "sector" {
			center, ok2 = resolve(a[0], p.points)
			first, ok1 = resolve(a[1], p.points)
		}
		if !ok1 || !ok2 || !ok3 {
			return false
		}
		center, first, last = p.pix(center), p.pix(first), p.pix(last)
		radius := math.Hypot(first.X-center.X, first.Y-center.Y)
		if e.Type == "angle" {
			radius = min(radius, 27)
		}
		if radius < 1 {
			return false
		}
		start, end := math.Atan2(first.Y-center.Y, first.X-center.X), math.Atan2(last.Y-center.Y, last.X-center.X)
		delta := math.Remainder(end-start, 2*math.Pi)
		sweep := 0
		if delta >= 0 {
			sweep = 1
		}
		x1, y1 := center.X+radius*math.Cos(start), center.Y+radius*math.Sin(start)
		x2, y2 := center.X+radius*math.Cos(end), center.Y+radius*math.Sin(end)
		d := fmt.Sprintf("M%.2f %.2fA%.2f %.2f 0 0 %d %.2f %.2f", x1, y1, radius, radius, sweep, x2, y2)
		if e.Type == "sector" {
			d += fmt.Sprintf("L%.2f %.2fZ", center.X, center.Y)
		}
		fmt.Fprintf(&p.s, `<path d="%s" fill="%s" fill-opacity=".2" stroke="%s" stroke-width="%.2f"/>`, d, fill, stroke, w)
	case "tangent":
		if len(a) < 2 {
			return false
		}
		pt, ok := resolve(a[0], p.points)
		f := p.functions[str(a[1])]
		if !ok || f == nil {
			return false
		}
		delta := (p.b[2] - p.b[0]) / 10000
		slope := (f(pt.X+delta) - f(pt.X-delta)) / (2 * delta)
		if !finite(slope) {
			return false
		}
		return p.curve(func(x float64) float64 { return pt.Y + slope*(x-pt.X) }, p.b[0], p.b[2], stroke, w)
	case "integral":
		if len(a) < 2 {
			return false
		}
		var limits []json.RawMessage
		if json.Unmarshal(a[0], &limits) != nil || len(limits) != 2 {
			return false
		}
		x0, ok := num(limits[0])
		x1, ok2 := num(limits[1])
		f := p.functions[str(a[1])]
		if !ok || !ok2 || f == nil {
			return false
		}
		x0, x1 = max(p.b[0], min(x0, x1)), min(p.b[2], max(x0, x1))
		if x1 <= x0 {
			return false
		}
		start := p.pix(Point{x0, 0})
		end := p.pix(Point{x1, 0})
		var d strings.Builder
		fmt.Fprintf(&d, "M%.2f %.2f", start.X, start.Y)
		for i := 0; i <= 160; i++ {
			x := x0 + (x1-x0)*float64(i)/160
			pt := p.pix(Point{x, f(x)})
			if !finite(pt.Y) {
				return false
			}
			fmt.Fprintf(&d, "L%.2f %.2f", pt.X, min(1140, max(-760, pt.Y)))
		}
		fmt.Fprintf(&d, "L%.2f %.2fZ", end.X, end.Y)
		fmt.Fprintf(&p.s, `<path d="%s" fill="%s" fill-opacity=".2" stroke="%s" stroke-width="%.2f"/>`, d.String(), colorValue(e.Opts.Fill, stroke), stroke, w)
	case "slider":
		if value, ok := p.variables[e.Opts.Name]; ok && len(a) >= 2 {
			u, valid := resolve(a[0], p.points)
			v, valid2 := resolve(a[1], p.points)
			if valid && valid2 {
				p.line(u, v, stroke, w)
				p.label(u, fmt.Sprintf("%s = %.3g", e.Opts.Name, value))
				return true
			}
		}
		return false
	case "polygon":
		if len(a) < 3 || len(a) > 20 {
			return false
		}
		var pts []string
		for _, vertex := range a {
			v, ok := resolve(vertex, p.points)
			if !ok {
				return false
			}
			v = p.pix(v)
			pts = append(pts, fmt.Sprintf("%.2f,%.2f", v.X, v.Y))
		}
		fmt.Fprintf(&p.s, `<polygon points="%s" fill="%s" fill-opacity=".2" stroke="%s" stroke-width="%.2f"/>`, strings.Join(pts, " "), fill, stroke, w)
	default:
		return false
	}
	return true
}

func (p *plotter) curve(f function, from, to float64, color string, stroke float64) bool {
	if from >= to {
		return false
	}
	var d strings.Builder
	prev := math.NaN()
	for i := 0; i <= 360; i++ {
		x := from + (to-from)*float64(i)/360
		pt := p.pix(Point{x, f(x)})
		if !finite(pt.Y) || pt.Y < -760 || pt.Y > 1140 {
			prev = math.NaN()
			continue
		}
		if !finite(prev) || math.Abs(pt.Y-prev) > 250 {
			d.WriteByte('M')
		} else {
			d.WriteByte('L')
		}
		d.WriteString(strconv.FormatFloat(pt.X, 'f', 2, 64))
		d.WriteByte(' ')
		d.WriteString(strconv.FormatFloat(pt.Y, 'f', 2, 64))
		prev = pt.Y
	}
	if d.Len() == 0 {
		return false
	}
	fmt.Fprintf(&p.s, `<path d="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round"/>`, d.String(), color, stroke)
	return true
}

// Parameter is a native numeric control for a legacy JSXGraph slider.
type Parameter struct {
	Name            string
	Min, Value, Max float64
}

func (cfg Config) Parameters() []Parameter {
	var params []Parameter
	for _, e := range cfg.Elements {
		name := e.Opts.Name
		if e.Type != "slider" || len(e.Attrs) < 3 || !variableName.MatchString(name) || name == "x" || name == "e" || name == "pi" {
			continue
		}
		var rangeValues []json.RawMessage
		if json.Unmarshal(e.Attrs[2], &rangeValues) != nil || len(rangeValues) != 3 {
			continue
		}
		lo, ok := num(rangeValues[0])
		v, ok2 := num(rangeValues[1])
		hi, ok3 := num(rangeValues[2])
		if ok && ok2 && ok3 && lo < hi && v >= lo && v <= hi {
			params = append(params, Parameter{name, lo, v, hi})
		}
		if len(params) == 4 {
			break
		}
	}
	return params
}

func (p *plotter) point(e Element) (Point, bool) {
	pt, ok := xy(e.Attrs)
	if !ok || e.Type != "glider" {
		return pt, ok
	}
	if len(e.Attrs) < 3 {
		return Point{}, false
	}
	if f := p.functions[str(e.Attrs[2])]; f != nil {
		pt.Y = f(pt.X)
		return pt, finite(pt.Y)
	}
	return pt, true
}
