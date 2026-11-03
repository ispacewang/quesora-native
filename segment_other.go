//go:build !windows

// 非 Windows 平台不调用 ICU；返回 nil 让上层使用可移植的规则分词。

package main

func nativeWords(s string) []string { return nil }
