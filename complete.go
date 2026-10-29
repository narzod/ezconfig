package main

// complete.go - context-aware tab completion.
//
// liner's WordCompleter hands us the whole line and the cursor position;
// we return (head, candidates, tail) and liner splices the chosen
// candidate between head and tail. Because liner inserts candidates
// verbatim, each candidate carries its own trailing text: a space after
// a finished word, "." after a section name, "=" after a key for `set`.
//
// Whatever part of the word is already settled - a section name and its
// delimiter, or a key and its "=" - is moved into head, so the candidates
// liner lists are only what remains to choose: "width  height" rather than
// "display.width=  display.height=".

import "strings"

var commands = []string{
	"set", "get", "unset", "wipe", "copy", "move", "keys", "sections", "list", "dump",
	"check", "describe", "status", "changes", "path", "save", "revert", "history", "delim", "version", "help", "quit",
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
	case "get":
		var settled string
		settled, cands = keyPaths(r.view(), word, " ") // keys with a value, defaults included
		head += settled

	case "describe":
		var settled string
		settled, cands = keyPaths(r.knownKeys(), word, " ") // every key the schema defines
		head += settled

	case "unset":
		var settled string
		settled, cands = keyPaths(r.ini, word, " ") // only what the config sets
		head += settled

	case "wipe":
		cands = sectionNames(r.ini, word, " ")

	case "copy", "move":
		if len(args) == 1 { // SRC only; DEST is a new name
			cands = sectionNames(r.ini, word, " ")
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
		if path, typed, ok := strings.Cut(word, "="); ok {
			// After "=": the allowed values, if the schema lists them,
			// otherwise the current value so it can be edited.
			head += path + "="
			if rl := r.schema.ruleFor(r.ini.splitPath(path)); rl != nil && rl.choices() != nil {
				for _, c := range rl.choices() {
					if strings.HasPrefix(strings.ToLower(c), strings.ToLower(typed)) {
						cands = append(cands, c)
					}
				}
			} else if v, found := r.view().Get(path); found {
				cands = []string{v}
			}
		} else {
			var settled string
			settled, cands = keyPaths(r.knownKeys(), word, "=")
			head += settled
		}

	case "keys", "list", "dump":
		if len(args) == 1 {
			cands = sectionNames(r.view(), word, "")
		}

	case "changes":
		if len(args) == 1 {
			cands = sectionNames(r.ini, word, "")
		}
	}
	return head, cands, tail
}

// sectionNames offers the root marker (the delimiter) first, then the
// section names of ini, for commands that take a SECTION.
func sectionNames(ini *Ini, word, suffix string) []string {
	var out []string
	if strings.HasPrefix(ini.delim, word) {
		out = append(out, ini.delim+suffix)
	}
	return append(out, withPrefix(ini.Sections(), word, suffix)...)
}

// keyPaths completes a key address. Without a delimiter it offers root
// keys (with suffix) and section names (with the delimiter). After "sec."
// that part is settled: it is returned separately, and the candidates are
// just that section's key names.
func keyPaths(ini *Ini, word, suffix string) (settled string, cands []string) {
	sep := ini.delim
	if sec, partial, ok := strings.Cut(word, sep); ok {
		if sec == "" {
			return "", nil
		}
		keys, _ := ini.Keys(sec)
		return sec + sep, withPrefix(keys, partial, suffix)
	}
	roots, _ := ini.Keys("")
	cands = withPrefix(roots, word, suffix)
	return "", append(cands, withPrefix(ini.Sections(), word, sep)...)
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

// knownKeys is the effective config plus every key the schema has a rule
// for, even with no value or default yet, so set and describe can offer
// keys like a required site code that nothing has set.
func (r *Repl) knownKeys() *Ini {
	known := r.view()
	if r.schema == nil {
		return known
	}
	known = known.Clone()
	for _, rl := range r.schema.rules {
		dst := &known.root
		if rl.sec != "" {
			dst = known.ensureSection(rl.sec)
		}
		if dst.find(rl.key) < 0 {
			dst.entries = append(dst.entries, entry{key: rl.key})
		}
	}
	return known
}
