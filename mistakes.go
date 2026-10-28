package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo"
	"os"
	"strings"
	"time"
)

func (a *app) wrongText(markdown bool) string {
	var out strings.Builder
	if markdown {
		out.WriteString("# 错题本\n\n")
	} else {
		out.WriteString("--- 错题本 ---\n\n")
	}
	index := 0
	for _, q := range a.questions {
		r, ok := a.answers[q.ID]
		if !ok || r.Correct || q.Bank != a.bank {
			continue
		}
		index++
		fmt.Fprintf(&out, "%d. %s\n\n", index, q.Prompt)
		for display, i := range responseOrder(q, r) {
			opt := q.Options[i]
			fmt.Fprintf(&out, "%c. %s\n", rune('A'+display), opt)
		}
		given, expected := responseLetters(q, r, r.Choice), responseLetters(q, r, q.Answer)
		if isTextQuestion(q) {
			given, expected = r.Text, q.TextAnswer
		}
		fmt.Fprintf(&out, "\n你的答案：%s\n正确答案：%s\n解析：%s\n\n", given, expected, q.Explanation)
	}
	return out.String()
}
func (a *app) exportWrong(format string) {
	if a.win == nil {
		return
	}
	a.exportOpen = false
	text, win := a.wrongText(format == "md"), a.win
	go func() {
		filename, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win, Title: "导出错题", DefaultPath: "错题本." + format, Filters: []mygo.FileFilter{{Name: "错题本", Extensions: []string{format}}}})
		if err == nil && filename != "" {
			err = os.WriteFile(filename, []byte(text), 0600)
		}
		a.publish(func() {
			if err != nil {
				a.libraryStatus = "导出失败：" + err.Error()
			} else if filename != "" {
				a.libraryStatus = "错题本已导出"
			}
		})
	}()
}
func (a *app) wrongBookData() ([]byte, int, error) {
	items := []savedMistake{}
	for _, q := range a.questions {
		if r, ok := a.answers[q.ID]; q.Bank == a.bank && ok && !r.Correct {
			items = append(items, savedMistake{q, r})
		}
	}
	data, err := json.Marshal(items)
	return data, len(items), err
}
func (a *app) saveWrongBook() {
	if a.store == nil || a.wrongSaving || a.savedWrong {
		return
	}
	data, count, err := a.wrongBookData()
	if err != nil || count == 0 {
		return
	}
	s, bank := a.store, a.bank
	key := "mistake-book:" + time.Now().UTC().Format("20060102T150405.000000000")
	a.wrongSaving = true
	if !a.enqueue(func() {
		_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES(?,?)", key, string(data))
		a.publish(func() {
			a.wrongSaving = false
			if err != nil {
				a.libraryStatus = "错题本保存失败：" + err.Error()
				return
			}
			current, _, currentErr := a.wrongBookData()
			a.savedWrong = a.bank == bank && currentErr == nil && bytes.Equal(current, data)
			a.libraryStatus = bank + " · 错题本已保存"
		})
	}) {
		a.wrongSaving = false
	}
}
func (a *app) clearWrong() {
	var ids []string
	for _, q := range a.questions {
		if r, ok := a.answers[q.ID]; q.Bank == a.bank && ok && !r.Correct {
			delete(a.answers, q.ID)
			ids = append(ids, q.ID)
		}
	}
	a.savedWrong = false
	a.navigate(a.index)
	if a.store != nil {
		s := a.store
		a.enqueue(func() {
			tx, err := s.db.Begin()
			if err == nil {
				defer tx.Rollback()
				for _, id := range ids {
					if _, err = tx.Exec("DELETE FROM practice WHERE question_id=?", id); err != nil {
						break
					}
				}
				if err == nil {
					err = tx.Commit()
				}
			}
			if err != nil {
				a.publish(func() { a.libraryStatus = "清空错题失败：" + err.Error() })
			}
		})
	}
}
