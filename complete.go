package main

// complete.go - context-aware tab completion.
//
// liner's WordCompleter hands us the whole line and the cursor position;
// we return (head, candidates, tail) and liner splices the chosen
// candidate between head and tail. Because liner inserts candidates
// verbatim, each candidate carries its own trailing text: a space after
// a finished word, "." after a section name, "=" after a key for `set`.

import "strings"

var commands = []string{
	"set", "get", "unset", "wipe", "copy", "move", "keys", "sections", "list", "dump",
	"status", "changes", "path", "save", "revert", "history", "delim", "version", "help", "quit",
}

func (r *Repl) complete(line string, pos int) (head string, cands []string, tail string) {
	before, tail := line[:pos], line[pos:]
	cut := strings.LastIndex(before, " ") + 1
	head, word := before[:cut], before[cut:]
	args := strings.Fields(head)

	// First word: the command itself.
	if len(args) == 0 {
		return head, withPrefix(commands, word, " "), tail
	}

	switch args[0] {
	case "get", "unset":
		cands = r.keyPaths(word, " ")

	case "wipe":
		cands = r.sectionNames(word, " ")

	case "copy", "move":
		if len(args) == 1 { // SRC only; DEST is a new name
			cands = r.sectionNames(word, " ")
		}

	case "history":
		if len(args) == 1 {
			cands = withPrefix([]string{"on", "off"}, word, "")
		}

	case "delim":
		if len(args) == 1 { // suggest the common delimiters not in use
			var alts []string
			for _, s := range []string{".", "/", ":", "|"} {
				if s != r.ini.delim {
					alts = append(alts, s)
				}
			}
			cands = withPrefix(alts, word, "")
		}

	case "set":
		if len(args) != 1 { // only one KEY=VALUE per set
			break
		}
		if path, _, ok := strings.Cut(word, "="); ok {
			// After "=": offer the current value so it can be edited.
			if v, found := r.ini.Get(path); found {
				cands = []string{path + "=" + v}
			}
		} else {
			cands = r.keyPaths(word, "=")
		}

	case "keys", "list", "dump", "changes":
		if len(args) == 1 {
			cands = r.sectionNames(word, "")
		}
	}
	return head, cands, tail
}

// sectionNames offers the root marker (the delimiter) first, then the
// section names, for commands that take a SECTION.
func (r *Repl) sectionNames(word, suffix string) []string {
	var out []string
	if strings.HasPrefix(r.ini.delim, word) {
		out = append(out, r.ini.delim+suffix)
	}
	return append(out, withPrefix(r.ini.Sections(), word, suffix)...)
}

// keyPaths completes a key address. Without a delimiter it offers root
// keys (with suffix) and section names (with the delimiter); after "sec."
// it offers that section's keys.
func (r *Repl) keyPaths(word, suffix string) []string {
	sep := r.ini.delim
	if sec, partial, ok := strings.Cut(word, sep); ok {
		if sec == "" {
			return nil
		}
		keys, _ := r.ini.Keys(sec)
		var out []string
		for _, k := range withPrefix(keys, partial, suffix) {
			out = append(out, sec+sep+k)
		}
		return out
	}
	roots, _ := r.ini.Keys("")
	out := withPrefix(roots, word, suffix)
	return append(out, withPrefix(r.ini.Sections(), word, sep)...)
}

// withPrefix filters items by prefix, sorts them, and appends suffix.
func withPrefix(items []string, prefix, suffix string) []string {
	var out []string
	for _, it := range sortedCopy(items) {
		if strings.HasPrefix(it, prefix) {
			out = append(out, it+suffix)
		}
	}
	return out
}
