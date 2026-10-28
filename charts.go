package main

import (
	"github.com/egoist/mygo/ui"
	"math"
)

func (a *app) functionGraph(c *ui.Context) {
	t := c.Theme()
	ui.Box(c.Key("function-graph")).FillWidth().Height(118).Background(t.Surface).Label("函数图像：y 等于 x 减一的平方，顶点为 1，0").Draw(func(p *ui.Painter, r ui.Rect) {
		left, top := r.X+32, r.Y+12
		w, h := r.W-64, r.H-26
		px := func(x float32) float32 { return left + (x+1)*w/4 }
		py := func(y float32) float32 { return top + h - y*h/5 }
		for i := 0; i <= 4; i++ {
			x := left + float32(i)*w/4
			strokeLine(p, x, top, x, top+h, 1, t.Border)
		}
		for i := 0; i <= 2; i++ {
			y := top + float32(i)*h/2
			strokeLine(p, left, y, left+w, y, 1, t.Border)
		}
		strokeLine(p, left, py(0), left+w, py(0), 1, t.TextMuted)
		strokeLine(p, px(0), top, px(0), top+h, 1, t.TextMuted)
		for i := 0; i < 100; i++ {
			x := -1 + float32(i)*4/100
			nx := x + 4.0/100
			strokeLine(p, px(x), py((x-1)*(x-1)), px(nx), py((nx-1)*(nx-1)), 2, t.Accent)
		}
		p.Text(left+w-132, top+4, "y = (x − 1)²", 13, t.Accent)
		p.Fill(ui.Rect{X: px(1) - 3, Y: py(0) - 3, W: 6, H: 6}, t.Accent, 3)
	})
}
func (a *app) radar(c *ui.Context) {
	t := c.Theme()
	var kinds []string
	var values []float32
	for _, kind := range []string{"单选题", "多选题", "判断题", "简答题", "填空题"} {
		g, b := a.typeCounts(kind)
		if g+b > 0 {
			kinds = append(kinds, kind)
			values = append(values, float32(g)/float32(g+b))
		}
	}
	if len(kinds) == 0 {
		return
	}
	ui.Box(c.Key("radar")).FillWidth().Height(160).Label("题型正确率雷达图").Draw(func(p *ui.Painter, r ui.Rect) {
		n := len(kinds)
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		radius := float32(53)
		point := func(i int, scale float32) (float32, float32) {
			angle := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
			return cx + float32(math.Cos(angle))*radius*scale, cy + float32(math.Sin(angle))*radius*scale
		}
		grid := t.Text.Alpha(.06)
		if a.dark {
			grid = t.Text.Alpha(.08)
		}
		for layer := 1; layer <= 4; layer++ {
			for i := range kinds {
				x, y := point(i, float32(layer)/4)
				nx, ny := point((i+1)%n, float32(layer)/4)
				strokeLine(p, x, y, nx, ny, 1, grid)
			}
		}
		path := &ui.Path{}
		for i := range kinds {
			x, y := point(i, 1)
			strokeLine(p, cx, cy, x, y, 1, grid)
			dx, dy := point(i, values[i])
			if i == 0 {
				path.MoveTo(dx, dy)
			} else {
				path.LineTo(dx, dy)
			}
		}
		path.Close()
		p.FillPath(path, t.Accent.Alpha(.15))
		p.StrokePath(path, 1.5, t.Accent.Alpha(.7))
		for i, kind := range kinds {
			x, y := point(i, values[i])
			p.Fill(ui.Rect{X: x - 3, Y: y - 3, W: 6, H: 6}, t.Accent, 3)
			lx, ly := point(i, 1.3)
			p.Text(lx-18, ly-6, kind, 9, t.TextMuted)
		}
	})
}

// Painter.Line is axis-aligned in MyGo 0.5.5; polylines preserve diagonal geometry.
func strokeLine(p *ui.Painter, x0, y0, x1, y1, width float32, color ui.Color) {
	p.StrokePolyline([][2]float32{{x0, y0}, {x1, y1}}, width, color)
}
