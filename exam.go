package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

var examKinds = []string{"单选题", "多选题", "判断题", "简答题", "填空题"}

type examSettings struct {
	Bank    string
	Minutes float64
	Counts  [5]float64
}
type examSession struct {
	ID                                          string
	Bank                                        string
	Minutes                                     int
	Questions                                   []question
	Responses                                   []response
	Marked                                      []bool
	Important                                   []bool
	Current                                     int
	Started, Deadline, Finished                 time.Time
	Submitted, ImportantFailed, TimedOut, Saved bool
	Correct, Score                              int
	preview                                     bool
}

func newExamSession(qs []question, settings examSettings, stars map[string]bool, now time.Time) (*examSession, error) {
	if err := validateExamSettings(settings); err != nil {
		return nil, err
	}

	e := &examSession{ID: now.UTC().Format("20060102T150405.000000000"), Bank: settings.Bank, Minutes: int(settings.Minutes), Started: now, Deadline: now.Add(time.Duration(settings.Minutes) * time.Minute)}
	for k, kind := range examKinds {
		count := settings.Counts[k]

		var pool []question
		for _, q := range qs {
			if q.Bank == settings.Bank && q.Kind == kind {
				pool = append(pool, q)
			}
		}
		rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		for _, q := range pool[:min(len(pool), int(count))] {
			e.Questions = append(e.Questions, q)
			r := response{OptionOrder: responseOrder(q, response{})}
			if kind == "单选题" || kind == "多选题" {
				rand.Shuffle(len(r.OptionOrder), func(i, j int) { r.OptionOrder[i], r.OptionOrder[j] = r.OptionOrder[j], r.OptionOrder[i] })
			}
			e.Responses = append(e.Responses, r)
			e.Marked = append(e.Marked, false)
			important, _ := q.Meta["isBaoMing"].(bool)
			e.Important = append(e.Important, important || stars[q.ID])
		}
	}
	if len(e.Questions) == 0 {
		return nil, fmt.Errorf("该题库中没有符合数量设置的题目")
	}
	return e, nil
}
func judgeQuestion(q question, r response) bool {
	if isTextQuestion(q) {
		if strings.TrimSpace(r.Text) == "" {
			return false
		}
		if q.Kind == "简答题" {
			return judgeShortAnswer(r.Text, q.TextAnswer)
		}
		return normalizeAnswer(r.Text) == normalizeAnswer(q.TextAnswer)
	}
	return r.Choice != 0 && q.Answer != 0 && r.Choice == q.Answer
}
func (e *examSession) answered(i int) bool {
	if isTextQuestion(e.Questions[i]) {
		return strings.TrimSpace(e.Responses[i].Text) != ""
	}
	return e.Responses[i].Choice != 0
}
func (e *examSession) completed() int {
	n := 0
	for i := range e.Questions {
		if e.answered(i) {
			n++
		}
	}
	return n
}
func (e *examSession) remaining(now time.Time) int {
	if e.Submitted {
		return 0
	}
	return max(0, int(math.Ceil(e.Deadline.Sub(now).Seconds())))
}
func (e *examSession) tick(now time.Time) {
	if !e.Submitted && !now.Before(e.Deadline) {
		e.TimedOut = true
		e.submit(e.Deadline)
	}
}
func (e *examSession) firstMissing() int {
	for i := range e.Questions {
		if !e.answered(i) {
			return i
		}
	}
	return -1
}
func (e *examSession) submit(now time.Time) {
	if e.Submitted {
		return
	}
	e.Submitted = true
	e.Finished = now
	e.Correct = 0
	e.ImportantFailed = false
	for i, q := range e.Questions {
		e.Responses[i].Correct = judgeQuestion(q, e.Responses[i])
		if e.Responses[i].Correct {
			e.Correct++
		} else if e.Important[i] {
			e.ImportantFailed = true
		}
	}
	e.Score = e.Correct
	if e.ImportantFailed {
		e.Score = 0
	}
}
func (e *examSession) choose(original int) {
	if e.Submitted || original < 0 || original >= len(e.Questions[e.Current].Options) {
		return
	}
	r := &e.Responses[e.Current]
	if e.Questions[e.Current].Kind == "多选题" {
		r.Choice ^= 1 << original
	} else {
		r.Choice = 1 << original
	}
}
func (e *examSession) gradeText() string {
	if e.ImportantFailed {
		return "重要题目失守，综合成绩归零"
	}
	ratio := float64(e.Score) / float64(len(e.Questions))
	if ratio == 1 {
		return "满分！太厉害了！"
	}
	if ratio >= .8 {
		return "优秀，继续加油！"
	}
	if ratio >= .6 {
		return "不错，还有进步空间"
	}
	return "继续努力，你可以的！"
}
func (a *app) openExamSetup() {
	a.paper = nil
	a.examForm = examSettings{Bank: a.bank, Minutes: 60, Counts: [5]float64{40, 30, 30, 0, 0}}
	a.examSetupOpen = true
	a.examMessage = ""
}
func (a *app) startExam() {
	e, err := a.prepareExamPaper()
	if err != nil {
		a.examMessage = err.Error()
		return
	}
	now := time.Now()
	e.Started = now
	e.ID = now.UTC().Format("20060102T150405.000000000")
	e.Deadline = now.Add(time.Duration(e.Minutes) * time.Minute)
	a.paper = nil
	a.exam = e
	a.examSetupOpen = false
	a.examMessage = ""
}
func (a *app) requestExamSubmit() {
	if a.exam == nil || a.exam.Submitted {
		return
	}
	a.exam.tick(time.Now())
	if a.exam.Submitted {
		return
	}
	if i := a.exam.firstMissing(); i >= 0 {
		a.exam.Current = i
		a.examMessage = fmt.Sprintf("第 %d 题未作答", i+1)
		return
	}
	a.examSubmitOpen = true
}
func (a *app) exitExam() {
	if a.exam == nil {
		return
	}
	if a.exam.Saved {
		a.exam = nil
	} else {
		a.examExitOpen = true
	}
}
func (a *app) saveExam() {
	e := a.exam
	if e == nil || !e.Submitted || e.Saved || a.examSaving {
		return
	}
	if a.store == nil {
		a.examMessage = "本地数据库不可用，成绩尚未保存"
		return
	}
	data, err := json.Marshal(e)
	if err != nil {
		a.examMessage = err.Error()
		return
	}
	s, id := a.store, e.ID
	a.examSaving = true
	if !a.enqueue(func() {
		_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", "exam:"+id, string(data))
		a.publish(func() {
			a.examSaving = false
			if err != nil {
				a.examMessage = "保存成绩失败：" + err.Error()
				return
			}
			e.Saved = true
			a.libraryStatus = "成绩已保存"
		})
	}) {
		a.examSaving = false
	}
}

type historyRecord struct {
	Key      string
	Exam     *examSession
	Mistakes []savedMistake
	Error    string
}
type savedMistake struct {
	Question question
	Response response
}

func (s *questionStore) loadHistory() ([]historyRecord, error) {
	rows, err := s.db.Query("SELECT key,value FROM settings WHERE key LIKE 'exam:%' OR key LIKE 'mistake-book:%' ORDER BY key DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []historyRecord{}
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		r := historyRecord{Key: key}
		if strings.HasPrefix(key, "exam:") {
			e := &examSession{}
			if err = json.Unmarshal([]byte(value), e); err == nil && len(e.Questions) > 0 && len(e.Responses) == len(e.Questions) && len(e.Important) == len(e.Questions) && len(e.Marked) == len(e.Questions) {
				e.Saved = true
				r.Exam = e
			} else {
				r.Error = "成绩记录无法读取"
			}
		} else {
			if err = json.Unmarshal([]byte(value), &r.Mistakes); err != nil {
				r.Error = "错题记录无法读取"
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (a *app) openHistory() {
	a.historyOpen = true
	a.historyMessage = ""
	if a.store == nil {
		return
	}
	if a.historyLoading {
		return
	}
	a.historyLoading = true
	s := a.store
	if !a.enqueue(func() {
		records, err := s.loadHistory()
		a.publish(func() {
			a.historyLoading = false
			if err != nil {
				a.historyMessage = "读取记录失败：" + err.Error()
				return
			}
			a.history = records
		})
	}) {
		a.historyLoading = false
	}
}
func (a *app) deleteHistory() {
	key := a.historyDeleteKey
	if key == "" || a.store == nil {
		return
	}
	s := a.store
	if a.enqueue(func() {
		_, err := s.db.Exec("DELETE FROM settings WHERE key=?", key)
		a.publish(func() {
			if err != nil {
				a.historyMessage = "删除失败：" + err.Error()
				return
			}
			for i, r := range a.history {
				if r.Key == key {
					a.history = append(a.history[:i], a.history[i+1:]...)
					break
				}
			}
			a.historyDeleteKey = ""
			a.historyDeleteOpen = false
		})
	}) {
		a.historyDeleteOpen = false
	}
}

// Keep a paper stable across the paired PDF exports and the subsequent exam.
func (a *app) prepareExamPaper() (*examSession, error) {
	if a.paper != nil && a.paperSettings == a.examForm {
		return a.paper, nil
	}
	e, err := newExamSession(a.questions, a.examForm, a.starred, time.Now())
	if err != nil {
		return nil, err
	}
	a.paper = e
	a.paperSettings = a.examForm
	return e, nil
}
func snapshotExamPaper(e *examSession) *examSession {
	copy := *e
	copy.Questions = append([]question(nil), e.Questions...)
	copy.Responses = append([]response(nil), e.Responses...)
	for i := range copy.Responses {
		copy.Responses[i].OptionOrder = append([]int(nil), e.Responses[i].OptionOrder...)
	}
	copy.Marked = append([]bool(nil), e.Marked...)
	copy.Important = append([]bool(nil), e.Important...)
	return &copy
}

func validateExamSettings(settings examSettings) error {
	if settings.Bank == "" {
		return fmt.Errorf("请选择题库")
	}
	if math.IsNaN(settings.Minutes) || math.IsInf(settings.Minutes, 0) || settings.Minutes < 1 || settings.Minutes > 180 || settings.Minutes != math.Trunc(settings.Minutes) {
		return fmt.Errorf("考试时长应为 1–180 的整数分钟")
	}
	for i, count := range settings.Counts {
		if math.IsNaN(count) || math.IsInf(count, 0) || count < 0 || count > 100 || count != math.Trunc(count) {
			return fmt.Errorf("%s数量应为 0–100 的整数", examKinds[i])
		}
	}
	return nil
}
func (e *examSession) percentage() int { return roundedPercent(e.Score, len(e.Questions)) }
