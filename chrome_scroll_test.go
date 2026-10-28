package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
	"time"
)

func TestBankWheelBrowsesTabsWithoutMovingQuestion(t *testing.T) {
	for _, width := range []int{800, 1450} {
		a := manyBanksApp(50)
		a.bank = a.questions[0].Bank
		a.navigate(0)
		tt := ui.NewTester(a.view, width, 800)
		tt.Frame()
		strip, _ := tt.Find("题库标签栏")
		area, _ := tt.Find("题库标签滚动区域")
		upload, _ := tt.Find("上传题库")
		selected := a.bank
		// The ordinary vertical wheel must reach the last tab, even over a child button.
		tt.Wheel(area.X+20, area.Y+12, 0, 100000)
		tt.Frame()
		last, ok := tt.Find("选择题库：" + a.questions[49].Bank)
		if !ok || last.W <= 0 || last.X < area.X-1 || last.X+last.W > area.X+area.W+1 {
			t.Fatalf("%d: vertical wheel did not reveal final bank: %+v", width, last)
		}
		// At the end it must not bubble into the vertically scrolling question pane.
		tt.Wheel(area.X+20, area.Y+12, 0, 100000)
		tt.Advance(200 * time.Millisecond)
		after, _ := tt.Find("题库标签栏")
		pinned, _ := tt.Find("上传题库")
		if after.Y != strip.Y || pinned != upload || a.bank != selected {
			t.Fatalf("wheel moved page, upload or selection: %+v %+v", strip, after)
		}
		// The strip's padding also accepts the wheel, without requiring Shift.
		tt.Wheel(area.X+20, strip.Y+2, 0, -100000)
		tt.Frame()
		first, ok := tt.Find("选择题库：" + selected)
		if !ok || first.W <= 0 || first.X < area.X-1 || first.X+first.W > area.X+area.W+1 {
			t.Fatalf("%d: reverse wheel did not return to first bank", width)
		}
		// Native horizontal wheel / touchpad deltas must keep their direction.
		tt.Wheel(area.X+20, area.Y+12, 100000, 0)
		tt.Frame()
		last, ok = tt.Find("选择题库：" + a.questions[49].Bank)
		if !ok || last.W <= 0 {
			t.Fatal("horizontal wheel no longer works")
		}
	}
}

func TestHistoryDrawerAvoidsNativeCaptionArea(t *testing.T) {
	for _, width := range []int{800, 1450} {
		for _, dark := range []bool{false, true} {
			a := newApp()
			a.dark = dark
			tt := ui.NewTester(a.view, width, 500)
			tt.SetTitleBar(ui.TitleBar{Height: 44, Right: 138})
			click(t, tt, "我的")
			panel, ok := tt.Find("我的记录面板")
			close, found := tt.Find("关闭我的")
			if !ok || !found || panel.Y < titleHeight || panel.Y+panel.H > 500.1 || close.Y < titleHeight {
				t.Fatalf("drawer overlaps captions or viewport: %+v close %+v", panel, close)
			}
			if !tt.HasText("我的记录") || !tt.HasText("暂无保存记录") {
				t.Fatal("history content missing")
			}
			click(t, tt, "关闭我的")
			if a.historyOpen {
				t.Fatal("drawer close failed")
			}
			click(t, tt, "我的")
			tt.Key(0, ui.KeyEscape)
			if a.historyOpen {
				t.Fatal("Escape no longer dismisses drawer")
			}
		}
	}
}

func TestChromeVersionAndPracticeModeTooltips(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := newApp()
		a.dark = dark
		tt := ui.NewTester(a.view, 1450, 800)
		tt.SetVibrancy(true)
		tt.SetTitleBar(ui.TitleBar{Height: 44, Right: 138})
		version, ok := tt.Find("应用版本")
		if !ok || version.W <= 0 || version.Y+version.H > titleHeight {
			t.Fatal("version missing or outside title bar", version)
		}
		if tt.HasText("AI  Beta") {
			t.Fatal("AI placeholder still visible")
		}
		for label, tip := range map[string]string{
			"随机": "开启后随机排列题目顺序，一轮内不重复；关闭后按原顺序练习。",
			"速刷": "开启后提交答案会自动进入下一题；关闭后停留查看本题结果。",
			"打乱": "开启后打乱选择题的选项顺序；关闭后恢复原顺序，正确答案随选项同步。",
		} {
			r, found := tt.Find(label)
			if !found {
				t.Fatal("mode missing", label)
			}
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Advance(time.Second)
			if !tt.HasText(tip) {
				t.Fatal("mode tooltip missing", label, tt.Texts())
			}
			tt.Move(0, 0)
			tt.Frame()
		}
	}
}

// Blank chrome must leave the DWM material visible, even when the drawer is open.
func TestChromeDoesNotCoverDesktopMaterial(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := newApp()
		a.dark = dark
		tt := ui.NewTester(a.view, 1450, 800)
		tt.SetVibrancy(true)
		for _, open := range []bool{false, true} {
			a.historyOpen = open
			tt.Frame()
			img := tt.Image()
			if col := img.RGBAAt(500, 20); col.A != 0 {
				t.Fatalf("title bar covers system material: %+v", col)
			}
			if !open {
				if col := img.RGBAAt(500, 60); col.A != 0 {
					t.Fatalf("toolbar covers system material: %+v", col)
				}
			}
		}
		a.historyOpen = false
		tt.SetVibrancy(false)
		if col := tt.Image().RGBAAt(500, 20); col.A != 255 {
			t.Fatalf("solid fallback missing: %+v", col)
		}
	}
}
