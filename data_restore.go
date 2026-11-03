// 数据恢复：先校验不可变备份，再保存恢复前快照，最后在一个事务中替换业务表。
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// readRestorableState 用只读仓库验证已知表和可解码记录，不初始化或修补待恢复文件。
// 返回的 app 只携带模型值；没有窗口、队列或构建 Context，不会从后台触发 UI 操作。
func readRestorableState(db *sql.DB, filename string) (*app, error) {
	var status string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&status); err != nil {
		return nil, fmt.Errorf("备份无法读取：%w", err)
	}
	if status != "ok" {
		return nil, fmt.Errorf("备份完整性检查失败：%s", status)
	}
	for _, table := range []string{"banks", "questions", "practice", "settings"} {
		var kind string
		if err := db.QueryRow("SELECT type FROM sqlite_master WHERE name=?", table).Scan(&kind); err != nil || kind != "table" {
			return nil, fmt.Errorf("不是受支持的 Quesora 原生备份：缺少数据表 %s", table)
		}
	}
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	bad := rows.Next()
	scanErr := rows.Err()
	rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if bad {
		return nil, fmt.Errorf("备份存在无效题库或答题关联")
	}
	store := &questionStore{db: db, path: filename}
	a := newApp()
	a.store = store
	a.questions, err = store.load()
	if err != nil {
		return nil, err
	}
	a.answers, a.starred, err = store.loadPractice()
	if err != nil {
		return nil, err
	}
	a.repeatPractice, err = store.loadRepeatPractice()
	if err != nil {
		return nil, err
	}
	records, err := store.loadHistory()
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		if r.Error != "" {
			return nil, fmt.Errorf("备份中存在损坏的历史记录：%s", r.Key)
		}
	}
	for _, q := range a.questions {
		if canonicalKind(q.Kind) != q.Kind || q.Bank == "" || strings.TrimSpace(q.Prompt) == "" {
			return nil, fmt.Errorf("备份题目 %s 的类型或内容无效", q.ID)
		}
	}
	a.shuffle = true
	a.leftOpen = false
	a.rightOpen = false
	a.bank = ""
	a.syncStars()
	if err = a.restorePreferences(); err != nil {
		return nil, fmt.Errorf("备份学习设置无法读取：%w", err)
	}
	return a, nil
}

// restoreFrom 不替换活动 .db 文件，因此已有连接和 WAL 不会指向失效文件。
// 先生成同目录备份再 ATTACH 私有快照；任何删除/插入失败都回滚全部四张业务表。
func (s *questionStore) restoreFrom(filename string) (safety string, err error) {
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
	src, err := openReadOnlyDatabase(source)
	if err != nil {
		return "", err
	}
	err = snapshotDatabase(src, snapshot, false)
	closeErr := src.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	checked, err := openReadOnlyDatabase(snapshot)
	if err != nil {
		return "", err
	}
	_, err = readRestorableState(checked, snapshot)
	closeErr = checked.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	safety = filepath.Join(filepath.Dir(current), "backups", "before-restore-"+time.Now().UTC().Format("20060102T150405.000000000")+".db")
	if err = s.backupTo(safety); err != nil {
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
