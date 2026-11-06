// 参考资料以纯 Go 解析为文本，不执行附件内容；源文件只读，资料只在用户发起生成时发送。
package main

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type aiMaterial struct{ Name, Text string }

func (a *app) readPickedAIMaterial(ctx context.Context, win *mygo.Window) (aiTaskResult, error) {
	paths, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "选择 AI 出题参考资料", Filters: []mygo.FileFilter{{Name: "参考资料", Extensions: []string{"txt", "md", "csv", "xlsx", "xls", "docx", "pdf"}}}})
	if e != nil {
		return aiTaskResult{}, errors.New("无法选择参考资料")
	}
	if len(paths) == 0 {
		return aiTaskResult{}, context.Canceled
	}
	if ctx.Err() != nil {
		return aiTaskResult{}, ctx.Err()
	}
	m, e := readAIMaterial(ctx, paths[0])
	return aiTaskResult{Text: fmt.Sprintf("已载入 %s · %d 字", m.Name, len([]rune(m.Text))), Material: &m}, e
}

// readAIMaterial bounds input size/page count and rejects empty scanned PDFs rather than inventing text.
func readAIMaterial(ctx context.Context, filename string) (m aiMaterial, err error) {
	defer func() {
		if recover() != nil {
			m = aiMaterial{}
			err = errors.New("资料格式异常，无法解析")
		}
	}()
	info, e := os.Stat(filename)
	if e != nil || !info.Mode().IsRegular() {
		return m, errors.New("无法读取参考资料")
	}
	if info.Size() > 16<<20 {
		return m, errors.New("参考资料不能超过 16 MiB")
	}
	var out strings.Builder
	appendText := func(s string) error {
		if out.Len()+len(s) > 240000 {
			return errors.New("参考资料过长，请拆分为不超过 60000 字的文件")
		}
		out.WriteString(s)
		return ctx.Err()
	}
	format := strings.ToLower(filepath.Ext(filename))
	if format == ".xls" || format == ".xlsx" {
		detected, e := detectQuestionFileFormat(filename)
		if e != nil {
			return m, errors.New("Excel 资料格式无法识别")
		}
		format = detected
	}
	switch format {
	case ".txt", ".md", ".csv":
		raw, e := os.ReadFile(filename)
		if e != nil {
			return m, errors.New("无法读取资料")
		}
		raw, e = decodeCSV(raw)
		if e != nil {
			return m, errors.New("资料编码无法识别")
		}
		if e = appendText(string(raw)); e != nil {
			return m, e
		}
	case ".docx":
		z, e := zip.OpenReader(filename)
		if e != nil {
			return m, errors.New("DOCX 文件无效")
		}
		defer z.Close()
		found := false
		for _, f := range z.File {
			if f.Name != "word/document.xml" {
				continue
			}
			found = true
			if f.UncompressedSize64 > 16<<20 {
				return m, errors.New("DOCX 内容过大")
			}
			r, e := f.Open()
			if e != nil {
				return m, e
			}
			defer r.Close()
			d := xml.NewDecoder(io.LimitReader(r, 16<<20))
			inText := false
			for {
				tok, e := d.Token()
				if e == io.EOF {
					break
				}
				if e != nil {
					return m, errors.New("DOCX 正文无法解析")
				}
				switch v := tok.(type) {
				case xml.StartElement:
					if v.Name.Local == "t" {
						inText = true
					}
				case xml.EndElement:
					if v.Name.Local == "t" {
						inText = false
					}
					if v.Name.Local == "p" {
						if e = appendText("\n"); e != nil {
							return m, e
						}
					}
				case xml.CharData:
					if inText {
						if e = appendText(string(v)); e != nil {
							return m, e
						}
					}
				}
			}
		}
		if !found {
			return m, errors.New("DOCX 缺少正文")
		}
	case ".xlsx":
		book, e := excelize.OpenFile(filename, excelize.Options{UnzipSizeLimit: 32 << 20, UnzipXMLSizeLimit: 16 << 20})
		if e != nil {
			return m, errors.New("Excel 资料无法解析")
		}
		defer book.Close()
		for _, name := range book.GetSheetList() {
			if e = appendText("\n# " + name + "\n"); e != nil {
				return m, e
			}
			rows, e := book.Rows(name)
			if e != nil {
				return m, e
			}
			for rows.Next() {
				cols, e := rows.Columns()
				if e != nil {
					rows.Close()
					return m, e
				}
				if e = appendText(strings.Join(cols, "\t") + "\n"); e != nil {
					rows.Close()
					return m, e
				}
			}
			e = rows.Error()
			rows.Close()
			if e != nil {
				return m, e
			}
		}
	case ".xls":
		sheets, e := readLegacyXLS(filename)
		if e != nil {
			return m, errors.New("XLS 资料无法解析")
		}
		for _, s := range sheets {
			for _, row := range s.Rows {
				if e = appendText(strings.Join(row, "\t") + "\n"); e != nil {
					return m, e
				}
			}
		}
	case ".pdf":
		f, e := os.Open(filename)
		if e != nil {
			return m, e
		}
		defer f.Close()
		r, e := pdf.NewReader(f, info.Size())
		if e != nil {
			return m, errors.New("PDF 无法解析，可能已加密或损坏")
		}
		if r.NumPage() > 200 {
			return m, errors.New("PDF 超过 200 页，请先拆分")
		}
		for i := 1; i <= r.NumPage(); i++ {
			if ctx.Err() != nil {
				return m, ctx.Err()
			}
			text, e := r.Page(i).GetPlainText(nil)
			if e != nil {
				return m, errors.New("PDF 文本无法提取")
			}
			if e = appendText(text + "\n"); e != nil {
				return m, e
			}
		}
	default:
		return m, errors.New("支持 TXT、Markdown、CSV、Excel、DOCX 和文本 PDF")
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return m, errors.New("资料没有可提取的文字；扫描 PDF 请先进行 OCR")
	}
	if len([]rune(text)) > 60000 {
		return m, errors.New("参考资料最多 60000 字，请先拆分")
	}
	return aiMaterial{Name: filepath.Base(filename), Text: text}, nil
}
