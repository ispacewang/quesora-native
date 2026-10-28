package main

import (
	"github.com/egoist/mygo/ui"
	"math"
	"testing"
)

func TestExamSetupFieldLayout(t *testing.T) {
	for _, size := range [][2]int{{1450, 800}, {800, 500}} {
		for _, dark := range []bool{false, true} {
			a := newApp()
			a.dark = dark
			a.openExamSetup()
			tt := ui.NewTester(a.view, size[0], size[1])
			var fields []ui.Rect
			for _, kind := range examKinds {
				r, ok := tt.Find("考试" + kind + "数量输入框")
				if !ok || r.W < 90 {
					t.Fatalf("%v %s %+v", size, kind, r)
				}
				fields = append(fields, r)
			}
			for i := 1; i < len(fields); i++ {
				if math.Abs(float64(fields[i].W-fields[0].W)) > 1 {
					t.Fatalf("unequal columns %+v", fields)
				}
				if i%3 != 0 && fields[i].X-fields[i-1].X-fields[i-1].W < 10 {
					t.Fatalf("overlapping fields %+v", fields)
				}
			}
			for _, label := range []string{"取消", "开始考试"} {
				r, ok := tt.Find(label)
				if !ok || r.W <= 0 || r.Y < 0 || r.Y+r.H > float32(size[1]) {
					t.Fatalf("footer %v %s %+v", size, label, r)
				}
			}
		}
	}
}
func TestExamSetupNumericEditing(t *testing.T) {
	a := newApp()
	a.openExamSetup()
	tt := ui.NewTester(a.view, 1450, 800)
	click(t, tt, "考试单选题数量")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("7")
	if a.examForm.Counts[0] != 7 {
		t.Fatal("quantity not applied", a.examForm.Counts)
	}
	tt.Key(0, ui.KeyUp)
	if a.examForm.Counts[0] != 8 {
		t.Fatal("up")
	}
	tt.Key(0, ui.KeyDown)
	if a.examForm.Counts[0] != 7 {
		t.Fatal("down")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("1.5")
	if !math.IsNaN(a.examForm.Counts[0]) {
		t.Fatal("fraction accepted")
	}
	click(t, tt, "开始考试")
	if a.exam != nil || !a.examSetupOpen || !tt.HasText("单选题数量应为 0–100 的整数") {
		t.Fatal("invalid form started exam")
	}
	click(t, tt, "考试单选题数量")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("2")
	click(t, tt, "考试时长")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("15")
	click(t, tt, "开始考试")
	if a.exam == nil || a.exam.Minutes != 15 {
		t.Fatalf("valid form: exam=%+v settings=%+v message=%q", a.exam, a.examForm, a.examMessage)
	}
}
func TestExamSetupEmptyAndLongBank(t *testing.T) {
	a := newApp()
	a.questions = nil
	a.bank = ""
	a.openExamSetup()
	tt := ui.NewTester(a.view, 800, 500)
	if !tt.HasText("请选择题库") || !tt.HasText("暂无题库，请先上传题库。") {
		t.Fatal("empty state")
	}
	click(t, tt, "开始考试")
	if a.exam != nil {
		t.Fatal("empty exam")
	}
	click(t, tt, "取消")
	a.questions = demoQuestions()
	a.questions[0].Bank = "一个用于验证下拉菜单和表单边界的很长很长的中文题库名称"
	a.bank = a.questions[0].Bank
	a.openExamSetup()
	tt.Frame()
	click(t, tt, "考试题库")
	click(t, tt, "选择考试题库：计算机基础")
	if a.examForm.Bank != "计算机基础" {
		t.Fatalf("bank selector: %q", a.examForm.Bank)
	}
}
