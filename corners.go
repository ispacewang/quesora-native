// 界面边角偏好。按组件短边/标称高度计算半径，并封顶，避免大卡片变成胶囊。
package main

import "github.com/egoist/mygo/ui"

func scaledCornerRadius(rounded bool, extent float32) float32 {
	if !rounded {
		return 0
	}
	return min(16, max(0, extent/5))
}
func cornerRadius(c *ui.Context, extent float32) float32 {
	return scaledCornerRadius(c.Theme().Radius > 0, extent)
}
func (a *app) setRounded(rounded bool) { a.rounded = rounded; a.persistPreferences() }
func (a *app) cornerSettings(c *ui.Context) {
	ui.Column(c.Key("corner-style")).FillWidth().Gap(8).Children(func() {
		ui.Text(c, "界面边角").FontSize(13).FontWeight(500)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			for _, v := range []struct {
				Name    string
				Rounded bool
			}{{"直角", false}, {"圆角", true}} {
				style := outlineButton
				if a.rounded == v.Rounded {
					style = primaryButton
				}
				control(c, v.Name, v.Name, nil, style, func() { a.setRounded(v.Rounded) }).Label("边角：" + v.Name).Grow(1).Basis(0)
			}
		})
		ui.Text(c, "圆角随组件尺寸调整：小标签更小，按钮与输入框适中，卡片和弹窗更大。").FillWidth().FontSize(11).LineHeight(1.5).TextColor(c.Theme().TextMuted)
	})
}
