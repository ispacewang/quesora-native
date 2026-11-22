package desktop

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/ispacewang/quesora-native/internal/domain"
)

func TestNativeGeometry(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1450, 800)
	for _, label := range []string{"统计侧栏", "答题主区域", "错题侧栏", "题卡", "题干", "选项组", "A. −1", "提交答案", "切换统计面板"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal(label)
		}
		t.Logf("%s: %+v", label, r)
	}
	for _, item := range []struct {
		label string
		x, w  float32
	}{{"统计侧栏", 0, 260}, {"答题主区域", 260, 850}, {"错题侧栏", 1110, 340}, {"题卡", 285, 800}, {"A. −1", 301, 768}} {
		r, _ := tt.Find(item.label)
		if math.Abs(float64(r.X-item.x)) > 1 || math.Abs(float64(r.W-item.w)) > 1 {
			t.Errorf("%s bounds %+v", item.label, r)
		}
	}
}
func TestFilterAndOptionMapping(t *testing.T) {
	a := newApp()
	a.toggleFilter("单选题")
	a.toggleFilter("多选题")
	kinds := map[string]bool{}
	for _, q := range a.visible() {
		kinds[q.Kind] = true
	}
	if len(kinds) != 2 || !kinds["单选题"] || !kinds["多选题"] {
		t.Fatal("type union", kinds)
	}
	a.toggleFilter("全部")
	a.starred["m2"] = true
	a.toggleFilter("重要")
	if qs := a.visible(); len(qs) != 1 || qs[0].ID != "m2" {
		t.Fatal("important filter")
	}
	a.toggleFilter("全部")
	a.navigate(0)
	a.optionOrder = []int{3, 2, 0, 1}
	q, _ := a.current()
	if a.displayAnswer(q) != "D" {
		t.Fatal("answer not mapped")
	}
	a.choose(a.displayOptions(q)[3])
	a.submit()
	if !a.answers[q.ID].Correct {
		t.Fatal("displayed choice grading failed")
	}
}
func TestRandomCycleAndQuickMode(t *testing.T) {
	a := newApp()
	a.random = true
	a.reorder()
	seen := map[string]bool{}
	for i := range a.visible() {
		a.navigate(i)
		q, _ := a.current()
		if seen[q.ID] {
			t.Fatal("random repeated")
		}
		seen[q.ID] = true
	}
	if len(seen) != len(a.visible()) {
		t.Fatal("random skipped")
	}
	a.random = false
	a.order = nil
	a.navigate(0)
	a.quick = true
	a.choose(1)
	a.submit()
	if a.index != 0 || !a.answers["m1"].Correct {
		t.Fatal("quick mode did not save and hold the answer")
	}
	a.navigate(len(a.visible()) - 1)
	a.nextQuestion()
	if !a.finishOpen {
		t.Fatal("missing end of cycle")
	}
}

func TestQuickKeyboardDoesNotStealTextInput(t *testing.T) {
	a := newApp()
	a.quick = true
	tt := ui.NewTester(a.view, 1450, 800)
	tt.Key(0, ui.KeyB)
	tt.Key(0, ui.KeySpace)
	if !a.answers["m1"].Correct || a.index != 0 {
		t.Fatal("quick keyboard choice should display its result")
	}
	tt.Advance(quickFeedbackDuration)
	if a.index != 1 {
		t.Fatal("quick keyboard choice did not advance")
	}
	a.filter = "填空题"
	a.filters = nil
	a.navigate(0)
	tt.Frame()
	click(t, tt, "填写答案")
	tt.Type("1")
	tt.Key(0, ui.KeySpace)
	if len(a.answers) != 1 {
		t.Fatal("space submitted while editing text")
	}
}
func TestTextQuestionWithExtraneousOptions(t *testing.T) {
	a := newApp()
	a.questions = []domain.Question{{ID: "short", Bank: "数学基础", Kind: "简答题", Prompt: "主要职责？", Options: []string{"原表多余内容"}, TextAnswer: "进程管理；内存管理；文件管理"}}
	a.navigate(0)
	a.typed = "进程管理与内存管理"
	a.submit()
	if !a.answers["short"].Correct {
		t.Fatal("short answer treated as choice")
	}
}

func TestSystemCaptionButtonsAreNotDuplicated(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1450, 800)
	tt.SetTitleBar(ui.TitleBar{Height: 44, Right: 138})
	tt.Frame()
	if _, ok := tt.Find("关闭窗口"); ok {
		t.Fatal("custom close button overlaps native captions")
	}
	if _, ok := tt.Find("深色模式"); ok {
		t.Fatal("removed theme toggle remains")
	}
	r, ok := tt.Find("应用版本")
	if !ok || r.X+r.W > 1312.1 {
		t.Fatal("title content overlaps system captions", r)
	}
}

func TestQuickModeShowsResultBeforeAdvancing(t *testing.T) {
	for _, tc := range []struct {
		key     ui.Key
		label   string
		correct bool
	}{
		{ui.KeyB, "回答正确！", true},
		{ui.KeyA, "回答错误", false},
	} {
		t.Run(tc.label, func(t *testing.T) {
			a := newApp()
			a.quick = true
			tt := ui.NewTester(a.view, 1450, 980)
			tt.Key(0, tc.key)
			tt.Key(0, ui.KeySpace)
			if a.index != 0 {
				t.Fatal("quick mode advanced before showing its result")
			}
			if r, ok := a.answers["m1"]; !ok || r.Correct != tc.correct {
				t.Fatal(r, ok)
			}
			tt.Advance(599 * time.Millisecond)
			bounds, ok := tt.Find(tc.label)
			if !ok || bounds.Y < 0 || bounds.Y+bounds.H > 980 || a.index != 0 {
				t.Fatalf("question result does not remain visible before auto advance: %v, index %d", bounds, a.index)
			}
			tt.Advance(time.Millisecond)
			if a.index != 1 {
				t.Fatal("quick mode did not advance after feedback")
			}
		})
	}
}

func TestQuickAdvanceDoesNotSkipAfterManualNavigation(t *testing.T) {
	a := newApp()
	a.quick = true
	tt := ui.NewTester(a.view, 1450, 800)
	tt.Key(0, ui.KeyB)
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.KeySpace)
	tt.Advance(quickFeedbackDuration)
	if a.index != 1 {
		t.Fatalf("timer skipped a question after manual navigation: %d", a.index)
	}
}

func TestQuickModeShowsCorrectedWrongQuestionBeforeMovingOn(t *testing.T) {
	a := newApp()
	a.quick, a.onlyWrong, a.repeatPractice = true, true, true
	a.answers["m1"] = domain.Response{}
	a.answers["m2"] = domain.Response{}
	a.answers["m3"] = domain.Response{}
	a.navigate(0)
	tt := ui.NewTester(a.view, 1450, 800)
	tt.Key(0, ui.KeyB)
	tt.Key(0, ui.KeySpace)
	if q, _ := a.current(); q.ID != "m1" {
		t.Fatal("corrected question disappeared before feedback", q.ID)
	}
	tt.Advance(quickFeedbackDuration)
	if q, _ := a.current(); q.ID != "m2" {
		t.Fatal("correction caused next wrong question to be skipped", q.ID)
	}
}
