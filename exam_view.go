package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"math/rand"
	"time"
)

func (a *app) examDialogs(c *ui.Context) {
	a.examSetupDialog(c)
	nativeModal(c.Key("exam-submit"), &a.examSubmitOpen, func() {
		ui.Text(c, "交卷确认").FontSize(18).FontWeight(600)
		ui.Text(c, "交卷后立即判分。").FontSize(14)
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			control(c, "cancel", "取消", nil, outlineButton, func() { a.examSubmitOpen = false })
			control(c, "confirm", "确定", nil, primaryButton, func() {
				if a.exam != nil {
					a.exam.submit(time.Now())
				}
				a.examSubmitOpen = false
				a.examMessage = ""
			}).Label("确认交卷")
		})
	}).Width(440)
	nativeModal(c.Key("exam-exit"), &a.examExitOpen, func() {
		ui.Text(c, "确认退出").FontSize(18).FontWeight(600)
		ui.Text(c, "退出后本次答题不保存。").FontSize(14)
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			control(c, "cancel", "取消", nil, outlineButton, func() { a.examExitOpen = false })
			control(c, "exit", "退出", nil, primaryButton, func() { a.exam = nil; a.examExitOpen = false; a.examSubmitOpen = false; a.examMessage = "" }).Label("确认退出考试")
		})
	}).Width(440)
}
func (a *app) examView(c *ui.Context) {
	e := a.exam
	e.tick(c.Now())
	if e.Submitted {
		a.examSubmitOpen = false
		a.examResult(c, e)
		return
	}
	c.After(time.Second)
	t := c.Theme()
	w, _ := c.Size()
	left := float32(208)
	if w < 900 {
		left = 148
	}
	ui.Column(c.Key("exam")).FillWidth().Grow(1).MinHeight(0).Background(t.Background).Children(func() {
		ui.Row(c.Key("exam-header")).FillWidth().MinHeight(46).Padding(12, 20).Gap(12).Wrap().AlignItems(ui.Center).Background(t.Surface.Alpha(.5)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, e.Bank).FontSize(12).FontWeight(500)
			ui.Text(c, fmt.Sprintf("时长 %d 分钟", e.Minutes)).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("进度 %d/%d", e.completed(), len(e.Questions))).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			remaining := e.remaining(c.Now())
			ui.Text(c, fmt.Sprintf("%02d:%02d", remaining/60, remaining%60)).Label("考试倒计时").FontSize(14).FontWeight(600).TextColor(t.Danger)
			control(c, "exit-exam", "退出考试", nil, outlineButton, a.exitExam).Height(28)
		})
		ui.Row(c).FillWidth().Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			ui.Scroll(c.Key("answer-card")).Width(left).Shrink(0).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
				ui.Column(c).FillWidth().Padding(16).Gap(16).Children(func() {
					ui.Text(c, "答题卡").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
					for _, kind := range examKinds {
						var indices []int
						for i, q := range e.Questions {
							if q.Kind == kind {
								indices = append(indices, i)
							}
						}
						if len(indices) == 0 {
							continue
						}
						ui.Column(c.Key(kind)).Gap(8).FillWidth().Children(func() {
							ui.Text(c, kind).FontSize(11).TextColor(t.TextMuted)
							cols := 4
							if w < 900 {
								cols = 3
							}
							for start := 0; start < len(indices); start += cols {
								ui.Row(c.Key(fmt.Sprint(start))).Gap(6).Children(func() {
									for _, idx := range indices[start:min(start+cols, len(indices))] {
										bg, fg, border := t.Background, t.TextMuted, t.Border
										if e.answered(idx) {
											bg = t.Success.Alpha(.10)
											fg = t.Success
											border = t.Success.Alpha(.3)
										}
										if e.Marked[idx] {
											bg = ui.Hex("#f59e0b").Alpha(.1)
											fg = ui.Hex("#c4871c")
											border = fg.Alpha(.4)
										}
										if idx == e.Current {
											bg = t.Accent
											fg = t.AccentText
											border = t.Accent
										}
										ui.ButtonBase(c.Key(fmt.Sprint(idx))).Label(fmt.Sprintf("考试第%d题", idx+1)).Size(28, 28).Border(1, border).Background(bg).OnClick(func() { e.Current = idx; e.preview = false; a.examMessage = "" }).Center().Children(func() {
											ui.Text(c, fmt.Sprint(idx+1)).FontSize(11).TextColor(fg)
											if e.Important[idx] {
												img := importantShield
												if a.dark {
													img = importantShieldDark
												}
												ui.Image(c, img).Absolute().Right(-4).Top(-4).Size(14, 14)
											}
										})
									}
								})
							}
						})
					}
				})
			})
			ui.Scroll(c.Key("exam-question-scroll")).Grow(1).MinWidth(0).Children(func() { a.examQuestion(c, e) })
			if w >= 1100 {
				ui.Column(c.Key("exam-hints")).Width(300).Shrink(0).Padding(20, 16).Gap(16).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
					ui.Text(c, "提示").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
					for _, s := range []string{"· 点击答题卡序号快速跳转", "· ☆ 标记可标记疑问题目", "· 交卷后自动判分"} {
						ui.Text(c, s).FontSize(12).TextColor(t.TextMuted).FillWidth().LineHeight(1.6)
					}
				})
			}
		})
	})
}
func (a *app) examQuestion(c *ui.Context, e *examSession) {
	t := c.Theme()
	idx := e.Current
	q := e.Questions[idx]
	r := &e.Responses[idx]
	ui.Column(c.Key("exam-question"+q.ID)).FillWidth().MaxWidth(740).AlignSelf(ui.Center).Padding(32, 28).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Wrap().Margin(0, 0, 24, 0).Children(func() {
			ui.Text(c, fmt.Sprintf("第 %d 题", idx+1)).FontSize(16).FontWeight(600).TextColor(t.Accent)
			ui.Text(c, q.Kind).FontSize(12).Padding(2, 10).Background(t.Accent).TextColor(t.AccentText)
			if e.Important[idx] {
				ui.Text(c, "重要题目").FontSize(12).Padding(2, 10).Background(t.Danger).TextColor(t.AccentText)
			}
			for _, key := range []string{"题目分类", "一级纲要"} {
				if text, _ := q.Meta[key].(string); text != "" {
					ui.Text(c, text).FontSize(12).Padding(2, 8).Background(t.Surface)
				}
			}
			ui.Spacer(c)
			label := "☆ 标记"
			if e.Marked[idx] {
				label = "★ 已标记"
			}
			control(c, "mark", label, nil, ghostButton, func() { e.Marked[idx] = !e.Marked[idx] }).Label("标记疑题")
		})
		ui.Text(c, q.Prompt).FontSize(16).FontWeight(600).LineHeight(1.625).FillWidth().Margin(0, 0, 32, 0)
		if isTextQuestion(q) {
			ui.Row(c).FillWidth().Children(func() {
				ui.Text(c, "输入答案").FontSize(10).TextColor(t.TextMuted)
				ui.Spacer(c)
				control(c, "preview", "预览", eyeIcon, ghostButton, func() { e.preview = !e.preview })
			})
			ui.TextArea(c.Key("exam-text"), &r.Text).Label("考试填写答案").FillWidth().Height(110).FontSize(14).LineHeight(1.5).Padding(8, 12).Radius(0).Placeholder("输入你的答案...").Background(t.Background).Border(1, t.Border)
			if e.preview {
				ui.Text(c, r.Text).FillWidth().Padding(12).LineHeight(1.625).Border(1, t.Border).Margin(8, 0, 0, 0)
			}
		} else {
			ui.Column(c.Key("exam-options")).FillWidth().Padding(1).Gap(8).Border(1, t.Border.Alpha(.5)).Children(func() {
				for display, original := range responseOrder(q, *r) {
					selected := r.Choice&(1<<original) != 0
					border, bg, fg := t.Border.Alpha(.5), t.Background, t.TextMuted
					if selected {
						border = t.Accent
						bg = t.Accent.Alpha(.05)
						fg = t.Accent
					}
					ui.ButtonBase(c.Key(fmt.Sprint(display))).Label(fmt.Sprintf("考试选项%c. %s", rune('A'+display), q.Options[original])).FillWidth().MinHeight(46).Padding(10, 14).Background(bg).Border(1, border).OnClick(func() { e.choose(original); a.examMessage = "" }).Children(func() {
						ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
							lb, lf := ui.Transparent, t.TextMuted
							if selected {
								lb = t.Accent
								lf = t.AccentText
							}
							ui.Box(c).Size(24, 24).Shrink(0).Border(1, border).Background(lb).Center().Children(func() { ui.Text(c, string(rune('A'+display))).FontSize(12).FontWeight(600).TextColor(lf) })
							ui.Text(c, q.Options[original]).Grow(1).MinWidth(0).FontSize(14).LineHeight(1.625).TextColor(fg).Margin(2, 0, 0, 0)
						})
					})
				}
			})
		}
		if a.examMessage != "" {
			ui.Text(c, a.examMessage).FontSize(12).TextColor(t.Danger).Margin(12, 0, 0, 0)
		}
		ui.Row(c).FillWidth().Margin(32, 0, 0, 0).Gap(8).Children(func() {
			control(c, "prev", "上一题", nil, outlineButton, func() { e.Current--; e.preview = false; a.examMessage = "" }).Height(40).Disabled(idx == 0)
			control(c, "next", "下一题", nil, outlineButton, func() { e.Current++; e.preview = false; a.examMessage = "" }).Height(40).Disabled(idx == len(e.Questions)-1)
			ui.Spacer(c)
			control(c, "submit", "交卷", nil, primaryButton, a.requestExamSubmit).Height(40)
		})
	})
}
func (a *app) examResult(c *ui.Context, e *examSession) {
	t := c.Theme()
	n := len(e.Questions)
	ratio := float32(e.Score) / float32(n)
	color := t.Danger
	if ratio >= .6 {
		color = t.Success
	}
	ui.Scroll(c.Key("exam-result-scroll")).FillWidth().Grow(1).MinHeight(0).Background(t.Background).Children(func() {
		ui.Column(c.Key("exam-result")).FillWidth().MaxWidth(resultWidth(e)).AlignSelf(ui.Center).Padding(24, 32).Gap(24).Children(func() {
			ui.Row(c).FillWidth().Children(func() {
				control(c, "back", "返回题库", nil, ghostButton, a.exitExam)
				ui.Spacer(c)
				label := "保存成绩"
				if e.Saved {
					label = "已保存"
				} else if a.examSaving {
					label = "保存中…"
				}
				control(c, "save", label, downloadIcon, outlineButton, a.saveExam).Disabled(e.Saved || a.examSaving)
			})
			ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(16).Children(func() {
				ui.Box(c).Size(144, 144).Center().Draw(func(p *ui.Painter, r ui.Rect) {
					cx, cy := r.X+72, r.Y+72
					circle := &ui.Path{}
					circle.Circle(cx, cy, 62.4)
					p.StrokePath(circle, 7.2, t.Border)
					var points [][2]float32
					for i := 0; i <= 120; i++ {
						angle := -math.Pi/2 + float64(i)/120*float64(ratio)*2*math.Pi
						points = append(points, [2]float32{cx + 62.4*float32(math.Cos(angle)), cy + 62.4*float32(math.Sin(angle))})
					}
					if ratio > 0 {
						p.StrokePolyline(points, 7.2, color)
					}
				}).Children(func() {
					ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
						ui.Text(c, fmt.Sprint(e.Score)).FontSize(36).FontWeight(700)
						ui.Text(c, fmt.Sprintf("/ %d", n)).FontSize(14).TextColor(t.TextMuted)
					})
				})
				ui.Text(c, e.gradeText()).FontSize(18).FontWeight(600).TextColor(color)
				ui.Text(c, fmt.Sprintf("正确率 %d%% · 答对 %d 题 · 答错 %d 题", e.percentage(), e.Correct, n-e.Correct)).FontSize(14).TextColor(t.TextMuted)
				if e.ImportantFailed {
					ui.Text(c, "重要题目答错或未作答，本次综合成绩归零。").FontSize(12).TextColor(t.Danger)
				}
				if e.TimedOut {
					ui.Text(c, "考试时间已到，试卷已自动提交。").FontSize(12).TextColor(t.TextMuted)
				}
			})
			ui.Row(c).FillWidth().Height(8).Gap(6).Children(func() {
				if e.Score > 0 {
					ui.Box(c).Grow(float32(e.Score)).Height(8).Background(t.Success)
				}
				if n-e.Correct > 0 {
					ui.Box(c).Grow(float32(n - e.Correct)).Height(8).Background(t.Danger)
				}
				if remainder := e.Correct - e.Score; remainder > 0 {
					ui.Box(c).Grow(float32(remainder)).Height(8)
				}
			})
			if a.examMessage != "" {
				ui.Text(c, a.examMessage).TextColor(t.Danger).FontSize(12)
			}
			if e.Correct == n {
				ui.Text(c, "没有错题，继续保持！").FontSize(14).TextColor(t.TextMuted).AlignSelf(ui.Center)
			} else {
				ui.Text(c, "错题回顾").FontSize(16).FontWeight(600)
				w, _ := c.Size()
				var wrong []int
				for i, r := range e.Responses {
					if !r.Correct {
						wrong = append(wrong, i)
					}
				}
				cols := 1
				if w >= 1000 {
					cols = 2
				}
				for start := 0; start < len(wrong); start += cols {
					ui.Row(c.Key(fmt.Sprint(start))).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
						for _, i := range wrong[start:min(start+cols, len(wrong))] {
							ui.Column(c.Key(fmt.Sprint(i))).Grow(1).MinWidth(0).Children(func() { reviewQuestion(c, e.Questions[i], e.Responses[i], e.Important[i], i+1) })
						}
						if cols == 2 && start+1 == len(wrong) {
							ui.Box(c).Grow(1).MinWidth(0)
						}
					})
				}
			}
		})
	})
	if e.Score == n && !e.Finished.IsZero() {
		drawConfetti(c, e.Finished)
	}
}
func reviewQuestion(c *ui.Context, q question, r response, important bool, index int) {
	t := c.Theme()
	border := t.Border
	if important {
		border = t.Danger.Alpha(.7)
	}
	ui.Column(c).FillWidth().Padding(16).Gap(12).Border(1, border).Children(func() {
		ui.Row(c).Gap(6).Wrap().Children(func() {
			ui.Text(c, fmt.Sprintf("%d · %s", index, q.Kind)).FontSize(11).TextColor(t.TextMuted)
			if important {
				ui.Text(c, "重要题目").FontSize(11).TextColor(t.Danger)
			}
		})
		ui.Text(c, q.Prompt).FillWidth().FontSize(14).FontWeight(600).LineHeight(1.625)
		if !isTextQuestion(q) {
			for display, original := range responseOrder(q, r) {
				fg := t.Text
				if q.Answer&(1<<original) != 0 {
					fg = t.Success
				} else if r.Choice&(1<<original) != 0 {
					fg = t.Danger
				}
				ui.Text(c, fmt.Sprintf("%c. %s", rune('A'+display), q.Options[original])).FillWidth().FontSize(13).LineHeight(1.6).TextColor(fg)
			}
		}
		given, expected := responseLetters(q, r, r.Choice), responseLetters(q, r, q.Answer)
		if isTextQuestion(q) {
			given, expected = r.Text, q.TextAnswer
		}
		if given == "" {
			given = "未作答"
		}
		ui.Text(c, "你的答案："+given).FillWidth().FontSize(12).TextColor(t.Danger)
		ui.Text(c, "正确答案："+expected).FillWidth().FontSize(12).TextColor(t.Success)
		if q.Explanation != "" {
			ui.Text(c, "解析："+q.Explanation).FillWidth().FontSize(12).LineHeight(1.6).TextColor(t.TextMuted)
		}
	})
}
func resultWidth(e *examSession) float32 {
	if e.Correct == len(e.Questions) {
		return 640
	}
	return 900
}
func smallScoreRing(c *ui.Context, e *examSession) {
	t := c.Theme()
	ratio := float32(e.Score) / float32(len(e.Questions))
	col := t.Danger
	if ratio >= .8 {
		col = t.Success
	} else if ratio >= .6 {
		col = ui.Hex("#c58b2e")
	}
	ui.Box(c).Size(52, 52).Shrink(0).Center().Draw(func(p *ui.Painter, r ui.Rect) {
		circle := &ui.Path{}
		circle.Circle(r.X+26, r.Y+26, 21)
		p.StrokePath(circle, 4.5, t.Border)
		if ratio > 0 {
			var points [][2]float32
			for i := 0; i <= 80; i++ {
				angle := -math.Pi/2 + float64(i)/80*float64(ratio)*2*math.Pi
				points = append(points, [2]float32{r.X + 26 + 21*float32(math.Cos(angle)), r.Y + 26 + 21*float32(math.Sin(angle))})
			}
			p.StrokePolyline(points, 4.5, col)
		}
	}).Children(func() { ui.Text(c, fmt.Sprintf("%d%%", e.percentage())).FontSize(11).FontWeight(700) })
}
func (a *app) historyView(c *ui.Context) {
	t := c.Theme()
	_, h := c.Size()
	ui.DialogBase(c.Key("history-drawer"), &a.historyOpen, func(back, panel ui.Element) {
		back.Top(titleHeight).Background(ui.RGBA(0, 0, 0, .1))
		panel.Label("我的记录面板").Absolute().Top(0).Right(0).Width(380).Height(max(0, h-titleHeight)).Padding(0).Radius(0).Background(t.Background).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Shadow(-6, 0, 20, 0, ui.RGBA(0, 0, 0, .1)).Children(func() {
			ui.Row(c).FillWidth().Padding(16, 20).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.5)).Children(func() {
				ui.Icon(c, historyIcon).Size(14, 14).TextColor(t.Accent)
				ui.Text(c, "我的记录").FontSize(14).FontWeight(600)
				ui.Spacer(c)
				control(c, "close", "", closeIcon, ghostButton, func() { a.historyOpen = false }).Label("关闭我的").Size(28, 28).Padding(7)
			})
			ui.Scroll(c.Key("history-scroll")).FillWidth().Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).FillWidth().Padding(16).Gap(12).Children(func() {
					if a.historyLoading {
						ui.Text(c, "正在读取记录…").FontSize(12).TextColor(t.TextMuted)
					}
					if a.historyMessage != "" {
						ui.Text(c, a.historyMessage).FontSize(12).TextColor(t.Danger).FillWidth()
					}
					if !a.historyLoading && len(a.history) == 0 {
						ui.Column(c).FillWidth().Padding(48, 16).AlignItems(ui.Center).Gap(20).Children(func() {
							ui.Box(c).Size(80, 80).Radius(40).Border(1, t.Border).BorderStyle(ui.BorderDashed).Center().Children(func() { ui.Icon(c, historyIcon).Size(28, 28).TextColor(t.TextMuted.Alpha(.4)) })
							ui.Text(c, "暂无保存记录").FontSize(14).FontWeight(500).TextColor(t.TextMuted)
							ui.Text(c, "考试成绩或错题本点击保存后，会在这里展开查看").FillWidth().MaxWidth(240).FontSize(12).TextAlign(ui.Center).TextColor(t.TextMuted).LineHeight(1.625)
						})
					}
					for _, isExam := range []bool{false, true} {
						count := 0
						for _, r := range a.history {
							if (r.Exam != nil) == isExam {
								count++
							}
						}
						if count == 0 {
							continue
						}
						heading := "保存的错题本"
						if isExam {
							heading = "考试记录"
						}
						ui.Text(c, heading).FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(0, 4)
						for _, record := range a.history {
							if (record.Exam != nil) != isExam {
								continue
							}
							expanded := a.historyExpanded == record.Key
							border, bg := t.Border.Alpha(.6), t.Background
							if expanded {
								border = t.Accent.Alpha(.3)
								bg = t.Accent.Alpha(.02)
							}
							ui.Column(c.Key(record.Key)).FillWidth().Border(1, border).Background(bg).Children(func() {
								ui.Row(c).FillWidth().Padding(16).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.ButtonBase(c.Key("expand")).Label("展开记录：" + record.Key).Grow(1).MinWidth(0).OnClick(func() {
										if a.historyExpanded == record.Key {
											a.historyExpanded = ""
										} else {
											a.historyExpanded = record.Key
										}
									}).Children(func() {
										ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
											if e := record.Exam; e != nil {
												smallScoreRing(c, e)
												ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
													ui.Text(c, e.Bank).FontSize(14).FontWeight(500).SingleLine()
													ui.Text(c, fmt.Sprintf("%s · %d分钟", e.Finished.Local().Format("2006-01-02 15:04"), e.Minutes)).FontSize(10).TextColor(t.TextMuted)
													ui.Row(c).Gap(8).Wrap().Margin(2, 0, 0, 0).Children(func() {
														ui.Text(c, fmt.Sprintf("%d/%d 正确", e.Score, len(e.Questions))).FontSize(10).FontWeight(500).TextColor(t.Success)
														if wrong := len(e.Questions) - e.Correct; wrong > 0 {
															ui.Text(c, fmt.Sprintf("%d 错误", wrong)).FontSize(10).FontWeight(500).TextColor(t.Danger)
														}
													})
												})
											} else {
												ui.Box(c).Size(48, 48).Shrink(0).Background(t.Danger.Alpha(.04)).Border(1, t.Danger.Alpha(.25)).Center().Children(func() {
													ui.Column(c).AlignItems(ui.Center).Gap(2).Children(func() {
														ui.Text(c, fmt.Sprint(len(record.Mistakes))).FontSize(14).FontWeight(700).TextColor(t.Danger)
														ui.Text(c, "错题").FontSize(9).TextColor(t.Danger)
													})
												})
												title := "错题本"
												if len(record.Mistakes) > 0 {
													title = record.Mistakes[0].Question.Bank + " · 错题本"
												}
												ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
													ui.Text(c, title).FontSize(14).FontWeight(500).SingleLine()
													ui.Text(c, fmt.Sprintf("%d 题", len(record.Mistakes))).FontSize(10).TextColor(t.TextMuted)
													if record.Error != "" {
														ui.Text(c, record.Error).FontSize(10).TextColor(t.Danger)
													}
												})
											}
											arrow := ui.Icon(c, rightIcon).Size(12, 12).Shrink(0).TextColor(t.TextMuted)
											if expanded {
												arrow.Rotate(90)
											}
										})
									})
									control(c, "delete", "", closeIcon, ghostButton, func() { a.historyDeleteKey = record.Key; a.historyDeleteOpen = true }).Label("删除记录："+record.Key).Size(20, 20).Padding(4)
								})
								if expanded {
									ui.Column(c).FillWidth().Padding(16).Gap(10).BorderWidth(1, 0, 0, 0).BorderColor(t.Border.Alpha(.3)).Children(func() {
										if e := record.Exam; e != nil {
											if e.Correct == len(e.Questions) {
												ui.Text(c, "满分通过！").FontSize(12).FontWeight(500).TextColor(t.Success)
											} else {
												ui.Text(c, "错题回顾").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
												for i, q := range e.Questions {
													if !e.Responses[i].Correct {
														reviewQuestion(c, q, e.Responses[i], e.Important[i], i+1)
													}
												}
											}
										} else {
											for i, item := range record.Mistakes {
												reviewQuestion(c, item.Question, item.Response, false, i+1)
											}
										}
									})
								}
							})
						}
					}
				})
			})
		})
	})
	nativeModal(c.Key("history-delete"), &a.historyDeleteOpen, func() {
		ui.Text(c, "删除记录").FontSize(18).FontWeight(600)
		ui.Text(c, "确定删除这条已保存的记录？").FontSize(14)
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			control(c, "cancel", "取消", nil, outlineButton, func() { a.historyDeleteOpen = false })
			control(c, "confirm", "确认删除", nil, primaryButton, a.deleteHistory)
		})
	}).Width(440)
}

// The celebration repaints for 1.2 seconds only; it schedules no idle frames.
func drawConfetti(c *ui.Context, start time.Time) {
	if elapsed := c.Now().Sub(start); elapsed < 0 || elapsed >= 1200*time.Millisecond {
		return
	}
	colors := []string{"#4a7dbf", "#5d9b6a", "#c2655a", "#e6b95c", "#8b72b5", "#e4946c", "#6ba8a8", "#db8f9b"}
	type particle struct {
		x, y, vx, vy, angle, size float32
		color                     ui.Color
	}
	rng := rand.New(rand.NewSource(start.UnixNano()))
	ps := make([]particle, 120)
	for i := range ps {
		ps[i] = particle{rng.Float32(), rng.Float32()*.4 - 0.4, (rng.Float32() - .5) * 5, rng.Float32()*3 + 1, rng.Float32() * 360, rng.Float32()*4 + 3, ui.Hex(colors[i%len(colors)])}
	}
	ui.Box(c.Key("confetti")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
		elapsed := p.Now().Sub(start)
		if elapsed < 0 || elapsed >= 1200*time.Millisecond {
			return
		}
		frames := float32(elapsed.Seconds() * 60)
		alpha := float32(1)
		progress := float32(elapsed) / float32(1200*time.Millisecond)
		if progress > .7 {
			alpha = (1 - progress) / .3
		}
		for _, v := range ps {
			x := r.X + v.x*r.W + v.vx*frames
			y := r.Y + v.y*r.H + v.vy*frames + .025*frames*frames
			p.Transform(ui.Rotation(v.angle+frames*3), x, y, func() { p.Fill(ui.Rect{X: x, Y: y, W: v.size, H: v.size * .5}, v.color.Alpha(alpha), 0) })
		}
		p.AnimationFrame()
	})
}
