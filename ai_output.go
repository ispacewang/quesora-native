// 模型响应只作为数据解析。字段错误时明确失败，不根据“正确”等自然语言猜测答案。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"strings"
)

func aiJSON(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "\x60\x60\x60") {
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "\x60\x60\x60"))
	}
	return text
}
func parseAIQuestions(text, bank string) ([]question, error) {
	raw := []byte(aiJSON(text))
	var rows []struct {
		Question, Type, Answer, Explanation string
		Options                             []string
	}
	if len(raw) > 0 && raw[0] == '{' {
		var wrap struct{ Questions json.RawMessage }
		if json.Unmarshal(raw, &wrap) == nil && len(wrap.Questions) > 0 {
			raw = wrap.Questions
		}
	}
	if json.Unmarshal(raw, &rows) != nil || len(rows) == 0 || len(rows) > 10 {
		return nil, errors.New("AI 返回的题目格式无效，请重新生成")
	}
	out := []question{}
	seen := map[string]bool{}
	for i, r := range rows {
		r.Question = strings.TrimSpace(r.Question)
		r.Answer = strings.TrimSpace(r.Answer)
		validKind := false
		for _, k := range examKinds {
			if k == r.Type {
				validKind = true
			}
		}
		if !validKind || r.Question == "" || len(r.Question) > 12000 || r.Answer == "" || len(r.Answer) > 12000 || len(r.Explanation) > 18000 || seen[r.Question] {
			return nil, fmt.Errorf("第 %d 道题的题型、题干或答案无效/重复", i+1)
		}
		seen[r.Question] = true
		q := question{Bank: bank, Kind: r.Type, Prompt: r.Question, TextAnswer: r.Answer, Explanation: r.Explanation, Meta: map[string]any{"一级纲要": "AI生成", "题目分类": r.Type}}
		if q.Kind == "简答题" || q.Kind == "填空题" {
			if len(r.Options) != 0 {
				return nil, fmt.Errorf("第 %d 道文本题包含异常选项", i+1)
			}
		} else {
			if r.Type == "判断题" {
				q.Options = []string{"正确", "错误"}
			} else {
				if len(r.Options) < 2 || len(r.Options) > 8 {
					return nil, fmt.Errorf("第 %d 道题选项数无效", i+1)
				}
				for _, s := range r.Options {
					s = strings.TrimSpace(optionPrefix.ReplaceAllString(s, ""))
					if s == "" || len(s) > 4000 {
						return nil, errors.New("AI 选项无效")
					}
					q.Options = append(q.Options, s)
				}
			}
			q.Answer = answerMask(r.Answer, r.Type, q.Options)
			normalized := strings.ToUpper(strings.NewReplacer(" ", "", ",", "", "，", "", "、", "").Replace(r.Answer))
			if r.Type != "判断题" {
				for _, ch := range normalized {
					if ch < 'A' || ch >= 'A'+rune(len(q.Options)) {
						return nil, fmt.Errorf("第 %d 道题答案超出选项范围", i+1)
					}
				}
			}
			if q.Answer == 0 || q.Answer>>len(q.Options) != 0 || (r.Type != "多选题" && bits.OnesCount(q.Answer) != 1) {
				return nil, fmt.Errorf("第 %d 道题答案无效", i+1)
			}
		}
		out = append(out, q)
	}
	return out, nil
}

type aiJudgment struct {
	Correct     bool
	Explanation string
}

func parseAIJudgment(text string) (aiJudgment, error) {
	var v struct {
		Correct     *bool
		Explanation string
	}
	if json.Unmarshal([]byte(aiJSON(text)), &v) != nil || v.Correct == nil || strings.TrimSpace(v.Explanation) == "" || len(v.Explanation) > 18000 {
		return aiJudgment{}, errors.New("AI 判题结果格式无效；原答题记录未修改")
	}
	return aiJudgment{*v.Correct, v.Explanation}, nil
}
