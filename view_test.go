package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
}
func TestPracticeFlow(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1450, 860)
	click(t, tt, "提交答案")
	if len(a.answers) != 0 {
		t.Fatal("empty answer accepted")
	}
	click(t, tt, "B. 0")
	click(t, tt, "提交答案")
	if good, _ := a.counts(); good != 1 || !tt.HasText("回答正确！") {
		t.Fatal("correct answer not recorded")
	}
	click(t, tt, "下一题")
	for _, label := range []string{"A. 图像关于 y 轴对称", "C. 最小值为 0", "D. 图像经过点 (2, 4)"} {
		click(t, tt, label)
	}
	click(t, tt, "提交答案")
	if good, _ := a.counts(); good != 2 {
		t.Fatal("multiple choice failed")
	}
	click(t, tt, "下一题")
	click(t, tt, "A. 正确")
	click(t, tt, "提交答案")
	if _, bad := a.counts(); bad != 1 || !tt.HasText("回答错误") {
		t.Fatal("wrong answer not recorded")
	}
	click(t, tt, "查看解析")
	if q, _ := a.current(); q.ID != "m3" {
		t.Fatal("wrong-answer navigation")
	}
}
func TestFiltersThemeAndPanels(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1450, 860)
	click(t, tt, "筛选填空题")
	click(t, tt, "填写答案")
	tt.Type("1")
	click(t, tt, "提交答案")
	if good, _ := a.counts(); good != 1 {
		t.Fatal("text input did not reach grading")
	}
	click(t, tt, "深色模式")
	if !a.dark {
		t.Fatal("theme")
	}
	for i := 0; i < 3; i++ {
		click(t, tt, "切换统计面板")
		tt.Frame()
		click(t, tt, "切换统计面板")
		click(t, tt, "切换错题面板")
		tt.Frame()
		click(t, tt, "切换错题面板")
	}
	click(t, tt, "选择题库：计算机基础")

	if q, _ := a.current(); q.ID != "c1" {
		t.Fatal("bank switch")
	}
	click(t, tt, "功能引导")
	click(t, tt, "重新练习")
	click(t, tt, "确认重置")
	if len(a.answers) != 0 {
		t.Fatal("reset")
	}
}
func TestResponsiveLayout(t *testing.T) {
	for _, size := range [][2]int{{800, 500}, {1000, 680}, {1200, 800}, {1450, 860}} {
		a := newApp()
		if size[0] < 1000 {
			a.leftOpen = false
			a.rightOpen = false
		}
		tt := ui.NewTester(a.view, size[0], size[1])
		for _, label := range []string{"选择题库：数学基础", "A. −1", "深色模式", "切换错题面板"} {
			r, ok := tt.Find(label)
			if !ok || r.W <= 0 || r.X < 0 || r.X+r.W > float32(size[0])+1 {
				t.Fatalf("%v %s bounds %+v", size, label, r)
			}
		}
	}
}
