// 练习题序、筛选、选项映射和轮次切换。原始答案位掩码始终独立于选项显示顺序。

package desktop

import (
	"math/rand"
	"sort"

	"github.com/ispacewang/quesora-native/internal/domain"
)

// reorder 为当前筛选题集生成本轮随机 ID 顺序；不会在每次绘制时重新抽题。
func (a *app) reorder() {
	a.order = nil
	if !a.random {
		return
	}
	for _, q := range a.visible() {
		a.order = append(a.order, q.ID)
	}
	rand.Shuffle(len(a.order), func(i, j int) { a.order[i], a.order[j] = a.order[j], a.order[i] })
}
func (a *app) sortQuestions(qs []domain.Question) {
	if !a.random || len(a.order) == 0 {
		return
	}
	rank := map[string]int{}
	for i, id := range a.order {
		rank[id] = i
	}
	sort.SliceStable(qs, func(i, j int) bool { return rank[qs[i].ID] < rank[qs[j].ID] })
}
func (a *app) toggleFilter(kind string) {
	if kind == "全部" {
		a.filter = "全部"
		a.filters = nil
		a.importantOnly = false
	} else if kind == "重要" {
		a.importantOnly = !a.importantOnly
		a.filter = "全部"
		a.filters = nil
	} else {
		a.importantOnly = false
		if a.filters == nil {
			a.filters = map[string]bool{}
			if a.filter != "全部" {
				a.filters[a.filter] = true
			}
		}
		if a.filters[kind] {
			delete(a.filters, kind)
		} else {
			a.filters[kind] = true
		}
		a.filter = "全部"
	}
	a.reorder()
	a.navigate(0)
}

// prepareOptions 创建显示位置到原始选项索引的映射，不改写题目或正确答案。
func (a *app) prepareOptions() {
	a.optionOrder = nil
	q, ok := a.current()
	if !ok {
		return
	}
	for i := range q.Options {
		a.optionOrder = append(a.optionOrder, i)
	}
	if a.shuffle && (q.Kind == "单选题" || q.Kind == "多选题") {
		rand.Shuffle(len(a.optionOrder), func(i, j int) { a.optionOrder[i], a.optionOrder[j] = a.optionOrder[j], a.optionOrder[i] })
	}
}
func (a *app) displayOptions(q domain.Question) []int {
	if len(a.optionOrder) == len(q.Options) {
		return a.optionOrder
	}
	out := make([]int, len(q.Options))
	for i := range out {
		out[i] = i
	}
	return out
}

// displayAnswer 将原始正确答案映射成当前显示字母，避免选项打乱后解析字母错位。
func (a *app) displayAnswer(q domain.Question) string {
	if domain.IsTextQuestion(q) {
		return q.TextAnswer
	}
	var mask uint
	for display, original := range a.displayOptions(q) {
		if q.Answer&(1<<original) != 0 {
			mask |= 1 << display
		}
	}
	return domain.Letters(mask)
}
func (a *app) nextQuestion() {
	visible := a.visible()
	if a.index >= len(visible)-1 {
		a.stopQuickAdvance()
		a.finishOpen = true
		return
	}
	next := a.index + 1
	// Leaving a corrected question removes it from the wrong-only list, shifting the successor left.
	if id := visible[a.index].ID; a.onlyWrong && id == a.attemptAnsweredID && a.answers[id].Correct {
		next--
	}
	a.navigate(next)
}

// restartCycle 在重新作答模式保留最近记录；普通模式只清空当前题库的练习记录后开启新轮次。
func (a *app) restartCycle() {
	a.finishOpen = false
	if a.repeatPractice {
		a.reorder()
		a.navigate(0)
		return
	}
	for _, q := range a.questions {
		if q.Bank == a.bank {
			delete(a.answers, q.ID)
		}
	}
	if a.store != nil {
		s, name := a.store, a.bank
		a.enqueue(func() {
			err := s.ClearPractice(name)
			if err != nil {
				a.publish(func() { a.libraryStatus = "重新练习失败：" + err.Error() })
			}
		})
	}
	a.reorder()
	a.navigate(0)
}
