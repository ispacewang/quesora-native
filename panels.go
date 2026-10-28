package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

func emptyIcon(c *ui.Context, s *ui.SVG) {
	ui.Box(c).Size(40, 40).Radius(20).Background(c.Theme().Surface).Opacity(.4).Center().Children(func() { icon(c, s, 16) })
}
func (a *app) stats(c *ui.Context) {
	t := c.Theme()
	good, bad := a.counts()
	total := good + bad
	rate := 0
	if total > 0 {
		rate = roundedPercent(good, total)
	}
	ui.Row(c).Height(38).Shrink(0).FillWidth().Center().BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.5)).Children(func() { ui.Text(c, "统计").FontSize(12).FontWeight(500).TextColor(t.TextMuted) })
	ui.Scroll(c.Key("stats-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(20, 16).Gap(20).Children(func() {
			ui.Text(c, "答题统计").FontSize(12).FontWeight(600).TextColor(t.TextMuted)
			if total == 0 {
				ui.Column(c).FillWidth().Padding(32, 0).AlignItems(ui.Center).Children(func() {
					emptyIcon(c, chartIcon)
					ui.Text(c, "暂无答题记录").FontSize(12).TextColor(t.TextMuted).Margin(12, 0, 0, 0)
				})
				return
			}
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				for i, value := range []string{fmt.Sprint(good), fmt.Sprint(bad), fmt.Sprintf("%d%%", rate)} {
					color := []ui.Color{t.Success, t.Danger, t.Text}[i]
					label := []string{"正确", "错误", "正确率"}[i]
					ui.Column(c).Grow(1).Basis(0).AlignItems(ui.Center).Children(func() {
						ui.Text(c, value).FontSize(24).FontWeight(700).TextColor(color)
						ui.Text(c, label).FontSize(10).TextColor(t.TextMuted)
					})
				}
			})
			ui.Progress(c, float64(rate)/100).FillWidth().Height(4)
			ui.Column(c).FillWidth().Gap(6).Children(func() { ui.Text(c, "题型正确率").FontSize(10).FontWeight(600).TextColor(t.TextMuted); a.radar(c) })
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, "题型明细").FontSize(10).FontWeight(600).TextColor(t.TextMuted)
				for _, kind := range []string{"单选题", "多选题", "判断题", "简答题", "填空题"} {
					correct, wrong := a.typeCounts(kind)
					if correct+wrong == 0 {
						continue
					}
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						caption(c, kind)
						ui.Spacer(c)
						ui.Text(c, fmt.Sprint(correct)).FontSize(12).TextColor(t.Success)
						ui.Text(c, " / ").FontSize(12).TextColor(t.TextMuted)
						ui.Text(c, fmt.Sprint(wrong)).FontSize(12).TextColor(t.Danger)
						ui.Text(c, fmt.Sprintf(" (%d%%)", roundedPercent(correct, correct+wrong))).FontSize(10).TextColor(t.TextMuted)
					})
				}
			})
		})
	})
}
func (a *app) typeCounts(kind string) (good, bad int) {
	for _, q := range a.questions {
		if q.Bank == a.bank && q.Kind == kind {
			if r, ok := a.answers[q.ID]; ok {
				if r.Correct {
					good++
				} else {
					bad++
				}
			}
		}
	}
	return
}
func (a *app) wrongPanel(c *ui.Context) {
	t := c.Theme()
	_, bad := a.counts()
	ui.Column(c.Key("wrong-content")).Fill().Padding(16).Children(func() {
		ui.Row(c).FillWidth().Padding(0, 0, 12, 0).Margin(0, 0, 12, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.5)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "错题本").FontSize(12).FontWeight(600).TextColor(t.TextMuted)
			ui.Spacer(c)
			if bad > 0 {
				ui.Text(c, fmt.Sprintf("%d 题", bad)).FontSize(11).TextColor(t.TextMuted)
				control(c, "clear-wrong", "清空", nil, ghostButton, a.clearWrong).Height(20).Padding(0, 4)
			}
		})
		if bad == 0 {
			ui.Column(c.Key("wrong-empty")).FillWidth().Grow(1).MinHeight(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				emptyIcon(c, pulseIcon)
				ui.Text(c, "暂无错题").FontSize(12).TextColor(t.TextMuted).Margin(12, 0, 0, 0)
				ui.Text(c, "答题后错题自动收集").FontSize(10).TextColor(t.TextMuted.Alpha(.6)).Margin(4, 0, 0, 0)
			})
		} else {
			ui.Scroll(c.Key("wrong-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					for _, q := range a.questions {
						r, ok := a.answers[q.ID]
						if !ok || r.Correct || q.Bank != a.bank {
							continue
						}
						ui.ButtonBase(c.Key("review-"+q.ID)).Label("查看解析").FillWidth().Padding(12, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.3)).OnClick(func() {
							a.filter = "全部"
							a.filters = nil
							a.importantOnly = false
							a.onlyWrong = false
							a.reorder()
							for i, item := range a.visible() {
								if item.ID == q.ID {
									a.navigateForReview(i)
									break
								}
							}
						}).Children(func() {
							ui.Column(c).FillWidth().Gap(8).Children(func() {
								ui.Text(c, q.Prompt).FontSize(14).FontWeight(500).LineHeight(1.625).FillWidth()
								ui.Column(c).FillWidth().Gap(2).Children(func() {
									for display, i := range responseOrder(q, r) {
										opt := q.Options[i]
										color := t.TextMuted
										if q.Answer&(1<<i) != 0 {
											color = t.Success
										}
										ui.Text(c, fmt.Sprintf("%c. %s", rune('A'+display), opt)).FontSize(11).LineHeight(1.5).TextColor(color).FillWidth()
									}
								})
								given, expected := responseLetters(q, r, r.Choice), responseLetters(q, r, q.Answer)
								if isTextQuestion(q) {
									given, expected = r.Text, q.TextAnswer
								}
								ui.Row(c).FillWidth().Gap(12).Wrap().Children(func() {
									ui.Text(c, "你的 "+given).FontSize(12).TextColor(t.Danger)
									ui.Text(c, "正确 "+expected).FontSize(12).TextColor(t.Success)
								})
							})
						})
					}
				})
			})
		}
		ui.Column(c.Key("wrong-footer")).FillWidth().Shrink(0).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border.Alpha(.5)).Gap(8).Children(func() {
			label := "保存到我的"
			if a.savedWrong {
				label = "已保存到我的"
			} else if a.wrongSaving {
				label = "保存中…"
			}
			control(c, "save-wrong", label, nil, primaryButton, a.saveWrongBook).FillWidth().Height(32).Disabled(a.savedWrong || a.wrongSaving || bad == 0 || a.store == nil)
			control(c, "export-wrong", "导出错题 ▾", nil, outlineButton, func() { a.exportOpen = !a.exportOpen }).FillWidth().Height(32).Disabled(bad == 0)
			if a.exportOpen {
				ui.Row(c).FillWidth().Gap(4).Children(func() {
					control(c, "export-md", "Markdown", nil, ghostButton, func() { a.exportWrong("md") })
					control(c, "export-txt", "纯文本", nil, ghostButton, func() { a.exportWrong("txt") })
				})
			}
		})
	})
}
