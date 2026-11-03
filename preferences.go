// 版本化学习偏好的快照/重载。先在 UI 线程序列化，再将不可变 JSON 交给串行任务队列。

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	"sort"
)

// learningPreferences 是 SQLite 中的版本化设置快照；QuestionID 在题序重建后用于恢复位置。
type learningPreferences struct {
	Version                                           int
	Dark, Random, Quick, Shuffle, LeftOpen, RightOpen bool
	Bank, QuestionID, Filter                          string
	Filters                                           []string
	ImportantOnly, OnlyWrong                          bool
}

// preferences 在 UI 线程捕获可序列化值，并排序筛选集合，使去重比较不受 map 迭代顺序影响。
func (a *app) preferences() learningPreferences {
	p := learningPreferences{Version: 1, Dark: a.dark, Random: a.random, Quick: a.quick, Shuffle: a.shuffle, LeftOpen: a.leftOpen, RightOpen: a.rightOpen, Bank: a.bank, Filter: a.filter, ImportantOnly: a.importantOnly, OnlyWrong: a.onlyWrong}
	if q, ok := a.current(); ok {
		p.QuestionID = q.ID
	}
	for kind, enabled := range a.filters {
		if enabled {
			p.Filters = append(p.Filters, kind)
		}
	}
	sort.Strings(p.Filters)
	return p
}

// persistPreferences 仅将捕获的不可变 JSON 入队；相同快照去重，写入失败允许后续重试。
func (a *app) persistPreferences() {
	if a.store == nil || a.jobs == nil || a.closed {
		return
	}
	data, err := json.Marshal(a.preferences())
	if err != nil {
		return
	}
	raw := string(data)
	if raw == a.lastPreferences {
		return
	}
	store := a.store
	if a.enqueue(func() {
		_, err := store.db.Exec("INSERT INTO settings(key,value) VALUES('learning-preferences',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", raw)
		if err != nil {
			a.publish(func() { a.lastPreferences = ""; a.libraryStatus = "学习设置保存失败：" + err.Error() })
		}
	}) {
		a.lastPreferences = raw
	}
}

// restorePreferences 在启动加载题目后恢复显示状态；失效题库回退，随机排列可重建但题目按 ID 恢复。
func (a *app) restorePreferences() error {
	var raw string
	err := a.store.db.QueryRow("SELECT value FROM settings WHERE key='learning-preferences'").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		if names := a.bankNames(); len(names) > 0 {
			a.bank = names[0]
		}
		a.navigate(0)
		return nil
	}
	if err != nil {
		return err
	}
	var p learningPreferences
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return err
	}
	if p.Version != 1 {
		return fmt.Errorf("不支持的学习设置版本")
	}
	a.dark, a.random, a.quick, a.shuffle = p.Dark, p.Random, p.Quick, p.Shuffle
	a.leftOpen, a.rightOpen = p.LeftOpen, p.RightOpen
	a.bank, a.filter = p.Bank, p.Filter
	if a.filter == "" {
		a.filter = "全部"
	}
	a.filters = nil
	for _, kind := range p.Filters {
		if a.filters == nil {
			a.filters = map[string]bool{}
		}
		a.filters[kind] = true
	}
	a.importantOnly, a.onlyWrong = p.ImportantOnly, p.OnlyWrong
	validBank := a.bank == ""
	names := a.bankNames()
	for _, name := range names {
		if name == a.bank {
			validBank = true
		}
	}
	if !validBank {
		a.bank = ""
		if len(names) > 0 {
			a.bank = names[0]
		}
		a.filter, a.filters, a.importantOnly, a.onlyWrong = "全部", nil, false, false
	}
	a.reorder()
	index := 0
	for i, q := range a.visible() {
		if q.ID == p.QuestionID {
			index = i
			break
		}
	}
	a.navigate(index)
	return nil
}

func (a *app) applyTheme() {
	if a.win != nil {
		if a.dark {
			mygo.Theme.SetSource(mygo.ThemeDark)
		} else {
			mygo.Theme.SetSource(mygo.ThemeLight)
		}
	}
	a.persistPreferences()
}
func (a *app) toggleTheme()   { a.dark = !a.dark; a.applyTheme() }
func (a *app) toggleRandom()  { a.random = !a.random; a.reorder(); a.navigate(0) }
func (a *app) toggleQuick()   { a.quick = !a.quick; a.persistPreferences() }
func (a *app) toggleShuffle() { a.shuffle = !a.shuffle; a.prepareOptions(); a.persistPreferences() }
