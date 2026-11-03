package main

import (
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRestoreRequiresConfirmationAndReloadsNativeState(t *testing.T) {
	a := newApp()
	a.store = restorableStore(t, filepath.Join(t.TempDir(), "live.db"))
	defer a.store.close()
	a.questions, _ = a.store.load()
	a.bank = a.questions[0].Bank
	a.navigate(0)
	source := restorableStore(t, filepath.Join(t.TempDir(), "备份.db"))
	defer source.close()
	source.db.Exec("UPDATE banks SET name='恢复后的题库'")
	source.saveRepeatPractice(true)
	a.pickRestore = func() (string, error) { return source.path, nil }
	updates := make(chan func(), 64)
	a.dispatch = func(fn func()) { updates <- fn }
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	a.settingsOpen = true
	a.chooseRestoreBackup()
	deadline := time.After(10 * time.Second)
	for a.restorePicking {
		select {
		case fn := <-updates:
			fn()
		case <-deadline:
			t.Fatal("picker timed out")
		}
	}
	if !a.restoreOpen || a.bank == "恢复后的题库" {
		t.Fatal("restore happened without confirmation")
	}
	tt := ui.NewTester(a.view, 800, 500)
	rect, ok := tt.Find("确认恢复")
	if !ok || rect.Y+rect.H > 500 {
		t.Fatal("confirmation inaccessible", rect)
	}
	click(t, tt, "确认恢复")
	if !a.restoreBusy {
		t.Fatal("restore did not start")
	}
	for a.restoreBusy {
		select {
		case fn := <-updates:
			fn()
			tt.Frame()
		case <-deadline:
			t.Fatal("restore timed out")
		}
	}
	if a.backupError || a.bank != "恢复后的题库" || !a.repeatPractice || !strings.Contains(a.backupMessage, "before-restore-") {
		t.Fatal(a.bank, a.backupMessage)
	}
	if a.restoreOpen || !a.settingsOpen {
		t.Fatal("incorrect modal state")
	}
}
