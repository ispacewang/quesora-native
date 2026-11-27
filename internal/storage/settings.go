// 设置仓储只处理键值和持久顺序，业务格式由调用方解释。
package storage

// Setting 是保存的原始快照；历史条目保留原 JSON，单条损坏由界面明确展示。
type Setting struct{ Key, Value string }

// Setting 读取一个设置；不存在时保留 sql.ErrNoRows 供调用方采用默认值。
func (s *Store) Setting(key string) (string, error) {
	var raw string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&raw)
	return raw, err
}

// SetSetting 原子保存设置；同名键覆盖原值。
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

// InsertSetting 新建独立快照；同名键报错，防止覆盖已保存的历史。
func (s *Store) InsertSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES(?,?)", key, value)
	return err
}

// DeleteSetting 删除指定键；不存在时无副作用。
func (s *Store) DeleteSetting(key string) error {
	_, err := s.db.Exec("DELETE FROM settings WHERE key=?", key)
	return err
}

// History 按既有键的倒序返回考试和错题本，关闭游标后再由调用方解码。
func (s *Store) History() ([]Setting, error) {
	rows, err := s.db.Query("SELECT key,value FROM settings WHERE key LIKE 'exam:%' OR key LIKE 'mistake-book:%' ORDER BY key DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []Setting{}
	for rows.Next() {
		var entry Setting
		if err = rows.Scan(&entry.Key, &entry.Value); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
