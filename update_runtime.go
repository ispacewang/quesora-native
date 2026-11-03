// 更新服务的系统边界。单元测试注入 HTTP/安装器替身，不在开发机器自动执行真实安装。
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (a *app) configureUpdater() {
	client := newUpdateHTTPClient()
	a.updates.fetch = func(ctx context.Context) (*updateCandidate, error) { return fetchUpdate(ctx, client, appVersion) }
	cache, cacheErr := os.UserCacheDir()
	if cacheErr == nil && filepath.IsAbs(cache) {
		a.updates.download = func(ctx context.Context, c updateCandidate, progress func(int64, int64)) (downloadedUpdate, error) {
			return downloadUpdate(ctx, client, c, filepath.Join(cache, "QuesoraNative", "updates", c.Version), progress)
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
