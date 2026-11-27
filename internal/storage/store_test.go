package storage

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ispacewang/quesora-native/internal/domain"
	"github.com/ispacewang/quesora-native/internal/importer"
)

func testStore(t *testing.T, name string) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), name+".db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func testBank(name string) importer.Bank {
	return importer.Bank{Name: name, Questions: []domain.Question{
		{Kind: "单选题", Prompt: name + "题目", Options: []string{"甲", "乙"}, TextAnswer: "B"},
	}}
}

func TestImportIsAtomicAndDeleteCascadesOnlyPractice(t *testing.T) {
	s := testStore(t, "atomic")
	if err := s.ImportBanks(importer.Result{Banks: []importer.Bank{testBank("已有")}}); err != nil {
		t.Fatal(err)
	}
	err := s.ImportBanks(importer.Result{Banks: []importer.Bank{testBank("新题库"), testBank("已有")}})
	if err == nil {
		t.Fatal("duplicate accepted")
	}
	qs, err := s.LoadQuestions()
	if err != nil || len(qs) != 1 || qs[0].Bank != "已有" {
		t.Fatal("partial import committed", qs, err)
	}
	if err = s.SaveResponse(qs[0].ID, domain.Response{Choice: 2}); err != nil {
		t.Fatal(err)
	}
	if err = s.SetSetting("exam:saved", "independent history"); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteBank("已有"); err != nil {
		t.Fatal(err)
	}
	answers, _, err := s.LoadPractice()
	if err != nil || len(answers) != 0 {
		t.Fatal("orphan practice", answers, err)
	}
	if raw, err := s.Setting("exam:saved"); err != nil || raw != "independent history" {
		t.Fatal("history lost", raw, err)
	}
}

func TestResponseUpdatesPreserveNoteAndBankResetPreservesOtherData(t *testing.T) {
	s := testStore(t, "practice")
	if err := s.ImportBanks(importer.Result{Banks: []importer.Bank{testBank("甲库"), testBank("乙库")}}); err != nil {
		t.Fatal(err)
	}
	qs, err := s.LoadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range qs {
		if err = s.SaveResponse(q.ID, domain.Response{Choice: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.SaveMistakeNote(qs[0].ID, "已保存备注"); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveResponse(qs[0].ID, domain.Response{Choice: 2, Correct: true}); err != nil {
		t.Fatal(err)
	}
	answers, _, err := s.LoadPractice()
	if err != nil || answers[qs[0].ID].Note != "已保存备注" || !answers[qs[0].ID].Correct {
		t.Fatal("response overwrote note", answers, err)
	}
	if err = s.SetSetting("stars", `{"1":true}`); err != nil {
		t.Fatal(err)
	}
	if err = s.InsertSetting("mistake-book:saved", "[]"); err != nil {
		t.Fatal(err)
	}
	if err = s.ClearPractice("甲库"); err != nil {
		t.Fatal(err)
	}
	answers, stars, err := s.LoadPractice()
	if err != nil || len(answers) != 1 || answers[qs[1].ID].Choice != 1 || !stars[qs[0].ID] {
		t.Fatal(answers, stars, err)
	}
	if err = s.ClearPractice(""); err != nil {
		t.Fatal(err)
	}
	if history, err := s.History(); err != nil || len(history) != 1 {
		t.Fatal("reset deleted history", history, err)
	}
}

func TestRestoreValidationRunsBeforeSafetyCopyOrLiveWrites(t *testing.T) {
	live, source := testStore(t, "live"), testStore(t, "source")
	if err := live.ImportBanks(importer.Result{Banks: []importer.Bank{testBank("现有")}}); err != nil {
		t.Fatal(err)
	}
	if err := source.ImportBanks(importer.Result{Banks: []importer.Bank{testBank("来源")}}); err != nil {
		t.Fatal(err)
	}
	if err := source.SetSetting("learning-preferences", "unsupported payload"); err != nil {
		t.Fatal(err)
	}
	reject := errors.New("unsupported learning version")
	called := false
	safety, err := live.RestoreFrom(source.Path(), func(state State) error {
		called = true
		if len(state.Questions) != 1 || state.Questions[0].Bank != "来源" || state.Learning == nil || *state.Learning != "unsupported payload" {
			t.Fatal("incomplete immutable snapshot", state)
		}
		return reject
	})
	if !called || !errors.Is(err, reject) || safety != "" {
		t.Fatal(called, safety, err)
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(live.Path()), "backups", "*"))
	if len(files) != 0 {
		t.Fatal("safety copy created before validation", files)
	}
	qs, err := live.LoadQuestions()
	if err != nil || len(qs) != 1 || qs[0].Bank != "现有" {
		t.Fatal("rejected restore changed live data", qs, err)
	}
	if _, err = live.RestoreFrom(source.Path(), nil); err == nil {
		t.Fatal("unchecked restore allowed")
	}
}

func TestReadStateDistinguishesMissingAndEmptyPreferences(t *testing.T) {
	s := testStore(t, "state")
	state, err := s.ReadState()
	if err != nil || state.Learning != nil {
		t.Fatal(state, err)
	}
	if err = s.SetSetting("learning-preferences", ""); err != nil {
		t.Fatal(err)
	}
	state, err = s.ReadState()
	if err != nil || state.Learning == nil || *state.Learning != "" {
		t.Fatal(state, err)
	}
}
