// SQLite 一致性快照与发布。读取已提交 WAL，校验完整性后才替换备份，禁止覆盖正在使用的库。

package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// openReadOnlyDatabase 使用转义后的 SQLite file URI 打开只读源，支持含空格、中文和引号的路径。
func openReadOnlyDatabase(filename string) (*sql.DB, error) {
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	if filepath.VolumeName(absolute) != "" {
		u.Path = "/" + u.Path
	}
	query := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(5000)"}}
	u.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err == nil {
		db.SetMaxOpenConns(1)
	}
	return db, err
}

// VACUUM INTO includes committed WAL data; copying only the .db file does not.
// Build the snapshot beside its destination before publishing a complete file.
func snapshotDatabase(db *sql.DB, filename string, replace bool) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".quesora-backup-*.db")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err = temporary.Close(); err != nil {
		return err
	}
	if _, err = db.Exec("VACUUM INTO ?", name); err != nil {
		return err
	}
	check, err := openReadOnlyDatabase(name)
	if err != nil {
		return err
	}
	var status string
	err = check.QueryRow("PRAGMA quick_check").Scan(&status)
	closeErr := check.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if status != "ok" {
		return fmt.Errorf("备份完整性检查失败：%s", status)
	}
	if replace {
		return os.Rename(name, filename)
	}
	// Link publishes without replacing a destination another instance created.
	if err = os.Link(name, filename); err != nil {
		if _, exists := os.Stat(filename); exists == nil {
			return nil
		}
		return err
	}
	return nil
}

// backupTo 保护活动数据库及其日志路径/硬链接别名；成功前不会替换已有目标备份。
func (s *questionStore) backupTo(filename string) error {
	source, err := filepath.Abs(s.path)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	for _, protected := range []string{source, source + "-wal", source + "-shm", source + "-journal"} {
		if strings.EqualFold(filepath.Clean(target), filepath.Clean(protected)) {
			return fmt.Errorf("请选择其他位置，不能覆盖正在使用的数据文件")
		}
	}
	if sourceInfo, err := os.Stat(source); err == nil {
		if targetInfo, err := os.Stat(target); err == nil && os.SameFile(sourceInfo, targetInfo) {
			return fmt.Errorf("不能覆盖正在使用的数据文件")
		}
	}
	return snapshotDatabase(s.db, target, true)
}
