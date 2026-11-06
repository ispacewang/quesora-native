package main

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLearningPreferencesRestoreModesFiltersAndQuestion(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "questions.db")
	store := populatedStore(t, filename)
	a := newApp()
	a.store = store
	a.questions, _ = store.load()
	a.bank = a.questions[0].Bank
	a.jobs = newLibraryJobs()
	a.dispatch = func(fn func()) { t.Error("unexpected persistence error") }
	a.setAppearance("dark")
	a.toggleQuick()
	a.toggleShuffle()
	a.toggleRandom()
	a.toggleFilter("单选题")
	a.toggleFilter("多选题")
	a.togglePanel(1450, true)
	a.navigate(len(a.visible()) - 1)
	a.toggleRepeatPractice()
	want := a.preferences()
	a.jobs.close()
	a.jobs = nil
	store.close()
	store, err := openQuestionStore(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	b := newApp()
	b.store = store
	b.questions, _ = store.load()
	b.bank = ""
	b.repeatPractice, err = store.loadRepeatPractice()
	if err != nil {
		t.Fatal(err)
	}
	if err = b.restorePreferences(); err != nil {
		t.Fatal(err)
	}
	if got := b.preferences(); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored %+v; wanted %+v", got, want)
	}
	if !b.repeatPractice {
		t.Fatal("legacy repeat-practice preference lost")
	}
}

func TestPreferencesMissingBankAndCorruptPayload(t *testing.T) {
	s := populatedStore(t, filepath.Join(t.TempDir(), "db"))
	defer s.close()
	a := newApp()
	a.store = s
	a.questions, _ = s.load()
	p := learningPreferences{Version: 1, Bank: "deleted", QuestionID: "missing", Filter: "不存在的题型", OnlyWrong: true, Filters: []string{"不存在的题型"}}
	raw, _ := json.Marshal(p)
	s.db.Exec("INSERT INTO settings(key,value) VALUES('learning-preferences',?)", string(raw))
	if err := a.restorePreferences(); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.current(); !ok || a.bank == "deleted" {
		t.Fatal("deleted bank left session unusable")
	}
	s.db.Exec("UPDATE settings SET value='{' WHERE key='learning-preferences'")
	if err := a.restorePreferences(); err == nil {
		t.Fatal("corrupt settings silently accepted")
	}
}

func TestSettingsBackupUIAndKeyboardIsolation(t *testing.T) {
	a := newApp()
	a.store = populatedStore(t, filepath.Join(t.TempDir(), "data", "questions.db"))
	defer a.store.close()
	a.questions, _ = a.store.load()
	a.bank = a.questions[0].Bank
	a.navigate(0)
	updates := make(chan func(), 64)
	a.dispatch = func(fn func()) { updates <- fn }
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	filename := filepath.Join(t.TempDir(), "全部记录.db")
	a.pickBackup = func() (string, error) { return filename, nil }
	tt := ui.NewTester(a.view, 1450, 800)
	click(t, tt, "设置")
	click(t, tt, "设置：速刷")
	if !a.quick {
		t.Fatal("settings toggle did not change quick mode")
	}
	tt.Key(0, ui.KeyA)
	tt.Key(0, ui.KeySpace)
	if a.chosen != 0 {
		t.Fatal("practice shortcuts ran through settings modal")
	}
	if !a.quick {
		click(t, tt, "设置：速刷")
	}
	tt.Wheel(725, 420, 0, 900)
	tt.Advance(200 * time.Millisecond)
	click(t, tt, "备份全部数据")
	deadline := time.After(10 * time.Second)
	for a.backupBusy {
		select {
		case fn := <-updates:
			fn()
			tt.Frame()
		case <-deadline:
			t.Fatal("backup timed out")
		}
	}
	if a.backupError {
		t.Fatal(a.backupMessage)
	}
	if _, err := os.Stat(filename); err != nil {
		t.Fatal(err)
	}
	saved, err := openQuestionStore(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer saved.close()
	qs, err := saved.load()
	if err != nil || len(qs) != 5 {
		t.Fatal(len(qs), err)
	}
	b := newApp()
	b.store = saved
	b.questions = qs
	if err = b.restorePreferences(); err != nil || !b.quick {
		t.Fatal("backup missed current settings", err)
	}
	click(t, tt, "关闭设置")
	if a.settingsOpen {
		t.Fatal("settings not closed")
	}
}

func TestGroupedToolbarAndSettingsFitSmallWindows(t *testing.T) {
	for _, width := range []int{800, 1450} {
		for _, dark := range []bool{false, true} {
			a := newApp()
			a.dark = dark
			tt := ui.NewTester(a.view, width, 500)
			heading, ok := tt.Find("刷题工具栏")
			if !ok {
				t.Fatal("toolbar missing")
			}
			for _, label := range []string{"题型筛选", "练习模式"} {
				r, ok := tt.Find(label)
				if !ok || r.X < heading.X-1 || r.X+r.W > heading.X+heading.W+1 {
					t.Fatal("toolbar group overflow", label, r, heading)
				}
			}
			click(t, tt, "设置")
			close, ok := tt.Find("关闭设置")
			if !ok || close.Y < 0 || close.Y+close.H > 500 {
				t.Fatal("settings footer offscreen", close)
			}
			click(t, tt, "关闭设置")
		}
	}
}
