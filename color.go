package main

// color.go - a little colour, the way git uses it.
//
// Colour is only for a person reading a terminal: it is off when output
// goes to a file or another program, and when NO_COLOR is set or TERM is
// dumb (conventions shared by many tools). It only ever marks something
// the text already says, so plain output reads the same: change lines
// keep their + - ~, errors and notes keep their words, headings keep
// their brackets. Data (get, list, emit, keys...) is never coloured.

import (
	"os"
	"strings"
)

var colorOut, colorErr bool // colour on standard output, standard error

// setupColor decides, once at startup, whether each stream gets colour.
func setupColor() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return
	}
	colorOut = isTerminal(os.Stdout) && enableVT(os.Stdout)
	colorErr = isTerminal(os.Stderr) && enableVT(os.Stderr)
}

const (
	styleBold   = "1"
	styleDim    = "2"
	styleRed    = "31"
	styleGreen  = "32"
	styleYellow = "33"
)

func paint(on bool, style, s string) string {
	if !on || s == "" {
		return s
	}
	return "\x1b[" + style + "m" + s + "\x1b[0m"
}

// out styles text bound for standard output.
func out(style, s string) string { return paint(colorOut, style, s) }

// changeText is a change line as the shell shows it: added green, removed
// red, changed yellow, like git diff.
func changeText(c change, delim string) string {
	style := styleYellow
	switch c.mark {
	case '+':
		style = styleGreen
	case '-':
		style = styleRed
	}
	return out(style, c.line(delim))
}

// noteText marks a "note:" line's prefix yellow.
func noteText(s string) string {
	if rest, ok := strings.CutPrefix(s, "note:"); ok {
		return out(styleYellow, "note:") + rest
	}
	return s
}
