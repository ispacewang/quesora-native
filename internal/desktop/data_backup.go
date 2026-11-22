// SQLite 一致性快照与发布。读取已提交 WAL，校验完整性后才替换备份，禁止覆盖正在使用的库。

package desktop

import (
	"fmt"

	"github.com/ispacewang/quesora-native/internal/storage"
	"os"
	"path/filepath"
	"strings"
	// backupTo 保护活动数据库及其日志路径/硬链接别名；成功前不会替换已有目标备份。
)

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
	return storage.Snapshot(s.db, target, true)
}
