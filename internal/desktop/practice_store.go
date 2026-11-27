// 练习记录、重要标记和重置的持久化；考试/已保存错题本快照不由练习重置删除。

package desktop

import (
	"encoding/json"

	"github.com/ispacewang/quesora-native/internal/domain"
)

func (a *app) saveResponse(id string, r domain.Response) {
	if a.store == nil {
		return
	}
	s := a.store
	a.enqueue(func() {
		if err := s.SaveResponse(id, r); err != nil {
			a.publish(func() { a.libraryStatus = "答题记录保存失败：" + err.Error() })
		}
	})
}
func (a *app) toggleStar(id string) {
	a.starred[id] = !a.starred[id]
	if a.store == nil {
		return
	}
	raw, err := json.Marshal(a.starred)
	if err != nil {
		return
	}
	s := a.store
	a.enqueue(func() {
		err := s.SetSetting("stars", string(raw))
		if err != nil {
			a.publish(func() { a.libraryStatus = "重要题目标记保存失败：" + err.Error() })
		}
	})
}

// resetPractice 只清空日常练习；题库、重要标记和保存的考试/错题本不受影响。
func (a *app) resetPractice() {
	a.practiceRevision++
	a.savedWrong = false
	a.exportOpen = false
	a.answers = map[string]domain.Response{}
	a.onlyWrong = false
	a.navigate(0)
	if a.store != nil {
		s := a.store
		a.enqueue(func() {
			err := s.ClearPractice("")
			if err != nil {
				a.publish(func() { a.libraryStatus = "重置记录失败：" + err.Error() })
			}
		})
	}
}
