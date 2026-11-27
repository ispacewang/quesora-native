package importer

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ispacewang/quesora-native/internal/diagram"
)

func TestSpreadsheetDiagramColumnsRetainValidFigures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "题图.csv")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := csv.NewWriter(file)
	err = w.WriteAll([][]string{
		{"题型", "题干", "选项", "答案", "配图", "配图 SVG"},
		{"单选题", "看一次函数图", "A. 甲|B. 乙", "A", `{"elements":[{"type":"functiongraph","attrs":["x+1"]}]}`, ""},
		{"单选题", "看几何图", "A. 甲|B. 乙", "B", "", `<svg viewBox="0 0 30 30"><circle cx="15" cy="15" r="5"/></svg>`},
		{"单选题", "保留题目", "A. 甲|B. 乙", "B", "", `<svg><image href="remote.png"/></svg>`},
	})
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	result, err := ParseFile(path)
	if err != nil || len(result.Banks) != 1 || len(result.Banks[0].Questions) != 3 {
		t.Fatal(result, err)
	}
	for _, q := range result.Banks[0].Questions[:2] {
		if svg, err := diagram.FromMeta(q.Meta); err != nil || svg == "" {
			t.Fatal("figure lost", err)
		}
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "SVG 配图") {
		t.Fatal("missing import warning")
	}
}
