package main

import "github.com/egoist/mygo/ui"

func previewTheme(dark bool) *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#fafaf9")
	t.Surface = ui.Hex("#f0f0ee")
	t.Text = ui.Hex("#2c2c2e")
	t.TextMuted = ui.Hex("#8e8e93")
	t.Border = ui.RGBA(0, 0, 0, .10)
	t.Accent = ui.Hex("#4a7dbf")
	t.AccentText = ui.Hex("#ffffff")
	t.Success = ui.Hex("#5d9b6a")
	t.Danger = ui.Hex("#c2655a")
	if dark {
		t = ui.DarkTheme()
		t.Background = ui.Hex("#1c1c1e")
		t.Surface = ui.Hex("#2c2c2e")
		t.Text = ui.Hex("#e5e5ea")
		t.TextMuted = ui.Hex("#8e8e93")
		t.Border = ui.RGBA(255, 255, 255, .10)
		t.Accent = ui.Hex("#6b9fd4")
		t.AccentText = ui.Hex("#1c1c1e")
		t.Success = ui.Hex("#6fa87a")
		t.Danger = ui.Hex("#d4786d")
	}
	t.SurfaceHover = t.Accent.Alpha(.05)
	t.SurfacePressed = t.Accent.Alpha(.10)
	t.Radius = 0
	t.FontSize = 14
	t.Font = "Noto Sans SC, Microsoft YaHei, Segoe UI"
	t.Spacing = 4
	return t
}
func caption(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(12).LineHeight(1.5).TextColor(c.Theme().TextMuted)
}
func line(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Shrink(0).Background(c.Theme().Border.Alpha(.5))
}

type buttonStyle int

const (
	outlineButton buttonStyle = iota
	primaryButton
	ghostButton
)

func control(c *ui.Context, key, label string, s *ui.SVG, style buttonStyle, fn func()) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Label(label).Height(28).Padding(0, 10).Radius(0).Border(1, t.Border.Alpha(.6)).Background(t.Background).TextColor(t.Text).FontSize(12).FontWeight(500).OnClick(fn)
	if style == primaryButton {
		b.Background(t.Accent).TextColor(t.AccentText).BorderColor(ui.Transparent)
	}
	if style == ghostButton {
		b.Background(ui.Transparent).BorderColor(ui.Transparent).TextColor(t.TextMuted)
	}
	if b.Hovered() {
		if style == primaryButton {
			b.Opacity(.9)
		} else {
			b.Background(t.Surface)
		}
	}
	b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if s != nil {
				icon(c, s, 14)
			}
			if label != "" {
				ui.Text(c, label).FontSize(12).FontWeight(500).SingleLine()
			}
		})
	})
	return b
}
func action(c *ui.Context, key, label string, active bool, fn func()) ui.Element {
	style := outlineButton
	if active {
		style = primaryButton
	}
	return control(c, key, label, nil, style, fn)
}

// Let Windows draw the material directly; an opaque tint would hide its blur.
func chromeBackground(c *ui.Context) ui.Color {
	if c.Vibrancy() {
		return ui.Transparent
	}
	return c.Theme().Background
}
