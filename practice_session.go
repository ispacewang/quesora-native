package main

import (
	"math/rand"
	"sort"
)

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
func (a *app) sortQuestions(qs []question) {
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
func (a *app) displayOptions(q question) []int {
	if len(a.optionOrder) == len(q.Options) {
		return a.optionOrder
	}
	out := make([]int, len(q.Options))
	for i := range out {
		out[i] = i
	}
	return out
}
func (a *app) displayAnswer(q question) string {
	if isTextQuestion(q) {
		return q.TextAnswer
	}
	var mask uint
	for display, original := range a.displayOptions(q) {
		if q.Answer&(1<<original) != 0 {
			mask |= 1 << display
		}
	}
	return letters(mask)
}
func (a *app) nextQuestion() {
	if a.index >= len(a.visible())-1 {
		a.finishOpen = true
		return
	}
	a.navigate(a.index + 1)
}
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
			_, err := s.db.Exec("DELETE FROM practice WHERE question_id IN (SELECT q.id FROM questions q JOIN banks b ON b.id=q.bank_id WHERE b.name=?)", name)
			if err != nil {
				a.publish(func() { a.libraryStatus = "重新练习失败：" + err.Error() })
			}
		})
	}
	a.reorder()
	a.navigate(0)
}

func isTextQuestion(q question) bool {
	return q.Kind == "填空题" || q.Kind == "简答题" || len(q.Options) == 0
}

func responseOrder(q question, r response) []int {
	if len(r.OptionOrder) == len(q.Options) {
		seen := map[int]bool{}
		valid := true
		for _, i := range r.OptionOrder {
			if i < 0 || i >= len(q.Options) || seen[i] {
				valid = false
				break
			}
			seen[i] = true
		}
		if valid {
			return r.OptionOrder
		}
	}
	order := make([]int, len(q.Options))
	for i := range order {
		order[i] = i
	}
	return order
}
func responseLetters(q question, r response, mask uint) string {
	var displayed uint
	for i, original := range responseOrder(q, r) {
		if mask&(1<<original) != 0 {
			displayed |= 1 << i
		}
	}
	return letters(displayed)
}
