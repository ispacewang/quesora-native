package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func templateFixture(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "测试题库.xlsx")
	if err := writeQuestionTemplate(p); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestTemplateFiveTypes(t *testing.T) {
	result, err := parseQuestionFile(templateFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Banks) != 1 || result.Banks[0].Name != "测试题库" {
		t.Fatalf("%+v", result)
	}
	qs := result.Banks[0].Questions
	if len(qs) != 5 || qs[0].Options[0] != "栈" || qs[0].Answer != 2 || qs[1].Answer != 5 || qs[2].Answer != 1 || qs[3].TextAnswer != "2" || qs[4].Kind != "简答题" || qs[0].Meta["isBaoMing"] != true {
		t.Fatalf("%+v", qs)
	}
}
func TestWorkbookSheets(t *testing.T) {
	p := templateFixture(t)
	book, err := excelize.OpenFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"第二套", "说明", "空表"} {
		if _, err = book.NewSheet(name); err != nil {
			t.Fatal(err)
		}
	}
	for i, row := range [][]any{{"题干", "题型", "选项", "答案", "说明"}, {"第二套问题", "多选", "A、甲|B. 乙|C）丙", "a， c", "解释"}} {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err = book.SetSheetRow("第二套", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	row := []any{"这里是说明文本"}
	book.SetSheetRow("说明", "A1", &row)
	if err = book.Save(); err != nil {
		t.Fatal(err)
	}
	book.Close()
	result, err := parseQuestionFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Banks) != 2 || result.Banks[0].Name != "测试题库-Sheet1" || result.Banks[1].Name != "测试题库-第二套" {
		t.Fatalf("%+v", result)
	}
	q := result.Banks[1].Questions[0]
	if q.Answer != 5 || q.Explanation != "解释" || q.Options[2] != "丙" || len(result.Warnings) != 1 {
		t.Fatalf("%+v %+v", q, result.Warnings)
	}
}
func TestCSVEncodingsAndQuotedNewline(t *testing.T) {
	csv := "题型,题干,选项,答案,解析\r\n单选题,\"第一行\n第二行\",A. 甲|B. 乙,B,说明\r\n"
	gb, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	utf16, _, err := transform.Bytes(unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder(), []byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"utf8": []byte(csv), "bom": append([]byte{239, 187, 191}, []byte(csv)...), "gb18030": gb, "utf16": utf16} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "题库.csv")
			os.WriteFile(p, data, 0600)
			result, err := parseQuestionFile(p)
			if err != nil {
				t.Fatal(err)
			}
			q := result.Banks[0].Questions[0]
			if q.Prompt != "第一行\n第二行" || q.Answer != 2 {
				t.Fatalf("%+v", q)
			}
		})
	}
}
func TestLegacyChineseXLS(t *testing.T) {
	result, err := parseQuestionFile(filepath.Join("testdata", "legacy.xls"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Banks) != 1 || len(result.Banks[0].Questions) != 2 {
		t.Fatalf("%+v", result)
	}
	q := result.Banks[0].Questions[0]
	if q.Prompt != "中文旧版 Excel 能否导入？" || q.Answer != 1 || q.Options[0] != "可以" {
		t.Fatalf("%+v", q)
	}
}
func TestInvalidFiles(t *testing.T) {
	for _, name := range []string{"bad.xlsx", "bad.xls", "bad.csv", "bad.txt"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name)
			os.WriteFile(p, []byte("not a question file"), 0600)
			if _, err := parseQuestionFile(p); err == nil {
				t.Fatal("invalid file accepted")
			}
		})
	}
	p := filepath.Join(t.TempDir(), "large.csv")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(maxImportBytes + 1)
	f.Close()
	if _, err = parseQuestionFile(p); err == nil {
		t.Fatal("oversized file accepted")
	}
}
func TestStoreReopenRollbackAndDelete(t *testing.T) {
	p := templateFixture(t)
	result, err := parseQuestionFile(p)
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "data", "questions.db")
	s, err := openQuestionStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.importBanks(result); err != nil {
		t.Fatal(err)
	}
	qs, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.saveResponse(qs[0].ID, response{Choice: 2, Correct: true, OptionOrder: []int{3, 0, 1, 2}}); err != nil {
		t.Fatal(err)
	}
	batch := importResult{Banks: []importedBank{{Name: "应回滚的新题库", Questions: result.Banks[0].Questions}, result.Banks[0]}}
	if err = s.importBanks(batch); err == nil {
		t.Fatal("duplicate accepted")
	}
	s.close()
	s, err = openQuestionStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	qs, err = s.load()
	if err != nil || len(qs) != 5 {
		t.Fatalf("rollback/reopen: %v %d", err, len(qs))
	}
	answers, _, err := s.loadPractice()
	if err != nil || !answers[qs[0].ID].Correct || responseLetters(qs[0], answers[qs[0].ID], qs[0].Answer) != "C" {
		t.Fatalf("practice: %v %+v", err, answers)
	}
	if err = s.deleteBank(result.Banks[0].Name); err != nil {
		t.Fatal(err)
	}
	qs, err = s.load()
	if err != nil || len(qs) != 0 {
		t.Fatal("delete", err)
	}
	answers, _, err = s.loadPractice()
	if err != nil || len(answers) != 0 {
		t.Fatal("cascade", err)
	}
	if _, err = os.Stat(p); err != nil {
		t.Fatal("source file was removed")
	}
}
func TestShortAnswerScoring(t *testing.T) {
	for _, tc := range []struct {
		user, standard string
		want           bool
	}{
		{"内存管理和进程管理", "进程管理；内存管理；文件管理", true},
		{"只有进程管理", "进程管理；内存管理；文件管理", false},
		{"apple banana", "apple;banana;cherry;date", false},
		{"apple banana cherry", "apple;banana;cherry;date", true},
		{"TCP 使用可靠传输", "参考答案：TCP 使用可靠传输", true},
		{"", "正确答案", false},
	} {
		if got := judgeShortAnswer(tc.user, tc.standard); got != tc.want {
			t.Errorf("%q: %v want %v", tc.user, got, tc.want)
		}
	}
	if normalizeAnswer(" ＡＢＣ，１２３。 ") != "abc123" {
		t.Fatal("text normalization")
	}
}
func TestUploadUIEndToEnd(t *testing.T) {
	a := newApp()
	a.questions = nil
	a.bank = ""
	var err error
	a.store, err = openQuestionStore(filepath.Join(t.TempDir(), "questions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.store.close()
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	updates := make(chan func(), 64)
	a.dispatch = func(fn func()) { updates <- fn }
	source := templateFixture(t)
	p := strings.TrimSuffix(source, ".xlsx") + ".xls"
	if err := os.Rename(source, p); err != nil {
		t.Fatal(err)
	}
	a.pickFile = func() ([]string, error) { return []string{p}, nil }
	tt := ui.NewTester(a.view, 1450, 860)
	pump := func() {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for a.importBusy {
			select {
			case fn := <-updates:
				fn()
				tt.Frame()
			case <-deadline:
				t.Fatal("import timeout")
			}
		}
	}
	click(t, tt, "上传题库")
	click(t, tt, "选择题库文件")
	pump()
	if a.importError || len(a.questions) != 5 || !tt.HasText("导入成功：1 个题库，共 5 道题。") {
		t.Fatalf("%s, %d", a.importMessage, len(a.questions))
	}
	click(t, tt, "关闭")
	click(t, tt, "B. 队列")
	click(t, tt, "提交答案")
	if good, _ := a.counts(); good != 1 {
		t.Fatal("imported question not playable")
	}
	click(t, tt, "筛选简答题")
	click(t, tt, "填写答案")
	tt.Type("内存管理；进程管理")
	click(t, tt, "提交答案")
	if good, _ := a.counts(); good != 2 {
		t.Fatal("imported short answer not playable")
	}
	a.startImport([]string{p})
	pump()
	if !a.importError || !strings.Contains(a.importMessage, "已存在") || len(a.questions) != 5 {
		t.Fatal("duplicate handling", a.importMessage)
	}
	a.importOpen = false
	tt.Frame()
	click(t, tt, "删除题库：测试题库")
	click(t, tt, "确认删除")
	pump()
	if len(a.questions) != 0 || !tt.HasText("选择一个题库开始答题") {
		t.Fatal("delete did not update view")
	}
	a.pickFile = func() ([]string, error) { return nil, nil }
	click(t, tt, "上传题库")
	click(t, tt, "选择题库文件")
	pump()
	if a.importError || a.importMessage != "" {
		t.Fatal("cancel was treated as an error")
	}
}
