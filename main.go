// 应用入口：命令行、数据目录、窗口和后台任务的创建/关闭顺序。快照模式在打开真实数据库前返回。

package main

import (
	"flag"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	snapshot := flag.String("snapshot", "", "render a preview PNG")
	dark := flag.Bool("dark", false, "dark appearance")
	showUpload := flag.Bool("show-upload", false, "open upload dialog")
	dataDir := flag.String("data-dir", "", "directory for native question data")
	scene := flag.String("scene", "single", "preview scenario")
	flag.Parse()
	darkSpecified := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "dark" {
			darkSpecified = true
		}
	})
	a := newApp()
	a.dark = *dark
	if darkSpecified {
		a.appearance = "light"
		if *dark {
			a.appearance = "dark"
		}
	}
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
	var locationErr error
	if *dataDir == "" {
		var location dataLocation
		location, locationErr = defaultDataLocation()
		if locationErr == nil {
			*dataDir = location.Dir
			locationErr = prepareInstalledData(location)
		}
	}
	a.shuffle = true
	a.leftOpen = false
	a.rightOpen = false
	a.questions = nil
	a.bank = ""
	var initErr = locationErr
	if initErr == nil {
		a.store, initErr = openQuestionStore(filepath.Join(*dataDir, "questions.db"))
	}
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

			if err := a.restorePreferences(); err != nil {
				a.libraryStatus = "学习设置读取失败，已使用默认显示方式"
				a.navigate(0)
			}
			if darkSpecified {
				a.dark = *dark
				a.appearance = "light"
				if *dark {
					a.appearance = "dark"
				}
			}
			if a.libraryStatus == "" {
				a.libraryStatus = "题库与答题记录自动保存"
			}
		}
	}
	a.configureAI()
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	mygo.App.WhenReady(func() {
		a.configureAppearance()
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Quesora · 原生界面还原", Width: 1450, Height: 800, MinWidth: 800, MinHeight: 500, TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 44, Vibrancy: mygo.VibrancyMica, Content: ui.View(a.view)})
		// Set both native window icon sizes from the reviewed artwork instead of a cached EXE icon.
		iconPNG, iconErr := nativeAssets.ReadFile("resources/app-icon.png")
		if iconErr == nil {
			iconErr = a.win.SetIcon(iconPNG)
		}
		if iconErr != nil {
			log.Print("设置窗口图标失败：", iconErr)
		}
		a.dispatch = a.win.Update
		a.configureUpdater()
		a.pickBackup = func() (string, error) {
			return mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: "备份全部题库与记录", DefaultPath: "Quesora备份-" + time.Now().Format("20060102-150405") + ".db", Filters: []mygo.FileFilter{{Name: "Quesora 数据备份", Extensions: []string{"db"}}}})
		}
		a.pickRestore = func() (string, error) {
			paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "选择原生数据库备份", Filters: []mygo.FileFilter{{Name: "Quesora 数据备份", Extensions: []string{"db"}}}})
			if err != nil || len(paths) == 0 {
				return "", err
			}
			return paths[0], nil
		}
		a.showDataFolder = func() error { return mygo.Shell.OpenPath(filepath.Dir(a.store.path)) }
		a.pickFile = func() ([]string, error) {
			return mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "上传题库", Filters: []mygo.FileFilter{{Name: "题库文件", Extensions: []string{"xlsx", "xls", "csv"}}}})
		}
		a.win.OnClosed(func() {
			if a.themeOff != nil {
				a.themeOff()
			}
			if a.ai.cancel != nil {
				a.ai.cancel()
			}
			if a.updates.cancel != nil {
				a.updates.cancel()
			}
			if !a.restoreBusy {
				a.persistPreferences()
			}
			a.closed = true
		})
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
