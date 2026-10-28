//go:build !windows

package main

// hideDir does nothing outside Windows: the leading dot in ".inirepl"
// already hides the folder.
func hideDir(dir string) {}
