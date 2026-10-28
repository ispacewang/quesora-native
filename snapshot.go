package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

//go:embed resources/qa-fixtures.json
var sceneFixtures []byte

func (a *app) configureScene(scene string) {
	var fixtures map[string]struct {
		ID                                  int
		Type, Question, Answer, Explanation string
		Options                             []string
		Meta                                map[string]any
	}
	if err := json.Unmarshal(sceneFixtures, &fixtures); err != nil {
		panic(err)
	}
	a.questions = nil
	a.answers = map[string]response{}
	a.bank = "数学基础"
	a.filter = "全部"
	a.filters = nil
	for _, kind := range []string{"single", "multi", "judge", "fill", "short"} {
		item := fixtures[kind]
		a.questions = append(a.questions, question{ID: strconv.Itoa(item.ID), Bank: "数学基础", Kind: item.Type, Prompt: item.Question, Options: item.Options, TextAnswer: item.Answer, Answer: answerMask(item.Answer, item.Type, item.Options), Explanation: item.Explanation, Meta: item.Meta})
	}
	a.questions = append(a.questions, question{ID: "c1", Bank: "计算机基础", Kind: "单选题", Prompt: "下列哪一种数据结构遵循先进先出？", Options: []string{"栈", "队列"}, Answer: 2})
	a.navigate(0)
	switch scene {
	case "history":
		a.historyOpen = true
	case "many-banks":
		for i := 1; i <= 18; i++ {
			q := question{ID: fmt.Sprintf("bank-%d", i), Bank: fmt.Sprintf("岗位安全培训题库·%02d·很长的题库名称用于验证完整显示", i), Kind: "单选题", Prompt: "下列哪一项是正确的操作？", Options: []string{"按规范操作", "省略检查步骤", "未经许可操作", "忽略设备报警"}, Answer: 1}
			a.questions = append(a.questions, q)
		}
		a.bank = a.questions[len(a.questions)-1].Bank
		a.navigate(0)
	case "exam-setup":
		a.openExamSetup()
	case "exam-setup-empty":
		a.questions = nil
		a.bank = ""
		a.openExamSetup()
	case "important":
		a.starred[a.questions[0].ID] = true
	case "exam", "exam-result":
		a.examForm = examSettings{Bank: a.bank, Minutes: 60, Counts: [5]float64{40, 30, 30, 10, 10}}
		a.exam, _ = newExamSession(a.questions, a.examForm, a.starred, time.Now())
		if scene == "exam-result" {
			for i, q := range a.exam.Questions {
				a.exam.Responses[i].Choice = q.Answer
				a.exam.Responses[i].Text = q.TextAnswer
			}
			a.exam.Responses[0].Choice = 0
			a.exam.submit(time.Now().Add(-time.Hour))
		}
	case "empty":
		a.questions = nil
		a.bank = ""
		a.leftOpen = false
		a.rightOpen = false
	case "unselected":
		a.bank = ""
		a.leftOpen = false
		a.rightOpen = false
	case "multi":
		a.navigate(1)
	case "judge":
		a.navigate(2)
	case "fill":
		a.navigate(3)
	case "short":
		a.navigate(4)
	case "selected":
		a.choose(1)
	case "correct":
		a.choose(1)
		a.submit()
	case "wrong":
		a.choose(0)
		a.submit()
	}
}
