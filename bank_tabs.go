package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

func (a *app) selectBank(name string) {
	a.savedWrong = false
	a.exportOpen = false
	if a.bank == name {
		a.bank = ""
	} else {
		a.bank = name
	}
	a.onlyWrong = false
	a.filter = "全部"
	a.filters = nil
	a.importantOnly = false
	a.reorder()
	a.navigate(0)
}
func (a *app) bankBar(c *ui.Context) {
	t := c.Theme()
	names := a.bankNames()
	bar := ui.Row(c.Key("bank")).Label("题库标签栏").FillWidth().Height(54).Shrink(0).Padding(10, 20).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.5))
	position := ui.Local(bar, "bank-scroll", func() ui.ScrollState { return ui.ScrollState{} })
	// Handle the wheel only in this strip; do not scroll the question beneath it.
	bar.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputScroll || ev.Mods&ui.Ctrl != 0 || position.MaxX <= 0 {
			return false
		}
		delta := ev.DX
		if delta == 0 {
			delta = ev.DY
		}
		if delta == 0 {
			return false
		}
		position.X = max(0, min(position.X+delta, position.MaxX))
		return true
	})
	bar.Children(func() {
		scroller := ui.ScrollHorizontal(c.Key("bank-tabs")).Label("题库标签滚动区域").TrackScroll(position).Grow(1).MinWidth(0).Height(32).Gap(8).AlignItems(ui.Center)
		revealed := ui.Local(scroller, "revealed-bank", func() string { return "" })
		if a.bank == "" {
			*revealed = ""
		}
		scroller.Children(func() {
			for _, name := range names {
				selected := a.bank == name
				tab := ui.Row(c.Key("tab-"+name)).Label("题库标签："+name).Height(30).Shrink(0).Border(1, ui.Transparent).Background(t.Surface.Alpha(.5)).AlignItems(ui.Center)
				if selected {
					tab.BorderColor(t.Accent).Background(t.Accent.Alpha(.15))
					if *revealed != name {
						tab.ScrollIntoView()
						*revealed = name
					}
				}
				tab.Children(func() {
					button := ui.ButtonBase(c.Key("select-"+name)).Label("选择题库："+name).Tooltip(name).Height(28).Padding(0, 0, 0, 12).TextColor(t.TextMuted).OnClick(func() { a.selectBank(name) })
					if selected {
						button.TextColor(t.Accent)
					}
					button.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
							ui.Icon(c, bookIcon).Size(14, 14).Shrink(0)
							ui.Text(c, name).FontSize(12).FontWeight(500).MaxWidth(136).SingleLine()
						})
					})
					control(c, "delete-"+name, "", closeIcon, ghostButton, func() { a.deleteName = name; a.deleteOpen = true }).Label("删除题库："+name).Tooltip("删除“"+name+"”").Size(28, 28).Padding(7).Disabled(a.importBusy || a.store == nil)
				})
			}
		})
		if len(names) > 3 {
			control(c, "all-banks", fmt.Sprintf("全部 %d ▾", len(names)), nil, outlineButton, nil).Label("全部题库").Height(30).Shrink(0).Tooltip("查看并切换全部题库").Menu(func(menu *ui.Menu) {
				for _, name := range names {
					if menu.Item(name).Checked(a.bank == name).Chosen() {
						*revealed = ""
						if a.bank != name {
							a.selectBank(name)
						}
					}
				}
			})
		}
		control(c, "upload-bank", "上传题库", plusIcon, ghostButton, a.openImport).Label("上传题库").Height(30).Shrink(0).Padding(0, 12).Border(1, t.TextMuted.Alpha(.20)).BorderStyle(ui.BorderDashed).Background(t.Surface.Alpha(.30)).Disabled(a.importBusy)
	})
}
