package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxImportBytes = 64 << 20
const maxImportRows = 100000

type importedBank struct {
	Name      string
	Questions []question
}
type importResult struct {
	Banks    []importedBank
	Warnings []string
}
type worksheet struct {
	Name string
	Rows [][]string
}

var optionPrefix = regexp.MustCompile(`^(?:[A-Za-z]\s*[.、)）：:．（）—–\-]\s*)+`)

func parseQuestionFile(filename string) (result importResult, err error) {
	info, err := os.Stat(filename)
	if err != nil {
		return result, fmt.Errorf("无法读取文件：%w", err)
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("请选择题库文件")
	}
	if info.Size() > maxImportBytes {
		return result, fmt.Errorf("文件超过 64 MiB，请拆分后导入")
	}
	format, err := detectQuestionFileFormat(filename)
	if err != nil {
		return result, err
	}
	if ext := strings.ToLower(filepath.Ext(filename)); format != ext {
		result.Warnings = append(result.Warnings, fmt.Sprintf("文件扩展名为 %s，实际内容为 %s，已按实际格式读取；原文件未修改。", ext, strings.ToUpper(strings.TrimPrefix(format, "."))))
	}
	var sheets []worksheet
	switch format {
	case ".xlsx":
		book, e := excelize.OpenFile(filename, excelize.Options{UnzipSizeLimit: 128 << 20, UnzipXMLSizeLimit: 16 << 20})
		if e != nil {
			return result, fmt.Errorf("Excel 文件无法解析：%w", e)
		}
		defer book.Close()
		for _, name := range book.GetSheetList() {
			iterator, e := book.Rows(name)
			if e != nil {
				return result, fmt.Errorf("工作表 %s：%w", name, e)
			}
			sheet := worksheet{Name: name}
			for iterator.Next() {
				if len(sheet.Rows) >= maxImportRows {
					iterator.Close()
					return result, fmt.Errorf("工作表 %s 超过十万行", name)
				}
				row, e := iterator.Columns()
				if e != nil {
					iterator.Close()
					return result, e
				}
				sheet.Rows = append(sheet.Rows, row)
			}
			e = iterator.Error()
			closeErr := iterator.Close()
			if e != nil {
				return result, e
			}
			if closeErr != nil {
				return result, closeErr
			}
			sheets = append(sheets, sheet)
		}
	case ".xls":
		sheets, err = readLegacyXLS(filename)
		if err != nil {
			return result, err
		}
	case ".csv":
		data, e := os.ReadFile(filename)
		if e != nil {
			return result, e
		}
		data, e = decodeCSV(data)
		if e != nil {
			return result, e
		}
		reader := csv.NewReader(bytes.NewReader(data))
		reader.FieldsPerRecord = -1
		first := strings.SplitN(string(data), "\n", 2)[0]
		if strings.Count(first, "\t") > strings.Count(first, ",") {
			reader.Comma = '\t'
		} else if strings.Count(first, ";") > strings.Count(first, ",") {
			reader.Comma = ';'
		}
		sheet := worksheet{Name: "Sheet1"}
		for {
			row, e := reader.Read()
			if e == io.EOF {
				break
			}
			if e != nil {
				return result, fmt.Errorf("CSV 第 %d 行附近格式错误：%w", len(sheet.Rows)+1, e)
			}
			if len(sheet.Rows) >= maxImportRows {
				return result, fmt.Errorf("CSV 超过十万行")
			}
			sheet.Rows = append(sheet.Rows, row)
		}
		sheets = []worksheet{sheet}
	default:
		return result, fmt.Errorf("不支持此文件类型，请选择 .xlsx、.xls 或 .csv")
	}
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	for _, sheet := range sheets {
		qs, warnings, e := questionsFromRows(sheet.Name, sheet.Rows)
		if e != nil {
			return result, e
		}
		result.Warnings = append(result.Warnings, warnings...)
		if len(qs) > 0 {
			result.Banks = append(result.Banks, importedBank{Name: sheet.Name, Questions: qs})
		}
	}
	if len(result.Banks) == 0 {
		return result, fmt.Errorf("文件中没有解析到可用题目，请检查题干、答案及表头说明；不能明确识别的行已列入提示")
	}
	for i := range result.Banks {
		name := base
		if len(result.Banks) > 1 {
			name += "-" + result.Banks[i].Name
		}
		result.Banks[i].Name = name
		for j := range result.Banks[i].Questions {
			result.Banks[i].Questions[j].Bank = name
		}
	}
	return result, nil
}
func decodeCSV(data []byte) ([]byte, error) {
	switch {
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		return data[3:], nil
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		out, _, err := transform.Bytes(unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder(), data)
		return out, err
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		out, _, err := transform.Bytes(unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder(), data)
		return out, err
	case utf8.Valid(data):
		return data, nil
	default:
		out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), data)
		return out, err
	}
}
func answerMask(answer, kind string, options []string) uint {
	if kind == "简答题" || kind == "填空题" {
		return 0
	}
	if kind == "判断题" {
		norm := strings.ToLower(strings.TrimSpace(answer))
		for i, opt := range options {
			if norm == strings.ToLower(opt) {
				return 1 << i
			}
		}
		switch norm {
		case "正确", "对", "是", "true", "√", "✓", "1", "t":
			return 1
		case "错误", "错", "否", "false", "×", "✗", "0", "f":
			return 2
		}
	}
	var mask uint
	for _, r := range strings.ToUpper(answer) {
		if r >= 'A' && r <= 'Z' {
			mask |= 1 << uint(r-'A')
		}
	}
	return mask
}
func writeQuestionTemplate(filename string) error {
	book := excelize.NewFile()
	defer book.Close()
	rows := [][]any{
		{"题型", "题干", "选项", "答案", "解析", "备注"},
		{"单选题", "下列哪种数据结构遵循先进先出？", "A. 栈|B. 队列|C. 树|D. 哈希表", "B", "队列遵循先进先出。", "重要题目"},
		{"多选题", "以下哪些是偶数？", "A. 2|B. 3|C. 4|D. 5", "AC", "2 和 4 是偶数。", ""},
		{"判断题", "地球围绕太阳公转。", "", "正确", "地球围绕太阳公转。", ""},
		{"填空题", "1 + 1 = ____。", "", "2", "基本加法。", ""},
		{"简答题", "列举操作系统的三项主要职责。", "", "进程管理；内存管理；文件管理", "答出三个要点。", ""}}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := book.SetSheetRow("Sheet1", cell, &row); err != nil {
			return err
		}
	}
	if err := book.SetColWidth("Sheet1", "B", "F", 36); err != nil {
		return err
	}
	return book.SaveAs(filename)
}
