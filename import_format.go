package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Exported workbooks may carry an older extension. Choose the reader from
// the container signature; retain the original filename for bank naming.
func detectQuestionFileFormat(filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".xls", ".xlsx", ".csv":
	default:
		return "", fmt.Errorf("不支持此文件类型，请选择 .xlsx、.xls 或 .csv")
	}
	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("无法读取文件：%w", err)
	}
	defer f.Close()
	var header [8]byte
	n, err := io.ReadFull(f, header[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("无法读取文件头：%w", err)
	}
	data := header[:n]
	if bytes.HasPrefix(data, []byte{'P', 'K', 3, 4}) || bytes.HasPrefix(data, []byte{'P', 'K', 5, 6}) || bytes.HasPrefix(data, []byte{'P', 'K', 7, 8}) {
		return ".xlsx", nil
	}
	if bytes.Equal(data, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
		return ".xls", nil
	}
	return ext, nil
}
