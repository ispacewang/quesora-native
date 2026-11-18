// 错题备注复用练习记录和串行队列，保存成功后才更新界面中的持久内容。
package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// saveMistakeNote 只更新备注字段，避免等待保存期间的新作答被旧作答覆盖。
func (s *questionStore) saveMistakeNote(id, note string) error {
	result, err := s.db.Exec("UPDATE practice SET payload=json_set(payload,'$.Note',?) WHERE question_id=?", note, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("答题记录尚未保存，请重新提交后保存备注")
	}
	return nil
}

func (a *app) saveMistakeNote() {
	q, ok := a.current()
	if !ok || a.mistakeNoteSaving || a.store == nil || a.mistakeNoteID != q.ID {
		return
	}
	r, done := a.currentResponse(q)
	if !done || r.Correct || a.quick {
		return
	}
	draft, note := a.mistakeNote, strings.TrimSpace(a.mistakeNote)
	store, id, revision := a.store, q.ID, a.practiceRevision
	a.mistakeNoteSaving = true
	a.mistakeNoteError = ""
	if !a.enqueue(func() {
		err := store.saveMistakeNote(id, note)
		a.publish(func() {
			a.mistakeNoteSaving = false
			// Reset/restore can replace practice records while this acknowledgement is queued.
			if a.practiceRevision != revision {
				return
			}
			if err != nil {
				message := "备注保存失败：" + err.Error()
				if a.mistakeNoteID == id {
					a.mistakeNoteError = message
				}
				a.libraryStatus = message
				return
			}
			// Apply only the saved field to the latest answer, which may have changed meanwhile.
			if latest, exists := a.answers[id]; exists {
				latest.Note = note
				a.answers[id] = latest
			}
			if a.mistakeNoteID == id && a.mistakeNote == draft {
				a.mistakeNote = note
			}
			a.savedWrong = false
		})
	}) {
		a.mistakeNoteSaving = false
		a.mistakeNoteError = "后台任务繁忙，请稍后保存备注。"
	}
}

func (a *app) mistakeNoteEditor(c *ui.Context, q question, r response) {
	if a.mistakeNoteID != q.ID {
		a.mistakeNoteID = q.ID
		a.mistakeNote = r.Note
		a.mistakeNoteError = ""
	}
	t := c.Theme()
	ui.Column(c.Key("mistake-note")).FillWidth().Radius(cornerRadius(c, 80)).Padding(16).Gap(10).
		Border(1, t.Border.Alpha(.6)).Background(t.Background).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "我的备注").FontSize(10).FontWeight(600).TextColor(t.TextMuted)
			ui.Spacer(c)
			if a.mistakeNoteError == "" && strings.TrimSpace(a.mistakeNote) == r.Note && r.Note != "" {
				ui.Text(c, "已保存").FontSize(10).TextColor(t.Success)
			}
		})
		ui.TextArea(c.Key("draft"), &a.mistakeNote).Label("错题备注").FillWidth().Height(80).
			FontSize(12).Radius(cornerRadius(c, 80)).Placeholder("写下这题为什么错、下次要注意什么…").
			Disabled(a.mistakeNoteSaving).OnChange(func() { a.mistakeNoteError = "" })
		if a.mistakeNoteError != "" {
			ui.Text(c, a.mistakeNoteError).FillWidth().FontSize(11).TextColor(t.Danger)
		}
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			label := "保存备注"
			if a.mistakeNoteSaving {
				label = "保存中…"
			}
			control(c, "save", label, nil, outlineButton, a.saveMistakeNote).Label("保存备注").
				Disabled(a.store == nil || a.mistakeNoteSaving || strings.TrimSpace(a.mistakeNote) == r.Note)
		})
	})
}

// mistakeNoteSummary shares the saved-note presentation between the sidebar and history.
func mistakeNoteSummary(c *ui.Context, note string) {
	if note == "" {
		return
	}
	t := c.Theme()
	ui.Column(c.Key("saved-note")).FillWidth().Radius(cornerRadius(c, 48)).Padding(8).Gap(4).
		Border(1, t.Border.Alpha(.5)).Background(t.Background).Children(func() {
		ui.Text(c, "我的备注").FontSize(10).FontWeight(600).TextColor(t.TextMuted)
		ui.Text(c, note).FillWidth().FontSize(11).LineHeight(1.6)
	})
}
