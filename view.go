package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const titleHeight float32 = 44
const toolbarHeight float32 = 40.667

func (a *app) view(c *ui.Context) {
	c.SetTheme(previewTheme(a.dark))
	t := c.Theme()
	w, _ := c.Size()
	leftVisible, rightVisible := a.visiblePanels(w)
	bg := t.Background
	if c.Vibrancy() {
		bg = ui.Transparent
	}
	c.Root().Background(bg)
	ui.Column(c.Key("app-shell")).Fill().Background(bg).Children(func() {
		a.titlebar(c)
		ui.Row(c.Key("toolbar")).Background(chromeBackground(c)).FillWidth().Height(toolbarHeight).Shrink(0).Padding(6, 16).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.5)).AlignItems(ui.Center).Children(func() {
			control(c, "help", "", helpIcon, ghostButton, func() { a.helpOpen = true }).Label("功能引导").TextColor(t.Text).Size(28, 28).Padding(0).Tooltip("功能引导")
			ui.Spacer(c)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				control(c, "exam", "考试", plusIcon, primaryButton, a.openExamSetup).Disabled(a.exam != nil)
				control(c, "history", "我的", historyIcon, ghostButton, a.openHistory).TextColor(t.Text).Tooltip("查看保存的考试成绩和错题本")
			})
		})
		if a.exam != nil {
			a.examView(c)
		} else {
			ui.Row(c.Key("body")).Background(t.Background).FillWidth().Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
				if leftVisible {
					ui.Column(c.Key("stats")).Label("统计侧栏").Width(260).Shrink(0).BorderWidth(0, 1, 0, 0).BorderColor(t.Border.Alpha(.5)).Children(func() { a.stats(c) })
				}
				ui.Column(c.Key("quiz")).Label("答题主区域").Grow(1).MinWidth(0).Children(func() { a.quiz(c) })
				if rightVisible {
					ui.Column(c.Key("wrong")).Label("错题侧栏").Width(340).Shrink(0).BorderWidth(0, 0, 0, 1).BorderColor(t.Border.Alpha(.5)).Children(func() { a.wrongPanel(c) })
				}
				ui.Row(c.Key("floating-controls")).Absolute().Left(0).Right(0).Bottom(24).Height(34).Justify(ui.Center).Gap(8).Children(func() {

					a.panelToggle(c, true)
					a.panelToggle(c, false)
				})
			})
		}
	})
	a.examDialogs(c)
	a.historyView(c)
	a.libraryDialogs(c)
	if a.libraryStatus != "" && a.libraryStatus != "题库与答题记录自动保存" && a.libraryStatus != a.toastLast {
		a.toastLast = a.libraryStatus
		c.Toast(a.libraryStatus)
	}
	if a.exam == nil && !a.examSetupOpen && !a.historyOpen && a.quick && !a.answerHandle.FocusWithin(c) && !a.importOpen && !a.deleteOpen && !a.helpOpen && !a.resetPracticeOpen && !a.finishOpen {
		for i, key := range []ui.Key{ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD} {
			c.OnShortcut(0, key, func() {
				if q, ok := a.current(); ok {
					order := a.displayOptions(q)
					if i < len(order) {
						a.choose(order[i])
					}
				}
			})
		}
		c.OnShortcut(0, ui.KeySpace, a.submit)
	}
	nativeModal(c.Key("help-dialog"), &a.helpOpen, func() {
		ui.Column(c).Gap(16).FillWidth().Children(func() {
			ui.Text(c, "功能引导").FontSize(18).FontWeight(600)
			ui.Text(c, "选择题库后开始答题。可在题卡右上方筛选题型，使用下方悬浮按钮展开统计与错题本。").FillWidth().LineHeight(1.6)
			control(c, "reset", "重新练习", nil, outlineButton, func() { a.helpOpen = false; a.resetPracticeOpen = true })
			control(c, "help-close", "知道了", nil, primaryButton, func() { a.helpOpen = false })
		})
	}).Width(440)
	nativeModal(c.Key("reset-practice-dialog"), &a.resetPracticeOpen, func() {
		ui.Text(c, "重置练习记录").FontSize(18).FontWeight(600)
		ui.Text(c, "将清空所有题库的答题记录。已保存的考试成绩、错题本和题库内容会保留。").FillWidth().FontSize(14).LineHeight(1.6)
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			control(c, "cancel", "取消", nil, outlineButton, func() { a.resetPracticeOpen = false })
			control(c, "confirm", "确认重置", nil, primaryButton, func() { a.resetPractice(); a.resetPracticeOpen = false })
		})
	}).Width(440)

}
func (a *app) titlebar(c *ui.Context) {
	bar := c.TitleBar()
	t := c.Theme()
	bg := chromeBackground(c)
	ui.Row(c.Key("title")).FillWidth().Height(44).Shrink(0).Background(bg).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Row(c).Padding(0, 0, 0, 12).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Image(c, appIcon).Size(20, 20)
			ui.Text(c, "Quesora").FontSize(14).FontWeight(600)
			ui.Text(c, "v"+appVersion).Label("应用版本").FontSize(11).TextColor(t.Text).SingleLine()
		})
		ui.Spacer(c)
		symbol, label := moonIcon, "深色模式"
		if a.dark {
			symbol, label = sunIcon, "浅色模式"
		}
		control(c, "theme", "", symbol, ghostButton, func() {
			a.dark = !a.dark
			if a.win != nil {
				if a.dark {
					mygo.Theme.SetSource(mygo.ThemeDark)
				} else {
					mygo.Theme.SetSource(mygo.ThemeLight)
				}
			}
		}).Label(label).Tooltip(label).TextColor(t.Text).Size(36, 44).Padding(10).Background(ui.Transparent)
		if bar.Right > 0 {
			ui.Box(c).Width(bar.Right).Height(44).Shrink(0)
			return
		}
		for _, v := range []struct {
			key, label string
			s          *ui.SVG
			fn         func()
		}{
			{"minimize", "最小化", minusIcon, func() {
				if a.win != nil {
					a.win.Minimize()
				}
			}},
			{"maximize", "最大化", squareIcon, func() {
				if a.win != nil {
					a.win.ToggleMaximize()
				}
			}},
			{"close-window", "关闭窗口", closeIcon, func() {
				if a.win != nil {
					a.win.Close()
				}
			}},
		} {
			b := control(c, v.key, "", v.s, ghostButton, v.fn).Label(v.label).Tooltip(v.label).TextColor(t.Text).Size(46, 44).Padding(15).Background(ui.Transparent)
			if b.Hovered() {
				if v.key == "close-window" {
					b.Background(t.Danger.Alpha(.7)).TextColor(ui.RGB(255, 255, 255))
				} else {
					b.Background(t.Text.Alpha(.05))
				}
			}
		}
	})
}
