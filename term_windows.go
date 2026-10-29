package main

import (
	"os"
	"syscall"
)

// isTerminal reports whether f is a console: it has a console mode. NUL
// is a character device but doesn't.
func isTerminal(f *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode) == nil
}
