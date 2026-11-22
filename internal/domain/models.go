package domain

// Question 是导入、练习、考试和持久化共用的标准题目。
// Answer 的第 i 位对应 Options[i]，不是打乱后界面上显示的字母。
type Question struct {
	ID, Bank, Kind, Prompt  string
	Options                 []string
	Answer                  uint
	TextAnswer, Explanation string
	Graph                   bool
	Meta                    map[string]any
}

// Response 保存一次提交及其显示顺序；Choice 使用原始选项位掩码。
// OptionOrder 使历史回顾和导出能重现提交时的选项字母。
type Response struct {
	OptionOrder   []int `json:",omitempty"`
	Choice        uint
	Text          string
	AIExplanation string `json:",omitempty"`
	Note          string `json:",omitempty"`
	Correct       bool
}
