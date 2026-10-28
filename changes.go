package main

// changes.go - unsaved changes: what differs between the config in memory
// and the config as loaded or last saved.
//
// The difference itself is the record: setting a key back to its saved
// value leaves nothing to save, so it isn't counted. The prompt ends in
// "*>" instead of "=>" while there is anything unsaved, and the changes
// command lists it:
//
//	~ net.port=9090 (was 80)
//	+ motd=hello
//	- name=demo

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Clone returns an independent copy of the Ini.
func (ini *Ini) Clone() *Ini {
	c := &Ini{delim: ini.delim}
	c.root.entries = append([]entry(nil), ini.root.entries...)
	for _, s := range ini.sections {
		c.sections = append(c.sections, &section{name: s.name, entries: append([]entry(nil), s.entries...)})
	}
	return c
}

// change is one line of the difference. sec is "" for the root keys.
// A change with no key is a section added or removed with no keys in it,
// which the key lines alone wouldn't show.
type change struct {
	mark     byte // '~' changed, '+' added, '-' removed
	sec, key string
	val, was string
}

// line renders a change using delim to join section and key.
func (c change) line(delim string) string {
	if c.key == "" {
		return fmt.Sprintf("%c [%s]", c.mark, c.sec)
	}
	path := c.key
	if c.sec != "" {
		path = c.sec + delim + c.key
	}
	switch c.mark {
	case '~':
		return fmt.Sprintf("~ %s=%s (was %s)", path, c.val, c.was)
	case '-':
		return fmt.Sprintf("- %s=%s", path, c.was)
	}
	return fmt.Sprintf("+ %s=%s", path, c.val)
}

// diffIni lists how cur differs from old, in cur's order, with each
// section's removed keys after its remaining ones and sections that only
// old has at the end.
func diffIni(old, cur *Ini) []change {
	var out []change
	compare := func(name string, o, c *section) {
		if c != nil {
			for _, e := range c.entries {
				switch i := findIn(o, e.key); {
				case i < 0:
					out = append(out, change{mark: '+', sec: name, key: e.key, val: e.value})
				case o.entries[i].value != e.value:
					out = append(out, change{mark: '~', sec: name, key: e.key, val: e.value, was: o.entries[i].value})
				}
			}
		}
		if o != nil {
			for _, e := range o.entries {
				if findIn(c, e.key) < 0 {
					out = append(out, change{mark: '-', sec: name, key: e.key, was: e.value})
				}
			}
		}
		switch { // an empty section's arrival or departure has no key lines
		case o == nil && c != nil && len(c.entries) == 0:
			out = append(out, change{mark: '+', sec: name})
		case c == nil && o != nil && len(o.entries) == 0:
			out = append(out, change{mark: '-', sec: name})
		}
	}
	compare("", &old.root, &cur.root)
	for _, s := range cur.sections {
		compare(s.name, old.section(s.name), s)
	}
	for _, s := range old.sections {
		if cur.section(s.name) == nil {
			compare(s.name, s, nil)
		}
	}
	return out
}

func findIn(s *section, key string) int {
	if s == nil {
		return -1
	}
	return s.find(key)
}

// unsaved counts the differences from the saved config.
func (r *Repl) unsaved() int {
	return len(diffIni(r.saved, r.ini))
}

// prompt is the config's parent folder and file name, then "=>" when
// everything is saved or "*>" while anything is unsaved. Both are two
// characters, so typed commands always start in the same column.
func (r *Repl) prompt() string {
	mark := "=> "
	if r.unsaved() > 0 {
		mark = "*> "
	}
	return promptLabel(r.path) + mark
}

// promptLabel shortens an absolute config path to its parent folder and
// file name: C:\SignLoop\config\config.ini becomes config\config.ini. A
// config at the top of a drive or the filesystem keeps the root instead,
// as in C:\config.ini or /config.ini.
func promptLabel(path string) string {
	dir, name := filepath.Dir(path), filepath.Base(path)
	if filepath.Dir(dir) == dir { // dir is a root: "/", `C:\`, `\\server\share\`
		if strings.HasSuffix(dir, string(filepath.Separator)) {
			return dir + name
		}
		return dir + string(filepath.Separator) + name
	}
	return filepath.Join(filepath.Base(dir), name)
}

// changesCmd implements: changes [SECTION]. The delimiter on its own
// limits it to the root keys.
func (r *Repl) changesCmd(args []string) {
	if len(args) > 1 {
		errorf("usage: changes [SECTION]")
		return
	}
	sec, filtered := firstOr(args, ""), len(args) == 1
	if filtered && sec == r.ini.delim {
		sec = ""
	} else if filtered && r.ini.section(sec) == nil && r.saved.section(sec) == nil {
		errorf("no such section: %s", sec)
		return
	}
	n := 0
	for _, c := range diffIni(r.saved, r.ini) {
		if filtered && c.sec != sec {
			continue
		}
		fmt.Println(c.line(r.ini.delim))
		n++
	}
	if n == 0 {
		switch {
		case !filtered:
			fmt.Println("no unsaved changes")
		case sec == "":
			fmt.Println("no unsaved changes to the root keys")
		default:
			fmt.Printf("no unsaved changes in %s\n", sec)
		}
	}
	if r.changedOnDisk() {
		fmt.Printf("note: %s changed on disk since it was loaded\n", filepath.Base(r.path))
	}
}
