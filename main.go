package main

import (
	"flag"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

func defaultDataDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "data"
	}
	dir := filepath.Dir(exe)
	// Source builds and packaged builds share data when run from this project.
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "mygo.json")); err == nil {
		dir = filepath.Dir(dir)
	}
	if cwd, err := os.Getwd(); err == nil {
		if _, err = os.Stat(filepath.Join(cwd, "mygo.json")); err == nil {
			dir = cwd
		}
	}
	return filepath.Join(dir, "data")
}
func main() {
	snapshot := flag.String("snapshot", "", "render a preview PNG")
	dark := flag.Bool("dark", false, "dark appearance")
	showUpload := flag.Bool("show-upload", false, "open upload dialog")
	dataDir := flag.String("data-dir", "", "directory for native question data")
	scene := flag.String("scene", "single", "preview scenario")
	flag.Parse()
	a := newApp()
	a.dark = *dark
	a.importOpen = *showUpload
	if *snapshot != "" {
		a.configureScene(*scene)
		f, err := os.Create(*snapshot)
		if err != nil {
			log.Fatal(err)
		}
		if err = png.Encode(f, ui.Render(a.view, 1450, 800, 1)); err != nil {
			log.Fatal(err)
		}
		if err = f.Close(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *dataDir == "" {
		*dataDir = defaultDataDir()
	}
	a.shuffle = true
	a.leftOpen = false
	a.rightOpen = false
	a.questions = nil
	a.bank = ""
	var initErr error
	a.store, initErr = openQuestionStore(filepath.Join(*dataDir, "questions.db"))
	if initErr == nil {
		defer a.store.close()
		a.questions, initErr = a.store.load()
		if initErr == nil {
			var answers map[string]response
			var stars map[string]bool
			answers, stars, initErr = a.store.loadPractice()
			if initErr == nil {
				a.answers = answers
				a.starred = stars
				var preferenceErr error
				a.repeatPractice, preferenceErr = a.store.loadRepeatPractice()
				if preferenceErr != nil {
					a.libraryStatus = "学习设置读取失败，已使用默认显示方式"
				}
			}
		}
		if initErr == nil {
			a.syncStars()

			a.navigate(0)
			if a.libraryStatus == "" {
				a.libraryStatus = "题库与答题记录自动保存"
			}
		}
	}
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	mygo.App.WhenReady(func() {
		if a.dark {
			mygo.Theme.SetSource(mygo.ThemeDark)
		} else {
			mygo.Theme.SetSource(mygo.ThemeLight)
		}
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Quesora · 原生界面还原", Width: 1450, Height: 800, MinWidth: 800, MinHeight: 500, TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 44, Vibrancy: mygo.VibrancyMica, Content: ui.View(a.view)})
		a.dispatch = a.win.Update
		a.pickFile = func() ([]string, error) {
			return mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "上传题库", Filters: []mygo.FileFilter{{Name: "题库文件", Extensions: []string{"xlsx", "xls", "csv"}}}})
		}
		a.win.OnClosed(func() { a.closed = true })
		a.win.OnFileDrop(func(e *mygo.FileDropEvent) {
			paths := append([]string(nil), e.Paths...)
			a.win.Update(func() {
				if !a.closed {
					a.startImport(paths)
				}
			})
		})
		if initErr != nil {
			a.importOpen = true
			a.importError = true
			a.importMessage = "加载本地题库失败：" + initErr.Error()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Print(err)
	}
}
