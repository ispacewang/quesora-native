// Package exportpdf writes native examination papers from captured model values.
package exportpdf

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/go-pdf/fpdf"
	"github.com/ispacewang/quesora-native/internal/assets"
	"github.com/ispacewang/quesora-native/internal/diagram"
	"github.com/ispacewang/quesora-native/internal/domain"
	"image"
	"io"
	"os"
	"path/filepath"
)

// Paper captures the fields needed for a paper or answer-sheet export.
type Paper struct {
	Bank      string
	Minutes   int
	Questions []domain.Question
	Responses []domain.Response
	Important []bool
	Rasterize func(svg string, maximum int) (image.Image, error)
}

// Write 用固定试卷快照生成 A4 图文 PDF，写入临时文件成功后才替换目标。
func Write(filename string, e Paper, answers bool) error {
	data, err := assets.Files.ReadFile("resources/fonts/noto-400.ttf.gz")
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
	layout := pageLayout{pdf: pdf, raster: e.Rasterize}
	for i, q := range e.Questions {
		pdf.SetFontSize(11)
		important := ""
		if i < len(e.Important) && e.Important[i] {
			important = " [重要题目]"
		}
		layout.text(fmt.Sprintf("%d. [%s]%s ", i+1, q.Kind, important), q.Prompt, 7)
		if svg, err := diagram.FromMeta(q.Meta); err == nil && svg != "" {
			layout.figure(svg)
		}
		var response domain.Response
		if i < len(e.Responses) {
			response = e.Responses[i]
		}
		if !domain.IsTextQuestion(q) {
			for d, original := range domain.ResponseOrder(q, response) {
				layout.text(fmt.Sprintf("   %c. ", rune('A'+d)), q.Options[original], 6)
			}
		}
		if answers {
			expected := domain.ResponseLetters(q, response, q.Answer)
			if domain.IsTextQuestion(q) {
				expected = q.TextAnswer
			}
			layout.text("正确答案：", expected, 7)
			if q.Explanation != "" {
				layout.text("解析：", q.Explanation, 6)
			}
		} else if domain.IsTextQuestion(q) {
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
