package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func drainUpdate(t *testing.T, a *app, updates <-chan func()) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for a.updates.busy || a.updates.installing {
		select {
		case fn := <-updates:
			fn()
		case <-deadline:
			t.Fatal("update timed out")
		}
	}
}
func TestUpdateNativeConfirmationAndSafeInstallHandoff(t *testing.T) {
	a := newApp()
	a.store = restorableStore(t, filepath.Join(t.TempDir(), "data", "live.db"))
	defer a.store.close()
	a.questions, _ = a.store.load()
	a.bank = a.questions[0].Bank
	a.navigate(0)
	queue := make(chan func(), 64)
	a.dispatch = func(fn func()) { queue <- fn }
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	c, err := selectUpdateRelease([]updateRelease{updateFixture("0.99.0-preview.1")}, appVersion)
	if err != nil {
		t.Fatal(err)
	}
	a.updates.fetch = func(context.Context) (*updateCandidate, error) { return c, nil }
	payload := []byte("MZ-fake-installer-not-executed")
	name := filepath.Join(t.TempDir(), "setup.exe")
	os.WriteFile(name, payload, 0600)
	a.updates.download = func(context.Context, updateCandidate, func(int64, int64)) (downloadedUpdate, error) {
		return downloadedUpdate{Path: name, Size: int64(len(payload)), SHA256: fmt.Sprintf("%x", sha256.Sum256(payload))}, nil
	}
	launched, closed := false, false
	a.updates.launch = func(path string) error {
		if path != name {
			return fmt.Errorf("wrong installer")
		}
		launched = true
		return nil
	}
	a.updates.close = func() { closed = true }
	a.checkForUpdate()
	drainUpdate(t, a, queue)
	if a.updates.candidate == nil {
		t.Fatal(a.updates.message)
	}
	a.downloadAvailableUpdate()
	drainUpdate(t, a, queue)
	if a.updates.downloaded == nil {
		t.Fatal(a.updates.message)
	}
	a.requestUpdateInstall()
	if launched || !a.updates.confirmOpen {
		t.Fatal("installer launched without confirmation")
	}
	tt := ui.NewTester(a.view, 800, 500)
	r, ok := tt.Find("确认安装更新")
	if !ok || r.Y+r.H > 500 {
		t.Fatal("confirmation clipped", r)
	}
	click(t, tt, "确认安装更新")
	drainUpdate(t, a, queue)
	if !launched || !closed || a.updates.failed {
		t.Fatal(a.updates.message)
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(a.store.path), "backups", "before-update-*.db"))
	if len(backups) != 1 {
		t.Fatal("pre-update data backup missing", backups)
	}
}
func TestUpdateCancellationHasNoFailureState(t *testing.T) {
	a := newApp()
	queue := make(chan func(), 8)
	a.dispatch = func(fn func()) { queue <- fn }
	a.updates.fetch = func(ctx context.Context) (*updateCandidate, error) { <-ctx.Done(); return nil, ctx.Err() }
	a.checkForUpdate()
	a.cancelUpdate()
	drainUpdate(t, a, queue)
	if a.updates.failed || !strings.Contains(a.updates.message, "取消") {
		t.Fatal(a.updates.message)
	}
}

func TestUpdateRateLimitPreventsImmediateRetry(t *testing.T) {
	a := newApp()
	queue := make(chan func(), 8)
	a.dispatch = func(fn func()) { queue <- fn }
	calls := 0
	a.updates.fetch = func(context.Context) (*updateCandidate, error) {
		calls++
		return nil, &updateRateLimitError{Until: time.Now().Add(time.Hour)}
	}
	a.checkForUpdate()
	drainUpdate(t, a, queue)
	a.checkForUpdate()
	if calls != 1 || a.updates.busy || !a.updates.failed {
		t.Fatal("rate limit immediately retried", calls, a.updates.message)
	}
}
func TestDataAndUpdateConfirmationsFitLongPaths(t *testing.T) {
	for _, kind := range []string{"restore", "update"} {
		a := newApp()
		if kind == "restore" {
			a.restoreOpen = true
			a.restorePath = strings.Repeat("很长的目录/", 50) + "backup.db"
			a.backupError = true
			a.backupMessage = strings.Repeat("错误详情。", 50)
		} else {
			a.updates.confirmOpen = true
			a.updates.failed = true
			a.updates.message = strings.Repeat("错误详情。", 50)
		}
		tt := ui.NewTester(a.view, 800, 500)
		label := "确认恢复"
		if kind == "update" {
			label = "确认安装更新"
		}
		r, ok := tt.Find(label)
		if !ok || r.Y+r.H > 500 || r.Y < 0 {
			t.Fatal("confirmation footer clipped", kind, r)
		}
	}
}
