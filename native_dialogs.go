package main

import (
	"github.com/egoist/mygo/ui"
	"strings"
)

func dialogClose(c *ui.Context, key string, open *bool) {
	control(c, key, "", closeIcon, ghostButton, func() { *open = false }).Label("关闭弹窗").Absolute().Right(16).Top(16).Size(16, 16).Padding(0)
}
func (a *app) libraryDialogs(c *ui.Context) {
	t := c.Theme()
	nativeModal(c.Key("import-dialog"), &a.importOpen, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Text(c, "上传题库").FontSize(18).FontWeight(600).LineHeight(1)
			ui.RichText(c).FillWidth().FontSize(14).LineHeight(1.43).TextColor(t.TextMuted).Children(func() {
				ui.Text(c, "支持 Excel/CSV，多 sheet 分别生成题库。自动识别表头、顶部说明、分列选项和五种题型；无法确定的数据会提示来源行。 ")
				ui.Text(c, "题库模板").TextColor(t.Accent).Underline().Label("题库模板").Focusable().Disabled(a.importBusy).OnClick(a.saveTemplate)
			})
			zone := ui.ButtonBase(c.Key("import-drop")).Label("选择题库文件").FillWidth().Padding(32, 16).Border(2, t.Border).BorderStyle(ui.BorderDashed).Center().Disabled(a.importBusy).OnClick(a.pickQuestionFile)
			if zone.Hovered() || zone.FileDragOver() {
				zone.BorderColor(t.Accent.Alpha(.5)).Background(t.Accent.Alpha(.05))
			}
			zone.Children(func() {
				ui.Column(c).AlignItems(ui.Center).Gap(16).Children(func() {
					ui.Icon(c, uploadIcon).Size(32, 32).TextColor(t.Accent.Alpha(.6))
					ui.RichText(c, ui.Span{Text: "拖入文件，或 ", Color: t.TextMuted}, ui.Span{Text: "点击上传", Color: t.Accent, Weight: 500}).FontSize(14)
				})
			})
			if paths := zone.DroppedFiles(); len(paths) > 0 && !a.importBusy {
				a.startImport(append([]string(nil), paths...))
			}
			if a.importMessage != "" {
				color := t.TextMuted
				if a.importError {
					color = t.Danger
				}
				ui.Text(c, a.importMessage).FillWidth().FontSize(12).TextColor(color).Selectable()
			}
			if len(a.importWarnings) > 0 {
				ui.Scroll(c.Key("import-warnings")).FillWidth().MaxHeight(100).Children(func() {
					ui.Text(c, strings.Join(a.importWarnings, "\n")).FontSize(11).TextColor(t.TextMuted).FillWidth()
				})
			}
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				label := "取消"
				if a.importBusy {
					label = "后台处理"
				}
				control(c, "close-import", label, nil, outlineButton, func() { a.importOpen = false }).Label("关闭").Height(36).Padding(0, 16)
			})
		})
		dialogClose(c, "close-import-x", &a.importOpen)
	}).Width(448).Radius(0).Padding(24).Border(1, t.Border)
	nativeModal(c.Key("delete-dialog"), &a.deleteOpen, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Text(c, "删除题库").FontSize(18).FontWeight(600)
			ui.Text(c, "删除“"+a.deleteName+"”及其答题记录？原始 Excel / CSV 文件会保留。").FillWidth().FontSize(14).TextColor(t.TextMuted)
			ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
				control(c, "cancel-delete", "取消", nil, outlineButton, func() { a.deleteOpen = false }).Disabled(a.importBusy)
				control(c, "confirm-delete", "确认删除", nil, primaryButton, a.deleteSelectedBank).Disabled(a.importBusy)
			})
		})
		dialogClose(c, "close-delete-x", &a.deleteOpen)
	}).Width(440).Radius(0).Padding(24).Border(1, t.Border)
}

func nativeModal(c *ui.Context, open *bool, fn func()) ui.Element {
	w, h := c.Size()
	return ui.DialogBase(c, open, func(back, panel ui.Element) {
		back.Background(ui.RGBA(0, 0, 0, .5))
		panel.Padding(24).Gap(16).Radius(0).Background(c.Theme().Background).Border(1, c.Theme().Border).MaxWidth(w-40).MaxHeight(h-40).Shadow(0, 10, 24, 0, ui.RGBA(0, 0, 0, .15))
		fn()
	})
}
