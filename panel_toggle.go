package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

func (a *app) panelToggle(c *ui.Context, left bool) {
	t := c.Theme()
	good, bad := a.counts()
	width, _ := c.Size()
	leftVisible, rightVisible := a.visiblePanels(width)
	key, label, open := "toggle-wrong", "错题本", rightVisible
	if left {
		key, label, open = "toggle-stats", "统计", leftVisible
	}
	button := ui.ButtonBase(c.Key(key)).Height(34).Padding(8, 12).Border(1, t.Border.Alpha(.6)).Background(ui.Transparent).Shadow(0, 2, 4, 0, ui.RGBA(0, 0, 0, .08)).OnClick(func() {
		a.togglePanel(width, left)
	})
	if left {
		button.Label("切换统计面板")
	} else {
		button.Label("切换错题面板")
	}
	if open {
		button.BorderColor(t.Accent.Alpha(.4))
	}
	if button.Hovered() {
		button.BorderColor(t.Accent.Alpha(.4))
	}
	tint := t.Background.Alpha(.86)
	if button.Hovered() {
		tint = t.Surface.Alpha(.9)
	}
	button.Draw(func(p *ui.Painter, r ui.Rect) {
		paintFrostedSurface(p, ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, tint)
	})
	button.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if left {
				symbol := rightIcon
				if open {
					symbol = leftIcon
				}
				icon(c, symbol, 12)
			}
			ui.Text(c, label).FontSize(12).FontWeight(500)
			if left && good+bad > 0 {
				ui.Text(c, fmt.Sprintf("%d/%d", good, good+bad)).FontSize(10).TextColor(t.TextMuted)
			}
			if !left {
				if bad > 0 {
					ui.Text(c, fmt.Sprint(bad)).FontSize(10).TextColor(t.Danger)
				}
				symbol := leftIcon
				if open {
					symbol = rightIcon
				}
				icon(c, symbol, 12)
			}
		})
	})
}

// Preserve a usable question column when a window cannot fit all three panes.
func (a *app) visiblePanels(width float32) (left, right bool) {
	left, right = a.leftOpen, a.rightOpen
	if width < 1100 && left && right {
		left = false
	}
	return
}
func (a *app) togglePanel(width float32, left bool) {
	a.leftOpen, a.rightOpen = a.visiblePanels(width)
	if left {
		a.leftOpen = !a.leftOpen
		if width < 1100 && a.leftOpen {
			a.rightOpen = false
		}
	} else {
		a.rightOpen = !a.rightOpen
		if width < 1100 && a.rightOpen {
			a.leftOpen = false
		}
	}
}
