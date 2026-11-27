// 使用固定试卷快照、内置字体和原生配图渲染器导出 PDF。

package desktop

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/ispacewang/quesora-native/internal/exportpdf"
)

// exportExamPDF 在启动后台文件选择/生成前固定题序和选项映射，保持试卷、答案与随后考试一致。
func (a *app) exportExamPDF(answers bool) {
	if a.paperBusy || a.win == nil {
		return
	}
	paper, err := a.prepareExamPaper()
	if err != nil {
		a.examMessage = err.Error()
		return
	}
	e := snapshotExamPaper(paper)
	a.paperBusy = true
	a.examMessage = "正在生成 PDF…"
	win := a.win
	label := "试卷"
	if answers {
		label = "答案"
	}
	safeName := strings.Map(func(r rune) rune {
		if strings.ContainsRune("<>:\"/\\|?*", r) {
			return '_'
		}
		return r
	}, e.Bank) + "_" + label + ".pdf"
	go func() {
		filename, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win, Title: "导出" + label, DefaultPath: safeName, Filters: []mygo.FileFilter{{Name: "PDF", Extensions: []string{"pdf"}}}})
		if err == nil && filename != "" {
			err = exportpdf.Write(filename, exportpdf.Paper{Bank: e.Bank, Minutes: e.Minutes, Questions: e.Questions, Responses: e.Responses, Important: e.Important, Rasterize: rasterizePaperSVG}, answers)
		}
		a.publish(func() {
			a.paperBusy = false
			a.examMessage = ""
			if err != nil {
				a.examMessage = "导出失败：" + err.Error()
			} else if filename != "" {
				a.libraryStatus = e.Bank + " · " + label + "已导出"
			}
		})
	}()
}
