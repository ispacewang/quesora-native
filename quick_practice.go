// 速刷反馈在窗口帧内计时。下一次导航或打开弹窗即取消自动跳转。
package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const quickFeedbackDuration = 600 * time.Millisecond
const quickModeHint = "显示判题结果后自动进入下一题，也可按空格继续。"

func (a *app) stopQuickAdvance() {
	a.quickFeedbackPending = false
	a.quickAdvanceAt = time.Time{}
}

// Start the timer on the first frame that shows the result, including after a slow AI response.
func (a *app) updateQuickAdvance(c *ui.Context, blocked bool) {
	if !a.quickFeedbackPending {
		return
	}
	if blocked || !a.quick {
		a.stopQuickAdvance()
		return
	}
	now := c.Now()
	if a.quickAdvanceAt.IsZero() {
		a.quickAdvanceAt = now.Add(quickFeedbackDuration)
	}
	if remaining := a.quickAdvanceAt.Sub(now); remaining > 0 {
		c.After(remaining)
		return
	}
	a.nextQuestion()
}

func (a *app) quickSpace() {
	if q, ok := a.current(); ok {
		if _, done := a.currentResponse(q); done {
			a.nextQuestion()
			return
		}
	}
	a.submit()
}
