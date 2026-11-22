// 练习领域模型和 UI 线程状态。选择答案按原始选项索引存储，显示顺序只在视图边界转换。

package desktop

import (
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/ispacewang/quesora-native/internal/domain"
)

// app 持有窗口生命周期内的模型、控件状态和稳定句柄。
// 生产环境的字段变更在 UI 线程进行；后台任务只能经 publish 发布结果。
type app struct {
	rounded                                                        bool
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
	quickFeedbackPending                                           bool
	quickAdvanceAt                                                 time.Time
	mistakeNote, mistakeNoteID, mistakeNoteError                   string
	practiceRevision                                               uint64
	mistakeNoteSaving                                              bool
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
	questions                                                      []domain.Question
	bank, filter                                                   string
	index                                                          int
	chosen                                                         uint
	typed                                                          string
	answers                                                        map[string]domain.Response
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
	return &app{appearance: "system", ai: newAIState(), questions: demoQuestions(), bank: "数学基础", filter: "全部", answers: map[string]domain.Response{}, starred: map[string]bool{}, leftOpen: true, rightOpen: true}
}

// visible 根据当前题库和组合筛选创建新切片，并应用本轮题序，不重排仓库原始数据。
func (a *app) visible() []domain.Question {
	var out []domain.Question
	for _, q := range a.questions {
		if q.Bank != a.bank || (a.filter != "全部" && q.Kind != a.filter) || (len(a.filters) > 0 && !a.filters[q.Kind]) || (a.importantOnly && !a.starred[q.ID]) {
			continue
		}
		if a.onlyWrong {
			r, ok := a.answers[q.ID]
			if !ok || r.Correct && q.ID != a.attemptAnsweredID {
				continue
			}
		}
		out = append(out, q)
	}
	a.sortQuestions(out)
	return out
}
func (a *app) current() (domain.Question, bool) {
	qs := a.visible()
	if len(qs) == 0 {
		return domain.Question{}, false
	}
	return qs[min(max(a.index, 0), len(qs)-1)], true
}

// navigate 开始一次新的题目访问，清空临时输入/动画并按重新作答设置恢复旧答案。
func (a *app) navigate(index int) {
	a.stopQuickAdvance()
	a.mistakeNoteID, a.mistakeNote, a.mistakeNoteError = "", "", ""
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

// submit 拒绝空答案和重复提交，更新每题最近记录；速刷先显示判题结果。
func (a *app) submit() {
	q, ok := a.current()
	if !ok {
		return
	}
	if _, done := a.currentResponse(q); done {
		return
	}
	if !domain.IsTextQuestion(q) && a.chosen == 0 || domain.IsTextQuestion(q) && strings.TrimSpace(a.typed) == "" {
		a.notice = "请先作答，再提交答案。"
		return
	}
	if a.ai.Config.Enabled && a.ai.Config.AutoJudge {
		a.submitAIAnswer(q, domain.Response{Choice: a.chosen, Text: a.typed, OptionOrder: append([]int(nil), a.displayOptions(q)...)})
		return
	}
	r := domain.Response{Choice: a.chosen, Text: a.typed, OptionOrder: append([]int(nil), a.displayOptions(q)...)}
	r.Correct = domain.JudgeQuestion(q, r)
	a.recordPracticeAnswer(q, r)
}

// recordPracticeAnswer 共用本地与 AI 判题的提交路径。先确认当前题，避免错题筛选在判对后立即移除它。
func (a *app) recordPracticeAnswer(q domain.Question, r domain.Response) {
	current, ok := a.current()
	r.Note = a.answers[q.ID].Note
	a.answers[q.ID] = r
	a.saveResponse(q.ID, r)
	a.savedWrong = false
	if !ok || current.ID != q.ID {
		return
	}
	a.attemptAnsweredID = q.ID
	a.reviewedQuestionID = ""
	a.notice = ""
	a.quickFeedbackPending = a.quick
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

func demoQuestions() []domain.Question {
	return []domain.Question{
		{ID: "m1", Bank: "数学基础", Kind: "单选题", Prompt: "已知函数 f(x) = x² − 2x + 1，则 f(x) 的最小值为：", Options: []string{"−1", "0", "1", "2"}, Answer: 2, TextAnswer: "", Explanation: "将函数配方：f(x) = (x − 1)²。平方恒不小于 0，当 x = 1 时取到最小值 0。", Graph: false, Meta: nil},
		{ID: "m2", Bank: "数学基础", Kind: "多选题", Prompt: "下列关于函数 y = x² 的说法，正确的有哪些？", Options: []string{"图像关于 y 轴对称", "在整个实数范围内单调递增", "最小值为 0", "图像经过点 (2, 4)"}, Answer: 13, TextAnswer: "", Explanation: "y = x² 是偶函数；在负半轴递减、正半轴递增，最小值为 0，且 2² = 4。", Graph: false, Meta: nil},
		{ID: "m3", Bank: "数学基础", Kind: "判断题", Prompt: "若 a > b，则对于任意实数 c，均有 ac > bc。", Options: []string{"正确", "错误"}, Answer: 2, TextAnswer: "", Explanation: "当 c < 0 时，不等号方向反转；当 c = 0 时，两边相等。", Graph: false, Meta: nil},
		{ID: "m4", Bank: "数学基础", Kind: "填空题", Prompt: "计算：∫₀¹ 2x dx = ____。", Options: nil, Answer: 0, TextAnswer: "1", Explanation: "2x 的一个原函数是 x²，因此定积分为 1² − 0² = 1。", Graph: false, Meta: nil},
		{ID: "m5", Bank: "数学基础", Kind: "单选题", Prompt: "从 1 到 6 的六个整数中随机选一个，选到质数的概率为：", Options: []string{"1/6", "1/3", "1/2", "2/3"}, Answer: 4, TextAnswer: "", Explanation: "质数是 2、3、5，共 3 个。概率为 3/6 = 1/2。", Graph: false, Meta: nil},
		{ID: "m6", Bank: "数学基础", Kind: "单选题", Prompt: "等差数列 2，5，8，11，… 的第 10 项是：", Options: []string{"26", "27", "29", "32"}, Answer: 4, TextAnswer: "", Explanation: "首项为 2，公差为 3。第 10 项 a₁₀ = 2 + 9 × 3 = 29。", Graph: false, Meta: nil},
		{ID: "c1", Bank: "计算机基础", Kind: "单选题", Prompt: "下列哪一种数据结构遵循“先进先出”的原则？", Options: []string{"栈", "队列", "二叉树", "哈希表"}, Answer: 2, TextAnswer: "", Explanation: "队列从队尾入队、从队首出队，遵循先进先出原则。", Graph: false, Meta: nil},
		{ID: "c2", Bank: "计算机基础", Kind: "多选题", Prompt: "下列哪些属于操作系统的主要职责？", Options: []string{"进程调度", "内存管理", "文件管理", "自动生成所有业务代码"}, Answer: 7, TextAnswer: "", Explanation: "操作系统负责进程、内存、文件及设备等资源的管理。", Graph: false, Meta: nil},
		{ID: "c3", Bank: "计算机基础", Kind: "判断题", Prompt: "二分查找通常要求待查找的数据已经有序。", Options: []string{"正确", "错误"}, Answer: 1, TextAnswer: "", Explanation: "二分查找利用顺序关系排除一半区间，因此通常需要有序数据。", Graph: false, Meta: nil},
	}
}

func roundedPercent(part, total int) int {
	if total <= 0 {
		return 0
	}
	return (part*100 + total/2) / total
}
