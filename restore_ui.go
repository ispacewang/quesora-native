// 恢复流程的原生确认和后台编排。选择文件不写数据，确认后才执行仓库恢复。
package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

// chooseRestoreBackup 仅选择候选文件；实际校验和覆盖要等用户确认。
func (a *app) chooseRestoreBackup() {
	if a.store == nil || a.pickRestore == nil || a.restoreBusy || a.restorePicking || a.backupBusy || a.importBusy || a.exam != nil {
		return
	}
	a.restorePicking = true
	a.backupError = false
	a.backupMessage = "请选择要恢复的原生备份…"
	pick := a.pickRestore
	go func() {
		filename, err := pick()
		a.publish(func() {
			a.restorePicking = false
			a.backupMessage = ""
			if err != nil {
				a.backupError = true
				a.backupMessage = "无法选择备份：" + err.Error()
				return
			}
			if filename != "" {
				a.restorePath = filename
				a.restoreOpen = true
			}
		})
	}()
}

// confirmRestore 捕获仓库和路径，后台只读写数据库；界面在事务期间禁止其他数据操作。
func (a *app) confirmRestore() {
	if !a.restoreOpen || a.restorePath == "" || a.restoreBusy || a.store == nil || a.backupBusy || a.importBusy || a.exam != nil {
		return
	}
	a.persistPreferences()
	a.restoreBusy = true
	a.backupError = false
	a.backupMessage = "正在校验备份并恢复数据…"
	store, filename := a.store, a.restorePath
	if !a.enqueue(func() {
		safety, err := store.restoreFrom(filename)
		var next *app
		if err == nil {
			next, err = readRestorableState(store.db, store.path)
		}
		a.publish(func() {
			a.restoreBusy = false
			if err != nil {
				a.backupError = true
				a.backupMessage = "恢复失败：" + err.Error()
				if safety != "" {
					a.backupMessage += "\n恢复前备份：" + safety
				}
				return
			}
			a.acceptRestoredState(next)
			a.restoreOpen = false
			a.settingsOpen = true
			a.backupMessage = fmt.Sprintf("数据已恢复。恢复前备份：%s", safety)
		})
	}) {
		a.restoreBusy = false
		a.backupError = true
		a.backupMessage = "后台任务繁忙，请稍后重试。"
	}
}

// acceptRestoredState 只替换领域/学习状态；窗口、稳定句柄、真实仓库和调度器继续由原 app 持有。
func (a *app) acceptRestoredState(next *app) {
	a.questions, a.answers, a.starred = next.questions, next.answers, next.starred
	a.bank, a.filter, a.filters = next.bank, next.filter, next.filters
	a.importantOnly, a.onlyWrong = next.importantOnly, next.onlyWrong
	a.random, a.quick, a.shuffle, a.repeatPractice = next.random, next.quick, next.shuffle, next.repeatPractice
	a.dark, a.leftOpen, a.rightOpen = next.dark, next.leftOpen, next.rightOpen
	a.order, a.index, a.optionOrder = next.order, next.index, next.optionOrder
	a.chosen, a.typed = next.chosen, next.typed
	a.attemptAnsweredID, a.reviewedQuestionID = "", ""
	a.importantMotion = importantMotion{}
	a.paper = nil
	a.history = nil
	a.savedWrong = false
	a.exportOpen = false
	a.notice = ""
	a.importOpen = false
	a.deleteOpen = false
	a.examSetupOpen = false
	a.historyOpen = false
	a.lastPreferences = ""
	a.applyTheme()
}

func (a *app) restoreDialog(c *ui.Context) {
	if a.restoreBusy {
		a.restoreOpen = true
	}
	_, height := c.Size()
	nativeModal(c.Key("restore-confirmation"), &a.restoreOpen, func() {
		ui.Text(c, "从备份恢复全部数据").FontSize(18).FontWeight(600)
		ui.Scroll(c.Key("restore-confirm-body")).FillWidth().MaxHeight(max(100, height-200)).Children(func() {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				ui.Text(c, "将替换当前全部题库、答题记录、已保存考试/错题本和学习设置。恢复前会自动保存当前数据的安全备份。").FillWidth().FontSize(13).LineHeight(1.5)
				ui.Text(c, a.restorePath).FillWidth().FontSize(11).TextColor(c.Theme().TextMuted).Selectable()
				if a.restoreBusy || a.backupError {
					color := c.Theme().TextMuted
					if a.backupError {
						color = c.Theme().Danger
					}
					ui.Text(c, a.backupMessage).FillWidth().FontSize(12).LineHeight(1.5).TextColor(color)
				}
			})
		})
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			control(c, "cancel-restore", "取消", nil, outlineButton, func() { a.restoreOpen = false }).Disabled(a.restoreBusy)
			label := "确认恢复"
			if a.restoreBusy {
				label = "正在恢复…"
			}
			control(c, "confirm-restore", label, nil, primaryButton, a.confirmRestore).Label("确认恢复").Disabled(a.restoreBusy)
		})
	}).Width(480)
}
