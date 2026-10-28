// Modified for Quesora: use mscfb for CFB traversal; support root storage names
// other than "Root Entry". Original BIFF parsing is github.com/extrame/xls v0.0.1.
package xls

import (
	"bytes"
	"fmt"
	"github.com/richardlehane/mscfb"
	"io"
	"os"
)

func Open(file, charset string) (*WorkBook, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return OpenReader(f, charset)
}
func OpenReader(reader io.ReadSeeker, charset string) (*WorkBook, error) {
	at, ok := reader.(io.ReaderAt)
	if !ok {
		data, err := io.ReadAll(io.LimitReader(reader, 64<<20+1))
		if err != nil {
			return nil, err
		}
		if len(data) > 64<<20 {
			return nil, fmt.Errorf("XLS exceeds 64 MiB")
		}
		at = bytes.NewReader(data)
	}
	doc, err := mscfb.New(at)
	if err != nil {
		return nil, err
	}
	for {
		entry, err := doc.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if entry.Name == "Workbook" || entry.Name == "Book" {
			data, err := io.ReadAll(io.LimitReader(doc, 64<<20+1))
			if err != nil {
				return nil, err
			}
			if len(data) > 64<<20 {
				return nil, fmt.Errorf("workbook stream exceeds 64 MiB")
			}
			return newWorkBookFromOle2(bytes.NewReader(data)), nil
		}
	}
	return nil, fmt.Errorf("Excel workbook stream is missing")
}
