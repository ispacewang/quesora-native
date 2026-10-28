package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"testing"
)

func manyBanksApp(count int) *app {
	a := newApp()
	a.questions = nil
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("岗位安全培训题库·%02d·很长的题库名称用于验证完整显示", i+1)
		a.questions = append(a.questions, question{ID: fmt.Sprint(i), Bank: name, Kind: "单选题", Prompt: "选择正确答案", Options: []string{"正确", "错误"}, Answer: 1})
	}
	a.bank = a.questions[count-1].Bank
	a.navigate(0)
	return a
}
func TestManyBankTabsStayOneRowAndSelectedVisible(t *testing.T) {
	for _, count := range []int{1, 4, 12, 50} {
		for _, width := range []int{800, 1450} {
			a := manyBanksApp(count)
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			bar, ok := tt.Find("题库标签栏")
			if !ok || math.Abs(float64(bar.H-54)) > 1 {
				t.Fatalf("%d banks / %d bar found=%v %+v texts=%v", count, width, ok, bar, tt.Texts())
			}
			upload, ok := tt.Find("上传题库")
			if !ok || upload.W <= 0 || upload.Y < bar.Y || upload.Y+upload.H > bar.Y+bar.H+1 {
				t.Fatalf("upload hidden: %+v", upload)
			}
			selected, ok := tt.Find("选择题库：" + a.bank)
			if !ok || selected.W <= 0 || selected.X < bar.X || selected.X+selected.W > upload.X {
				t.Fatalf("selected bank hidden for %d / %d: %+v bar %+v", count, width, selected, bar)
			}
			if count > 3 {
				click(t, tt, "全部题库")
				if len(tt.Menu()) != count {
					t.Fatalf("not all banks in menu: %d", len(tt.Menu()))
				}
				first := a.questions[0].Bank
				if err := tt.ChooseMenuItem(first); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if a.bank != first {
					t.Fatal("menu did not select bank")
				}
				selected, _ = tt.Find("选择题库：" + first)
				if selected.W <= 0 {
					t.Fatal("new selection not revealed")
				}
				click(t, tt, "全部题库")
				if err := tt.ChooseMenuItem(first); err != nil {
					t.Fatal(err)
				}
				if a.bank != first {
					t.Fatal("current bank menu choice deselected it")
				}
			}
		}
	}
}
func TestFrostedSurfaceActuallyBlursBackdrop(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			for x := float32(0); x < r.W; x += 4 {
				col := ui.RGB(0, 0, 0)
				if int(x/4)%2 == 0 {
					col = ui.RGB(255, 255, 255)
				}
				p.Fill(ui.Rect{X: x, Y: 0, W: 4, H: r.H}, col, 0)
			}
		}).Children(func() {
			ui.Box(c).Absolute().Left(30).Top(25).Size(140, 40).Draw(func(p *ui.Painter, r ui.Rect) { paintFrostedSurface(p, r, ui.Hex("#fafaf9").Alpha(.86)) })
		})
	}, 200, 90)
	img := tt.Image()
	min, max := uint8(255), uint8(0)
	for x := 60; x < 140; x++ {
		v := img.RGBAAt(x, 45)
		if v.A != 255 {
			t.Fatal("surface remains transparent")
		}
		if v.R < min {
			min = v.R
		}
		if v.R > max {
			max = v.R
		}
	}
	if max-min > 6 || min < 200 {
		t.Fatalf("backdrop not frosted: range %d..%d", min, max)
	}
}
