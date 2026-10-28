package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"strconv"
	"strings"
)

func (a *app) examSetupDialog(c *ui.Context) {
	t := c.Theme()
	_, h := c.Size()
	nativeModal(c.Key("exam-setup"), &a.examSetupOpen, func() {
		status := a.examSetupStatus()
		message := status.Problem
		if message == "" {
			message = a.examMessage
		}
		formHeight := max(float32(160), h-244)
		if message != "" {
			formHeight = max(120, formHeight-40)
		}
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Column(c).FillWidth().Gap(6).Padding(0, 24, 0, 0).Children(func() {
				ui.Text(c, "新建考试").FontSize(18).FontWeight(600)
				ui.Text(c, "选择题库，设置题量与作答时间。").FontSize(12).TextColor(t.TextMuted)
			})
			ui.Scroll(c.Key("exam-setup-fields")).FillWidth().MaxHeight(formHeight).MinHeight(0).Children(func() {
				ui.Column(c).FillWidth().Gap(20).Children(func() {
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						ui.Text(c, "题库").FontSize(13).FontWeight(500)
						a.examBankSelect(c)
						if len(a.bankNames()) == 0 {
							ui.Text(c, "暂无题库，请先上传题库。").FontSize(11).TextColor(t.TextMuted)
						}
					})
					ui.Column(c).FillWidth().Gap(10).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
							ui.Text(c, "各题型数量").FontSize(13).FontWeight(500)
							ui.Spacer(c)
							ui.Text(c, fmt.Sprintf("实际抽取 %d 题", status.Actual)).FontSize(11).TextColor(t.TextMuted)
						})
						// Grid keeps the last two fields the same width as the first three.
						ui.Grid(c.Key("exam-count-grid")).FillWidth().Columns(3).Gap(12).Children(func() {
							for i, kind := range examKinds {
								ui.Column(c.Key(kind)).MinWidth(0).Gap(6).Children(func() {
									available := status.Available[i]
									ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
										ui.Text(c, kind).FontSize(11).FontWeight(500)
										ui.Spacer(c)
										ui.Text(c, fmt.Sprintf("共%d", available)).FontSize(10).TextColor(t.TextMuted)
									})
									examNumberField(c.Key("number"), "考试"+kind+"数量", &a.examForm.Counts[i], 0, 100, "题", a.examFormEdited)
								})
							}
						})
						if status.Shortage {
							ui.Text(c, "题量不足时取该题型的全部题目。").FontSize(11).TextColor(t.TextMuted).FillWidth().LineHeight(1.5)
						}
					})
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						ui.Text(c, "时长（分钟）").FontSize(13).FontWeight(500)
						examNumberField(c.Key("exam-minutes"), "考试时长", &a.examForm.Minutes, 1, 180, "分钟", a.examFormEdited)
						ui.Text(c, "可设置 1–180 分钟").FontSize(11).TextColor(t.TextMuted)
					})
				})
			})
			if message != "" {
				color := t.Danger
				if a.paperBusy {
					color = t.TextMuted
				}
				ui.Text(c, message).FillWidth().FontSize(12).LineHeight(1.5).TextColor(color)
			}
			ui.Row(c.Key("exam-setup-actions")).FillWidth().Padding(16, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border.Alpha(.6)).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					examExportButton(c, "paper", "试卷", downloadIcon, func() { a.exportExamPDF(false) }).Disabled(a.paperBusy || !status.Valid)
					examExportButton(c, "answers", "答案", checkIcon, func() { a.exportExamPDF(true) }).Disabled(a.paperBusy || !status.Valid)
					ui.Text(c, "Beta").FontSize(9).FontWeight(600).Padding(1, 4).Background(t.TextMuted.Alpha(.08)).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				control(c, "cancel", "取消", nil, outlineButton, func() { a.examSetupOpen = false }).Height(36).MinWidth(64).Padding(0, 14).Center()
				control(c, "start", "开始考试", nil, primaryButton, a.startExam).Label("开始考试").Height(36).MinWidth(92).Padding(0, 16).Center().Disabled(a.paperBusy || !status.Valid)
			})
		})
		dialogClose(c, "setup-close", &a.examSetupOpen)
	}).Width(440).Gap(0)
}
func (a *app) examBankSelect(c *ui.Context) {
	t := c.Theme()
	sel := ui.SelectBase(c.Key("exam-bank"), &a.examForm.Bank)
	trigger := sel.Trigger.OnChange(a.examFormEdited).Label("考试题库").FillWidth().MinWidth(0).Height(38).Padding(0, 12).Border(1, t.Border).Background(t.Background).Radius(0)
	if trigger.Hovered() {
		trigger.BorderColor(t.TextMuted.Alpha(.5))
	}
	if trigger.Focused() || sel.Open() {
		trigger.BorderColor(t.Accent)
	}
	trigger.Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(8).Children(func() {
			name := a.examForm.Bank
			fg := t.Text
			if name == "" {
				name = "请选择题库"
				fg = t.TextMuted
			}
			ui.Text(c, name).Grow(1).MinWidth(0).SingleLine().FontSize(13).TextColor(fg)
			ui.Icon(c, rightIcon).Rotate(90).Size(12, 12).Shrink(0).TextColor(t.TextMuted)
		})
	})
	sel.Popup(func(panel ui.Element) {
		panel.Padding(4).Border(1, t.Border).Radius(0).Background(t.Background).Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, .12))
		ui.Scroll(c).FillWidth().MaxHeight(200).Children(func() {
			for _, bank := range a.bankNames() {
				item := sel.Item(bank).Label("选择考试题库："+bank).FillWidth().Padding(8, 10)
				if item.Highlighted() {
					item.Background(t.Surface)
				}
				item.Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, bank).Grow(1).MinWidth(0).SingleLine().FontSize(13)
						if bank == a.examForm.Bank {
							ui.Icon(c, checkIcon).Size(13, 13).Shrink(0).TextColor(t.Accent)
						}
					})
				})
			}
		})
	})
}
func examExportButton(c *ui.Context, key, label string, symbol *ui.SVG, fn func()) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Label(label).Height(28).Padding(0, 6).TextColor(t.TextMuted).OnClick(fn)
	if b.Hovered() {
		b.Background(t.Surface).TextColor(t.Text)
	}
	b.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() { ui.Icon(c, symbol).Size(12, 12); ui.Text(c, label).FontSize(11) })
	})
	return b
}

type examNumberEdit struct {
	Text  string
	Value float64
}

func sameExamNumber(a, b float64) bool { return a == b || math.IsNaN(a) && math.IsNaN(b) }
func examInteger(text string, lo, hi float64) float64 {
	v, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || float64(v) < lo || float64(v) > hi {
		return math.NaN()
	}
	return float64(v)
}

// Plain fields avoid the stock NumberInput's fixed-width text and extra buttons.
// The range, step, text editing and Up/Down keys remain native and accessible.
func examNumberField(c *ui.Context, label string, value *float64, lo, hi float64, suffix string, onEdit func()) {
	t := c.Theme()
	row := ui.Row(c).Label(label+"输入框").FillWidth().MinWidth(0).Height(36).Padding(0, 10).Gap(8).AlignItems(ui.Center).Border(1, t.Border).Background(t.Background)
	edit := ui.Local(row, "edit", func() examNumberEdit {
		return examNumberEdit{Text: strconv.FormatFloat(*value, 'f', 0, 64), Value: *value}
	})
	if !sameExamNumber(edit.Value, *value) {
		edit.Text = strconv.FormatFloat(*value, 'f', 0, 64)
		edit.Value = *value
	}
	row.Children(func() {
		numeric := *value
		if math.IsNaN(numeric) {
			numeric = lo
		}
		input := ui.TextInputBase(c.Key("input"), &edit.Text).Label(label).Grow(1).MinWidth(0).Height(34).Padding(0).FontSize(13).Range(lo, hi, numeric).Step(1)
		if input.Changed() {
			*value = examInteger(edit.Text, lo, hi)
			edit.Value = *value
			if onEdit != nil {
				onEdit()
			}
		}
		adjust := func(delta float64) {
			v := *value
			if math.IsNaN(v) {
				v = lo
			}
			v = max(lo, min(hi, v+delta))
			*value = v
			edit.Value = v
			edit.Text = strconv.FormatFloat(v, 'f', 0, 64)
			if onEdit != nil {
				onEdit()
			}
		}
		if input.Shortcut(0, ui.KeyUp) {
			adjust(1)
		}
		if input.Shortcut(0, ui.KeyDown) {
			adjust(-1)
		}
		if input.Focused() {
			row.BorderColor(t.Accent)
		} else if row.Hovered() {
			row.BorderColor(t.TextMuted.Alpha(.5))
		}
		if math.IsNaN(*value) {
			row.BorderColor(t.Danger)
		}
		ui.Text(c, suffix).FontSize(11).Shrink(0).TextColor(t.TextMuted)
	})
}
