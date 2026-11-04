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
	styleCyan   = "36"
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

// listText is list's KEY=VALUE lines for the shell, with each key in cyan.
// addrs are the keys in the same order as lines, so a line is split where
// its key ends rather than at an "=", which a section name may contain.
func listText(addrs, lines []string) []string {
	if !colorOut || len(addrs) != len(lines) {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if !strings.HasPrefix(l, addrs[i]+"=") {
			out[i] = l
			continue
		}
		out[i] = paint(true, styleCyan, addrs[i]) + l[len(addrs[i]):]
	}
	return out
}

// jsonText colours emit's JSON for the shell the way jq does: keys cyan,
// string values green, everything else plain.
func jsonText(js string) string {
	if !colorOut {
		return js
	}
	var b strings.Builder
	for i := 0; i < len(js); i++ {
		if js[i] != '"' {
			b.WriteByte(js[i])
			continue
		}
		j := i + 1 // find the closing quote, skipping escaped characters
		for j < len(js) && js[j] != '"' {
			if js[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(js) {
			b.WriteString(js[i:])
			break
		}
		str := js[i : j+1]
		k := j + 1
		for k < len(js) && (js[k] == ' ' || js[k] == '\t') {
			k++
		}
		if k < len(js) && js[k] == ':' {
			b.WriteString(paint(true, styleCyan, str))
		} else {
			b.WriteString(paint(true, styleGreen, str))
		}
		i = j
	}
	return b.String()
}
