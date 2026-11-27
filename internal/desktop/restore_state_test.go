package desktop

import (
	"path/filepath"
	"testing"

	"github.com/ispacewang/quesora-native/internal/domain"
	"github.com/ispacewang/quesora-native/internal/storage"
)

func TestRestoreWithoutPreferencesResetsPriorSessionModes(t *testing.T) {
	source := restorableStore(t, filepath.Join(t.TempDir(), "source.db"))
	defer source.Close()
	next, err := source.readRestorableState()
	if err != nil {
		t.Fatal(err)
	}
	a := newApp()
	a.rounded, a.dark, a.random, a.quick = true, true, true, true
	a.appearance = "dark"
	a.filter, a.filters = "不存在", map[string]bool{"不存在": true}
	a.importantOnly, a.onlyWrong = true, true
	a.leftOpen, a.rightOpen = true, true
	a.order = []string{"stale"}
	a.acceptRestoredState(next)
	if a.rounded || a.dark || a.random || a.quick || a.importantOnly || a.onlyWrong || a.leftOpen || a.rightOpen || a.appearance != "system" || a.filter != "全部" || len(a.filters) != 0 || !a.shuffle {
		t.Fatal("previous session leaked into backup defaults", a.preferences())
	}
	if len(a.order) != 0 {
		t.Fatal("old question order retained", a.order)
	}
	if q, ok := a.current(); !ok || q.Bank != next.Questions[0].Bank {
		t.Fatal("restored selection unusable", q)
	}
}

func TestRestoreRejectsCorruptHistoryAndUnknownPreferenceVersion(t *testing.T) {
	live := restorableStore(t, filepath.Join(t.TempDir(), "live.db"))
	defer live.Close()
	source := restorableStore(t, filepath.Join(t.TempDir(), "source.db"))
	defer source.Close()
	for _, record := range []struct{ key, value string }{
		{"exam:fixture", "{"},
		{"learning-preferences", `{"Version":99,"Appearance":"light"}`},
	} {
		original, originalErr := source.Setting(record.key)
		if err := source.SetSetting(record.key, record.value); err != nil {
			t.Fatal(err)
		}
		if safety, err := live.restoreFrom(source.path()); err == nil || safety != "" {
			t.Fatal("invalid backup accepted", record.key, safety, err)
		}
		if originalErr == nil {
			source.SetSetting(record.key, original)
		} else {
			source.DeleteSetting(record.key)
		}
	}
}

// Persisted null important marks must not panic while publishing an otherwise valid backup.
func TestRestoredStateNormalizesNullImportantMarks(t *testing.T) {
	a := newApp()
	next, err := decodeRestorableState(storage.State{
		Questions: []domain.Question{{ID: "1", Bank: "题库", Kind: "单选题", Prompt: "题目", Meta: map[string]any{"isBaoMing": true}}},
		Answers:   map[string]domain.Response{},
	})
	if err != nil {
		t.Fatal(err)
	}
	a.acceptRestoredState(next)
	if !a.starred["1"] {
		t.Fatal("embedded important flag lost")
	}
}
