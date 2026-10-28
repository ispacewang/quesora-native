package main

import (
	"database/sql"
	"encoding/json"
	"errors"
)

func (s *questionStore) loadPractice() (map[string]response, map[string]bool, error) {
	answers := map[string]response{}
	stars := map[string]bool{}
	rows, err := s.db.Query("SELECT question_id,payload FROM practice")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, data string
		if err = rows.Scan(&id, &data); err != nil {
			return nil, nil, err
		}
		var r response
		if err = json.Unmarshal([]byte(data), &r); err != nil {
			return nil, nil, err
		}
		answers[id] = r
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	rows.Close()
	var raw string
	err = s.db.QueryRow("SELECT value FROM settings WHERE key='stars'").Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}
	if err == nil {
		if err = json.Unmarshal([]byte(raw), &stars); err != nil {
			return nil, nil, err
		}
	}
	return answers, stars, nil
}
func (s *questionStore) saveResponse(id string, r response) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO practice(question_id,payload) VALUES(?,?) ON CONFLICT(question_id) DO UPDATE SET payload=excluded.payload", id, string(payload))
	return err
}
func (a *app) saveResponse(id string, r response) {
	if a.store == nil {
		return
	}
	s := a.store
	a.enqueue(func() {
		if err := s.saveResponse(id, r); err != nil {
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
		_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES('stars',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(raw))
		if err != nil {
			a.publish(func() { a.libraryStatus = "重要题目标记保存失败：" + err.Error() })
		}
	})
}
func (a *app) resetPractice() {
	a.savedWrong = false
	a.exportOpen = false
	a.answers = map[string]response{}
	a.onlyWrong = false
	a.navigate(0)
	if a.store != nil {
		s := a.store
		a.enqueue(func() {
			_, err := s.db.Exec("DELETE FROM practice")
			if err != nil {
				a.publish(func() { a.libraryStatus = "重置记录失败：" + err.Error() })
			}
		})
	}
}
