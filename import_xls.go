package main

import (
	"fmt"
	"github.com/extrame/xls"
	"os"
)

// Recover only at the legacy parser boundary; malformed BIFF records can panic in this library.
func readLegacyXLS(filename string) (sheets []worksheet, err error) {
	defer func() {
		if r := recover(); r != nil {
			sheets = nil
			err = fmt.Errorf("旧版 XLS 文件损坏或不兼容，请另存为 .xlsx 后导入")
		}
	}()
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	book, err := xls.OpenReader(f, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("XLS 文件无法解析：%w", err)
	}
	for i := 0; i < book.NumSheets(); i++ {
		sheet := book.GetSheet(i)
		if sheet == nil {
			continue
		}
		out := worksheet{Name: sheet.Name}
		for r := 0; r <= int(sheet.MaxRow); r++ {
			row := sheet.Row(r)
			if row == nil {
				out.Rows = append(out.Rows, nil)
				continue
			}
			if row.LastCol() > 1024 {
				return nil, fmt.Errorf("工作表 %s 的列数过多", sheet.Name)
			}
			cols := make([]string, row.LastCol())
			for c := range cols {
				cols[c] = row.Col(c)
			}
			out.Rows = append(out.Rows, cols)
		}
		sheets = append(sheets, out)
	}
	return sheets, nil
}
