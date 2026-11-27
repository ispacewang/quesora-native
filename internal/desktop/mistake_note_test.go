package desktop

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/ispacewang/quesora-native/internal/domain"
	"github.com/ispacewang/quesora-native/internal/importer"
)

// Wait behind the real serial persistence queue, then process its UI acknowledgements.
func drainMistakeJobs(t *testing.T, a *app, updates <-chan func()) {
	t.Helper()
	done := make(chan struct{})
	if !a.enqueue(func() { a.publish(func() { close(done) }) }) {
		t.Fatal("could not queue note barrier")
	}
	timeout := time.After(5 * time.Second)
	for {
		select {
		case f := <-updates:
			f()
		case <-done:
			return
		case <-timeout:
			t.Fatal("note persistence timed out")
		}
	}
}

func TestMistakeNoteEditorPersistsAndExports(t *testing.T) {
	a := newApp()
	s, e := openQuestionStore(filepath.Join(t.TempDir(), "questions.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	batch := importer.Result{Banks: []importer.Bank{{Name: "数学基础", Questions: demoQuestions()[:2]}}}
	if e = s.ImportBanks(batch); e != nil {
		t.Fatal(e)
	}
	a.questions, e = s.LoadQuestions()
	if e != nil {
		t.Fatal(e)
	}
	q := a.questions[0]
	a.store = s
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	updates := make(chan func(), 32)
	a.dispatch = func(f func()) { updates <- f }
	a.answers[q.ID] = domain.Response{Choice: 1, Correct: false}
	if e = s.SaveResponse(q.ID, a.answers[q.ID]); e != nil {
		t.Fatal(e)
	}
	a.navigate(0)
	tt := ui.NewTester(a.view, 1450, 1100)
	if _, ok := tt.Find("错题备注"); !ok {
		t.Fatal("the original mistake-note editor is missing")
	}
	click(t, tt, "错题备注")
	tt.Type("负数乘法要改变不等号方向")
	click(t, tt, "保存备注")
	drainMistakeJobs(t, a, updates)
	tt.Frame()
	got, _, e := s.LoadPractice()
	if e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(got[q.ID])
	if !strings.Contains(string(data), "负数乘法要改变不等号方向") {
		t.Fatal("note was not persisted", string(data))
	}
	for _, md := range []bool{false, true} {
		text := a.wrongText(md)
		if !strings.Contains(text, "我的备注") || !strings.Contains(text, "负数乘法要改变不等号方向") {
			t.Fatal("note missing from export", text)
		}
	}
	a.saveWrongBook()
	drainMistakeJobs(t, a, updates)
	history, e := s.loadHistory()
	if e != nil || len(history) != 1 {
		t.Fatal("saved mistake book missing", e)
	}
	snapshot, _ := json.Marshal(history)
	if !strings.Contains(string(snapshot), "负数乘法要改变不等号方向") {
		t.Fatal("saved book lost its note")
	}
	a.answers = got
	a.navigate(1)
	a.navigate(0)
	tt.Frame()
	if !tt.HasText("负数乘法要改变不等号方向") {
		t.Fatal("note unavailable after revisiting")
	}
	a.quick = true
	tt.Frame()
	if _, ok := tt.Find("错题备注"); ok {
		t.Fatal("note editor should follow the original non-quick practice layout")
	}
}

// A late save acknowledgement must not restore a note after learning records were reset.
func TestMistakeNotePendingSaveDoesNotSurviveReset(t *testing.T) {
	a, _, updates := mistakeNoteFixture(t)
	q, _ := a.current()
	a.mistakeNoteID, a.mistakeNote = q.ID, "old note"
	a.saveMistakeNote()
	a.resetPractice()
	a.recordPracticeAnswer(q, domain.Response{Choice: 1})
	drainMistakeJobs(t, a, updates)
	if a.answers[q.ID].Note != "" {
		t.Fatal("old save acknowledgement resurrected reset note")
	}
	a.mistakeNoteID, a.mistakeNote = q.ID, "unsaved draft"
	a.resetPractice()
	a.recordPracticeAnswer(q, domain.Response{Choice: 1})
	ui.NewTester(a.view, 1450, 1100)
	if a.mistakeNote != "" {
		t.Fatal("editor reused pre-reset draft")
	}
}

func mistakeNoteFixture(t *testing.T) (*app, *questionStore, <-chan func()) {
	t.Helper()
	a := newApp()
	s, e := openQuestionStore(filepath.Join(t.TempDir(), "questions.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	if e = s.ImportBanks(importer.Result{Banks: []importer.Bank{{Name: "数学基础", Questions: demoQuestions()[:2]}}}); e != nil {
		t.Fatal(e)
	}
	a.questions, e = s.LoadQuestions()
	if e != nil {
		t.Fatal(e)
	}
	a.store = s
	q := a.questions[0]
	a.answers[q.ID] = domain.Response{Choice: 1}
	if e = s.SaveResponse(q.ID, a.answers[q.ID]); e != nil {
		t.Fatal(e)
	}
	a.jobs = newLibraryJobs()
	t.Cleanup(a.jobs.close)
	updates := make(chan func(), 64)
	a.dispatch = func(f func()) { updates <- f }
	a.navigate(0)
	return a, s, updates
}

func TestMistakeNoteKeepsLatestGradeAndClearsDurably(t *testing.T) {
	a, s, updates := mistakeNoteFixture(t)
	q, _ := a.current()
	a.mistakeNoteID, a.mistakeNote = q.ID, "first note"
	a.saveMistakeNote()
	// Re-answer before the UI receives the database acknowledgement.
	a.recordPracticeAnswer(q, domain.Response{Choice: q.Answer, Correct: true})
	drainMistakeJobs(t, a, updates)
	got, _, e := s.LoadPractice()
	if e != nil {
		t.Fatal(e)
	}
	if !got[q.ID].Correct || got[q.ID].Note != "first note" || !a.answers[q.ID].Correct || a.answers[q.ID].Note != "first note" {
		t.Fatal("note save replaced the latest grade", got, a.answers)
	}
	a.recordPracticeAnswer(q, domain.Response{Choice: 1})
	drainMistakeJobs(t, a, updates)
	a.mistakeNote = ""
	a.saveMistakeNote()
	// This grade captured the old note while the clear is still queued.
	a.recordPracticeAnswer(q, domain.Response{Choice: 1})
	drainMistakeJobs(t, a, updates)
	reopened, e := openQuestionStore(s.path())
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	got, _, e = reopened.LoadPractice()
	if e != nil {
		t.Fatal(e)
	}
	if got[q.ID].Note != "" || a.answers[q.ID].Note != "" {
		t.Fatal("cleared note reappeared", got)
	}
}

func TestMistakeNoteFailureKeepsDraft(t *testing.T) {
	a, s, updates := mistakeNoteFixture(t)
	q, _ := a.current()
	a.mistakeNoteID, a.mistakeNote = q.ID, "keep this draft"
	if _, e := fixtureDB(t, s).Exec("DELETE FROM practice"); e != nil {
		t.Fatal(e)
	}
	a.saveMistakeNote()
	drainMistakeJobs(t, a, updates)
	if a.mistakeNoteSaving || a.mistakeNote != "keep this draft" || a.mistakeNoteError == "" || a.answers[q.ID].Note != "" {
		t.Fatal("failed save lost draft or claimed success")
	}
	tt := ui.NewTester(a.view, 1450, 1100)
	if !tt.HasText(a.mistakeNoteError) || tt.HasText("已保存") {
		t.Fatal("failure feedback is missing")
	}
}

func TestMistakeNoteSnapshotsStayIndependent(t *testing.T) {
	a, s, updates := mistakeNoteFixture(t)
	q, _ := a.current()
	a.mistakeNoteID, a.mistakeNote = q.ID, "snapshot note"
	a.saveMistakeNote()
	drainMistakeJobs(t, a, updates)
	a.saveWrongBook()
	drainMistakeJobs(t, a, updates)
	a.mistakeNote = "new note"
	a.saveMistakeNote()
	drainMistakeJobs(t, a, updates)
	history, e := s.loadHistory()
	if e != nil || len(history) != 1 {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(history)
	if !strings.Contains(string(raw), "snapshot note") || strings.Contains(string(raw), "new note") || a.savedWrong {
		t.Fatal("editing changed saved snapshot")
	}
	tt := ui.NewTester(func(c *ui.Context) { reviewQuestion(c, q, history[0].Mistakes[0].Response, false, 0) }, 800, 1100)
	if !tt.HasText("snapshot note") {
		t.Fatal("saved history omitted note")
	}
}

func TestMistakeNoteEditorReachableAtMinimumSize(t *testing.T) {
	a, _, updates := mistakeNoteFixture(t)
	a.leftOpen, a.rightOpen = false, false
	tt := ui.NewTester(a.view, 800, 500)
	tt.Scroll(400, 350, 0, 1200)
	click(t, tt, "错题备注")
	tt.Type("窄窗口备注")
	click(t, tt, "保存备注")
	drainMistakeJobs(t, a, updates)
	tt.Frame()
	if !tt.HasText("已保存") {
		t.Fatal("note save feedback unreachable")
	}
}
