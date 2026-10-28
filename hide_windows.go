//go:build windows

package main

import "syscall"

// hideDir sets the Windows "hidden" attribute on dir. A leading dot hides
// files elsewhere, but not on Windows. Failure is ignored: the folder
// still works, it's just visible.
func hideDir(dir string) {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return
	}
	attrs, err := syscall.GetFileAttributes(p)
	if err != nil {
		return
	}
	syscall.SetFileAttributes(p, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}
