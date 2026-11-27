// 备份的桌面业务校验只处理值；后台不创建 app 或初始化原生资源。
package desktop

import (
	"fmt"

	"github.com/ispacewang/quesora-native/internal/storage"
)

// restorableState 是经过版本和历史记录校验的持久数据；界面在 UI 线程应用它。
type restorableState struct {
	storage.State
	Preferences *learningPreferences
}

// decodeRestorableState 不访问数据库和 UI，供恢复前预检与恢复后的状态发布复用。
func decodeRestorableState(state storage.State) (*restorableState, error) {
	for _, record := range decodeHistory(state.History) {
		if record.Error != "" {
			return nil, fmt.Errorf("备份中存在损坏的历史记录：%s", record.Key)
		}
	}
	if state.Starred == nil {
		state.Starred = map[string]bool{}
	}
	next := &restorableState{State: state}
	if state.Learning != nil {
		p, err := decodeLearningPreferences(*state.Learning)
		if err != nil {
			return nil, fmt.Errorf("备份学习设置无法读取：%w", err)
		}
		next.Preferences = &p
	}
	return next, nil
}

func (s *questionStore) readRestorableState() (*restorableState, error) {
	state, err := s.ReadState()
	if err != nil {
		return nil, err
	}
	return decodeRestorableState(state)
}

// restoreFrom 在存储层事务开始前验证桌面业务格式，不让校验依赖窗口或任务队列。
func (s *questionStore) restoreFrom(filename string) (string, error) {
	return s.Store.RestoreFrom(filename, func(state storage.State) error {
		_, err := decodeRestorableState(state)
		return err
	})
}
