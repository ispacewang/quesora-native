package main

import "testing"

func TestAnswerState(t *testing.T) {
	a := newApp()
	a.submit()
	if len(a.answers) != 0 {
		t.Fatal("empty answer submitted")
	}
	a.choose(1)
	a.submit()
	a.submit()
	good, bad := a.counts()
	if good != 1 || bad != 0 {
		t.Fatalf("counts %d/%d", good, bad)
	}
	a.navigate(1)
	a.choose(0)
	a.choose(2)
	a.choose(3)
	a.submit()
	good, _ = a.counts()
	if good != 2 {
		t.Fatal("multi-select grading")
	}
	a.navigate(2)
	a.choose(0)
	a.submit()
	_, bad = a.counts()
	if bad != 1 {
		t.Fatal("wrong-answer collection")
	}
	a.filter = "填空题"
	a.navigate(0)
	a.typed = " 1 "
	a.submit()
	good, _ = a.counts()
	if good != 3 {
		t.Fatal("fill-in grading")
	}
	a.bank = "计算机基础"
	a.filter = "全部"
	a.navigate(0)
	good, bad = a.counts()
	if good != 0 || bad != 0 {
		t.Fatal("bank isolation")
	}
}
