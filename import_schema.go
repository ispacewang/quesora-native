package main

import (
	"fmt"
	"golang.org/x/text/unicode/norm"
	"math/bits"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Schema discovery is separate from row normalization: column positions are never
// assumed, and unresolved answers are reported instead of fabricated.
type questionSchema struct {
	row, end    int
	fields      map[string]int
	options     map[int]int // printed option ordinal -> worksheet column
	notes       string
	labels      bool
	textAnswers bool
	zeroBased   bool
	typeCodes   map[string]string
}

var headerParens = regexp.MustCompile("[（(\\[【].*$")
var optionHeader = regexp.MustCompile("^(?:选项|备选项|备选答案|option|choice)?([a-z])(?:选项|答案|内容)?$")
var numericOptionHeader = regexp.MustCompile("^(?:选项|option|choice)([0-9]{1,2})$")
var typeCodeRE = regexp.MustCompile("([0-9]+)\\s*(?:表示|代表|为|是|[:：=、.．-])?\\s*(单项选择|多项选择|单选|多选|判断|填空|简答|问答|论述)")
var labeledOptionRE = regexp.MustCompile("(?m)(?:^|[\\r\\n|｜;；]\\s*|[ \\t]+)([A-Za-z])[.．、:：)）]\\s*")
var answerPrefixRE = regexp.MustCompile("(?i)^(?:正确答案|参考答案|标准答案|答案|answer)\\s*[:：=]\\s*")
var missingChoiceAnswer = regexp.MustCompile("^[A-D]$|^[A-Za-z](?:[、,，;；/|＋+][A-Za-z])+$")
var choiceSyntax = regexp.MustCompile("^[A-Z0-9\\s,，、;；/|｜+&\\[\\]()（）.．]+$")

func headerName(s string) string {
	s = strings.ToLower(norm.NFKC.String(strings.TrimSpace(strings.TrimPrefix(s, "\ufeff"))))
	s = headerParens.ReplaceAllString(s, "")
	// A header may explain its meaning after a colon, e.g. 题干：填写题目内容.
	if p := strings.IndexAny(s, ":：\n"); p >= 0 {
		s = s[:p]
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("*_-/\\:：.．", r) {
			return -1
		}
		return r
	}, s)
}
func headerField(s string) (string, int) {
	n := headerName(s)
	if m := optionHeader.FindStringSubmatch(n); m != nil {
		return "option", int(m[1][0] - 'a')
	}
	if m := numericOptionHeader.FindStringSubmatch(n); m != nil {
		i, _ := strconv.Atoi(m[1])
		if i >= 1 && i <= 26 {
			return "option", i - 1
		}
	}
	groups := []struct {
		key   string
		names []string
	}{
		{"prompt", []string{"题干", "试题内容", "题目内容", "试题题干", "题目题干", "问题内容", "问题描述", "试题描述", "题目描述", "考题内容", "题目", "试题", "问题", "question", "prompt", "stem", "questiontext", "questionstem", "题干内容", "考题正文"}},
		{"answer", []string{"答案", "正确答案", "标准答案", "参考答案", "试题答案", "题目答案", "参考解答", "正确选项", "答案选项", "answer", "correctanswer", "key"}},
		{"kind", []string{"题型", "题目类型", "试题类型", "题目题型", "试题题型", "类型", "questiontype", "type"}},
		{"options", []string{"选项", "选项内容", "备选项", "备选答案", "所有选项", "答案选项内容", "候选答案", "options", "choices"}},
		{"explanation", []string{"解析", "答案解析", "试题解析", "题目解析", "解题思路", "解答解析", "答案说明", "解析说明", "说明", "explanation", "analysis", "solution"}},
		{"important", []string{"重要题目", "是否重要", "是否为重要题目", "重要标记", "保命题", "是否保命题", "重要", "isimportant", "isbaoming"}},
		{"备注", []string{"备注", "标注", "remark", "remarks", "note"}},
		{"试题编号", []string{"试题编号", "题目编号", "题号", "序号", "编号", "id", "questionid"}},
		{"试题分数", []string{"试题分数", "分值", "分数", "score", "points"}},
		{"题目分类", []string{"题目分类", "试题分类", "知识点", "分类", "考点", "category"}},
		{"一级纲要", []string{"一级纲要", "一级分类", "章节", "章", "chapter"}},
		{"二级纲要", []string{"二级纲要", "二级分类", "节"}},
		{"题目依据", []string{"题目依据", "试题依据", "依据", "来源", "source"}},
	}
	for _, g := range groups {
		for _, alias := range g.names {
			if n == alias {
				return g.key, -1
			}
		}
	}
	// Descriptive headers such as “字段二（这里填写题目内容）”. Specific
	// concepts outrank the words 题目/答案 occurring inside another concept.
	raw := strings.ToLower(norm.NFKC.String(s))
	if len([]rune(raw)) <= 180 {
		for _, entry := range []struct {
			key     string
			phrases []string
		}{
			{"explanation", []string{"答案解析", "解题思路", "试题解析", "解析说明", "本列为解析"}},
			{"kind", []string{"题目类型", "试题类型", "题型"}},
			{"important", []string{"是否重要", "是否保命", "重要标记"}},
			{"answer", []string{"正确答案", "标准答案", "参考答案", "正确选项", "填写答案", "本列为答案"}},
			{"options", []string{"所有选项", "选项内容", "备选答案", "备选项", "填写选项"}},
			{"prompt", []string{"题目内容", "试题内容", "题干内容", "问题内容", "问题描述", "填写题干", "本列为题干"}},
		} {
			for _, phrase := range entry.phrases {
				if strings.Contains(raw, phrase) {
					return entry.key, -1
				}
			}
		}
	}

	return "", -1
}
func schemaAt(rows [][]string, row int) questionSchema {
	s := questionSchema{row: row, end: row, fields: map[string]int{}, options: map[int]int{}, typeCodes: map[string]string{}}
	add := func(c int, text string) {
		key, opt := headerField(text)
		if key == "option" {
			s.options[opt] = c
		} else if key != "" {
			if _, ok := s.fields[key]; !ok {
				s.fields[key] = c
			}
		}
	}
	for c, text := range rows[row] {
		add(c, text)
	}
	// A second header row is accepted only when it is itself a header, not the
	// first question. This handles merged “选项” parents with A/B/C/D children.
	if row+1 < len(rows) {
		hits := 0
		for _, text := range rows[row+1] {
			if key, _ := headerField(text); key != "" {
				hits++
			}
		}
		promptIsHeader := true
		if col, exists := s.fields["prompt"]; exists && col < len(rows[row+1]) && strings.TrimSpace(rows[row+1][col]) != "" {
			key, _ := headerField(rows[row+1][col])
			promptIsHeader = key == "prompt"
		}
		if hits >= 2 && promptIsHeader {
			s.end = row + 1
			for c, text := range rows[row+1] {
				add(c, text)
			}
		}
	}
	for _, r := range rows[max(0, row-8) : s.end+1] {
		s.notes += " " + strings.Join(r, " ")
	}
	lower := strings.ToLower(norm.NFKC.String(s.notes))
	s.zeroBased = strings.Contains(lower, "从0开始") || strings.Contains(lower, "0开始编号") || strings.Contains(lower, "0-based")
	if c, ok := s.fields["answer"]; ok {
		h := ""
		for r := s.row; r <= s.end; r++ {
			if c < len(rows[r]) {
				h += " " + rows[r][c]
			}
		}
		s.textAnswers = strings.Contains(h, "选项内容") || strings.Contains(h, "实际内容")
		s.labels = strings.Contains(h, "编号") || strings.Contains(h, "序号") || strings.Contains(h, "字母") || strings.Contains(h, "索引")
	}
	for _, m := range typeCodeRE.FindAllStringSubmatch(s.notes, -1) {
		s.typeCodes[m[1]] = canonicalKind(m[2])
	}
	return s
}
func discoverSchema(rows [][]string) (questionSchema, bool) {
	best := questionSchema{}
	score := -1
	for i := 0; i < min(len(rows), 50); i++ {
		s := schemaAt(rows, i)
		if _, ok := s.fields["prompt"]; !ok {
			continue
		}
		n := 6
		if _, ok := s.fields["answer"]; ok {
			n += 6
		}
		if _, ok := s.fields["kind"]; ok {
			n += 2
		}
		if _, ok := s.fields["options"]; ok {
			n += 2
		}
		if len(s.options) > 0 {
			n += 2
		}
		if _, ok := s.fields["explanation"]; ok {
			n++
		}
		if n > score {
			score = n
			best = s
		}
	}
	return best, score >= 6
}
func canonicalKind(s string) string {
	n := strings.ToLower(strings.TrimSpace(norm.NFKC.String(s)))
	for _, v := range []struct {
		kind    string
		aliases []string
	}{{"多选题", []string{"多选", "多项选择", "不定项", "multiple", "multi"}}, {"单选题", []string{"单选", "单项选择", "single"}}, {"判断题", []string{"判断", "是非", "true/false", "boolean", "judge"}}, {"填空题", []string{"填空", "fill", "blank"}}, {"简答题", []string{"简答", "问答", "论述", "解答", "essay", "short"}}} {
		for _, alias := range v.aliases {
			if strings.Contains(n, alias) {
				return v.kind
			}
		}
	}
	return ""
}
func splitCombinedOptions(raw string) ([]string, []int) {
	raw = strings.TrimSpace(norm.NFKC.String(raw))
	if raw == "" {
		return nil, nil
	}
	matches := labeledOptionRE.FindAllStringSubmatchIndex(raw, -1)
	validLabels := len(matches) >= 2
	for i, m := range matches {
		if i > 0 && strings.ToUpper(raw[m[2]:m[3]]) <= strings.ToUpper(raw[matches[i-1][2]:matches[i-1][3]]) {
			validLabels = false
		}
	}
	if validLabels {
		var opts []string
		var labels []int
		for i, m := range matches {
			end := len(raw)
			if i+1 < len(matches) {
				end = matches[i+1][0]
			}
			opts = append(opts, strings.TrimSpace(raw[m[1]:end]))
			labels = append(labels, int(strings.ToUpper(raw[m[2]:m[3]])[0]-'A'))
		}
		return opts, labels
	}
	chunks := strings.FieldsFunc(raw, func(r rune) bool { return r == '|' || r == '｜' || r == '\n' || r == '\r' || r == ';' || r == '；' })
	opts := []string{}
	labels := []int{}
	for _, text := range chunks {
		opt := strings.TrimSpace(optionPrefix.ReplaceAllString(strings.TrimSpace(text), ""))
		if opt != "" {
			labels = append(labels, len(opts))
			opts = append(opts, opt)
		}
	}
	return opts, labels
}
func truthValue(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(norm.NFKC.String(raw))) {
	case "正确", "对", "是", "true", "√", "✓", "1", "t", "yes", "y":
		return true, true
	case "错误", "错", "否", "false", "×", "✗", "0", "f", "no", "n":
		return false, true
	}
	return false, false
}
func normalizedChoice(raw string, q question, labels []int, s questionSchema) (uint, bool) {
	raw = strings.TrimSpace(answerPrefixRE.ReplaceAllString(norm.NFKC.String(raw), ""))
	if q.Kind == "判断题" {
		if v, ok := truthValue(raw); ok {
			for i, opt := range q.Options {
				if v2, ok2 := truthValue(opt); ok2 && v == v2 {
					return 1 << i, true
				}
			}
		}
	}
	var textMask uint
	for i, opt := range q.Options {
		if normalizeAnswer(raw) == normalizeAnswer(opt) {
			textMask = 1 << i
			break
		}
	}
	if textMask == 0 {
		valid := true
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return strings.ContainsRune("|｜;；,，、\n", r) }) {
			found := false
			for i, opt := range q.Options {
				if normalizeAnswer(part) == normalizeAnswer(opt) {
					textMask |= 1 << i
					found = true
					break
				}
			}
			if !found {
				valid = false
				break
			}
		}
		if !valid {
			textMask = 0
		}
	}
	var labelMask uint
	upper := strings.ToUpper(raw)
	if choiceSyntax.MatchString(upper) {
		valid := true
		tokens := strings.FieldsFunc(upper, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		add := func(idx int) bool {
			for i, label := range labels {
				if label == idx {
					labelMask |= 1 << i
					return true
				}
			}
			return false
		}
		for _, token := range tokens {
			if idx, err := strconv.Atoi(token); err == nil {
				if !s.zeroBased {
					idx--
				}
				if !add(idx) {
					valid = false
				}
			} else {
				for _, r := range token {
					if !add(int(r - 'A')) {
						valid = false
					}
				}
			}
		}
		if !valid {
			labelMask = 0
		}
	}
	if s.textAnswers {
		return textMask, textMask != 0
	}
	if s.labels {
		return labelMask, labelMask != 0
	}
	// A numeric answer may mean an ordinal or the numeric option content. If
	// both interpretations are valid and disagree, the row needs clarification.
	if textMask != 0 && labelMask != 0 && textMask != labelMask {
		return 0, false
	}
	if textMask != 0 {
		return textMask, true
	}
	return labelMask, labelMask != 0
}

func questionsFromRows(sheet string, rows [][]string) ([]question, []string, error) {
	var qs []question
	var warnings []string
	nonempty := false
	for _, row := range rows {
		for _, v := range row {
			if strings.TrimSpace(v) != "" {
				nonempty = true
			}
		}
	}
	if !nonempty {
		return nil, nil, nil
	}
	s, ok := discoverSchema(rows)
	if !ok {
		return nil, []string{sheet + "：无法识别题干列，已跳过该工作表"}, nil
	}
	if _, ok = s.fields["answer"]; !ok {
		return nil, []string{sheet + "：无法识别答案列，已跳过该工作表"}, nil
	}
	optLabels := []int{}
	for label := range s.options {
		optLabels = append(optLabels, label)
	}
	sort.Ints(optLabels)
	for index := s.end + 1; index < len(rows); index++ {
		row := rows[index]
		get := func(key string) string {
			c, ok := s.fields[key]
			if !ok || c >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[c])
		}
		prompt := get("prompt")
		if prompt == "" {
			continue
		}
		if key, _ := headerField(prompt); key == "prompt" {
			if key, _ := headerField(get("answer")); key == "answer" {
				continue
			}
		}
		warn := func(reason string) {
			warnings = append(warnings, fmt.Sprintf("%s 第 %d 行：%s", sheet, index+1, reason))
		}
		rawAnswer := get("answer")
		if rawAnswer == "" {
			warn("答案为空，已跳过，未生成答案")
			continue
		}
		genericChoice := strings.EqualFold(get("kind"), "choice") || get("kind") == "选择题" || get("kind") == "选择"
		kind := canonicalKind(get("kind"))
		if kind == "" {
			kind = s.typeCodes[get("kind")]
		}
		if get("kind") != "" && kind == "" && !genericChoice {
			warn("无法识别题型“" + get("kind") + "”，已跳过；请在表头说明编号对应题型")
			continue
		}
		q := question{Prompt: prompt, Kind: kind, Explanation: get("explanation"), Meta: map[string]any{"来源工作表": sheet, "来源行": index + 1, "原始答案": rawAnswer, "原始题型": get("kind")}}
		for _, key := range []string{"一级纲要", "二级纲要", "题目分类", "题目依据", "试题分数", "试题编号", "备注"} {
			q.Meta[key] = get(key)
		}
		important := get("important")
		if v, ok := truthValue(important); ok {
			q.Meta["isBaoMing"] = v
		} else if important == "重要" || important == "重要题目" || important == "保命题" {
			q.Meta["isBaoMing"] = true
		}
		remark := get("备注")
		if important == "" && (strings.Contains(remark, "重要题目") || strings.Contains(remark, "保命题")) && !strings.Contains(remark, "非重要") && !strings.Contains(remark, "不是重要") {
			q.Meta["isBaoMing"] = true
		}
		var labels []int
		if kind != "填空题" && kind != "简答题" {
			if len(optLabels) > 0 {
				for _, label := range optLabels {
					c := s.options[label]
					if c < len(row) && strings.TrimSpace(row[c]) != "" {
						q.Options = append(q.Options, strings.TrimSpace(optionPrefix.ReplaceAllString(strings.TrimSpace(row[c]), "")))
						labels = append(labels, label)
					}
				}
			} else {
				q.Options, labels = splitCombinedOptions(get("options"))
			}
			if len(q.Options) == 0 {
				if p := strings.Index(prompt, "\n"); p > 0 {
					opts, ls := splitCombinedOptions(prompt[p+1:])
					if len(opts) >= 2 && len(labeledOptionRE.FindAllString(prompt[p+1:], -1)) >= 2 {
						q.Prompt = strings.TrimSpace(prompt[:p])
						q.Options, labels = opts, ls
					}
				}
			}
		}
		if len(q.Options) > 26 {
			return nil, warnings, fmt.Errorf("%s 第 %d 行：超过 26 个选项", sheet, index+1)
		}

		if q.Kind == "" {
			q.Meta["自动识别题型"] = true
			if len(q.Options) == 0 {
				if strings.Contains(prompt, "__") || strings.Contains(prompt, "（ ）") || strings.Contains(prompt, "( )") {
					q.Kind = "填空题"
				} else if _, ok := truthValue(rawAnswer); ok && rawAnswer != "0" && rawAnswer != "1" {
					q.Kind = "判断题"
				} else if missingChoiceAnswer.MatchString(strings.TrimSpace(rawAnswer)) {
					warn("答案形似选项编号，但未发现选项，已跳过")
					continue
				} else {
					q.Kind = "简答题"
				}
			} else {
				q.Kind = "单选题"
				if len(q.Options) == 2 {
					v, ok := truthValue(q.Options[0])
					v2, ok2 := truthValue(q.Options[1])
					if ok && ok2 && v != v2 {
						q.Kind = "判断题"
					}
				}
			}
		}

		if q.Kind == "判断题" && len(q.Options) == 0 {
			q.Options = []string{"正确", "错误"}
			labels = []int{0, 1}
		}
		if q.Kind == "填空题" || q.Kind == "简答题" {
			q.TextAnswer = rawAnswer
			q.Options = nil
		} else {
			if len(q.Options) < 2 {
				warn("选择题不足两个有效选项，已跳过")
				continue
			}
			mask, valid := normalizedChoice(rawAnswer, q, labels, s)
			if !valid {
				warn("答案“" + rawAnswer + "”不能唯一对应到选项，已跳过")
				continue
			}
			q.Answer = mask
			q.TextAnswer = letters(mask)
			if bits.OnesCount(mask) > 1 {
				if kind == "单选题" || q.Kind == "判断题" {
					warn("题型与多项答案冲突，已跳过")
					continue
				}
				q.Kind = "多选题"
			}
		}
		qs = append(qs, q)
	}
	return qs, warnings, nil
}
