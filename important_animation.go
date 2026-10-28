package main

import (
	"bytes"
	"github.com/egoist/mygo/ui"
	"image"
	"image/color"
	"image/png"
	"time"
)

var importantSprite, importantSpriteDark, importantShield, importantShieldDark *ui.Bitmap

type importantMotion struct {
	ID    string
	Phase int
	Start time.Time
}

func loadImportantAssets() {
	importantSprite, importantSpriteDark = loadInvertedAsset("resources/zhongyao-q-96.png")
	importantShield, importantShieldDark = loadInvertedAsset("resources/zhongyao-shield-32.png")
}
func loadInvertedAsset(name string) (*ui.Bitmap, *ui.Bitmap) {
	data, err := nativeAssets.ReadFile(name)
	if err != nil {
		panic(err)
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	inverted := image.NewNRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			v := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			inverted.SetNRGBA(x, y, color.NRGBA{255 - v.R, 255 - v.G, 255 - v.B, v.A})
		}
	}
	return ui.NewBitmap(src), ui.NewBitmap(inverted)
}
func cubicEase(x, x1, y1, x2, y2 float64) float32 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	lo, hi := 0., 1.
	for i := 0; i < 24; i++ {
		t := (lo + hi) / 2
		v := 3*(1-t)*(1-t)*t*x1 + 3*(1-t)*t*t*x2 + t*t*t
		if v < x {
			lo = t
		} else {
			hi = t
		}
	}
	t := (lo + hi) / 2
	return float32(3*(1-t)*(1-t)*t*y1 + 3*(1-t)*t*t*y2 + t*t*t)
}
func importantFrame(elapsed time.Duration, exit bool) (angle, opacity float32, active bool) {
	duration := 750 * time.Millisecond
	if exit {
		duration = 580 * time.Millisecond
	}
	progress := float64(elapsed) / float64(duration)
	active = progress < 1
	progress = max(0, min(1, progress))
	ease := cubicEase(progress, .22, 1, .36, 1)
	if exit {
		ease = cubicEase(progress, .64, 0, .78, 0)
	}
	if exit {
		angle = -90 * ease
		opacity = 1 - ease
	} else {
		angle = -90 * (1 - ease)
		opacity = ease
	}
	return
}
func (a *app) importantOptions(c *ui.Context, q question, done, correct bool, fn func()) {
	important := a.starred[q.ID]
	stage := ui.Column(c.Key("bao-options-stage")).FillWidth().Margin(0, 0, 24, 0)
	if important {
		phase := 0
		if done && correct {
			phase = 1
		}
		if a.importantMotion.ID != q.ID {
			a.importantMotion = importantMotion{ID: q.ID, Phase: phase, Start: c.Now()}
			if phase == 1 {
				a.importantMotion.Start = c.Now().Add(-time.Second)
			}
		} else if a.importantMotion.Phase != phase {
			a.importantMotion.Phase = phase
			a.importantMotion.Start = c.Now()
		}
		motion := a.importantMotion
		bitmap := importantSprite
		if a.dark {
			bitmap = importantSpriteDark
		}
		// Draw on the stage itself: the panel paints over it and stays clickable.
		stage.Label("重要题目动画").Padding(0, 118, 0, 0).Draw(func(p *ui.Painter, r ui.Rect) {
			angle, opacity, active := importantFrame(p.Now().Sub(motion.Start), motion.Phase == 1)
			if opacity > 0 {
				rect := ui.Rect{X: r.X + r.W - 150, Y: r.Y + r.H - 176, W: 160, H: 160}
				p.Transform(ui.Rotation(angle), rect.X, rect.Y+160, func() { p.ImageRegion(bitmap, ui.Rect{}, rect, ui.ImageOptions{Tint: ui.RGBA(255, 255, 255, opacity)}) })
			}
			if active {
				p.AnimationFrame()
			}
		})
	}
	stage.Children(func() {
		panel := ui.Column(c.Key("options-" + q.ID)).Label("选项组").FillWidth().Gap(6).Background(c.Theme().Background)
		if important {
			panel.BorderWidth(0, 1, 0, 0).BorderColor(c.Theme().Border.Alpha(.5))
		}
		panel.Children(fn)
	})
}
