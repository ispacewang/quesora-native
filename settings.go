// 设置和数据操作视图；原生文件对话框与磁盘 I/O 在后台运行，状态仅通过 publish 回写。

package main

import (
	"github.com/egoist/mygo/ui"
	"path/filepath"
)

func (a *app) openSettings() {
	a.settingsOpen = true
	if !a.backupBusy {
		a.backupMessage = ""
		a.backupError = false
	}
}

// backupData 在文件选择后排队保存当前设置和数据快照，使已入队答案先于备份提交。
func (a *app) backupData() {
	if a.backupBusy || a.store == nil || a.pickBackup == nil || a.restoreBusy {
		return
	}
	a.backupBusy, a.backupError = true, false
	a.backupMessage = "请选择备份保存位置…"
	pick := a.pickBackup
	go func() {
		filename, err := pick()
		a.publish(func() {
			if err != nil {
				a.backupBusy = false
				a.backupError = true
				a.backupMessage = "无法选择位置：" + err.Error()
				return
			}
			if filename == "" {
				a.backupBusy = false
				a.backupMessage = ""
				return
			}
			if filepath.Ext(filename) == "" {
				filename += ".db"
			}
			a.backupMessage = "正在备份全部题库与记录…"
			a.persistPreferences()
			store := a.store
			if !a.enqueue(func() {
				err := store.backupTo(filename)
				a.publish(func() {
					a.backupBusy = false
					a.backupError = err != nil
					if err != nil {
						a.backupMessage = "备份失败：" + err.Error()
					} else {
						a.backupMessage = "备份已保存：" + filename
					}
				})
			}) {
				a.backupBusy = false
				a.backupError = true
				a.backupMessage = "后台任务繁忙，请稍后重试。"
			}
		})
	}()
}

func (a *app) openDataFolder() {
	if a.store == nil || a.showDataFolder == nil {
		return
	}
	show := a.showDataFolder
	go func() {
		if err := show(); err != nil {
			a.publish(func() { a.backupError = true; a.backupMessage = "无法打开数据目录：" + err.Error() })
		}
	}()
}

func settingRow(c *ui.Context, label, description string, active bool, toggle func()) {
	t := c.Theme()
	ui.Row(c.Key(label)).FillWidth().Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(500)
			ui.Text(c, description).FillWidth().FontSize(11).LineHeight(1.5).TextColor(t.TextMuted)
		})
		text, style := "已关闭", outlineButton
		if active {
			text, style = "已开启", primaryButton
		}
		control(c, "toggle", text, nil, style, toggle).Label("设置：" + label).Width(72).Shrink(0)
	})
}

// settingsDialog 固定底部关闭操作，正文独立滚动，使最小窗口仍可到达全部数据操作。
func (a *app) settingsDialog(c *ui.Context) {
	t := c.Theme()
	_, height := c.Size()
	nativeModal(c.Key("settings-dialog"), &a.settingsOpen, func() {
		ui.Column(c).FillWidth().Padding(20, 24).Gap(4).Children(func() {
			ui.Text(c, "设置与数据").FontSize(18).FontWeight(600)
			ui.Text(c, "Quesora Native · v"+appVersion).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Scroll(c.Key("settings-scroll")).FillWidth().MaxHeight(max(120, height-210)).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 24, 20).Gap(20).Children(func() {
				ui.Text(c, "外观与学习").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
				settingRow(c, "深色模式", "切换界面配色，关闭程序后保留选择。", a.dark, a.toggleTheme)
				settingRow(c, "重新作答", "重新进入已答题时隐藏旧答案，提交后更新记录。", a.repeatPractice, a.toggleRepeatPractice)
				settingRow(c, "随机题序", "本轮随机排列题目，一轮内不重复。", a.random, a.toggleRandom)
				settingRow(c, "速刷", "提交答案后自动进入下一题。", a.quick, a.toggleQuick)
				settingRow(c, "打乱选项", "随机调整选择题的选项顺序。", a.shuffle, a.toggleShuffle)
				line(c)
				ui.Text(c, "数据与备份").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
				ui.Text(c, "题库、答题记录、已保存的考试和学习设置会一起备份。").FillWidth().FontSize(12).LineHeight(1.6)
				if a.store != nil {
					ui.Text(c, filepath.Dir(a.store.path)).Label("当前数据目录").FillWidth().FontSize(11).TextColor(t.TextMuted).Selectable()
				} else {
					ui.Text(c, "数据目录暂不可用，请检查启动时的错误提示。").FillWidth().FontSize(12).TextColor(t.Danger)
				}
				ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
					control(c, "folder", "打开数据目录", bookIcon, outlineButton, a.openDataFolder).Label("打开数据目录").Disabled(a.store == nil || a.showDataFolder == nil)
					text := "备份全部数据"
					if a.backupBusy {
						text = "正在备份…"
					}
					control(c, "backup", text, downloadIcon, primaryButton, a.backupData).Label("备份全部数据").Disabled(a.store == nil || a.backupBusy || a.importBusy || a.restoreBusy)
					control(c, "restore", "从备份恢复…", historyIcon, outlineButton, a.chooseRestoreBackup).Label("从备份恢复").Disabled(a.store == nil || a.restoreBusy || a.restorePicking || a.backupBusy || a.importBusy || a.exam != nil)
				})
				if a.backupMessage != "" {
					color := t.TextMuted
					if a.backupError {
						color = t.Danger
					}
					ui.Text(c, a.backupMessage).FillWidth().FontSize(12).LineHeight(1.5).TextColor(color).Selectable()
				}
				control(c, "reset", "重置练习记录…", nil, ghostButton, func() { a.settingsOpen = false; a.resetPracticeOpen = true }).TextColor(t.Danger).Disabled(a.store == nil || a.backupBusy || a.importBusy || a.exam != nil)
			})
		})
		ui.Row(c).FillWidth().Padding(16, 24).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Justify(ui.End).Children(func() {
			control(c, "close", "完成", nil, outlineButton, func() { a.settingsOpen = false }).Label("关闭设置").Height(32)
		})
	}).Width(520).Padding(0).Gap(0).Disabled(a.restoreBusy)
}
