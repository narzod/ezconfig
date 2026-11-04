package main

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal reports whether f is a terminal: it answers a request for its
// terminal settings. /dev/null is a character device but doesn't.
func isTerminal(f *os.File) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&t)))
	return errno == 0
}

// enableVT: Unix terminals understand colour codes already.
func enableVT(*os.File) bool { return true }
