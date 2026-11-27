// 历史记录解码属于桌面业务层；存储层只返回原始键值快照。
package desktop

import (
	"encoding/json"
	"strings"

	"github.com/ispacewang/quesora-native/internal/storage"
)

func (s *questionStore) loadHistory() ([]historyRecord, error) {
	entries, err := s.History()
	if err != nil {
		return nil, err
	}
	return decodeHistory(entries), nil
}

// decodeHistory 保留单条损坏记录，避免它阻断其他已保存成绩的查看。
func decodeHistory(entries []storage.Setting) []historyRecord {
	out := []historyRecord{}
	for _, entry := range entries {
		key, value := entry.Key, entry.Value
		r := historyRecord{Key: key}
		if strings.HasPrefix(key, "exam:") {
			e := &examSession{}
			if err := json.Unmarshal([]byte(value), e); err == nil && len(e.Questions) > 0 && len(e.Responses) == len(e.Questions) && len(e.Important) == len(e.Questions) && len(e.Marked) == len(e.Questions) {
				e.Saved = true
				r.Exam = e
			} else {
				r.Error = "成绩记录无法读取"
			}
		} else {
			if err := json.Unmarshal([]byte(value), &r.Mistakes); err != nil {
				r.Error = "错题记录无法读取"
			}
		}
		out = append(out, r)
	}
	return out
}
