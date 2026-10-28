package main

import (
	"github.com/egoist/mygo/ui"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestNarrowWindowKeepsQuestionUsable(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 800, 500)
	for _, label := range []string{"切换统计面板", "切换错题面板", "切换统计面板"} {
		click(t, tt, label)
		r, ok := tt.Find("答题主区域")
		if !ok || r.W < 440 {
			t.Fatalf("question squeezed after %s: %+v", label, r)
		}
		left, lok := tt.Find("统计侧栏")
		right, rok := tt.Find("错题侧栏")
		if lok && rok && left.W > 0 && right.W > 0 {
			t.Fatal("two sidebars occupy narrow window")
		}
	}
}
func TestWrongBookSaveDoesNotClaimChangedOrOtherBank(t *testing.T) {
	a := newApp()
	s, err := openQuestionStore(filepath.Join(t.TempDir(), "questions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	a.store = s
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	updates := make(chan func(), 10)
	a.dispatch = func(fn func()) { updates <- fn }
	a.answers["m1"] = response{Choice: 1}
	a.saveWrongBook()
	a.saveWrongBook()
	a.answers["m2"] = response{Choice: 1}
	select {
	case fn := <-updates:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("save timeout")
	}
	if a.savedWrong {
		t.Fatal("new wrong answers falsely marked saved")
	}
	done := make(chan struct{})
	a.enqueue(func() { close(done) })
	<-done
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM settings WHERE key LIKE 'mistake-book:%'").Scan(&count); err != nil || count != 1 {
		t.Fatal("repeated click saved duplicate", count, err)
	}
	a.savedWrong = true
	a.selectBank("计算机基础")
	if a.savedWrong {
		t.Fatal("saved state crossed bank")
	}
}
func TestEmptyWrongBookButtonsDoNothing(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1450, 800)
	click(t, tt, "导出错题 ▾")
	if a.exportOpen {
		t.Fatal("empty wrong export opened")
	}
}
func TestExamPercentageRounding(t *testing.T) {
	e := &examSession{Score: 1, Questions: make([]question, 8)}
	if e.percentage() != 13 {
		t.Fatal(e.percentage())
	}
	e.Score = 7
	if e.percentage() != 88 {
		t.Fatal(e.percentage())
	}
}
func TestExamSetupSummaryAndValidation(t *testing.T) {
	a := newApp()
	a.openExamSetup()
	tt := ui.NewTester(a.view, 1450, 800)
	if !tt.HasText("实际抽取 5 题") || !tt.HasText("题量不足时取该题型的全部题目。") {
		t.Fatal("missing actual paper explanation")
	}
	a.examForm.Counts = [5]float64{}
	tt.Frame()
	if !tt.HasText("请至少设置一种题型的数量。") {
		t.Fatal("zero selection has no explanation")
	}
	click(t, tt, "开始考试")
	if a.exam != nil {
		t.Fatal("empty paper started")
	}
	a.examForm.Counts[0] = math.NaN()
	tt.Frame()
	if !tt.HasText("单选题数量应为 0–100 的整数") {
		t.Fatal("invalid number hidden")
	}
}

func TestImportResetsWrongFilterBeforeRandomQueue(t *testing.T) {
	a := newApp()
	a.random = true
	a.onlyWrong = true
	qs := demoQuestions()[:3]
	for i := range qs {
		qs[i].Bank = "新题库"
	}
	result := importResult{Banks: []importedBank{{Name: "新题库", Questions: qs}}}
	a.finishImport(result, qs, nil)
	if a.onlyWrong || len(a.order) != 3 {
		t.Fatalf("random queue not rebuilt: %v %v", a.onlyWrong, a.order)
	}
}

func TestSmallExamValidationStaysVisible(t *testing.T) {
	a := newApp()
	a.openExamSetup()
	a.examForm.Counts = [5]float64{}
	tt := ui.NewTester(a.view, 800, 500)
	for _, label := range []string{"请至少设置一种题型的数量。", "开始考试"} {
		r, ok := tt.Find(label)
		if !ok || r.H <= 0 || r.Y < 0 || r.Y+r.H > 500 {
			t.Fatalf("hidden feedback %s %+v", label, r)
		}
	}
}

func TestResetPracticeConfirmationDoesNotEraseOnCancel(t *testing.T) {
	a := newApp()
	a.answers["m1"] = response{Choice: 2, Correct: true}
	tt := ui.NewTester(a.view, 1450, 800)
	click(t, tt, "功能引导")
	click(t, tt, "重新练习")
	if len(a.answers) != 1 || !tt.HasText("重置练习记录") {
		t.Fatal("reset was not confirmed")
	}
	click(t, tt, "取消")
	if len(a.answers) != 1 {
		t.Fatal("cancel erased answers")
	}
	click(t, tt, "功能引导")
	click(t, tt, "重新练习")
	click(t, tt, "确认重置")
	if len(a.answers) != 0 {
		t.Fatal("confirmed reset failed")
	}
}
