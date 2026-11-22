package domain

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// AnswerMask 解码仓库中已规范化的选择答案；原始表格的歧义判定由导入模块负责。
func AnswerMask(answer, kind string, options []string) uint {
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
func IsTextQuestion(q Question) bool {
	return q.Kind == "填空题" || q.Kind == "简答题" || len(q.Options) == 0
}

// ResponseOrder 校验历史选项映射是完整排列；缺失或损坏时回退原顺序，防止越界读取。
func ResponseOrder(q Question, r Response) []int {
	if len(r.OptionOrder) == len(q.Options) {
		seen := map[int]bool{}
		valid := true
		for _, i := range r.OptionOrder {
			if i < 0 || i >= len(q.Options) || seen[i] {
				valid = false
				break
			}
			seen[i] = true
		}
		if valid {
			return r.OptionOrder
		}
	}
	order := make([]int, len(q.Options))
	for i := range order {
		order[i] = i
	}
	return order
}

func ResponseLetters(q Question, r Response, mask uint) string {
	var displayed uint
	for i, original := range ResponseOrder(q, r) {
		if mask&(1<<original) != 0 {
			displayed |= 1 << i
		}
	}
	return Letters(displayed)
}

// JudgeQuestion 共用于练习和考试：选择题要求掩码完全匹配，填空归一化匹配，简答使用本地要点规则。
func JudgeQuestion(q Question, r Response) bool {
	if IsTextQuestion(q) {
		if strings.TrimSpace(r.Text) == "" {
			return false
		}
		if q.Kind == "简答题" {
			return JudgeShortAnswer(r.Text, q.TextAnswer)
		}
		return NormalizeAnswer(r.Text) == NormalizeAnswer(q.TextAnswer)
	}
	return r.Choice != 0 && q.Answer != 0 && r.Choice == q.Answer
}

// CanonicalKind 将可识别的中英文题型名称映射为五种标准题型，未知名称返回空串。
func CanonicalKind(s string) string {
	n := strings.ToLower(strings.TrimSpace(norm.NFKC.String(s)))
	for _, v := range []struct {
		kind    string
		aliases []string
	}{{"多选题", []string{"多选", "多项选择", "不定项", "multiple", "multi"}}, {"单选题", []string{"单选", "单项选择", "single"}}, {"判断题", []string{"判断", "是非", "true/false", "boolean", "judge"}}, {"填空题", []string{"填空", "fill", "blank"}}, {"简答题", []string{"简答", "问答", "论述", "解答", "essay", "short"}}} {
		for _, alias := range v.aliases {
			if strings.Contains(n, alias) {
				return v.kind
			}
		}
	}
	return ""
}

// Letters 将位掩码转换为按原始选项顺序排列的字母，最多支持 A–Z。
func Letters(mask uint) string {
	var s []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) != 0 {
			s = append(s, string(rune('A'+i)))
		}
	}
	return strings.Join(s, "、")
}
