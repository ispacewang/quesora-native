//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var icu = syscall.NewLazyDLL(filepath.Join(os.Getenv("SystemRoot"), "System32", "icu.dll"))
var breakOpen = icu.NewProc("ubrk_open")
var breakClose = icu.NewProc("ubrk_close")
var breakNext = icu.NewProc("ubrk_next")
var breakStatus = icu.NewProc("ubrk_getRuleStatus")

// Windows ICU uses the same segmentation family as Intl.Segmenter.
// https://learn.microsoft.com/en-us/windows/win32/intl/international-components-for-unicode--icu-
func nativeWords(s string) []string {
	for _, p := range []*syscall.LazyProc{breakOpen, breakClose, breakNext, breakStatus} {
		if p.Find() != nil {
			return nil
		}
	}
	data := utf16.Encode([]rune(s))
	if len(data) == 0 {
		return []string{}
	}
	locale := []byte{'z', 'h', 0}
	var status int32
	iterator, _, _ := breakOpen.Call(1, uintptr(unsafe.Pointer(&locale[0])), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&status)))
	if iterator == 0 || status > 0 {
		if iterator != 0 {
			breakClose.Call(iterator)
		}
		return nil
	}
	defer runtime.KeepAlive(data)
	defer runtime.KeepAlive(locale)
	defer breakClose.Call(iterator)
	out := []string{}
	start := 0
	for {
		end, _, _ := breakNext.Call(iterator)
		pos := int(int32(end))
		if pos < 0 {
			break
		}
		if pos > len(data) || pos < start {
			return nil
		}
		kind, _, _ := breakStatus.Call(iterator)
		if kind >= 100 {
			out = append(out, string(utf16.Decode(data[start:pos])))
		}
		start = pos
	}
	return out
}
