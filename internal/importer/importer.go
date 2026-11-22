// 文件到标准题库的入口：限制资源用量、选择格式读取器、归一化工作表并保留来源警告。

package importer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/ispacewang/quesora-native/internal/domain"
)

const MaxImportBytes = 64 << 20
const maxImportRows = 100000

// Bank 表示一次解析中可独立保存的工作表题库。
type Bank struct {
	Name      string
	Questions []domain.Question
}

// Result 同时携带标准题库和可定位来源行的警告，允许有效行导入、无效行显式跳过。
type Result struct {
	Banks    []Bank
	Warnings []string
}

// Worksheet 是各文件读取器的公共中间表示，列语义留给 questionSchema 判定。
type Worksheet struct {
	Name string
	Rows [][]string
}

var optionPrefix = regexp.MustCompile(`^(?:[A-Za-z]\s*[.、)）：:．（）—–\-]\s*)+`)

// ParseFile 只读取源文件，限制文件/解压/行数，并将所有格式交给相同的语义归一化流程。
func ParseFile(filename string) (result Result, err error) {
	info, err := os.Stat(filename)
	if err != nil {
		return result, fmt.Errorf("无法读取文件：%w", err)
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("请选择题库文件")
	}
	if info.Size() > MaxImportBytes {
		return result, fmt.Errorf("文件超过 64 MiB，请拆分后导入")
	}
	format, err := DetectFormat(filename)
	if err != nil {
		return result, err
	}
	if ext := strings.ToLower(filepath.Ext(filename)); format != ext {
		result.Warnings = append(result.Warnings, fmt.Sprintf("文件扩展名为 %s，实际内容为 %s，已按实际格式读取；原文件未修改。", ext, strings.ToUpper(strings.TrimPrefix(format, "."))))
	}
	var sheets []Worksheet
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
			sheet := Worksheet{Name: name}
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
		sheets, err = ReadLegacyXLS(filename)
		if err != nil {
			return result, err
		}
	case ".csv":
		data, e := os.ReadFile(filename)
		if e != nil {
			return result, e
		}
		data, e = DecodeCSV(data)
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
		sheet := Worksheet{Name: "Sheet1"}
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
		sheets = []Worksheet{sheet}
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
			result.Banks = append(result.Banks, Bank{Name: sheet.Name, Questions: qs})
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

// DecodeCSV 优先识别 BOM/UTF-8，无 BOM 且非 UTF-8 时按 GB18030 兼容常见中文 CSV。
func DecodeCSV(data []byte) ([]byte, error) {
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

func WriteTemplate(filename string) error {
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

// StripOptionPrefix removes printed choice labels without altering option text.
func StripOptionPrefix(s string) string { return optionPrefix.ReplaceAllString(s, "") }
