package exportpdf

import (
	"bytes"
	"fmt"
	"github.com/go-pdf/fpdf"
	"github.com/ispacewang/quesora-native/internal/assets"
)

// paperChrome places the shared application artwork on every printed page.
func paperChrome(pdf *fpdf.Fpdf) error {
	logo, err := assets.Files.ReadFile("resources/app-icon.png")
	if err != nil {
		return err
	}
	iconOptions := fpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("quesora-brand", iconOptions, bytes.NewReader(logo))
	pdf.AliasNbPages("")
	pdf.SetHeaderFunc(func() {
		pdf.ImageOptions("quesora-brand", leftEdge, 12.5, 6, 6, false, iconOptions, 0, "")
		pdf.SetFont("noto", "B", 11)
		pdf.SetTextColor(63, 48, 39)
		pdf.Text(leftEdge+9, 16.8, "Quesora")
		pdf.SetDrawColor(222, 211, 201)
		pdf.SetLineWidth(.24)
		pdf.Line(leftEdge, 22, leftEdge+lineWidth, 22)
		pdf.SetY(30)
	})
	pdf.SetFooterFunc(func() {
		pdf.SetDrawColor(222, 211, 201)
		pdf.SetLineWidth(.24)
		pdf.Line(leftEdge, 280, leftEdge+lineWidth, 280)
		pdf.SetFont("noto", "", 8)
		pdf.SetTextColor(110, 93, 80)
		pdf.SetY(282)
		pdf.CellFormat(lineWidth, 5, fmt.Sprintf("%d / {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	return pdf.Error()
}

func (l *pageLayout) section(index int, group questionGroup, leadHeight float64) {
	l.ensureSpace(13 + leadHeight)
	y := l.pdf.GetY()
	l.pdf.SetFillColor(247, 237, 226)
	l.pdf.Rect(leftEdge, y, lineWidth, 9, "F")
	l.pdf.SetFillColor(206, 89, 46)
	l.pdf.Rect(leftEdge, y, 1.0, 9, "F")
	l.pdf.SetFont("noto", "B", 12)
	l.pdf.SetTextColor(63, 48, 39)
	numbers := []string{"一", "二", "三", "四", "五", "六"}
	l.pdf.Text(leftEdge+4, y+6, numbers[index]+"、"+group.kind)
	l.pdf.SetFont("noto", "", 9)
	l.pdf.SetTextColor(110, 92, 79)
	count := fmt.Sprintf("%d 题", len(group.indices))
	l.pdf.Text(leftEdge+lineWidth-4-l.pdf.GetStringWidth(count), y+5.8, count)
	l.pdf.SetXY(leftEdge, y+13)
}
