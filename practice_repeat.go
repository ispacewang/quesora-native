package main

import (
	"database/sql"
	"errors"
	"strconv"
)

// Practice statistics retain the last submitted response. The current visit
// independently decides whether that response is visible or can be replaced.
func (a *app) currentResponse(q question) (response, bool) {
	if a.repeatPractice && a.attemptAnsweredID != q.ID && a.reviewedQuestionID != q.ID {
		return response{}, false
	}
	r, ok := a.answers[q.ID]
	return r, ok
}
func (a *app) restoreCurrentAnswer() {
	if q, ok := a.current(); ok {
		if r, done := a.answers[q.ID]; done {
			a.chosen = r.Choice
			a.typed = r.Text
			a.optionOrder = append([]int(nil), responseOrder(q, r)...)
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
		if err := s.saveRepeatPractice(value); err != nil {
			a.publish(func() { a.libraryStatus = "重新作答设置保存失败：" + err.Error() })
		}
	})
}
func (s *questionStore) loadRepeatPractice() (bool, error) {
	var raw string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key='repeat-practice'").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(raw)
}
func (s *questionStore) saveRepeatPractice(value bool) error {
	_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES('repeat-practice',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", strconv.FormatBool(value))
	return err
}
