// Package mathtext interprets the formula delimiters used in Quesora question banks.
package mathtext

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Kind uint8

const (
	Plain Kind = iota
	Inline
	Display
)

type Part struct {
	Kind   Kind
	Source string // original text, including delimiters for explicit math
	TeX    string
}

var mathFeature = regexp.MustCompile(`\\(?:sqrt|frac|sum|int|lim|prod|begin|sin|cos|tan|log|ln|pi|alpha|beta|theta|infty|times|cdot|leq|geq|neq)\b|[√πθ∞±×÷≤≥≠≈∂∇∑∫∏]|[a-zA-Z0-9)\]][\^_]\{[^}]+\}|\b[a-zA-Z][\^_]\d+\b|\b[a-zA-Z]_[a-zA-Z]\b`)

// Split returns nil for ordinary text, so existing text widgets can take their original path.
// In mixed prose, formulas require delimiters. An isolated formula may omit them.
func Split(source string) []Part {
	if len(source) > 32000 {
		return nil
	}
	if !strings.ContainsAny(source, "$\\") {
		if isolatedMath(source) {
			return []Part{{Kind: Inline, Source: source, TeX: strings.TrimSpace(source)}}
		}
		return nil
	}
	var parts []Part
	var haveDelimiter bool
	plainStart := 0
	for i := 0; i < len(source); {
		marker, end, kind := opener(source[i:])
		if marker == "" {
			if source[i] == '\\' && i+1 < len(source) {
				i += 2
			} else {
				i++
			}
			continue
		}
		haveDelimiter = true
		start := i + len(marker)
		closeAt := findEnd(source, start, end, kind, marker == "$")
		if closeAt < 0 {
			i = start
			continue
		}
		tex := strings.TrimSpace(source[start:closeAt])
		if tex == "" {
			i = closeAt + len(end)
			continue
		}
		if len(parts) >= 127 {
			return nil
		}
		if plainStart < i {
			parts = append(parts, Part{Kind: Plain, Source: source[plainStart:i]})
		}
		plainStart = closeAt + len(end)
		parts = append(parts, Part{Kind: kind, Source: source[i:plainStart], TeX: tex})
		i = plainStart
	}
	if len(parts) == 0 {
		if !haveDelimiter && isolatedMath(source) {
			return []Part{{Kind: Inline, Source: source, TeX: strings.TrimSpace(source)}}
		}
		return nil
	}
	if plainStart < len(source) {
		parts = append(parts, Part{Kind: Plain, Source: source[plainStart:]})
	}
	return parts
}

func opener(s string) (string, string, Kind) {
	switch {
	case strings.HasPrefix(s, "$$"):
		return "$$", "$$", Display
	case strings.HasPrefix(s, `\[`):
		return "\\[", "\\]", Display
	case strings.HasPrefix(s, `\(`):
		return "\\(", "\\)", Inline
	case s[0] == '$' && len(s) > 1:
		r, _ := utf8.DecodeRuneInString(s[1:])
		if !unicode.IsSpace(r) {
			return "$", "$", Inline
		}
	}
	return "", "", Plain
}

func findEnd(s string, start int, end string, kind Kind, dollar bool) int {
	for i := start; i < len(s) && i-start <= maxFormulaBytes; i++ {
		if kind == Inline && (s[i] == '\n' || s[i] == '\r') {
			return -1
		}
		if strings.HasPrefix(s[i:], end) {
			if !dollar {
				return i
			}
			if i == start || strings.HasPrefix(s[i:], "$$") {
				return -1
			}
			next, _ := utf8.DecodeRuneInString(s[i+1:])
			previous, _ := utf8.DecodeLastRuneInString(s[start:i])
			// Dollar amounts stay literal when the candidate closer also starts an amount.
			if unicode.IsSpace(previous) || unicode.IsDigit(next) {
				return -1
			}
			return i
		}
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
	}
	return -1
}

func isolatedMath(s string) bool {
	if len(s) > maxFormulaBytes || strings.ContainsAny(s, "$\r\n") {
		return false
	}
	for _, r := range s {
		// CJK punctuation belongs to prose. Treating the whole line as TeX can drop it.
		if unicode.Is(unicode.Han, r) || r >= 0x3000 && r <= 0x303f || r >= 0xff01 && r <= 0xff60 {
			return false
		}
	}
	return mathFeature.MatchString(s)
}
