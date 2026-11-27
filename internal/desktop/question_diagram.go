package desktop

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/ispacewang/quesora-native/internal/diagram"
	"github.com/ispacewang/quesora-native/internal/domain"
)

type figureView struct {
	svg             *ui.SVG
	paperBackground bool
	zoom            float32
	offset          ui.Point
	config          diagram.Config
	sliders         []sliderValue
}

type sliderValue struct {
	param diagram.Parameter
	value float64
}

func loadFigure(q domain.Question) figureView {
	raw, err := diagram.FromMeta(q.Meta)
	if err != nil || raw == "" {
		return figureView{}
	}
	svg, err := ui.ParseSVG([]byte(raw))
	if err != nil {
		return figureView{}
	}
	w, h := svg.Size()
	if !(w > 0 && h > 0) || w > 4096 || h > 4096 {
		return figureView{}
	}
	state := figureView{svg: svg, zoom: 1}
	if original, ok := q.Meta["diagramSvg"].(string); ok && original != "" {
		_, err := diagram.SanitizeSVG(original)
		state.paperBackground = err == nil
	}
	if q.Meta["diagramSvg"] == nil {
		if data, e := json.Marshal(q.Meta["diagram"]); e == nil {
			state.config, _ = diagram.Parse(data)
			for _, pa := range state.config.Parameters() {
				state.sliders = append(state.sliders, sliderValue{pa, pa.Value})
			}
		}
	}
	return state
}

// questionDiagram keeps zoom and pan local to a question card. Exported figures
// always use the original framing, regardless of the current viewport.
func questionDiagram(c *ui.Context, q domain.Question, interactive bool) {
	if q.Meta["diagram"] == nil && q.Meta["diagramSvg"] == nil {
		return
	}
	t := c.Theme()
	ui.Column(c.Key("figure-"+q.ID)).FillWidth().Margin(0, 0, 16, 0).Gap(6).Children(func() {
		board := ui.Box(c.Key("figure-board")).FillWidth().MaxWidth(560).AlignSelf(ui.Center).Clip()
		state := ui.Local(board, "camera", func() figureView { return loadFigure(q) })
		if state.svg == nil {
			board.Children(func() { ui.Text(c, "无法显示此题配图").TextColor(t.TextMuted).FontSize(11) })
			return
		}
		w, h := state.svg.Size()
		background := t.Surface
		if state.paperBackground {
			background = ui.Hex("#F8FAFC")
		}
		board.Height(min(300, max(110, 440*h/w))).Radius(10).Background(background).Border(1, t.Border.Alpha(.6)).
			Label("题目配图").Children(func() {
			canvas := ui.Box(c.Key("figure-canvas")).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
				zoom := state.zoom
				x := r.X + r.W*(1-zoom)/2
				y := r.Y + r.H*(1-zoom)/2
				p.Image(state.svg, ui.Rect{X: x, Y: y, W: r.W * zoom, H: r.H * zoom}, ui.Contain)
			})
			if interactive && state.zoom > 1 {
				limit := (state.zoom - 1) * 95
				canvas.Draggable(&state.offset, ui.DraggableOptions{Bounds: &ui.Rect{X: -limit, Y: -limit, W: 2 * limit, H: 2 * limit}, Rigid: true})
			}
		})
		if !interactive {
			return
		}
		for i := range state.sliders {
			slider := &state.sliders[i]
			ui.Row(c.Key(fmt.Sprintf("figure-slider-%d", i))).FillWidth().MaxWidth(500).AlignSelf(ui.Center).Gap(10).AlignItems(ui.Center).Children(func() {
				label := fmt.Sprintf("%s = %.3g", slider.param.Name, slider.value)
				ui.Text(c, label).FontSize(11).Width(78)
				ui.Slider(c.Key("value"), &slider.value, slider.param.Min, slider.param.Max).Label("图形参数 " + slider.param.Name).Grow(1).OnChange(func() {
					values := make(map[string]float64, len(state.sliders))
					for _, v := range state.sliders {
						values[v.param.Name] = v.value
					}
					if raw, err := state.config.SVGWith(values); err == nil {
						if updated, err := ui.ParseSVG([]byte(raw)); err == nil {
							state.svg = updated
						}
					}
				})
			})
		}
		ui.Row(c.Key("figure-tools")).FillWidth().Justify(ui.Center).Gap(6).Children(func() {
			control(c, "zoom-out", "−", nil, outlineButton, func() { state.zoom = max(1, state.zoom/1.25); state.offset = ui.Point{} }).Label("缩小配图").Disabled(state.zoom <= 1)
			control(c, "zoom-in", "＋", nil, outlineButton, func() { state.zoom = min(4, state.zoom*1.25) }).Label("放大配图").Disabled(state.zoom >= 4)
			for i, arrow := range []struct {
				label, name string
				x, y        float32
			}{{"←", "左移配图", -1, 0}, {"↑", "上移配图", 0, -1}, {"↓", "下移配图", 0, 1}, {"→", "右移配图", 1, 0}} {
				if state.zoom <= 1 {
					break
				}
				control(c, fmt.Sprintf("pan-%d", i), arrow.label, nil, ghostButton, func() {
					limit := (state.zoom - 1) * 95
					state.offset.X = min(limit, max(-limit, state.offset.X+34*arrow.x))
					state.offset.Y = min(limit, max(-limit, state.offset.Y+34*arrow.y))
				}).Label(arrow.name)
			}
			control(c, "zoom-reset", "重置", nil, ghostButton, func() { state.zoom = 1; state.offset = ui.Point{} }).Label("重置配图视图").Disabled(state.zoom <= 1)
		})
	})
}

// rasterizePaperSVG adapts the native renderer for the platform-neutral PDF writer.
func rasterizePaperSVG(source string, maximum int) (image.Image, error) {
	svg, err := ui.ParseSVG([]byte(source))
	if err != nil {
		return nil, err
	}
	w, h := svg.Size()
	if !(w > 0 && h > 0) || w > 4096 || h > 4096 {
		return nil, fmt.Errorf("配图尺寸无效")
	}
	scale := min(float32(maximum)/w, float32(maximum)/h)
	return svg.Image(max(1, int(math.Round(float64(w*scale)))), max(1, int(math.Round(float64(h*scale)))), color.NRGBA{R: 25, G: 32, B: 45, A: 255}), nil
}
