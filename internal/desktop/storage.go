// desktop 的仓储适配器仅补充历史解码和恢复校验；SQL 与连接生命周期由 storage 拥有。
package desktop

import "github.com/ispacewang/quesora-native/internal/storage"

// questionStore 组合独立仓储与桌面业务解释；previewPath 只供不打开数据库的离屏预览展示。
type questionStore struct {
	*storage.Store
	previewPath string
}

func openQuestionStore(filename string) (*questionStore, error) {
	s, err := storage.Open(filename)
	if err != nil {
		return nil, err
	}
	return &questionStore{Store: s}, nil
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

// path 在真实运行时使用仓储拥有的路径，不在界面层维护第二份数据库位置。
func (s *questionStore) path() string {
	if s.Store != nil {
		return s.Store.Path()
	}
	return s.previewPath
}
