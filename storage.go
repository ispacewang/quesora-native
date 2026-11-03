// SQLite 题库仓储。表结构集中在打开流程，批量导入使用单个事务，删除依赖外键级联。

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// questionStore 封装单库连接和实际路径；关闭前须先等待串行任务队列排空。
type questionStore struct {
	db   *sql.DB
	path string
}

// openQuestionStore 创建/打开原生数据库并初始化兼容表结构。
// 仅使用一个连接，使 foreign_keys 等连接级设置对所有后续操作一致生效。
func openQuestionStore(filename string) (*questionStore, error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	schema := `PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS banks(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL UNIQUE,created_at TEXT DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE IF NOT EXISTS questions(id INTEGER PRIMARY KEY AUTOINCREMENT,bank_id INTEGER NOT NULL REFERENCES banks(id) ON DELETE CASCADE,question TEXT NOT NULL,options TEXT,answer TEXT,explanation TEXT,type TEXT,meta TEXT);
 CREATE TABLE IF NOT EXISTS practice(question_id INTEGER PRIMARY KEY REFERENCES questions(id) ON DELETE CASCADE,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT);`
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &questionStore{db, filename}, nil
}
func (s *questionStore) close() error { return s.db.Close() }

// importBanks 将所有有效工作表作为一个原子批次保存；同名题库或任一写入失败会整批回滚。
func (s *questionStore) importBanks(batch importResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, bank := range batch.Banks {
		var count int
		if err = tx.QueryRow("SELECT count(*) FROM banks WHERE name=?", bank.Name).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("题库“%s”已存在，请修改文件名或先删除已有题库", bank.Name)
		}
		res, e := tx.Exec("INSERT INTO banks(name) VALUES(?)", bank.Name)
		if e != nil {
			return e
		}
		id, e := res.LastInsertId()
		if e != nil {
			return e
		}
		stmt, e := tx.Prepare("INSERT INTO questions(bank_id,question,options,answer,explanation,type,meta) VALUES(?,?,?,?,?,?,?)")
		if e != nil {
			return e
		}
		for _, q := range bank.Questions {
			opts, e := json.Marshal(q.Options)
			if e != nil {
				stmt.Close()
				return e
			}
			meta, e := json.Marshal(q.Meta)
			if e != nil {
				stmt.Close()
				return e
			}
			answer := q.TextAnswer
			if answer == "" && q.Answer != 0 {
				answer = strings.ReplaceAll(letters(q.Answer), "、", "")
			}
			if _, e = stmt.Exec(id, q.Prompt, string(opts), answer, q.Explanation, q.Kind, string(meta)); e != nil {
				stmt.Close()
				return e
			}
		}
		if e = stmt.Close(); e != nil {
			return e
		}
	}
	return tx.Commit()
}

// load 按题库/题目主键恢复稳定顺序；损坏 JSON 明确报错，避免用空值掩盖数据问题。
func (s *questionStore) load() ([]question, error) {
	rows, err := s.db.Query("SELECT q.id,b.name,q.type,q.question,q.options,q.answer,q.explanation,q.meta FROM questions q JOIN banks b ON b.id=q.bank_id ORDER BY b.id,q.id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var questions []question
	for rows.Next() {
		var q question
		var id int64
		var options, meta, kind, prompt, answer, explanation sql.NullString
		if err = rows.Scan(&id, &q.Bank, &kind, &prompt, &options, &answer, &explanation, &meta); err != nil {
			return nil, err
		}
		q.ID = strconv.FormatInt(id, 10)
		q.Kind = kind.String
		q.Prompt = prompt.String
		q.TextAnswer = answer.String
		q.Explanation = explanation.String
		if options.Valid && options.String != "" && options.String != "null" {
			if err = json.Unmarshal([]byte(options.String), &q.Options); err != nil {
				return nil, fmt.Errorf("题目 %d 的选项数据损坏", id)
			}
		}
		if meta.Valid && meta.String != "" && meta.String != "null" {
			if err = json.Unmarshal([]byte(meta.String), &q.Meta); err != nil {
				return nil, fmt.Errorf("题目 %d 的附加数据损坏", id)
			}
		}
		q.Answer = answerMask(q.TextAnswer, q.Kind, q.Options)
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// deleteBank 只删除库内题目及外键关联练习记录，不触碰导入的原始文件。
func (s *questionStore) deleteBank(name string) error {
	_, err := s.db.Exec("DELETE FROM banks WHERE name=?", name)
	return err
}
func (a *app) bankNames() []string {
	names := []string{}
	seen := map[string]bool{}
	for _, q := range a.questions {
		if !seen[q.Bank] {
			seen[q.Bank] = true
			names = append(names, q.Bank)
		}
	}
	return names
}
