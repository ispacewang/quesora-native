package main

import (
	"archive/zip"
	"context"
	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIMaterialReadersPreserveSourceAndExtractText(t *testing.T) {
	dir := t.TempDir()
	word := filepath.Join(dir, "资料.docx")
	f, _ := os.Create(word)
	z := zip.NewWriter(f)
	w, _ := z.Create("word/document.xml")
	w.Write([]byte("<document><p><t>上岗前检查设备。</t></p><p><t>异常时停止操作。</t></p></document>"))
	z.Close()
	f.Close()
	book := excelize.NewFile()
	book.SetCellValue("Sheet1", "A1", "安全规范")
	xlsx := filepath.Join(dir, "资料.xlsx")
	if e := book.SaveAs(xlsx); e != nil {
		t.Fatal(e)
	}
	book.Close()
	pdfpath := filepath.Join(dir, "notes.pdf")
	p := fpdf.New("P", "mm", "A4", "")
	p.AddPage()
	p.SetFont("Arial", "", 12)
	p.Text(10, 20, "SAFETY CHECK")
	if e := p.OutputFileAndClose(pdfpath); e != nil {
		t.Fatal(e)
	}
	renamed := filepath.Join(dir, "扩展名不符.xls")
	rawXLSX, _ := os.ReadFile(xlsx)
	os.WriteFile(renamed, rawXLSX, 0600)
	txt := filepath.Join(dir, "资料.md")
	os.WriteFile(txt, []byte("# 安全规范\n上岗前检查设备。"), 0600)
	for _, v := range []struct{ path, want string }{{word, "上岗前检查设备。"}, {xlsx, "安全规范"}, {renamed, "安全规范"}, {pdfpath, "SAFETY CHECK"}, {txt, "安全规范"}} {
		t.Run(filepath.Ext(v.path), func(t *testing.T) {
			before, _ := os.ReadFile(v.path)
			m, e := readAIMaterial(context.Background(), v.path)
			if e != nil || !strings.Contains(m.Text, v.want) {
				t.Fatal(m, e)
			}
			after, _ := os.ReadFile(v.path)
			if string(before) != string(after) {
				t.Fatal("reference source modified")
			}
		})
	}
}
func TestAIMaterialRejectsEmptyMalformedAndOversizedInput(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []struct{ name, text string }{{"empty.txt", " "}, {"broken.pdf", "not a PDF"}, {"large.txt", strings.Repeat("字", 60001)}} {
		p := filepath.Join(dir, v.name)
		os.WriteFile(p, []byte(v.text), 0600)
		if _, e := readAIMaterial(context.Background(), p); e == nil {
			t.Fatal("accepted invalid material", v.name)
		}
	}
	p := filepath.Join(dir, "cancel.txt")
	os.WriteFile(p, []byte("test"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := readAIMaterial(ctx, p); e == nil {
		t.Fatal("cancelled read accepted")
	}
}
