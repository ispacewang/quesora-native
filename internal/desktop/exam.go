// 考试领域状态、组卷、计时、判分和记录保存。考试作答与日常练习 answers 相互隔离。

package desktop

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/ispacewang/quesora-native/internal/domain"
)

// examSettings 与组卷表单双向绑定；数值使用 float64 以保留 NaN 等无效输入供统一校验。
type examSettings struct {
	Bank    string
	Minutes float64
	Counts  [5]float64
}

// examSession 是与练习隔离的试卷快照；题序、选项映射和重要题规则在组卷时固定。
type examSession struct {
	ID                                          string
	Bank                                        string
	Minutes                                     int
	Questions                                   []domain.Question
	Responses                                   []domain.Response
	Marked                                      []bool
	Important                                   []bool
	Current                                     int
	Started, Deadline, Finished                 time.Time
	Submitted, ImportantFailed, TimedOut, Saved bool
	Correct, Score                              int
	preview                                     bool
}

// newExamSession 按题型随机无重复抽题，库存不足取现有数量，并固定每题的选项顺序。
func newExamSession(qs []domain.Question, settings examSettings, stars map[string]bool, now time.Time) (*examSession, error) {
	if err := validateExamSettings(settings); err != nil {
		return nil, err
	}

	e := &examSession{ID: now.UTC().Format("20060102T150405.000000000"), Bank: settings.Bank, Minutes: int(settings.Minutes), Started: now, Deadline: now.Add(time.Duration(settings.Minutes) * time.Minute)}
	for k, kind := range domain.QuestionKinds() {
		count := settings.Counts[k]

		var pool []domain.Question
		for _, q := range qs {
			if q.Bank == settings.Bank && q.Kind == kind {
				pool = append(pool, q)
			}
		}
		rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		for _, q := range pool[:min(len(pool), int(count))] {
			e.Questions = append(e.Questions, q)
			r := domain.Response{OptionOrder: domain.ResponseOrder(q, domain.Response{})}
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

func (e *examSession) answered(i int) bool {
	if domain.IsTextQuestion(e.Questions[i]) {
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

// remaining 按绝对截止时间计算向上取整的剩余秒数，避免依赖绘制/定时器触发次数。
func (e *examSession) remaining(now time.Time) int {
	if e.Submitted {
		return 0
	}
	return max(0, int(math.Ceil(e.Deadline.Sub(now).Seconds())))
}

// tick 使到期交卷幂等，并用真实截止时间作为结束时间。
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

// submit 判分并锁定考试结果；重要题答错/未答将综合分归零，但保留实际答对题数。
func (e *examSession) submit(now time.Time) {
	if e.Submitted {
		return
	}
	e.Submitted = true
	e.Finished = now
	e.Correct = 0
	e.ImportantFailed = false
	for i, q := range e.Questions {
		e.Responses[i].Correct = domain.JudgeQuestion(q, e.Responses[i])
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

// saveExam 在 UI 线程捕获完整 JSON 快照，再串行落库；只有成功保存后才标记为 Saved。
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
		err := s.SetSetting("exam:"+id, string(data))
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

// historyRecord 容纳考试或错题本快照；单条无法解码时保留错误信息而非中断整个历史列表。
type historyRecord struct {
	Key      string
	Exam     *examSession
	Mistakes []savedMistake
	Error    string
}

// savedMistake 同时保存题目和提交，后续题库修改或删除不改变已保存错题本内容。
type savedMistake struct {
	Question domain.Question
	Response domain.Response
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
		err := s.DeleteSetting(key)
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

// snapshotExamPaper 复制后台 PDF 导出需要的可变切片，防止选项顺序随后续操作变化。
func snapshotExamPaper(e *examSession) *examSession {
	copy := *e
	copy.Questions = append([]domain.Question(nil), e.Questions...)
	copy.Responses = append([]domain.Response(nil), e.Responses...)
	for i := range copy.Responses {
		copy.Responses[i].OptionOrder = append([]int(nil), e.Responses[i].OptionOrder...)
	}
	copy.Marked = append([]bool(nil), e.Marked...)
	copy.Important = append([]bool(nil), e.Important...)
	return &copy
}

// validateExamSettings 拒绝空题库、非整数、NaN/Inf 和越界数量，避免先转换为整数掩盖错误。
func validateExamSettings(settings examSettings) error {
	if settings.Bank == "" {
		return fmt.Errorf("请选择题库")
	}
	if math.IsNaN(settings.Minutes) || math.IsInf(settings.Minutes, 0) || settings.Minutes < 1 || settings.Minutes > 180 || settings.Minutes != math.Trunc(settings.Minutes) {
		return fmt.Errorf("考试时长应为 1–180 的整数分钟")
	}
	for i, count := range settings.Counts {
		if math.IsNaN(count) || math.IsInf(count, 0) || count < 0 || count > 100 || count != math.Trunc(count) {
			return fmt.Errorf("%s数量应为 0–100 的整数", domain.QuestionKinds()[i])
		}
	}
	return nil
}
func (e *examSession) percentage() int { return roundedPercent(e.Score, len(e.Questions)) }
