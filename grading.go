// 离线文本归一化与简答要点覆盖评分。该启发式规则不等同于语义理解或 AI 判题。

package main

import (
	"golang.org/x/text/unicode/norm"
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var answerPrefix = regexp.MustCompile(`(?i)^(?:参考答案|标准答案|答案|答)\s*[:：]\s*`)
var answerPoints = regexp.MustCompile(`[，,、；;。.!！?？\n]+`)
var answerAlternatives = regexp.MustCompile(`[/／|｜]`)
var fallbackTerms = regexp.MustCompile(`[\p{Han}]{2,}|[a-zA-Z0-9]+`)
var stopWords = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range strings.Fields("的 了 是 在 并 和 与 或 对 通过 可以 能够 需要 应 要 将 把 为 及 以及 一个 进行 从而 因此 因为 所以 具有 主要 相关 其中 该 这 其") {
		m[s] = true
	}
	return m
}()

// normalizeAnswer 应用 NFKC、大小写和常见空白/标点归一化，不用于保存或展示原始答案。
func normalizeAnswer(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("，,、；;。.!！?？:：\"“”‘’'`（）()【】[]{}<>《》", r) {
			return -1
		}
		return unicode.ToLower(r)
	}, norm.NFKC.String(s))
}

// extractTerms 优先使用系统 ICU，缺失时回退正则；去除短词、停用词和重复词。
func extractTerms(s string) []string {
	parts := nativeWords(norm.NFKC.String(s))
	if parts == nil {
		parts = fallbackTerms.FindAllString(s, -1)
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range parts {
		term := normalizeAnswer(part)
		if utf8.RuneCountInString(term) >= 2 && !stopWords[term] && !seen[term] {
			seen[term] = true
			out = append(out, term)
		}
	}
	return out
}

// coversTerms 要求覆盖至少两个且不少于六成的去重词项，避免单个泛词造成误判。
func coversTerms(user string, terms []string) bool {
	count := 0
	for _, term := range terms {
		if strings.Contains(user, term) {
			count++
		}
	}
	return count >= max(2, int(math.Ceil(float64(len(terms))*.6)))
}

// judgeShortAnswer 先匹配完整规范答案，再按要点或词项覆盖判定；规则存在语义与系统分词差异的边界。
func judgeShortAnswer(user, standard string) bool {
	standard = strings.TrimSpace(answerPrefix.ReplaceAllString(norm.NFKC.String(standard), ""))
	u, s := normalizeAnswer(user), normalizeAnswer(standard)
	if u == "" || s == "" {
		return false
	}
	if u == s {
		return true
	}
	var points []string
	seen := map[string]bool{}
	for _, p := range answerPoints.Split(standard, -1) {
		p = normalizeAnswer(p)
		if utf8.RuneCountInString(p) >= 2 && !seen[p] {
			seen[p] = true
			points = append(points, p)
		}
	}
	if len(points) > 1 {
		matched := 0
		for _, p := range points {
			for _, alt := range answerAlternatives.Split(p, -1) {
				alt = normalizeAnswer(alt)
				if alt != "" && (strings.Contains(u, alt) || coversTerms(u, extractTerms(alt))) {
					matched++
					break
				}
			}
		}
		return matched >= max(2, int(math.Ceil(float64(len(points))*.6)))
	}
	terms := extractTerms(standard)
	if len(terms) < 2 {
		return strings.Contains(u, s)
	}
	return coversTerms(u, terms)
}
