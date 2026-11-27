// 已提交记录和本次访问状态的分离：重新作答只控制旧答案可见性，提交后才替换历史最近答案。

package desktop

import (
	"github.com/ispacewang/quesora-native/internal/domain"
)

// Practice statistics retain the last submitted response. The current visit
// independently decides whether that response is visible or can be replaced.
func (a *app) currentResponse(q domain.Question) (domain.Response, bool) {
	if a.repeatPractice && a.attemptAnsweredID != q.ID && a.reviewedQuestionID != q.ID {
		return domain.Response{}, false
	}
	r, ok := a.answers[q.ID]
	return r, ok
}
func (a *app) restoreCurrentAnswer() {
	if q, ok := a.current(); ok {
		if r, done := a.answers[q.ID]; done {
			a.chosen = r.Choice
			a.typed = r.Text
			a.optionOrder = append([]int(nil), domain.ResponseOrder(q, r)...)
		}
	}
}
func (a *app) navigateForReview(index int) {
	a.navigate(index)
	if q, ok := a.current(); ok {
		a.reviewedQuestionID = q.ID
	}
	a.restoreCurrentAnswer()
}
func (a *app) toggleRepeatPractice() {
	a.repeatPractice = !a.repeatPractice
	a.navigate(a.index)
	if a.store == nil {
		return
	}
	s, value := a.store, a.repeatPractice
	a.enqueue(func() {
		if err := s.SaveRepeatPractice(value); err != nil {
			a.publish(func() { a.libraryStatus = "重新作答设置保存失败：" + err.Error() })
		}
	})
}
