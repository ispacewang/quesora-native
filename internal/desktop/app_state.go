// 按功能分组的窗口状态。所有可变字段归 UI 线程，不将整组对象传给后台任务。
package desktop

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/ispacewang/quesora-native/internal/domain"
)

// appearanceState 保存外观选择和系统主题订阅；关闭窗口时释放 themeOff。
type appearanceState struct {
	rounded    bool
	appearance string
	systemDark func() bool
	themeOff   func()
	dark       bool
}

// settingsState 保存设置、备份与恢复对话框；业务数据只在 restorableState 中传递。
type settingsState struct {
	restoreOpen    bool
	restoreBusy    bool
	restorePicking bool
	restorePath    string
	pickRestore    func() (string, error)
	settingsOpen   bool
	backupBusy     bool
	backupError    bool
	backupMessage  string
	pickBackup     func() (string, error)
	showDataFolder func() error
}

// practiceState 保存当前练习轮次、筛选和最近提交；不包含独立考试的作答。
type practiceState struct {
	repeatPractice       bool
	resetPracticeOpen    bool
	attemptAnsweredID    string
	reviewedQuestionID   string
	quickFeedbackPending bool
	quickAdvanceAt       time.Time
	practiceRevision     uint64
	answerHandle         ui.Handle
	importantMotion      importantMotion
	filters              map[string]bool
	importantOnly        bool
	random               bool
	quick                bool
	shuffle              bool
	showPreview          bool
	finishOpen           bool
	order                []string
	optionOrder          []int
	bank                 string
	filter               string
	index                int
	chosen               uint
	typed                string
	answers              map[string]domain.Response
	starred              map[string]bool
	leftOpen             bool
	rightOpen            bool
	onlyWrong            bool
	notice               string
	lastPreferences      string
}

// mistakeState 保存错题备注草稿和导出/保存状态；历史错题本持有独立快照。
type mistakeState struct {
	mistakeNote       string
	mistakeNoteID     string
	mistakeNoteError  string
	mistakeNoteSaving bool
	savedWrong        bool
	wrongSaving       bool
	exportOpen        bool
}

// examState 保存考试表单、当前试卷和 PDF 缓存；题序和选项映射在组卷时固定。
type examState struct {
	exam           *examSession
	examForm       examSettings
	examSetupOpen  bool
	examSubmitOpen bool
	examExitOpen   bool
	examSaving     bool
	examMessage    string
	paperBusy      bool
	paper          *examSession
	paperSettings  examSettings
}

// historyState 保存历史列表和删除确认，不复用当前考试会话。
type historyState struct {
	historyOpen       bool
	historyLoading    bool
	historyDeleteOpen bool
	historyMessage    string
	historyDeleteKey  string
	historyExpanded   string
	history           []historyRecord
}

// libraryState 保存题库内容及导入/删除界面状态；文件与 SQL 操作交给串行任务。
type libraryState struct {
	questions      []domain.Question
	pickFile       func() ([]string, error)
	importOpen     bool
	importBusy     bool
	importError    bool
	deleteOpen     bool
	importMessage  string
	libraryStatus  string
	deleteName     string
	importWarnings []string
}
