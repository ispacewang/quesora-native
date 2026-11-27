package diagram

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type function func(float64) float64

var tokens = regexp.MustCompile(`[a-zA-Z]+[0-9]*|(?:[0-9]+\.?[0-9]*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?|[+*/^()-]`)

var mathFunctions = map[string]func(float64) float64{
	"sin": math.Sin, "cos": math.Cos, "tan": math.Tan, "asin": math.Asin, "acos": math.Acos, "atan": math.Atan,
	"sqrt": math.Sqrt, "abs": math.Abs, "exp": math.Exp, "log": math.Log, "ln": math.Log,
	"log10": math.Log10, "floor": math.Floor, "ceil": math.Ceil, "sinh": math.Sinh, "cosh": math.Cosh,
}

// parseFunction accepts arithmetic with a single variable. Tokens never reach a JS runtime.
func parseFunction(raw string, vars map[string]float64) (function, error) {
	if len(raw) > 192 {
		return nil, fmt.Errorf("函数表达式过长")
	}
	raw = strings.ToLower(strings.Join(strings.Fields(raw), ""))
	raw = strings.ReplaceAll(raw, "math.", "")
	raw = strings.ReplaceAll(raw, "**", "^")
	ts := tokens.FindAllString(raw, -1)
	if strings.Join(ts, "") != raw || len(ts) > 100 {
		return nil, fmt.Errorf("不支持的函数表达式")
	}
	p := exprParser{tokens: ts, variables: vars}
	f := p.parse(0)
	if p.failed || p.at != len(ts) {
		return nil, fmt.Errorf("不支持的函数表达式")
	}
	return f, nil
}

type exprParser struct {
	tokens    []string
	variables map[string]float64
	at        int
	failed    bool
}

func (p *exprParser) peek() string {
	if p.at < len(p.tokens) {
		return p.tokens[p.at]
	}
	return ""
}
func (p *exprParser) pop() string {
	s := p.peek()
	if s != "" {
		p.at++
	}
	return s
}
func (p *exprParser) require(s string) {
	if p.pop() != s {
		p.failed = true
	}
}

var variableName = regexp.MustCompile(`^[a-z][a-z0-9]{0,11}$`)
var priority = map[string]int{"+": 1, "-": 1, "*": 2, "/": 2, "^": 4}

func (p *exprParser) parse(minimum int) function {
	token := p.pop()
	var left function
	switch token {
	case "+":
		left = p.parse(3)
	case "-":
		arg := p.parse(3)
		left = func(x float64) float64 { return -arg(x) }
	case "(":
		left = p.parse(0)
		p.require(")")
	case "x":
		left = func(x float64) float64 { return x }
	case "pi":
		left = func(float64) float64 { return math.Pi }
	case "e":
		left = func(float64) float64 { return math.E }
	default:
		if v, ok := p.variables[token]; ok {
			left = func(float64) float64 { return v }
		} else if f := mathFunctions[token]; f != nil {
			p.require("(")
			arg := p.parse(0)
			p.require(")")
			left = func(x float64) float64 { return f(arg(x)) }
		} else if v, err := strconv.ParseFloat(token, 64); err == nil && finite(v) {
			left = func(float64) float64 { return v }
		} else {
			p.failed = true
			return func(float64) float64 { return math.NaN() }
		}
	}
	for !p.failed {
		op := p.peek()
		prec := priority[op]
		if prec == 0 || prec < minimum {
			break
		}
		p.pop()
		next := prec + 1
		if op == "^" {
			next = prec
		}
		a, b := left, p.parse(next)
		left = func(x float64) float64 {
			l, r := a(x), b(x)
			switch op {
			case "+":
				return l + r
			case "-":
				return l - r
			case "*":
				return l * r
			case "/":
				return l / r
			default:
				return math.Pow(l, r)
			}
		}
	}
	return left
}
