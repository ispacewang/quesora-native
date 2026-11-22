// 更新服务的系统边界。单元测试注入 HTTP/安装器替身，不在开发机器自动执行真实安装。
package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ispacewang/quesora-native/internal/buildinfo"
	"github.com/ispacewang/quesora-native/internal/updater"
)

func (a *app) configureUpdater() {
	client := updater.NewHTTPClient()
	a.updates.fetch = func(ctx context.Context) (*updater.Candidate, error) {
		return updater.Fetch(ctx, client, buildinfo.Version)
	}
	cache, cacheErr := os.UserCacheDir()
	if cacheErr == nil && filepath.IsAbs(cache) {
		a.updates.download = func(ctx context.Context, c updater.Candidate, progress func(int64, int64)) (updater.Download, error) {
			return updater.DownloadInstaller(ctx, client, c, filepath.Join(cache, "QuesoraNative", "updates", c.Version), progress)
		}
	} else {
		a.updates.message = "更新缓存目录不可用，请从发布页面下载。"
	}
	if exe, e := os.Executable(); e == nil {
		if info, e := os.Stat(filepath.Join(filepath.Dir(exe), installedMarker)); e == nil {
			a.updates.installed = info.Mode().IsRegular()
		}
	}
	a.updates.launch = func(filename string) error {
		// 用固定参数直接创建进程，不经 shell；保留可见安装向导，不静默安装或强制结束程序。
		command := exec.Command(filename, "/CLOSEAPPLICATIONS", "/NOFORCECLOSEAPPLICATIONS", "/NORESTART")
		if e := command.Start(); e != nil {
			return fmt.Errorf("无法打开安装器：%w", e)
		}
		_ = command.Process.Release()
		return nil
	}
	a.updates.close = func() {
		if a.win != nil {
			a.win.Close()
		}
	}
}
