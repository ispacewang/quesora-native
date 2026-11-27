package desktop

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAppearanceThreeChoicesAndNoTitlebarToggle(t *testing.T) {
	a := newApp()
	a.systemDark = func() bool { return true }
	a.openSettings()
	tt := ui.NewTester(a.view, 800, 500)
	for _, name := range []string{"跟随系统", "浅色", "深色"} {
		if _, ok := tt.Find("外观：" + name); !ok {
			t.Fatal("missing appearance", name)
		}
	}
	click(t, tt, "外观：深色")
	if !a.dark || a.appearance != "dark" {
		t.Fatal("dark mode not selected")
	}
	click(t, tt, "外观：浅色")
	if a.dark {
		t.Fatal("light mode ignored")
	}
	click(t, tt, "外观：跟随系统")
	if !a.dark || a.appearance != "system" {
		t.Fatal("system appearance ignored")
	}
	click(t, tt, "关闭设置")
	for _, label := range []string{"深色模式", "浅色模式"} {
		if _, ok := tt.Find(label); ok {
			t.Fatal("old titlebar theme toggle remains")
		}
	}
}
func TestAppearanceMigrationAndSystemUpdates(t *testing.T) {
	s := populatedStore(t, filepath.Join(t.TempDir(), "questions.db"))
	defer s.Close()
	a := newApp()
	a.store = s
	a.questions, _ = s.LoadQuestions()
	raw, _ := json.Marshal(learningPreferences{Version: 1, Dark: true})
	fixtureDB(t, s).Exec("INSERT INTO settings(key,value) VALUES('learning-preferences',?)", string(raw))
	if e := a.restorePreferences(); e != nil || a.appearance != "dark" {
		t.Fatal("old dark choice not migrated", e)
	}
	a.systemDark = func() bool { return false }
	a.setAppearance("system")
	a.systemAppearanceChanged(true)
	if !a.dark {
		t.Fatal("system update ignored")
	}
	a.setAppearance("light")
	a.systemAppearanceChanged(true)
	if a.dark {
		t.Fatal("manual appearance overwritten")
	}
	p := a.preferences()
	if p.Version != 2 || p.Appearance != "light" {
		t.Fatal("appearance not persisted", p)
	}
}
