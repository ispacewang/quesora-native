package main

import (
	"strings"
	"testing"
)

func TestAdaptiveQuestionSchema(t *testing.T) {
	rows := [][]string{{"培训题库"}, {"说明：题型 1=单选 2=多选 3=判断 4=填空 5=简答"}, {"试题编号", "试题内容（必填）", "题目类型", "选项A", "B选项", "选项C", "选项D", "正确答案（填选项编号，从1开始）", "答案解析", "是否重要"}, {"01", "谁是偶数？", "2", "1", "2", "3", "4", "2,4", "解释", "是"}, {"02", "地球是圆的", "3", "", "", "", "", "TRUE", "", "否"}, {"03", "1+1=__", "4", "无关", "", "", "", "2", "", ""}, {"04", "列出职责", "5", "", "", "", "", "内存管理；进程管理", "", ""}}
	qs, w, e := questionsFromRows("说明表", rows)
	if e != nil || len(w) != 0 || len(qs) != 4 {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
	if qs[0].Kind != "多选题" || qs[0].Answer != 10 || qs[0].Meta["isBaoMing"] != true || qs[0].Meta["来源行"] != 4 || qs[1].Answer != 1 || len(qs[2].Options) != 0 || qs[3].Kind != "简答题" {
		t.Fatalf("%+v", qs)
	}
}
func TestTwoRowAndSparseOptionLabels(t *testing.T) {
	rows := [][]string{{"题目", "选项", "", "", "参考答案"}, {"", "A", "B", "D", ""}, {"问题", "甲", "乙", "丁", "D"}, {"二", "甲", "乙", "丁", "乙"}}
	qs, w, e := questionsFromRows("Sheet", rows)
	if e != nil || len(w) != 0 || len(qs) != 2 || qs[0].Answer != 4 || qs[1].Answer != 2 {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
}
func TestSemanticImportDoesNotGuessInvalidAnswers(t *testing.T) {
	rows := [][]string{{"题干", "选项", "答案", "答案解析"}, {"有效", "A. 甲\nB. 乙", "B", "A是错的"}, {"不能扫描字母", "A. 甲|B. 乙", "Because it is right", ""}, {"越界", "甲|乙", "C", ""}, {"缺失", "甲|乙", "", ""}, {"题干", "选项", "答案", "答案解析"}}
	qs, w, e := questionsFromRows("Sheet", rows)
	if e != nil || len(qs) != 1 || len(w) != 3 || qs[0].Answer != 2 || !strings.Contains(w[0], "第 3 行") {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
}
func TestEnglishHeadersAndEmbeddedOptions(t *testing.T) {
	rows := [][]string{{"Question", "Correct Answer", "Analysis"}, {"Choose a color\nA. red\nB. green\nC. blue", "green", "good"}, {"Choose two", "A;C", ""}}
	qs, w, e := questionsFromRows("Sheet", rows)
	if e != nil || len(qs) != 1 || len(w) != 1 || qs[0].Kind != "单选题" || qs[0].Answer != 2 || qs[0].Prompt != "Choose a color" {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
}
func TestNumericZeroBasedAndReversedJudgment(t *testing.T) {
	rows := [][]string{{"题型", "题干", "选项", "答案（索引从0开始）"}, {"单选题", "问题", "甲|乙|丙", "0"}, {"判断题", "描述", "A. 错误|B. 正确", "正确"}}
	qs, w, e := questionsFromRows("Sheet", rows)
	if e != nil || len(w) != 0 || len(qs) != 2 || qs[0].Answer != 1 || qs[1].Answer != 2 {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
}

func TestDescriptiveHeadersAndTypeInstructions(t *testing.T) {
	rows := [][]string{{"格式说明：1表示单项选择；2表示多项选择"}, {"字段一（这里填写试题内容）", "字段二（题目类型：参照说明）", "字段三（填写选项，用分号分隔）", "字段四（标准答案，填写选项内容）", "字段五（答案解析）"}, {"有哪些偶数？", "2", "2;3;4;5", "2；4", "2和4"}}
	qs, w, e := questionsFromRows("培训题", rows)
	if e != nil || len(w) != 0 || len(qs) != 1 || qs[0].Kind != "多选题" || qs[0].Answer != 5 || qs[0].Explanation != "2和4" {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
}
func TestBlankAnswerOneIsNotInferredAsJudgment(t *testing.T) {
	qs, w, e := questionsFromRows("Sheet", [][]string{{"题干", "答案"}, {"填写 1-0=____", "1"}})
	if e != nil || len(w) != 0 || len(qs) != 1 || qs[0].Kind != "填空题" {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
}

func TestAmbiguousNumericAnswerIsReported(t *testing.T) {
	qs, w, e := questionsFromRows("Sheet", [][]string{{"题干", "选项", "答案"}, {"偶数", "2|3|4|5", "2"}})
	if e != nil || len(qs) != 0 || len(w) != 1 || !strings.Contains(w[0], "不能唯一") {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
}

func TestEnglishFreeAnswerAndGenericChoice(t *testing.T) {
	qs, w, e := questionsFromRows("English", [][]string{{"Question", "Answer"}, {"Which city is the capital of France?", "Paris"}})
	if e != nil || len(w) != 0 || len(qs) != 1 || qs[0].TextAnswer != "Paris" || qs[0].Kind != "简答题" {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
	qs, w, e = questionsFromRows("Sheet", [][]string{{"题型", "题干", "选项", "答案"}, {"选择题", "下列偶数", "2|3|4|5", "AC"}})
	if e != nil || len(w) != 0 || len(qs) != 1 || qs[0].Kind != "多选题" {
		t.Fatalf("%v %v %+v", e, w, qs)
	}
}
