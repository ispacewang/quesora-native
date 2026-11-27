package desktop

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *app) loadShareDirectory() {
	a.share.directory = ""
	if a.store == nil {
		return
	}
	if path, err := a.store.Setting(shareDirectoryKey); err == nil && filepath.IsAbs(path) {
		a.share.directory = path
	}
}

func (a *app) configureShare(win *mygo.Window) {
	a.share.defaultDirectory, _ = mygo.App.Path(mygo.PathDownloads)
	a.share.pick = func(start string) (string, error) {
		entries, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "选择分享图片的下载目录", DefaultPath: start, Directory: true})
		if err != nil || len(entries) == 0 {
			return "", err
		}
		return entries[0], nil
	}
}

func (a *app) setShareDirectory(reset bool) {
	if a.store == nil || a.share.picking || a.share.pick == nil {
		return
	}
	a.share.picking = true
	a.share.settingsMessage = ""
	pick, start, store := a.share.pick, a.share.destination(), a.store
	go func() {
		path, err := "", error(nil)
		if !reset {
			path, err = pick(start)
			if err == nil && path != "" {
				info, statErr := os.Stat(path)
				if statErr != nil || !info.IsDir() || !filepath.IsAbs(path) {
					err = fmt.Errorf("请选择有效的文件夹")
				}
			}
		}
		a.publish(func() {
			if err != nil {
				a.share.picking = false
				a.share.settingsMessage = "无法设置下载目录：" + err.Error()
				return
			}
			if path == "" && !reset {
				a.share.picking = false
				return
			}
			if !a.enqueue(func() {
				err := store.SetSetting(shareDirectoryKey, path)
				a.publish(func() {
					a.share.picking = false
					if err != nil {
						a.share.settingsMessage = "无法保存下载目录"
						return
					}
					a.share.directory = path
				})
			}) {
				a.share.picking = false
				a.share.settingsMessage = "后台任务繁忙，请稍后再试。"
			}
		})
	}()
}

func (a *app) shareSettings(c *ui.Context) {
	t := c.Theme()
	ui.Column(c.Key("share-settings")).FillWidth().Gap(7).Children(func() {
		ui.Text(c, "分享图片下载目录").FontSize(13).FontWeight(500)
		path := a.share.destination()
		if path == "" {
			path = "系统下载文件夹"
		}
		ui.Text(c, path).FillWidth().FontSize(11).TextColor(t.TextMuted).Selectable()
		ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
			control(c, "share-dir", "选择文件夹…", nil, outlineButton, func() { a.setShareDirectory(false) }).Disabled(a.share.picking || a.share.pick == nil || a.store == nil)
			if a.share.directory != "" {
				control(c, "share-dir-reset", "恢复默认", nil, ghostButton, func() { a.setShareDirectory(true) }).Disabled(a.share.picking)
			}
		})
		if a.share.settingsMessage != "" {
			ui.Text(c, a.share.settingsMessage).FillWidth().FontSize(11).TextColor(t.Danger)
		}
	})
}
