package main

import (
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRepeatPracticeStartsFreshWithoutLosingLastRecord(t *testing.T) {
	a := newApp()
	a.choose(0)
	a.submit()
	q, _ := a.current()
	if _, done := a.currentResponse(q); !done {
		t.Fatal("submitted result missing")
	}
	a.toggleRepeatPractice()
	if _, done := a.currentResponse(q); done || a.chosen != 0 {
		t.Fatal("old answer exposed in repeat mode")
	}
	if _, bad := a.counts(); bad != 1 {
		t.Fatal("repeat toggle erased statistics")
	}
	a.choose(1)
	a.submit()
	if r, done := a.currentResponse(q); !done || !r.Correct {
		t.Fatal("new attempt not graded")
	}
	a.submit()
	if good, bad := a.counts(); good != 1 || bad != 0 {
		t.Fatal("latest response did not replace old one")
	}
	a.navigate(1)
	a.navigate(0)
	if _, done := a.currentResponse(q); done {
		t.Fatal("next visit is not fresh")
	}
	a.toggleRepeatPractice()
	if r, done := a.currentResponse(q); !done || !r.Correct || a.chosen != 2 {
		t.Fatal("history mode did not restore last result")
	}
}
func TestRepeatTextAndExplicitReview(t *testing.T) {
	a := newApp()
	a.repeatPractice = true
	a.answers["m4"] = response{Text: "1", Correct: true}
	a.navigate(3)
	q, _ := a.current()
	if a.typed != "" {
		t.Fatal("old text revealed")
	}
	a.typed = "2"
	a.submit()
	if a.answers[q.ID].Correct {
		t.Fatal("new text not graded")
	}
	a.answers["m1"] = response{Choice: 1, Correct: false, OptionOrder: []int{3, 0, 1, 2}}
	a.navigateForReview(0)
	q, _ = a.current()
	if _, done := a.currentResponse(q); !done || !reflect.DeepEqual(a.optionOrder, []int{3, 0, 1, 2}) {
		t.Fatal("explicit review lost result or option order")
	}
	a.navigate(0)
	if _, done := a.currentResponse(q); done {
		t.Fatal("review state leaked into another visit")
	}
	a.restartCycle()
	if len(a.answers) != 2 {
		t.Fatal("repeat cycle erased prior records")
	}
}
func TestRepeatModeSettingPersists(t *testing.T) {
	s, err := openQuestionStore(filepath.Join(t.TempDir(), "questions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if value, err := s.loadRepeatPractice(); err != nil || value {
		t.Fatal("default preference", value, err)
	}
	a := newApp()
	a.store = s
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	a.dispatch = func(fn func()) { fn() }
	a.toggleRepeatPractice()
	done := make(chan struct{})
	a.enqueue(func() { close(done) })
	<-done
	value, err := s.loadRepeatPractice()
	if err != nil || !value {
		t.Fatal("preference not saved", value, err)
	}
	if err = s.saveRepeatPractice(false); err != nil {
		t.Fatal(err)
	}
	if value, err = s.loadRepeatPractice(); err != nil || value {
		t.Fatal("preference not restored", value, err)
	}
}
func TestRepeatToggleUIAndWrongReview(t *testing.T) {
	a := newApp()
	a.choose(0)
	a.submit()
	tt := ui.NewTester(a.view, 1450, 800)
	click(t, tt, "重新作答")
	if !a.repeatPractice || tt.HasText("回答错误") {
		t.Fatal("repeat mode did not hide feedback")
	}
	click(t, tt, "B. 0")
	click(t, tt, "提交答案")
	if !tt.HasText("回答正确！") {
		t.Fatal("repeat answer not submitted")
	}
	a.navigate(1)
	a.choose(0)
	a.submit()
	tt.Frame()
	click(t, tt, "查看解析")
	if !tt.HasText("回答错误") {
		t.Fatal("wrong review concealed explanation")
	}
}
