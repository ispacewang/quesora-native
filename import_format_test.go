package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkbookContentFormatOverridesExtension(t *testing.T) {
	xlsx, err := os.ReadFile(templateFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := os.ReadFile(filepath.Join("testdata", "legacy.xls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		count   int
		warning bool
	}{
		{"现代题库.xls", xlsx, 5, true},
		{"现代题库.XLSX", xlsx, 5, false},
		{"旧版题库.xlsx", legacy, 2, true},
		{"旧版题库.XLS", legacy, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(p, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			result, err := parseQuestionFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Banks) != 1 || len(result.Banks[0].Questions) != tc.count {
				t.Fatalf("unexpected result: %+v", result)
			}
			q := result.Banks[0].Questions[0]
			if q.Bank != strings.TrimSuffix(tc.name, filepath.Ext(tc.name)) || q.Answer == 0 {
				t.Fatalf("invalid bank or answer: %+v", q)
			}
			if tc.warning && (len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, " "), "扩展名")) {
				t.Fatal("format mismatch not explained")
			}
			after, err := os.ReadFile(p)
			if err != nil || !bytes.Equal(after, tc.data) {
				t.Fatal("source file changed", err)
			}
		})
	}
}

func TestBrokenXLSXNamedXLSDoesNotReachLegacyParser(t *testing.T) {
	p := filepath.Join(t.TempDir(), "损坏.xls")
	// The first 8 bytes match the user's reported bad-signature value 43016997712.
	if err := os.WriteFile(p, []byte{'P', 'K', 3, 4, 10, 0, 0, 0}, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := parseQuestionFile(p)
	if err == nil || strings.Contains(err.Error(), "mscfb") || strings.Contains(err.Error(), "XLS 文件") {
		t.Fatalf("wrong parser selected: %v", err)
	}
}
