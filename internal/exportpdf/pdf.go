// Package exportpdf writes native examination papers from captured model values.
package exportpdf

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
	"github.com/ispacewang/quesora-native/internal/assets"
	"github.com/ispacewang/quesora-native/internal/diagram"
	"github.com/ispacewang/quesora-native/internal/domain"
	"github.com/ispacewang/quesora-native/internal/mathtext"
	"golang.org/x/image/font/sfnt"
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

func paperFont(weight int) ([]byte, error) {
	data, err := assets.Files.ReadFile(fmt.Sprintf("resources/fonts/noto-%d.ttf.gz", weight))
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return io.ReadAll(gz)
}

type questionGroup struct {
	kind    string
	indices []int
}

// groups keeps the source index for each question, so both PDFs use the same
// numbers, option order, and answer records even when given a mixed paper.
func groups(qs []domain.Question) []questionGroup {
	kinds := domain.QuestionKinds()
	byKind := make(map[string][]int, len(kinds))
	var other []int
	for i, q := range qs {
		known := false
		for _, kind := range kinds {
			if q.Kind == kind {
				known = true
				break
			}
		}
		if known {
			byKind[q.Kind] = append(byKind[q.Kind], i)
		} else {
			other = append(other, i)
		}
	}
	var out []questionGroup
	for _, kind := range kinds {
		if len(byKind[kind]) > 0 {
			out = append(out, questionGroup{kind, byKind[kind]})
		}
	}
	if len(other) > 0 {
		out = append(out, questionGroup{"其他题", other})
	}
	return out
}

// Write 用题型分组和固定试卷快照生成 A4 图文 PDF。写入成功后才替换目标。
func Write(filename string, e Paper, answers bool) error {
	font, err := paperFont(400)
	if err != nil {
		return err
	}
	bold, err := paperFont(700)
	if err != nil {
		return err
	}
	coverage, err := sfnt.Parse(font)
	if err != nil {
		return err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(leftEdge, 30, leftEdge)
	pdf.SetAutoPageBreak(true, 297-bottomEdge)
	pdf.AddUTF8FontFromBytes("noto", "", font)
	pdf.AddUTF8FontFromBytes("noto", "B", bold)
	pdf.SetFont("noto", "", 11)
	pdf.SetTitle("Quesora · "+e.Bank, true)
	pdf.SetAuthor("Quesora", true)
	if err := paperChrome(pdf); err != nil {
		return err
	}
	pdf.AddPage()
	layout := pageLayout{pdf: pdf, raster: e.Rasterize, coverage: coverage}
	label := "试卷"
	if answers {
		label = "答案与解析"
	}
	pdf.SetTextColor(141, 61, 35)
	pdf.SetFont("noto", "B", 10)
	layout.text("", label, 5)
	pdf.Ln(2)
	pdf.SetFont("noto", "B", 23)
	pdf.SetTextColor(39, 34, 30)
	title := strings.TrimSpace(e.Bank)
	if title == "" {
		title = "练习题库"
	}
	layout.text("", title, 11)
	pdf.SetFont("noto", "", 10)
	pdf.SetTextColor(107, 92, 82)
	layout.text("", fmt.Sprintf("%d 分钟  ·  共 %d 题", e.Minutes, len(e.Questions)), 6)
	pdf.Ln(8)

	for section, group := range groups(e.Questions) {
		layout.section(section, group, questionLead(e.Questions[group.indices[0]]))
		for _, i := range group.indices {
			q := e.Questions[i]
			layout.ensureSpace(questionLead(q))
			pdf.SetTextColor(43, 37, 33)
			pdf.SetFont("noto", "", 11)
			pdf.Text(leftEdge, pdf.GetY()+4.6, fmt.Sprintf("%02d.", i+1))
			flag := ""
			if i < len(e.Important) && e.Important[i] {
				flag = "【重要】 "
			}
			layout.textAt(flag, q.Prompt, 7, 12)
			if svg, err := diagram.FromMeta(q.Meta); err == nil && svg != "" {
				layout.figure(svg)
			}
			var response domain.Response
			if i < len(e.Responses) {
				response = e.Responses[i]
			}
			if !domain.IsTextQuestion(q) {
				pdf.Ln(1.5)
				for option, original := range domain.ResponseOrder(q, response) {
					layout.textAt(fmt.Sprintf("%c.  ", rune('A'+option)), q.Options[original], 7, 12)
				}
			}
			if answers {
				pdf.Ln(1.5)
				pdf.SetTextColor(141, 61, 35)
				expected := domain.ResponseLetters(q, response, q.Answer)
				if domain.IsTextQuestion(q) {
					expected = q.TextAnswer
				}
				layout.textAt("答案  ", expected, 6.5, 12)
				pdf.SetTextColor(94, 82, 74)
				if q.Explanation != "" {
					layout.textAt("解析  ", q.Explanation, 6.5, 12)
				}
			} else if domain.IsTextQuestion(q) {
				layout.ensureSpace(27)
				pdf.SetDrawColor(221, 210, 199)
				for j := 0; j < 3; j++ {
					y := pdf.GetY() + 8
					pdf.Line(leftEdge+12, y, leftEdge+lineWidth, y)
					pdf.SetY(y)
				}
			}
			pdf.Ln(7)
		}
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

// questionLead keeps a heading, its first question and the first figure or
// formula together when they fit. Long paragraphs still flow line by line.
func questionLead(q domain.Question) float64 {
	visibleLines := 0
	for _, line := range strings.Split(q.Prompt, "\n") {
		visibleLines += max(1, (utf8.RuneCountInString(line)+39)/40)
	}
	estimate := float64(min(visibleLines, 5))*7 + 9
	for _, part := range mathtext.Split(q.Prompt) {
		if part.Kind == mathtext.Display {
			if vector, err := mathtext.Render(part.TeX, 20, true); err == nil {
				estimate += 5 + min(55, float64(vector.Ascent+vector.Descent)*mmPerDIP)
			}
		}
	}
	if q.Meta["diagram"] != nil || q.Meta["diagramSvg"] != nil {
		estimate += 70
	}
	return min(110, estimate)
}
