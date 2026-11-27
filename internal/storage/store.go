// SQLite 题库仓储。表结构集中在打开流程，批量导入使用单个事务，删除依赖外键级联。

package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/ispacewang/quesora-native/internal/domain"
	"github.com/ispacewang/quesora-native/internal/importer"
)

// Store 封装单库连接和实际路径；关闭前须先等待串行任务队列排空。
type Store struct {
	db   *sql.DB
	path string
}

// Open 创建/打开原生数据库并初始化兼容表结构。
// 仅使用一个连接，使 foreign_keys 等连接级设置对所有后续操作一致生效。
func Open(filename string) (*Store, error) {
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
	return &Store{db, filename}, nil
}

// Close 应在调用方排空串行任务后调用；之后不再读写该仓储。
func (s *Store) Close() error { return s.db.Close() }

// ImportBanks 将所有有效工作表作为一个原子批次保存；同名题库或任一写入失败会整批回滚。
func (s *Store) ImportBanks(batch importer.Result) error {
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
				answer = strings.ReplaceAll(domain.Letters(q.Answer), "、", "")
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

// LoadQuestions 按题库/题目主键恢复稳定顺序；损坏 JSON 明确报错，避免用空值掩盖数据问题。
func (s *Store) LoadQuestions() ([]domain.Question, error) {
	rows, err := s.db.Query("SELECT q.id,b.name,q.type,q.question,q.options,q.answer,q.explanation,q.meta FROM questions q JOIN banks b ON b.id=q.bank_id ORDER BY b.id,q.id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var questions []domain.Question
	for rows.Next() {
		var q domain.Question
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
		q.Answer = domain.AnswerMask(q.TextAnswer, q.Kind, q.Options)
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// DeleteBank 只删除库内题目及外键关联练习记录，不触碰导入的原始文件。
func (s *Store) DeleteBank(name string) error {
	_, err := s.db.Exec("DELETE FROM banks WHERE name=?", name)
	return err
}

// Path 返回打开时使用的数据库路径。
func (s *Store) Path() string { return s.path }
