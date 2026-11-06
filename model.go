// 练习领域模型和 UI 线程状态。选择答案按原始选项索引存储，显示顺序只在视图边界转换。

package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"strings"
)

// question 是导入、练习、考试和持久化共用的标准题目。
// Answer 的第 i 位对应 Options[i]，不是打乱后界面上显示的字母。
type question struct {
	ID, Bank, Kind, Prompt  string
	Options                 []string
	Answer                  uint
	TextAnswer, Explanation string
	Graph                   bool
	Meta                    map[string]any
}

// response 保存一次提交及其显示顺序；Choice 使用原始选项位掩码。
// OptionOrder 使历史回顾和导出能重现提交时的选项字母。
type response struct {
	OptionOrder   []int `json:",omitempty"`
	Choice        uint
	Text          string
	AIExplanation string `json:",omitempty"`
	Correct       bool
}

// app 持有窗口生命周期内的模型、控件状态和稳定句柄。
// 生产环境的字段变更在 UI 线程进行；后台任务只能经 publish 发布结果。
type app struct {
	appearance                                                     string
	systemDark                                                     func() bool
	themeOff                                                       func()
	ai                                                             aiState
	updates                                                        updateState
	restoreOpen, restoreBusy, restorePicking                       bool
	restorePath                                                    string
	pickRestore                                                    func() (string, error)
	settingsOpen, backupBusy, backupError                          bool
	backupMessage, lastPreferences                                 string
	pickBackup                                                     func() (string, error)
	showDataFolder                                                 func() error
	repeatPractice                                                 bool
	resetPracticeOpen                                              bool
	attemptAnsweredID, reviewedQuestionID                          string
	answerHandle                                                   ui.Handle
	importantMotion                                                importantMotion
	exam                                                           *examSession
	examForm                                                       examSettings
	examSetupOpen, examSubmitOpen, examExitOpen, examSaving        bool
	examMessage                                                    string
	historyOpen, historyLoading, historyDeleteOpen                 bool
	historyMessage, historyDeleteKey, historyExpanded              string
	history                                                        []historyRecord
	paperBusy                                                      bool
	paper                                                          *examSession
	paperSettings                                                  examSettings
	toastLast                                                      string
	savedWrong, wrongSaving, exportOpen                            bool
	filters                                                        map[string]bool
	importantOnly, random, quick, shuffle, showPreview, finishOpen bool
	order                                                          []string
	optionOrder                                                    []int
	questions                                                      []question
	bank, filter                                                   string
	index                                                          int
	chosen                                                         uint
	typed                                                          string
	answers                                                        map[string]response
	starred                                                        map[string]bool
	dark, leftOpen, rightOpen, onlyWrong                           bool
	notice                                                         string
	helpOpen                                                       bool
	win                                                            *mygo.Window
	store                                                          *questionStore
	jobs                                                           *libraryJobs
	dispatch                                                       func(func())
	pickFile                                                       func() ([]string, error)
	closed, importOpen, importBusy, importError, deleteOpen        bool
	importMessage, libraryStatus, deleteName                       string
	importWarnings                                                 []string
}

// newApp 提供离屏测试的示例初态；main 在真实启动时用本地题库替换示例。
func newApp() *app {
	loadNativeAssets()
	return &app{appearance: "system", ai: newAIState(), questions: demoQuestions(), bank: "数学基础", filter: "全部", answers: map[string]response{}, starred: map[string]bool{}, leftOpen: true, rightOpen: true}
}

// visible 根据当前题库和组合筛选创建新切片，并应用本轮题序，不重排仓库原始数据。
func (a *app) visible() []question {
	var out []question
	for _, q := range a.questions {
		if q.Bank != a.bank || (a.filter != "全部" && q.Kind != a.filter) || (len(a.filters) > 0 && !a.filters[q.Kind]) || (a.importantOnly && !a.starred[q.ID]) {
			continue
		}
		if a.onlyWrong {
			r, ok := a.answers[q.ID]
			if !ok || r.Correct {
				continue
			}
		}
		out = append(out, q)
	}
	a.sortQuestions(out)
	return out
}
func (a *app) current() (question, bool) {
	qs := a.visible()
	if len(qs) == 0 {
		return question{}, false
	}
	return qs[min(max(a.index, 0), len(qs)-1)], true
}

// navigate 开始一次新的题目访问，清空临时输入/动画并按重新作答设置恢复旧答案。
func (a *app) navigate(index int) {
	if a.ai.Busy && a.ai.Task == "answer" {
		a.cancelAITask()
	}
	a.importantMotion = importantMotion{}
	a.attemptAnsweredID = ""
	a.reviewedQuestionID = ""
	a.index = min(max(index, 0), max(0, len(a.visible())-1))
	a.chosen = 0
	a.typed = ""
	a.notice = ""
	a.showPreview = false
	a.prepareOptions()
	if !a.repeatPractice {
		a.restoreCurrentAnswer()
	}
	a.persistPreferences()
}

// choose 接收原始选项索引；多选按位切换，单选覆盖当前选择。
func (a *app) choose(i int) {
	q, ok := a.current()
	if !ok || i < 0 || i >= len(q.Options) {
		return
	}
	if _, done := a.currentResponse(q); done {
		return
	}
	if q.Kind == "多选题" {
		a.chosen ^= 1 << i
	} else {
		a.chosen = 1 << i
	}
	a.notice = ""
}

// submit 拒绝空答案和重复提交，更新每题最近记录，并在速刷模式切到下一题。
func (a *app) submit() {
	q, ok := a.current()
	if !ok {
		return
	}
	if _, done := a.currentResponse(q); done {
		return
	}
	if !isTextQuestion(q) && a.chosen == 0 || isTextQuestion(q) && strings.TrimSpace(a.typed) == "" {
		a.notice = "请先作答，再提交答案。"
		return
	}
	if a.ai.Config.Enabled && a.ai.Config.AutoJudge {
		a.submitAIAnswer(q, response{Choice: a.chosen, Text: a.typed, OptionOrder: append([]int(nil), a.displayOptions(q)...)})
		return
	}
	correct := judgeQuestion(q, response{Choice: a.chosen, Text: a.typed})
	a.savedWrong = false
	a.answers[q.ID] = response{Choice: a.chosen, Text: a.typed, Correct: correct, OptionOrder: append([]int(nil), a.displayOptions(q)...)}
	a.attemptAnsweredID = q.ID
	a.reviewedQuestionID = ""
	a.saveResponse(q.ID, a.answers[q.ID])
	if a.quick {
		a.nextQuestion()
	}
	a.notice = ""
}

// counts 统计当前题库的最近提交结果；未答题不计入正确或错误数量。
func (a *app) counts() (correct, wrong int) {
	for _, q := range a.questions {
		if q.Bank != a.bank {
			continue
		}
		if r, ok := a.answers[q.ID]; ok {
			if r.Correct {
				correct++
			} else {
				wrong++
			}
		}
	}
	return
}

// letters 将位掩码转换为按原始选项顺序排列的字母，最多支持 A–Z。
func letters(mask uint) string {
	var s []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) != 0 {
			s = append(s, string(rune('A'+i)))
		}
	}
	return strings.Join(s, "、")
}
func demoQuestions() []question {
	return []question{
		{"m1", "数学基础", "单选题", "已知函数 f(x) = x² − 2x + 1，则 f(x) 的最小值为：", []string{"−1", "0", "1", "2"}, 2, "", "将函数配方：f(x) = (x − 1)²。平方恒不小于 0，当 x = 1 时取到最小值 0。", false, nil},
		{"m2", "数学基础", "多选题", "下列关于函数 y = x² 的说法，正确的有哪些？", []string{"图像关于 y 轴对称", "在整个实数范围内单调递增", "最小值为 0", "图像经过点 (2, 4)"}, 13, "", "y = x² 是偶函数；在负半轴递减、正半轴递增，最小值为 0，且 2² = 4。", false, nil},
		{"m3", "数学基础", "判断题", "若 a > b，则对于任意实数 c，均有 ac > bc。", []string{"正确", "错误"}, 2, "", "当 c < 0 时，不等号方向反转；当 c = 0 时，两边相等。", false, nil},
		{"m4", "数学基础", "填空题", "计算：∫₀¹ 2x dx = ____。", nil, 0, "1", "2x 的一个原函数是 x²，因此定积分为 1² − 0² = 1。", false, nil},
		{"m5", "数学基础", "单选题", "从 1 到 6 的六个整数中随机选一个，选到质数的概率为：", []string{"1/6", "1/3", "1/2", "2/3"}, 4, "", "质数是 2、3、5，共 3 个。概率为 3/6 = 1/2。", false, nil},
		{"m6", "数学基础", "单选题", "等差数列 2，5，8，11，… 的第 10 项是：", []string{"26", "27", "29", "32"}, 4, "", "首项为 2，公差为 3。第 10 项 a₁₀ = 2 + 9 × 3 = 29。", false, nil},
		{"c1", "计算机基础", "单选题", "下列哪一种数据结构遵循“先进先出”的原则？", []string{"栈", "队列", "二叉树", "哈希表"}, 2, "", "队列从队尾入队、从队首出队，遵循先进先出原则。", false, nil},
		{"c2", "计算机基础", "多选题", "下列哪些属于操作系统的主要职责？", []string{"进程调度", "内存管理", "文件管理", "自动生成所有业务代码"}, 7, "", "操作系统负责进程、内存、文件及设备等资源的管理。", false, nil},
		{"c3", "计算机基础", "判断题", "二分查找通常要求待查找的数据已经有序。", []string{"正确", "错误"}, 1, "", "二分查找利用顺序关系排除一半区间，因此通常需要有序数据。", false, nil},
	}
}

func roundedPercent(part, total int) int {
	if total <= 0 {
		return 0
	}
	return (part*100 + total/2) / total
}
