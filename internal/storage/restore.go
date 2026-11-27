// 恢复从私有只读快照校验，再保留安全备份并事务替换业务表。
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ispacewang/quesora-native/internal/domain"
)

// State 仅含持久数据，不携带数据库连接、窗口、任务队列或控件。
// Learning 为 nil 表示该版本尚未保存学习设置；空字符串仍是待校验的原始值。
type State struct {
	Questions      []domain.Question
	Answers        map[string]domain.Response
	Starred        map[string]bool
	RepeatPractice bool
	Learning       *string
	History        []Setting
}

// ReadState 校验原生表结构、完整性及关联，并读取可独立解释的快照。
// 调用方必须串行使用仓储，避免同一应用在多次查询之间写入。
func (s *Store) ReadState() (State, error) {
	var state State
	var status string
	if err := s.db.QueryRow("PRAGMA quick_check").Scan(&status); err != nil {
		return state, fmt.Errorf("备份无法读取：%w", err)
	}
	if status != "ok" {
		return state, fmt.Errorf("备份完整性检查失败：%s", status)
	}
	for _, table := range []string{"banks", "questions", "practice", "settings"} {
		var kind string
		if err := s.db.QueryRow("SELECT type FROM sqlite_master WHERE name=?", table).Scan(&kind); err != nil || kind != "table" {
			return state, fmt.Errorf("不是受支持的 Quesora 原生备份：缺少数据表 %s", table)
		}
	}
	rows, err := s.db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return state, err
	}
	bad := rows.Next()
	scanErr := rows.Err()
	rows.Close()
	if scanErr != nil {
		return state, scanErr
	}
	if bad {
		return state, fmt.Errorf("备份存在无效题库或答题关联")
	}
	state.Questions, err = s.LoadQuestions()
	if err != nil {
		return state, err
	}
	state.Answers, state.Starred, err = s.LoadPractice()
	if err != nil {
		return state, err
	}
	state.RepeatPractice, err = s.LoadRepeatPractice()
	if err != nil {
		return state, err
	}
	state.History, err = s.History()
	if err != nil {
		return state, err
	}
	for _, q := range state.Questions {
		if domain.CanonicalKind(q.Kind) != q.Kind || q.Bank == "" || strings.TrimSpace(q.Prompt) == "" {
			return state, fmt.Errorf("备份题目 %s 的类型或内容无效", q.ID)
		}
	}
	raw, err := s.Setting("learning-preferences")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return state, err
	}
	if err == nil {
		state.Learning = &raw
	}
	return state, nil
}

// RestoreFrom 要求 validate 检查 State 中业务快照的版本与内容。
// 校验回调只接收数据，必须无外部写入；通过前不会创建安全备份或修改活动库。
// RestoreFrom 不替换活动 .db 文件，因此已有连接和 WAL 不会指向失效文件。
// 先生成同目录备份再 ATTACH 私有快照；任何删除/插入失败都回滚全部四张业务表。
func (s *Store) RestoreFrom(filename string, validate func(State) error) (safety string, err error) {
	if validate == nil {
		return "", fmt.Errorf("恢复必须提供业务快照校验")
	}
	source, err := filepath.Abs(filename)
	if err != nil {
		return "", err
	}
	current, err := filepath.Abs(s.path)
	if err != nil {
		return "", err
	}
	for _, protected := range []string{current, current + "-wal", current + "-shm", current + "-journal"} {
		if strings.EqualFold(filepath.Clean(source), filepath.Clean(protected)) {
			return "", fmt.Errorf("不能将当前正在使用的数据文件作为恢复源")
		}
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<30 {
		return "", fmt.Errorf("请选择不超过 1 GiB 的原生数据库备份")
	}
	if target, e := os.Stat(current); e == nil && os.SameFile(info, target) {
		return "", fmt.Errorf("不能从当前数据库的别名恢复")
	}
	// 放在活动库旁，确保快照发布和安全备份均位于当前用户可写位置。
	temp, err := os.MkdirTemp(filepath.Dir(current), ".quesora-restore-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	snapshot := filepath.Join(temp, "source.db")
	src, err := OpenReadOnly(source)
	if err != nil {
		return "", err
	}
	err = Snapshot(src, snapshot, false)
	closeErr := src.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	checked, err := OpenReadOnly(snapshot)
	if err != nil {
		return "", err
	}
	state, readErr := (&Store{db: checked, path: snapshot}).ReadState()
	err = readErr
	if err == nil {
		err = validate(state)
	}
	closeErr = checked.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	safety = filepath.Join(filepath.Dir(current), "backups", "before-restore-"+time.Now().UTC().Format("20060102T150405.000000000")+".db")
	if err = s.BackupTo(safety); err != nil {
		return "", fmt.Errorf("恢复前安全备份失败，未修改当前数据：%w", err)
	}
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return safety, err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "ATTACH DATABASE ? AS restore_source", snapshot); err != nil {
		return safety, err
	}
	defer conn.ExecContext(ctx, "DETACH DATABASE restore_source")
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return safety, err
	}
	defer tx.Rollback()
	statements := []string{
		"DELETE FROM practice", "DELETE FROM questions", "DELETE FROM banks", "DELETE FROM settings",
		"INSERT INTO banks(id,name,created_at) SELECT id,name,created_at FROM restore_source.banks",
		"INSERT INTO questions(id,bank_id,question,options,answer,explanation,type,meta) SELECT id,bank_id,question,options,answer,explanation,type,meta FROM restore_source.questions",
		"INSERT INTO practice(question_id,payload) SELECT question_id,payload FROM restore_source.practice",
		"INSERT INTO settings(key,value) SELECT key,value FROM restore_source.settings",
	}
	for _, query := range statements {
		if _, err = tx.ExecContext(ctx, query); err != nil {
			return safety, fmt.Errorf("恢复失败，已回滚当前数据：%w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return safety, err
	}
	return safety, nil
}
