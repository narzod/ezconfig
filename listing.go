package main

// listing.go - list in its three formats, and export.
//
//	list [FORMAT] [SECTION]   flat (KEY=VALUE lines, the default), ini or json
//	export [FILE]             the JSON, written to a file
//
// All three formats show the same thing: the effective config, with the
// schema's defaults filled in and only valid values. If the first word is
// a format it is the format, and what follows is the section; a section
// named flat, ini or json is written "name." (list json json.).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var listFormats = map[string]bool{"flat": true, "ini": true, "json": true}

const listUsage = "usage: list [flat|ini|json] [SECTION]"

// parseList reads list's arguments: an optional format word, then an
// optional section ("" for everything).
func parseList(args []string, delim string) (format, sec string, err error) {
	format = "flat"
	if len(args) > 0 && listFormats[args[0]] {
		format, args = args[0], args[1:]
	}
	switch len(args) {
	case 0:
	case 1:
		sec = sectionArgs(args, delim)[0]
	default:
		return "", "", errors.New(listUsage)
	}
	return format, sec, nil
}

// listing is what list produces: the text, whether the section exists,
// root keys json had to rename, and, for json, a config it can't represent.
type listing struct {
	text    string
	found   bool
	lines   []string // flat: the lines, for colouring
	addrs   []string // flat: each line's key
	renamed []string
	err     error
}

// render produces list's output in format, without colour.
func render(view *Ini, format, sec string) listing {
	switch format {
	case "json":
		out, ok, renamed, err := view.JSON(sec)
		return listing{text: out, found: ok, renamed: renamed, err: err}
	case "ini":
		out, ok := view.INI(sec)
		return listing{text: out, found: ok}
	}
	lines, ok := view.List(sec)
	addrs, _ := view.KeyPaths(sec)
	text := ""
	if len(lines) > 0 {
		text = strings.Join(lines, "\n") + "\n"
	}
	return listing{text: text, found: ok, lines: lines, addrs: addrs}
}

// INI renders the config, or one section, as INI text: root keys first,
// then each section under its header, with no comments, since it shows the
// effective values rather than the file.
func (ini *Ini) INI(sec string) (string, bool) {
	var b strings.Builder
	entries := func(es []entry) {
		for _, e := range es {
			fmt.Fprintf(&b, "%s=%s\n", e.key, e.value)
		}
	}
	section := func(s *section) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[%s]\n", s.name)
		entries(s.entries)
	}
	switch {
	case sec == "":
		entries(ini.root.entries)
		for _, s := range ini.sections {
			section(s)
		}
	case sec == ini.delim:
		entries(ini.root.entries)
	default:
		s := ini.section(sec)
		if s == nil {
			return "", false
		}
		section(s)
	}
	return b.String(), true
}

// colourListing styles list's output for the shell's terminal: keys cyan
// in flat and ini, section headers bold in ini, JSON as jq colours it.
func colourListing(l listing, format string) string {
	if !colorOut {
		return l.text
	}
	switch format {
	case "json":
		return jsonText(l.text)
	case "ini":
		var b strings.Builder
		for _, line := range strings.SplitAfter(l.text, "\n") {
			body := strings.TrimSuffix(line, "\n")
			switch {
			case strings.HasPrefix(body, "["):
				b.WriteString(out(styleBold, body))
			case body == "":
			default:
				k, v, _ := strings.Cut(body, "=") // a key can't contain "="
				b.WriteString(out(styleCyan, k) + "=" + v)
			}
			if strings.HasSuffix(line, "\n") {
				b.WriteString("\n")
			}
		}
		return b.String()
	}
	text := strings.Join(listText(l.addrs, l.lines), "\n")
	if text != "" {
		text += "\n"
	}
	return text
}

// exportTarget is where export writes: FILE if given, otherwise the
// config's name with .json, next to it (config.ini gives config.json).
func exportTarget(configPath, file string) string {
	if file != "" {
		if abs, err := filepath.Abs(file); err == nil {
			return abs
		}
		return file
	}
	return strings.TrimSuffix(configPath, filepath.Ext(configPath)) + ".json"
}

// samePath reports whether two paths name the same file, ignoring case
// on Windows, whether or not the files exist yet.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if fa, err1 := os.Stat(a); err1 == nil {
		if fb, err2 := os.Stat(b); err2 == nil {
			return os.SameFile(fa, fb)
		}
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// exportJSON writes view's JSON to target, atomically. It refuses to
// write over the config or the schema, and writes nothing when the JSON
// can't represent the config.
// code is the exit status a failure means: 4 for a target that is the
// config or schema, 2 for a config JSON can't represent, 3 for a failed
// write.
func exportJSON(view *Ini, configPath, schemaPath, target string) (renamed []string, code int, err error) {
	if samePath(target, configPath) || samePath(target, schemaPath) {
		return nil, exitArgs, fmt.Errorf("export: %s is the config or the schema itself; choose another file", target)
	}
	out, _, renamed, jerr := view.JSON("")
	if jerr != nil {
		return nil, exitSyntax, fmt.Errorf("export: %v; nothing written", jerr)
	}
	if err := writeAtomic(target, []byte(out)); err != nil {
		return nil, exitFile, fmt.Errorf("export %s: %v", target, err)
	}
	return renamed, exitOK, nil
}

// exportNote is the message after a successful export.
func exportNote(view *Ini, target string) string {
	n, _ := view.KeyCount()
	return fmt.Sprintf("exported %s to %s", plural(n, "key"), target)
}

// removedNote helps people who type a command that has been replaced.
func removedNote(cmd string) string {
	if cmd == "emit" {
		return "emit is now list json (and export writes the JSON to a file)"
	}
	return ""
}
