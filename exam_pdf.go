package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/go-pdf/fpdf"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func writeExamPDF(filename string, e *examSession, answers bool) error {
	data, err := nativeAssets.ReadFile("resources/fonts/noto-400.ttf.gz")
	if err != nil {
		return err
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	font, err := io.ReadAll(gz)
	gz.Close()
	if err != nil {
		return err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddUTF8FontFromBytes("noto", "", font)
	pdf.SetFont("noto", "", 11)
	pdf.SetTitle("Quesora · "+e.Bank, true)
	pdf.AddPage()
	label := "试卷"
	if answers {
		label = "答案"
	}
	pdf.SetFontSize(18)
	pdf.MultiCell(0, 10, e.Bank+" · "+label, "", "L", false)
	pdf.SetFontSize(10)
	pdf.MultiCell(0, 7, fmt.Sprintf("时长 %d 分钟    共 %d 题", e.Minutes, len(e.Questions)), "", "L", false)
	pdf.Ln(4)
	for i, q := range e.Questions {
		pdf.SetFontSize(11)
		important := ""
		if e.Important[i] {
			important = " [重要题目]"
		}
		pdf.MultiCell(0, 7, fmt.Sprintf("%d. [%s]%s %s", i+1, q.Kind, important, q.Prompt), "", "L", false)
		if !isTextQuestion(q) {
			for d, original := range responseOrder(q, e.Responses[i]) {
				pdf.MultiCell(0, 6, fmt.Sprintf("   %c. %s", rune('A'+d), q.Options[original]), "", "L", false)
			}
		}
		if answers {
			expected := responseLetters(q, e.Responses[i], q.Answer)
			if isTextQuestion(q) {
				expected = q.TextAnswer
			}
			pdf.MultiCell(0, 7, "正确答案："+expected, "", "L", false)
			if q.Explanation != "" {
				pdf.MultiCell(0, 6, "解析："+q.Explanation, "", "L", false)
			}
		} else if isTextQuestion(q) {
			pdf.MultiCell(0, 8, "________________________________________________\n________________________________________________\n________________________________________________", "", "L", false)
		}
		pdf.Ln(5)
	}
	if pdf.Error() != nil {
		return pdf.Error()
	}
	tmp, err := os.CreateTemp(filepath.Dir(filename), ".quesora-paper-*.pdf")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = pdf.Output(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, filename)
}
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
			err = writeExamPDF(filename, e, answers)
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
