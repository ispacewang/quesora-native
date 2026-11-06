// 更新 UI 只保存持久模型值；所有网络/文件/进程操作由显式动作启动，后台结果通过 publish 回到窗口。
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"strings"
	"time"
)

// updateState 保存窗口线程上的更新状态及可注入系统边界；后台通过 generation 检查后发布不可变结果。
type updateState struct {
	retryAfter, lastChecked                          time.Time
	busy, installing, failed, confirmOpen, installed bool
	message                                          string
	received, total                                  int64
	generation                                       uint64
	candidate                                        *updateCandidate
	downloaded                                       *downloadedUpdate
	cancel                                           context.CancelFunc
	fetch                                            func(context.Context) (*updateCandidate, error)
	download                                         func(context.Context, updateCandidate, func(int64, int64)) (downloadedUpdate, error)
	launch                                           func(string) error
	close                                            func()
}

// beginUpdate 为每次操作分配代次；取消后的晚到进度不能覆盖下一次操作状态。
func (a *app) beginUpdate(message string) (context.Context, uint64) {
	ctx, cancel := context.WithCancel(context.Background())
	a.updates.generation++
	a.updates.busy = true
	a.updates.failed = false
	a.updates.message = message
	a.updates.received = 0
	a.updates.total = 0
	a.updates.cancel = cancel
	return ctx, a.updates.generation
}
func (a *app) finishUpdate(generation uint64, err error) bool {
	if a.updates.generation != generation {
		return false
	}
	a.updates.busy = false
	if a.updates.cancel != nil {
		a.updates.cancel()
		a.updates.cancel = nil
	}
	if err != nil {
		var rate *updateRateLimitError
		if errors.As(err, &rate) {
			a.updates.retryAfter = rate.Until
		}
		a.updates.failed = !errors.Is(err, context.Canceled)
		if a.updates.failed {
			a.updates.message = "更新操作失败：" + err.Error()
		} else {
			a.updates.message = "已取消更新操作。"
		}
		return false
	}
	return true
}
func (a *app) checkForUpdate() {
	if a.updates.busy || a.updates.installing || a.updates.fetch == nil || a.restoreBusy {
		return
	}
	if time.Now().Before(a.updates.retryAfter) {
		a.updates.failed = true
		a.updates.message = (&updateRateLimitError{Until: a.updates.retryAfter}).Error()
		return
	}
	// Repeated manual checks reuse a recent success and retain an already verified installer.
	if !a.updates.lastChecked.IsZero() && time.Since(a.updates.lastChecked) < time.Minute {
		a.updates.failed = false
		a.updates.message = "刚刚已检查更新，可在 " + a.updates.lastChecked.Add(time.Minute).Local().Format("15:04:05") + " 后再次刷新；也可打开发布页面。"
		return
	}
	a.updates.candidate = nil
	a.updates.downloaded = nil
	ctx, generation := a.beginUpdate("正在检查官方发布版本…")
	fetch := a.updates.fetch
	go func() {
		candidate, err := fetch(ctx)
		a.publish(func() {
			if !a.finishUpdate(generation, err) {
				return
			}
			a.updates.lastChecked = time.Now()
			a.updates.candidate = candidate
			if candidate == nil {
				a.updates.message = "没有比当前版本更高的可用 Windows 安装版。"
			} else {
				a.updates.message = "发现新版本 v" + candidate.Version + "，下载后会校验完整性。"
			}
		})
	}()
}
func (a *app) cancelUpdate() {
	if a.updates.cancel != nil {
		a.updates.cancel()
		a.updates.message = "正在取消…"
	}
}
func (a *app) downloadAvailableUpdate() {
	if a.updates.busy || a.updates.installing || a.updates.candidate == nil || a.updates.download == nil || a.restoreBusy {
		return
	}
	candidate, download := *a.updates.candidate, a.updates.download
	a.updates.downloaded = nil
	ctx, generation := a.beginUpdate("正在下载安装包…")
	go func() {
		result, err := download(ctx, candidate, func(received, total int64) {
			a.publish(func() {
				if a.updates.generation == generation && a.updates.busy {
					a.updates.received = received
					a.updates.total = total
				}
			})
		})
		a.publish(func() {
			if !a.finishUpdate(generation, err) {
				return
			}
			a.updates.downloaded = &result
			a.updates.message = "安装包已下载，SHA-256 校验通过。"
		})
	}()
}
func (a *app) canInstallUpdate() bool {
	return a.store != nil && a.updates.downloaded != nil && a.updates.launch != nil && !a.updates.busy && !a.updates.installing && !a.restoreBusy && !a.backupBusy && !a.importBusy && a.exam == nil
}
func (a *app) requestUpdateInstall() {
	if a.canInstallUpdate() {
		a.updates.confirmOpen = true
	}
}

// confirmUpdateInstall 先排队保存学习状态，再复核安装包并创建数据备份，全部成功后才启动安装器。
func (a *app) confirmUpdateInstall() {
	if !a.updates.confirmOpen || !a.canInstallUpdate() {
		return
	}
	a.persistPreferences()
	a.updates.installing = true
	a.updates.failed = false
	a.updates.message = "正在备份数据并复核安装包…"
	downloaded, launch, store := *a.updates.downloaded, a.updates.launch, a.store
	if !a.enqueue(func() {
		err := verifyDownloadedUpdate(downloaded)
		if err == nil {
			safety := filepath.Join(filepath.Dir(store.path), "backups", "before-update-"+time.Now().UTC().Format("20060102T150405.000000000")+".db")
			err = store.backupTo(safety)
		}
		if err == nil {
			err = launch(downloaded.Path)
		}
		a.publish(func() {
			a.updates.installing = false
			if err != nil {
				a.updates.failed = true
				a.updates.message = "无法启动更新：" + err.Error()
				return
			}
			a.updates.confirmOpen = false
			a.updates.message = "安装器已启动。"
			if a.updates.close != nil {
				a.updates.close()
			}
		})
	}) {
		a.updates.installing = false
		a.updates.failed = true
		a.updates.message = "后台任务繁忙，请稍后再安装。"
	}
}

func (a *app) updateSection(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "软件更新").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
	channel := "正式版本"
	if strings.Contains(appVersion, "-") {
		channel = "预览和正式版本"
	}
	ui.Text(c, "当前 v"+appVersion+" · 检查"+channel).FillWidth().FontSize(12)
	if !a.updates.installed {
		ui.Text(c, "当前为便携运行。安装包会创建安装版；保留便携方式请在发布页下载 ZIP 并保留 data 目录。").FillWidth().FontSize(11).LineHeight(1.5).TextColor(t.TextMuted)
	}
	ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
		control(c, "check-update", "检查更新", historyIcon, outlineButton, a.checkForUpdate).Label("检查更新").Disabled(a.updates.busy || a.updates.installing || a.updates.fetch == nil)
		services := c.Services()
		control(c, "update-release-page", "发布页面", nil, ghostButton, func() { services.OpenURL("https://github.com/" + updateRepository + "/releases") }).Label("更新发布页面")
		if a.updates.busy {
			control(c, "cancel-update", "取消", nil, outlineButton, a.cancelUpdate).Label("取消更新操作")
		}
		if a.updates.candidate != nil && !a.updates.busy {
			if a.updates.downloaded == nil {
				control(c, "download-update", "下载安装包", downloadIcon, primaryButton, a.downloadAvailableUpdate).Label("下载安装包").Disabled(a.updates.download == nil || a.updates.installing)
			} else {
				control(c, "install-update", "安装更新…", downloadIcon, primaryButton, a.requestUpdateInstall).Label("安装更新").Disabled(!a.canInstallUpdate())
			}
		}
	})
	if a.updates.message != "" {
		color := t.TextMuted
		if a.updates.failed {
			color = t.Danger
		}
		text := a.updates.message
		if a.updates.busy && a.updates.total > 0 {
			text += fmt.Sprintf(" %.1f / %.1f MiB", float64(a.updates.received)/1048576, float64(a.updates.total)/1048576)
		}
		ui.Text(c, text).FillWidth().FontSize(12).LineHeight(1.5).TextColor(color).Selectable()
	}
	if a.exam != nil {
		ui.Text(c, "请先结束或退出当前考试，再安装更新。").FillWidth().FontSize(11).TextColor(t.TextMuted)
	}
}

// updateInstallDialog keeps confirmation actions visible while long notices scroll independently.
func (a *app) updateInstallDialog(c *ui.Context) {
	_, height := c.Size()
	nativeModal(c.Key("update-install-confirmation"), &a.updates.confirmOpen, func() {
		ui.Text(c, "安装软件更新").FontSize(18).FontWeight(600)
		ui.Scroll(c.Key("update-confirm-body")).FillWidth().MaxHeight(max(100, height-220)).Children(func() {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				ui.Text(c, "将先备份当前题库和记录，再关闭程序并启动安装向导。安装版会沿用原安装位置，完成后可启动新版。").FillWidth().FontSize(13).LineHeight(1.5)
				if !a.updates.installed {
					ui.Text(c, "这是便携副本。安装器不会替换此副本；安装版首次启动后，可从当前数据目录的 backups 文件夹恢复更新前备份。").FillWidth().FontSize(12).LineHeight(1.5).TextColor(c.Theme().TextMuted)
				}
				if a.updates.failed || a.updates.installing {
					ui.Text(c, a.updates.message).FillWidth().FontSize(12).LineHeight(1.5).TextColor(c.Theme().TextMuted)
				}
			})
		})
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			control(c, "cancel-install", "取消", nil, outlineButton, func() { a.updates.confirmOpen = false }).Disabled(a.updates.installing)
			control(c, "confirm-install", "确认安装", nil, primaryButton, a.confirmUpdateInstall).Label("确认安装更新").Disabled(!a.canInstallUpdate())
		})
	}).Width(480)
}
