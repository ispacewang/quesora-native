package main

import (
	"fmt"
	"github.com/egoist/mygo"
	"path/filepath"
	"sync"
)

// Serial jobs preserve the order of imports, answers, resets and deletes.
type libraryJobs struct {
	queue chan func()
	done  sync.WaitGroup
}

func newLibraryJobs() *libraryJobs {
	j := &libraryJobs{queue: make(chan func(), 128)}
	j.done.Add(1)
	go func() {
		defer j.done.Done()
		for job := range j.queue {
			job()
		}
	}()
	return j
}
func (j *libraryJobs) close() { close(j.queue); j.done.Wait() }
func (a *app) enqueue(work func()) bool {
	if a.jobs == nil || a.closed {
		return false
	}
	select {
	case a.jobs.queue <- work:
		return true
	default:
		a.libraryStatus = "操作较多，请稍后重试"
		return false
	}
}
func (a *app) publish(fn func()) {
	a.dispatch(func() {
		if !a.closed {
			fn()
		}
	})
}
func (a *app) openImport() {
	a.importOpen = true
	if !a.importBusy {
		a.importMessage = ""
		a.importError = false
		a.importWarnings = nil
	}
}
func (a *app) pickQuestionFile() {
	if a.importBusy || a.pickFile == nil {
		return
	}
	a.importBusy = true
	a.importMessage = "请选择题库文件…"
	a.importError = false
	pick := a.pickFile
	go func() {
		paths, err := pick()
		a.publish(func() {
			a.importBusy = false
			if err != nil {
				a.importError = true
				a.importMessage = "无法选择文件：" + err.Error()
				return
			}
			a.importMessage = ""
			if len(paths) > 0 {
				a.startImport(paths)
			}
		})
	}()
}
func (a *app) startImport(paths []string) {
	if a.importBusy {
		return
	}
	a.importOpen = true
	a.importWarnings = nil
	a.importError = false
	if len(paths) != 1 {
		a.importError = true
		a.importMessage = "请每次导入一个文件；同一 Excel 的多个工作表会自动拆分为题库。"
		return
	}
	if a.store == nil {
		a.importError = true
		a.importMessage = "题库数据库不可用，请检查数据目录后重新打开程序。"
		return
	}
	filename := paths[0]
	a.importBusy = true
	a.importMessage = "正在解析 " + filepath.Base(filename) + "…"
	store := a.store
	if !a.enqueue(func() {
		result, err := parseQuestionFile(filename)
		var qs []question
		if err == nil {
			a.publish(func() { a.importMessage = "正在保存题库…" })
			err = store.importBanks(result)
		}
		if err == nil {
			qs, err = store.load()
		}
		a.publish(func() { a.finishImport(result, qs, err) })
	}) {
		a.importBusy = false
		a.importError = true
		a.importMessage = "后台任务队列繁忙，请稍后重试。"
	}
}
func (a *app) finishImport(result importResult, qs []question, err error) {
	a.importBusy = false
	a.importWarnings = result.Warnings
	if err != nil {
		a.importError = true
		a.importMessage = "导入失败：" + err.Error()
		return
	}
	a.questions = qs
	a.savedWrong = false
	a.exportOpen = false
	a.syncStars()
	a.bank = result.Banks[0].Name
	a.filter = "全部"
	a.filters = nil
	a.importantOnly = false
	a.onlyWrong = false
	a.reorder()
	a.navigate(0)
	count := 0
	for _, b := range result.Banks {
		count += len(b.Questions)
	}
	a.importError = false
	a.importMessage = fmt.Sprintf("导入成功：%d 个题库，共 %d 道题。", len(result.Banks), count)
	a.libraryStatus = a.importMessage
}
func (a *app) saveTemplate() {
	if a.importBusy || a.win == nil {
		return
	}
	a.importBusy = true
	a.importMessage = "请选择模板保存位置…"
	a.importError = false
	win := a.win
	go func() {
		filename, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win, Title: "保存题库模板", DefaultPath: "题库模板.xlsx", Filters: []mygo.FileFilter{{Name: "Excel 工作簿", Extensions: []string{"xlsx"}}}})
		if err == nil && filename != "" {
			if filepath.Ext(filename) == "" {
				filename += ".xlsx"
			}
			err = writeQuestionTemplate(filename)
		}
		a.publish(func() {
			a.importBusy = false
			a.importError = err != nil
			switch {
			case err != nil:
				a.importMessage = "模板保存失败：" + err.Error()
			case filename != "":
				a.importMessage = "模板已保存，可填写后上传。"
			default:
				a.importMessage = ""
			}
		})
	}()
}
func (a *app) deleteSelectedBank() {
	if a.importBusy || a.store == nil || a.deleteName == "" {
		return
	}
	name := a.deleteName
	store := a.store
	a.importBusy = true
	if !a.enqueue(func() {
		err := store.deleteBank(name)
		var qs []question
		if err == nil {
			qs, err = store.load()
		}
		a.publish(func() {
			a.importBusy = false
			if err != nil {
				a.libraryStatus = "删除失败：" + err.Error()
				return
			}
			a.deleteOpen = false
			a.questions = qs
			a.savedWrong = false
			a.exportOpen = false
			a.syncStars()
			a.bank = ""
			if names := a.bankNames(); len(names) > 0 {
				a.bank = names[0]
			}
			a.filter = "全部"
			a.filters = nil
			a.importantOnly = false
			a.onlyWrong = false
			a.reorder()
			a.navigate(0)
			a.libraryStatus = "已删除题库：" + name
		})
	}) {
		a.importBusy = false
	}
}
func (a *app) syncStars() {
	for _, q := range a.questions {
		if _, exists := a.starred[q.ID]; !exists {
			a.starred[q.ID], _ = q.Meta["isBaoMing"].(bool)
		}
	}
}
