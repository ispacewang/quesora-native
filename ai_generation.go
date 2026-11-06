// AI 分批出题的去重身份和受限上下文；不保存答案或创建额外网络请求。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"sort"
	"strings"
)

// aiQuestionFingerprint 按题型、题干与选项集合识别重复。相同通用题干、不同选项
// 可以是不同题；换序、空白、答案或解析变化不能把同一道题伪装为新题。
func aiQuestionFingerprint(q question) [32]byte {
	normalize := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	options := make([]string, len(q.Options))
	for i, s := range q.Options {
		options[i] = normalize(s)
	}
	sort.Strings(options)
	data, _ := json.Marshal([]any{q.Kind, normalize(q.Prompt), options})
	return sha256.Sum256(data)
}

type aiQuestionHint struct {
	Question string   `json:"question"`
	Type     string   `json:"type"`
	Options  []string `json:"options,omitempty"`
}

// aiPreviousQuestions 优先保留最近批次的题目，给下一次独立请求提供去重上下文。
// 题干/选项摘要及 JSON 总字节数都有上限，避免 500 题或长题干撑满输入预算。
// 本地查重仍使用完整内容，摘要截断不会影响实际重复判断。
func aiPreviousQuestions(qs []question) []aiQuestionHint {
	clip := func(s string, n int) string {
		r := []rune(s)
		if len(r) > n {
			return string(r[:n]) + "…"
		}
		return s
	}
	hints := []aiQuestionHint{}
	used := 2
	for i := len(qs) - 1; i >= 0; i-- {
		q := qs[i]
		hint := aiQuestionHint{Question: clip(q.Prompt, 512), Type: q.Kind}
		for _, option := range q.Options {
			hint.Options = append(hint.Options, clip(option, 128))
		}
		data, _ := json.Marshal(hint)
		if used+len(data)+1 > 48000 {
			break
		}
		hints = append(hints, hint)
		used += len(data) + 1
	}
	for i, j := 0, len(hints)-1; i < j; i, j = i+1, j-1 {
		hints[i], hints[j] = hints[j], hints[i]
	}
	return hints
}
