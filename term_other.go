//go:build !linux && !windows && !darwin && !freebsd && !netbsd && !openbsd

package main

import "os"

// isTerminal guesses from the file type where there's no better check.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// enableVT: Unix terminals understand colour codes already.
func enableVT(*os.File) bool { return true }
