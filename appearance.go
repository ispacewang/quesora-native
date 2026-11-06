// 外观偏好与有效配色分离：系统变化只更新颜色，不覆盖用户选择。
package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func validAppearance(v string) bool { return v == "system" || v == "light" || v == "dark" }
func (a *app) systemAppearanceChanged(dark bool) {
	switch a.appearance {
	case "dark":
		a.dark = true
	case "light":
		a.dark = false
	default:
		a.dark = dark
	}
}
func (a *app) setAppearance(mode string) {
	if !validAppearance(mode) {
		return
	}
	a.appearance = mode
	a.applyTheme()
}

// configureAppearance subscribes once on the UI thread; closing the window removes the listener.
func (a *app) configureAppearance() {
	a.systemDark = mygo.Theme.IsDark
	mygo.Theme.SetSource(mygo.ThemeSource(a.appearance))
	a.systemAppearanceChanged(a.systemDark())
	a.themeOff = mygo.Theme.OnUpdated(func() {
		if a.closed {
			return
		}
		a.systemAppearanceChanged(a.systemDark())
		if a.win != nil {
			a.win.Invalidate()
		}
	})
}
func (a *app) applyTheme() {
	if a.win != nil {
		mygo.Theme.SetSource(mygo.ThemeSource(a.appearance))
	}
	dark := a.dark
	if a.systemDark != nil {
		dark = a.systemDark()
	}
	a.systemAppearanceChanged(dark)
	a.persistPreferences()
}
func (a *app) appearanceSettings(c *ui.Context) {
	ui.Column(c.Key("appearance")).FillWidth().Gap(8).Children(func() {
		ui.Text(c, "外观").FontSize(13).FontWeight(500)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			for _, v := range []struct{ ID, Name string }{{"system", "跟随系统"}, {"light", "浅色"}, {"dark", "深色"}} {
				style := outlineButton
				if a.appearance == v.ID {
					style = primaryButton
				}
				control(c, v.ID, v.Name, nil, style, func() { a.setAppearance(v.ID) }).Label("外观：" + v.Name).Grow(1).Basis(0)
			}
		})
		ui.Text(c, "跟随系统会响应 Windows 外观变化；手动选择会在下次启动时保留。").FillWidth().FontSize(11).LineHeight(1.5).TextColor(c.Theme().TextMuted)
	})
}
