// 练习持久化独立于窗口状态；更新备注时不会覆盖较新的作答。
package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ispacewang/quesora-native/internal/domain"
)

// LoadPractice 先关闭结果集再读取重要标记，避免单连接仓库被未释放游标阻塞。
func (s *Store) LoadPractice() (map[string]domain.Response, map[string]bool, error) {
	answers := map[string]domain.Response{}
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
		var r domain.Response
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

// SaveResponse 覆盖最近作答，并保留队列中已提交的备注，兼容没有 Note 字段的旧记录。
func (s *Store) SaveResponse(id string, r domain.Response) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO practice(question_id,payload) VALUES(?,?) ON CONFLICT(question_id) DO UPDATE SET payload=CASE
 WHEN json_type(practice.payload,'$.Note') IS NULL THEN excluded.payload
 ELSE json_set(excluded.payload,'$.Note',json_extract(practice.payload,'$.Note')) END`, id, string(payload))
	return err
}

// SaveMistakeNote 只更新备注字段，避免等待保存期间的新作答被旧作答覆盖。
func (s *Store) SaveMistakeNote(id, note string) error {
	result, err := s.db.Exec("UPDATE practice SET payload=json_set(payload,'$.Note',?) WHERE question_id=?", note, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("答题记录尚未保存，请重新提交后保存备注")
	}
	return nil
}

// LoadRepeatPractice 恢复独立的重新作答开关，旧版本缺省为 false。
func (s *Store) LoadRepeatPractice() (bool, error) {
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

// SaveRepeatPractice 保存重新作答开关，不改变最近作答记录。
func (s *Store) SaveRepeatPractice(value bool) error {
	_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES('repeat-practice',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", strconv.FormatBool(value))
	return err
}

// ClearPractice 删除全部练习，bank 非空时仅删除该题库；不修改重要标记或历史。
func (s *Store) ClearPractice(bank string) error {
	if bank == "" {
		_, err := s.db.Exec("DELETE FROM practice")
		return err
	}
	_, err := s.db.Exec("DELETE FROM practice WHERE question_id IN (SELECT q.id FROM questions q JOIN banks b ON b.id=q.bank_id WHERE b.name=?)", bank)
	return err
}

// DeleteResponses 在一个事务中删除指定记录，任一失败都不删除其余记录。
func (s *Store) DeleteResponses(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err = tx.Exec("DELETE FROM practice WHERE question_id=?", id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
