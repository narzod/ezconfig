package main

import (
	"os"
	"syscall"
)

var procSetConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")

// enableVT asks the console to understand colour codes (Windows 10 and
// later). It reports false where the console can't, so no codes are sent.
func enableVT(f *os.File) bool {
	const enableVirtualTerminalProcessing = 0x0004
	h := syscall.Handle(f.Fd())
	var mode uint32
	if syscall.GetConsoleMode(h, &mode) != nil {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	ok, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(mode|enableVirtualTerminalProcessing))
	return ok != 0
}

// isTerminal reports whether f is a console: it has a console mode. NUL
// is a character device but doesn't.
func isTerminal(f *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode) == nil
}
