package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"strings"
)

func (a *app) quiz(c *ui.Context) {
	_, height := c.Size()
	ui.Scroll(c.Key("quiz-scroll")).Fill().Children(func() {
		ui.Column(c).FillWidth().MinHeight(max(0, height-titleHeight-toolbarHeight)).Padding(0, 0, 80, 0).Children(func() {
			a.bankBar(c)
			q, ok := a.current()
			if !ok {
				ui.Column(c.Key("empty-question")).Grow(1).Justify(ui.Center).FillWidth().Padding(80, 20).AlignItems(ui.Center).Gap(16).Children(func() {
					ui.Box(c).Size(48, 48).Radius(24).Background(c.Theme().Surface).Opacity(.4).Center().Children(func() { icon(c, infoIcon, 18) })
					text := "题库为空"
					if a.bank == "" {
						text = "选择一个题库开始答题"
					}
					ui.Text(c, text).FontSize(14).TextColor(c.Theme().TextMuted)
					if a.bank != "" {
						control(c, "show-all", "查看全部题目", nil, ghostButton, func() {
							a.filter = "全部"
							a.filters = nil
							a.importantOnly = false
							a.onlyWrong = false
							a.reorder()
							a.navigate(0)
						})
					}
				})
				return
			}
			ui.Column(c.Key("question-shell")).FillWidth().Padding(16).Children(func() {
				ui.Column(c.Key("question-content")).Label("题卡").FillWidth().MaxWidth(800).AlignSelf(ui.Center).Padding(20, 16).Children(func() {
					a.questionToolbar(c, q)
					ui.Text(c, q.Prompt).Label("题干").FontSize(16).FontWeight(600).LineHeight(1.625).FillWidth().Margin(0, 0, 32, 0)
					if q.Graph {
						a.functionGraph(c)
					}
					r, done := a.currentResponse(q)
					a.importantOptions(c, q, done, r.Correct, func() {
						if len(q.Options) == 0 || q.Kind == "简答题" || q.Kind == "填空题" {
							a.textAnswer(c, q, done)
						} else {
							if q.Kind == "多选题" && !done {
								ui.Row(c).FillWidth().AlignItems(ui.Center).Margin(0, 0, 2, 0).Children(func() {
									ui.Text(c, "多选题 · 可多选").FontSize(10).TextColor(c.Theme().TextMuted)
									ui.Spacer(c)
									mask := uint(1<<len(q.Options)) - 1
									label := "一键全选"
									if a.chosen == mask {
										label = "取消全选"
									}
									control(c, "select-all", label, nil, ghostButton, func() {
										if a.chosen == mask {
											a.chosen = 0
										} else {
											a.chosen = mask
										}
									}).Height(22)
								})
							}
							for display, original := range a.displayOptions(q) {
								a.option(c, q, display, original, done)
							}
						}
					})
					if a.notice != "" {
						ui.Text(c, a.notice).FontSize(12).TextColor(c.Theme().Danger).FillWidth().Margin(0, 0, 8, 0)
					}
					ui.Row(c.Key("question-actions")).FillWidth().Padding(16, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border.Alpha(.4)).Gap(10).Children(func() {
						if done {
							control(c, "next", "下一题 →", nil, primaryButton, a.nextQuestion).Label("下一题").Height(32)
						} else {
							label := "提交答案"
							if a.quick {
								label = "提交并继续 →"
							}
							b := control(c, "submit", label, nil, primaryButton, a.submit).Label("提交答案").Height(32)
							enabled := a.chosen != 0
							if isTextQuestion(q) {
								enabled = strings.TrimSpace(a.typed) != ""
							}
							if !enabled {
								b.Disabled(true)
							}
						}
					})
					if done {
						a.answerFeedback(c, q, r)
					}
				})
			})
		})
	})
	nativeModal(c.Key("finish-dialog"), &a.finishOpen, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Text(c, "你已答完整个题库").FontSize(18).FontWeight(600)
			ui.Text(c, "所有题目都已刷完，再来一遍或退出刷题？").FontSize(14)
			ui.Row(c).Gap(8).Children(func() {
				ui.Spacer(c)
				control(c, "exit-practice", "退出", nil, outlineButton, func() { a.finishOpen = false; a.bank = ""; a.navigate(0) })
				control(c, "restart-cycle", "再来一遍", nil, primaryButton, a.restartCycle)
			})
		})
	}).Width(440)
}
func (a *app) questionToolbar(c *ui.Context, q question) {
	t := c.Theme()
	ui.Row(c.Key("question-heading")).FillWidth().AlignItems(ui.Center).Gap(8).Wrap().Padding(0, 0, 12, 0).Margin(0, 0, 16, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.4)).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, q.Kind).FontSize(12).FontWeight(600).LineHeight(1.33).Padding(2, 10).Background(t.Accent).TextColor(t.AccentText)
			if a.starred[q.ID] {
				ui.Text(c, "重要题目").FontSize(12).Padding(2, 10).Background(t.Danger).TextColor(t.AccentText)
			}
			if name, _ := q.Meta["题目分类"].(string); name != "" {
				ui.Text(c, name).FontSize(12).Padding(2, 10).Background(t.Surface)
			}
		})
		ui.Spacer(c)
		ui.Row(c.Key("modes")).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Row(c.Key("filters")).Gap(2).AlignItems(ui.Center).Children(func() {
				for _, kind := range []string{"全部", "单选题", "多选题", "判断题", "简答题", "填空题", "重要"} {
					label := kind
					if kind == "简答题" {
						label = "简答"
					}
					if kind == "填空题" {
						label = "填空"
					}
					active := a.filter == kind
					if len(a.filters) > 0 {
						active = a.filters[kind]
					}
					if kind == "重要" {
						active = a.importantOnly
					}
					b := ui.ButtonBase(c.Key("filter-"+kind)).Label("筛选"+kind).Padding(2, 6).Border(1, ui.Transparent).Height(20).TextColor(t.TextMuted).OnClick(func() { a.toggleFilter(kind) })
					if active {
						b.BorderColor(t.Accent.Alpha(.4)).Background(t.Accent.Alpha(.1)).TextColor(t.Accent)
					}
					b.Children(func() {
						ui.Text(c, label).FontSize(10).LineHeight(1.4)
						if active {
							count := 0
							for _, item := range a.questions {
								if item.Bank == a.bank && (kind == "全部" || kind == item.Kind || kind == "重要" && a.starred[item.ID]) {
									count++
								}
							}
							ui.Text(c, fmt.Sprint(count)).Absolute().Top(-10).Left(0).Right(0).FontSize(9).TextColor(t.TextMuted.Alpha(.7)).TextAlign(ui.Center)
						}
					})
				}
			})
			for _, mode := range []struct {
				name   string
				tip    string
				active bool
				s      *ui.SVG
				fn     func()
			}{
				{"重新作答", "开启后，做过的题也可重新作答；提交后更新该题记录。关闭后显示上次结果。", a.repeatPractice, historyIcon, a.toggleRepeatPractice},
				{"随机", "开启后随机排列题目顺序，一轮内不重复；关闭后按原顺序练习。", a.random, shuffleIcon, func() { a.random = !a.random; a.reorder(); a.navigate(0) }}, {"速刷", "开启后提交答案会自动进入下一题；关闭后停留查看本题结果。", a.quick, zapIcon, func() { a.quick = !a.quick }}, {"打乱", "开启后打乱选择题的选项顺序；关闭后恢复原顺序，正确答案随选项同步。", a.shuffle, shuffleIcon, func() { a.shuffle = !a.shuffle; a.prepareOptions() }},
			} {
				style := outlineButton
				if mode.active {
					style = primaryButton
				}
				control(c, "mode-"+mode.name, mode.name, mode.s, style, mode.fn).Tooltip(mode.tip)
			}
		})
	})
}
func (a *app) option(c *ui.Context, q question, display, original int, done bool) {
	t := c.Theme()
	selected := a.chosen&(1<<original) != 0
	correct := q.Answer&(1<<original) != 0
	border, bg, fg, letterBG, letterFG := t.Border.Alpha(.5), t.Background, t.Text, ui.Transparent, t.TextMuted
	if selected {
		border = t.Accent
		bg = t.Accent.Alpha(.05)
		letterBG = t.Accent
		letterFG = t.AccentText
	}
	var resultIcon *ui.SVG
	if done {
		if correct {
			border = t.Success.Alpha(.3)
			bg = t.Success.Alpha(.06)
			letterBG = t.Success
			letterFG = t.AccentText
			resultIcon = checkIcon
		} else if selected {
			border = t.Danger.Alpha(.3)
			bg = t.Danger.Alpha(.05)
			letterBG = t.Danger
			letterFG = t.AccentText
			resultIcon = closeIcon
		} else {
			border = t.Border.Alpha(.2)
			fg = t.Text.Alpha(.6)
			letterFG = t.TextMuted.Alpha(.6)
		}
	}
	label := fmt.Sprintf("%c. %s", rune('A'+display), q.Options[original])
	b := ui.ButtonBase(c.Key(fmt.Sprintf("option-%s-%d", q.ID, display))).Label(label).FillWidth().MinHeight(50).Padding(12, 14).Border(1, border).Background(bg).Disabled(done).OnClick(func() { a.choose(original) })
	if b.Hovered() && !done && !selected {
		b.BorderColor(t.Accent.Alpha(.4)).Background(t.Accent.Alpha(.02))
	}
	b.Children(func() {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
			lc := t.TextMuted.Alpha(.25)
			if selected || done && correct {
				lc = letterBG
			}
			ui.Box(c).Size(24, 24).Shrink(0).Border(1, lc).Background(letterBG).Center().Children(func() { ui.Text(c, string(rune('A'+display))).FontSize(12).FontWeight(600).TextColor(letterFG) })
			ui.Text(c, q.Options[original]).FontSize(14).LineHeight(1.625).TextColor(fg).Grow(1).MinWidth(0).Margin(2, 0, 0, 0)
			if resultIcon != nil {
				color := t.Success
				if !correct {
					color = t.Danger
				}
				ui.Icon(c, resultIcon).Size(14, 14).TextColor(color).Margin(4, 0, 0, 0)
			}
		})
	})
}
func (a *app) textAnswer(c *ui.Context, q question, done bool) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Text(c, q.Kind).FontSize(10).TextColor(t.TextMuted)
		ui.Spacer(c)
		control(c, "answer-preview", "预览", eyeIcon, ghostButton, func() { a.showPreview = !a.showPreview }).Height(22)
	})
	placeholder := "输入你的答案..."
	if q.Kind == "填空题" {
		placeholder = "输入正确答案..."
	}
	ui.TextArea(c.Key("answer-"+q.ID), &a.typed).Label("填写答案").Bind(&a.answerHandle).Placeholder(placeholder).FillWidth().Height(110).FontSize(14).LineHeight(1.5).Padding(8, 12).Radius(0).Border(1, t.Border).Background(t.Background).Disabled(done)
	if a.showPreview && a.typed != "" {
		ui.Text(c, a.typed).FillWidth().FontSize(14).LineHeight(1.625).Padding(12).Border(1, t.Border.Alpha(.5)).Background(t.Surface.Alpha(.3))
	}
}
func (a *app) answerFeedback(c *ui.Context, q question, r response) {
	t := c.Theme()
	color := t.Danger
	symbol := closeIcon
	label := "回答错误"
	if r.Correct {
		color = t.Success
		symbol = checkIcon
		label = "回答正确！"
	}
	ui.Column(c.Key("feedback")).FillWidth().Gap(12).Margin(20, 0, 0, 0).Children(func() {
		ui.Row(c).FillWidth().Padding(12, 16).Gap(8).AlignItems(ui.Center).Background(color.Alpha(.06)).BorderWidth(0, 0, 0, 3).BorderColor(color).Children(func() {
			ui.Icon(c, symbol).Size(16, 16).TextColor(color)
			ui.Text(c, label).FontSize(14).FontWeight(500).TextColor(color)
		})
		if !r.Correct {
			ui.Row(c).Padding(0, 16).Gap(8).Children(func() {
				caption(c, "正确答案：")
				ui.Text(c, a.displayAnswer(q)).FontSize(14).FontWeight(600).TextColor(t.Success)
			})
		}
		if q.Explanation != "" {
			ui.Column(c.Key("explanation")).FillWidth().Padding(16).Gap(4).Background(t.Surface.Alpha(.5)).Children(func() {
				ui.Text(c, "解析").FontSize(10).FontWeight(600).TextColor(t.TextMuted)
				ui.Text(c, q.Explanation).FontSize(14).LineHeight(1.625).FillWidth()
			})
		}
	})
}
