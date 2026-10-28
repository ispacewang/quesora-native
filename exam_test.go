package main

import (
	"bytes"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func examFixture(t *testing.T) *examSession {
	t.Helper()
	a := newApp()
	e, err := newExamSession(a.questions, examSettings{Bank: a.bank, Minutes: 1, Counts: [5]float64{40, 30, 30, 10, 10}}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestExamPaperGradingAndDeadline(t *testing.T) {
	e := examFixture(t)
	if len(e.Questions) != 6 {
		t.Fatal(len(e.Questions))
	}
	seen := map[string]bool{}
	for i, q := range e.Questions {
		if seen[q.ID] {
			t.Fatal("repeated question")
		}
		seen[q.ID] = true
		e.Responses[i].Choice = q.Answer
		e.Responses[i].Text = q.TextAnswer
		e.Responses[i].OptionOrder = responseOrder(q, e.Responses[i])
	}
	if e.firstMissing() != -1 || e.completed() != 6 {
		t.Fatal("answers lost")
	}
	e.submit(e.Started.Add(time.Second))
	if e.Score != 6 || e.Correct != 6 {
		t.Fatalf("%+v", e)
	}
	e.Responses[0].Choice = 0
	e.submit(time.Now())
	if e.Score != 6 {
		t.Fatal("resubmission changed score")
	}
	e = examFixture(t)
	e.Important[0] = true
	e.tick(e.Deadline.Add(10 * time.Second))
	if !e.Submitted || !e.TimedOut || !e.ImportantFailed || e.Score != 0 || !e.Finished.Equal(e.Deadline) {
		t.Fatalf("%+v", e)
	}
}
func TestExamImportantRuleKeepsActualCorrectCount(t *testing.T) {
	e := examFixture(t)
	for i, q := range e.Questions {
		e.Responses[i].Choice = q.Answer
		e.Responses[i].Text = q.TextAnswer
	}
	e.Important[0] = true
	e.Responses[0].Choice = 0
	e.submit(time.Now())
	if e.Score != 0 || e.Correct != 5 || !e.ImportantFailed {
		t.Fatalf("%+v", e)
	}
}
func TestExamUIFlow(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1450, 800)
	click(t, tt, "考试")
	a.examForm.Counts[3] = 10
	a.examForm.Counts[4] = 10
	click(t, tt, "开始考试")
	if a.exam == nil {
		t.Fatal(a.examMessage)
	}
	click(t, tt, "标记疑题")
	if !a.exam.Marked[0] {
		t.Fatal("mark")
	}
	click(t, tt, "交卷")
	if a.examSubmitOpen || a.examMessage == "" {
		t.Fatal("missing answer guard")
	}
	for i, q := range a.exam.Questions {
		a.exam.Current = i
		tt.Frame()
		if isTextQuestion(q) {
			click(t, tt, "考试填写答案")
			tt.Type(q.TextAnswer)
		} else {
			r := a.exam.Responses[i]
			for display, original := range responseOrder(q, r) {
				if q.Answer&(1<<original) != 0 {
					click(t, tt, "考试选项"+string(rune('A'+display))+". "+q.Options[original])
				}
			}
		}
	}
	click(t, tt, "交卷")
	click(t, tt, "确认交卷")
	if !a.exam.Submitted || a.exam.Score != len(a.exam.Questions) {
		t.Fatal("exam result", a.exam)
	}
	if len(a.answers) != 0 {
		t.Fatal("exam changed practice")
	}
	click(t, tt, "返回题库")
	click(t, tt, "确认退出考试")
	if a.exam != nil {
		t.Fatal("exit")
	}
}
func TestImportantAnimationDoesNotBlockAnswers(t *testing.T) {
	a := newApp()
	a.starred["m1"] = true
	tt := ui.NewTester(a.view, 1450, 860)
	r, _ := tt.Find("B. 0")
	click(t, tt, "B. 0")
	if a.chosen != 2 {
		t.Fatalf("choice=%d bounds=%+v", a.chosen, r)
	}
	click(t, tt, "提交答案")
	if good, _ := a.counts(); good != 1 {
		t.Fatalf("answers=%+v chosen=%d", a.answers, a.chosen)
	}
	angle, alpha, active := importantFrame(time.Second, true)
	if angle != -90 || alpha != 0 || active {
		t.Fatal("exit never stops")
	}
	_, alpha, active = importantFrame(time.Second, false)
	if alpha != 1 || active {
		t.Fatal("entry never stops")
	}
}
func TestExamHistoryRoundTripAndPDF(t *testing.T) {
	s, err := openQuestionStore(filepath.Join(t.TempDir(), "questions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	e := examFixture(t)
	e.submit(time.Now())
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO settings(key,value) VALUES(?,?)", "exam:"+e.ID, data); err != nil {
		t.Fatal(err)
	}
	rs, err := s.loadHistory()
	if err != nil || len(rs) != 1 || rs[0].Exam.Bank != e.Bank || !rs[0].Exam.Saved {
		t.Fatal(err, rs)
	}
	for _, answers := range []bool{false, true} {
		p := filepath.Join(t.TempDir(), "paper.pdf")
		if err = writeExamPDF(p, e, answers); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(p)
		if !bytes.HasPrefix(b, []byte("%PDF-")) || !bytes.Contains(b, []byte("/Type /Page")) {
			t.Fatal("invalid pdf")
		}
	}
}

func TestExamLayoutsAndHistoryDrawer(t *testing.T) {
	for _, size := range [][2]int{{800, 500}, {1000, 680}, {1450, 800}} {
		a := newApp()
		a.exam = examFixture(t)
		tt := ui.NewTester(a.view, size[0], size[1])
		for _, label := range []string{"考试倒计时", "标记疑题", "考试第1题"} {
			r, ok := tt.Find(label)
			if !ok || r.W <= 0 || r.X < 0 || r.X+r.W > float32(size[0])+1 {
				t.Fatalf("%v %s %+v", size, label, r)
			}
		}
		a.exam.submit(time.Now())
		tt.Frame()
		click(t, tt, "保存成绩")
		if a.exam.Saved {
			t.Fatal("saved without database")
		}
	}
	a := newApp()
	a.historyOpen = true
	e := examFixture(t)
	e.submit(time.Now())
	e.Saved = true
	a.history = []historyRecord{{Key: "exam:test", Exam: e}, {Key: "mistake-book:test", Mistakes: []savedMistake{{e.Questions[0], e.Responses[0]}}}}
	tt := ui.NewTester(a.view, 1450, 800)
	click(t, tt, "展开记录：exam:test")
	tt.Frame()
	click(t, tt, "关闭我的")
	tt.Frame()
	if a.historyOpen {
		t.Fatal("history not closed")
	}
}

func TestPairedPDFAndExamUseSamePaper(t *testing.T) {
	a := newApp()
	a.openExamSetup()
	p, err := a.prepareExamPaper()
	if err != nil {
		t.Fatal(err)
	}
	again, err := a.prepareExamPaper()
	if err != nil || !reflect.DeepEqual(p.Questions, again.Questions) || !reflect.DeepEqual(p.Responses, again.Responses) {
		t.Fatal("export pair changed paper")
	}
	snapshot := snapshotExamPaper(p)
	a.startExam()
	if !reflect.DeepEqual(a.exam.Questions, snapshot.Questions) || !reflect.DeepEqual(a.exam.Responses, snapshot.Responses) {
		t.Fatal("exam did not use exported paper")
	}
	a.exam.Responses[0].Choice = 1
	if snapshot.Responses[0].Choice != 0 {
		t.Fatal("export snapshot shares mutable answers")
	}
	a.openExamSetup()
	p, err = a.prepareExamPaper()
	if err != nil {
		t.Fatal(err)
	}
	a.examForm.Counts[0] = 1
	newPaper, err := a.prepareExamPaper()
	if err != nil || len(newPaper.Questions) >= len(p.Questions) {
		t.Fatal("changed settings reused old paper")
	}
}
