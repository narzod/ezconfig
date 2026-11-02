package main

// ini.go - a small in-memory INI model.
//
// Root keys (before any [section]) and section keys are kept in file order.
// Addressing: "key" is a root key, "section.key" is a key in a section
// (split on the first dot). Comments and blank lines are not preserved
// on save in this prototype.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type entry struct {
	key   string
	value string
	notes []string // comment lines just above the key, kept on save
}

type section struct {
	name    string
	entries []entry
	notes   []string // comment lines just above the [name] header
}

func (s *section) find(key string) int {
	for i, e := range s.entries {
		if e.key == key {
			return i
		}
	}
	return -1
}

type Ini struct {
	root     section
	sections []*section

	// Comments that belong to the file rather than to a key or section: a
	// block at the very top followed by a blank line, and any at the end.
	head, tail []string

	// delim separates section and key in an address ("net.port"); on its
	// own it names the root (the keys before any section).
	delim string
}

func NewIni() *Ini { return &Ini{delim: "."} }

// SyntaxError reports a line of the file that isn't valid INI.
type SyntaxError struct {
	Line int
	Msg  string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

// LoadIni reads path into a new Ini using delimiter sep.
// A missing file yields an empty Ini.
func LoadIni(path, sep string) (*Ini, error) {
	ini := NewIni()
	ini.delim = sep
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return ini, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ini, ini.parse(f)
}

func (ini *Ini) parse(r io.Reader) error {
	// Comment lines are kept with whatever follows them: a key or a
	// section header. A comment block at the very top that a blank line
	// separates from the rest is the file's own; so is one at the end.
	cur := &ini.root
	sc := bufio.NewScanner(r)
	n := 0
	var pending []string
	started := false // a key or section has been read
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
			if !started && len(pending) > 0 && ini.head == nil {
				ini.head, pending = pending, nil
			}
			continue
		case line[0] == ';' || line[0] == '#':
			pending = append(pending, line)
			continue
		}
		started = true
		if line[0] == '[' {
			if !strings.HasSuffix(line, "]") {
				return &SyntaxError{n, "unterminated section header"}
			}
			cur = ini.ensureSection(strings.TrimSpace(line[1 : len(line)-1]))
			cur.notes = append(cur.notes, pending...)
			pending = nil
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return &SyntaxError{n, "expected key=value"}
		}
		setIn(cur, strings.TrimSpace(k), strings.TrimSpace(v))
		if len(pending) > 0 {
			i := cur.find(strings.TrimSpace(k))
			cur.entries[i].notes = append(cur.entries[i].notes, pending...)
			pending = nil
		}
	}
	if !started && ini.head == nil {
		ini.head = pending
	} else {
		ini.tail = pending
	}
	return sc.Err()
}

func setIn(s *section, key, value string) {
	if i := s.find(key); i >= 0 {
		s.entries[i].value = value
		return
	}
	s.entries = append(s.entries, entry{key: key, value: value})
}

func (ini *Ini) section(name string) *section {
	for _, s := range ini.sections {
		if s.name == name {
			return s
		}
	}
	return nil
}

func (ini *Ini) ensureSection(name string) *section {
	if s := ini.section(name); s != nil {
		return s
	}
	s := &section{name: name}
	ini.sections = append(ini.sections, s)
	return s
}

// sectionOrRoot finds a section by name, where the delimiter on its own
// names the root. It returns nil if there is no such section.
func (ini *Ini) sectionOrRoot(name string) *section {
	if name == ini.delim {
		return &ini.root
	}
	return ini.section(name)
}

// splitPath turns "sec.key" into ("sec","key") and "key" into ("","key"),
// splitting on the first delimiter.
func (ini *Ini) splitPath(path string) (sec, key string) {
	if s, k, ok := strings.Cut(path, ini.delim); ok {
		return s, k
	}
	return "", path
}

func (ini *Ini) Get(path string) (string, bool) {
	sec, key := ini.splitPath(path)
	s := &ini.root
	if sec != "" {
		if s = ini.section(sec); s == nil {
			return "", false
		}
	}
	if i := s.find(key); i >= 0 {
		return s.entries[i].value, true
	}
	return "", false
}

// CheckSet reports whether path=value could be stored without producing
// a file that reads back differently: names must be non-empty, contain no
// brackets or line breaks, and keys must not look like comments; values
// must be a single line.
func (ini *Ini) CheckSet(path, value string) error {
	sec, key := ini.splitPath(path)
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("missing key name in %q", path)
	}
	if strings.ContainsAny(key, "[]\r\n") || strings.HasPrefix(key, ";") || strings.HasPrefix(key, "#") {
		return fmt.Errorf("invalid key name in %q", path)
	}
	if strings.ContainsAny(sec, "[]\r\n") {
		return fmt.Errorf("invalid section name in %q", path)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("value for %q must be a single line", path)
	}
	return nil
}

func (ini *Ini) Set(path, value string) error {
	if err := ini.CheckSet(path, value); err != nil {
		return err
	}
	sec, key := ini.splitPath(path)
	s := &ini.root
	if sec != "" {
		s = ini.ensureSection(sec)
	}
	setIn(s, key, value)
	return nil
}

func (ini *Ini) Unset(path string) bool {
	sec, key := ini.splitPath(path)
	s := &ini.root
	if sec != "" {
		if s = ini.section(sec); s == nil {
			return false
		}
	}
	i := s.find(key)
	if i < 0 {
		return false
	}
	s.entries = append(s.entries[:i], s.entries[i+1:]...)
	return true
}

func (ini *Ini) UnsetSection(name string) bool {
	for i, s := range ini.sections {
		if s.name == name {
			ini.sections = append(ini.sections[:i], ini.sections[i+1:]...)
			return true
		}
	}
	return false
}

// ClearRoot removes every root key, leaving the sections alone.
func (ini *Ini) ClearRoot() { ini.root.entries = nil }

// CopySection copies src's keys to dst. Either may be the delimiter,
// meaning the root. A new section dst is appended; it must not already
// exist (as a section or a root key) and must be a usable name. Copying
// into the root is allowed only while the root is empty, and refuses keys
// that would share a name with a section.
func (ini *Ini) CopySection(src, dst string) error {
	s := ini.sectionOrRoot(src)
	if s == nil {
		return fmt.Errorf("no such section: %s", src)
	}
	if dst == ini.delim {
		if src == ini.delim {
			return fmt.Errorf("cannot copy the root to itself")
		}
		if err := ini.checkIntoRoot(s, nil); err != nil {
			return err
		}
		ini.root.entries = append([]entry(nil), s.entries...)
		return nil
	}
	if err := ini.checkNewSection(dst); err != nil {
		return err
	}
	entries := append([]entry(nil), s.entries...)
	ini.sections = append(ini.sections, &section{name: dst, entries: entries})
	return nil
}

// MoveSection moves src's keys to dst and removes src. Either may be the
// delimiter, meaning the root. Moving one section to a new name renames
// it in place, keeping its position in the file; moving the root keys
// into a section appends that section. The same checks as CopySection
// apply to dst.
func (ini *Ini) MoveSection(src, dst string) error {
	s := ini.sectionOrRoot(src)
	if s == nil {
		return fmt.Errorf("no such section: %s", src)
	}
	switch {
	case src == dst:
		return fmt.Errorf("%s is already there", src)
	case dst == ini.delim: // a section's keys become the root keys
		if err := ini.checkIntoRoot(s, s); err != nil {
			return err
		}
		ini.root.entries = s.entries
		ini.UnsetSection(src)
		return nil
	}
	if err := ini.checkNewSection(dst); err != nil {
		return err
	}
	if src == ini.delim { // the root keys become a new section
		ini.sections = append(ini.sections, &section{name: dst, entries: ini.root.entries})
		ini.ClearRoot()
		return nil
	}
	s.name = dst // a rename, in place
	return nil
}

// checkNewSection reports whether dst can be created as a new section.
func (ini *Ini) checkNewSection(dst string) error {
	if dst == "" || strings.Contains(dst, ini.delim) || strings.ContainsAny(dst, "[]= \t") {
		return fmt.Errorf("invalid section name: %s", dst)
	}
	if ini.section(dst) != nil {
		return fmt.Errorf("section exists: %s", dst)
	}
	if _, ok := ini.Get(dst); ok {
		return fmt.Errorf("a root key named %s exists", dst)
	}
	return nil
}

// checkIntoRoot reports whether s's keys can become the root keys: the
// root must be empty and no key may share its name with a section. leaving
// is a section about to be removed (the source of a move), which doesn't
// count; nil when copying.
func (ini *Ini) checkIntoRoot(s, leaving *section) error {
	if len(ini.root.entries) > 0 {
		return fmt.Errorf("the root is not empty (wipe %s first)", ini.delim)
	}
	for _, e := range s.entries {
		if other := ini.section(e.key); other != nil && other != leaving {
			return fmt.Errorf("key %s would share its name with section %s", e.key, e.key)
		}
	}
	return nil
}

// KeyPaths returns full key addresses in file order: every key when sec
// is "", only the root keys when sec is the delimiter, otherwise only
// that section's. They are list's lines without the values. ok is false
// if sec doesn't exist.
func (ini *Ini) KeyPaths(sec string) (paths []string, ok bool) {
	lines, ok := ini.List(sec)
	for _, l := range lines {
		p, _, _ := strings.Cut(l, "=") // keys never contain "="
		paths = append(paths, p)
	}
	return paths, ok
}

// Keys returns key names in file order: root keys when sec is "" or the
// delimiter, otherwise the keys of that section. ok is false if sec
// doesn't exist.
func (ini *Ini) Keys(sec string) (keys []string, ok bool) {
	s := &ini.root
	if sec != "" {
		if s = ini.sectionOrRoot(sec); s == nil {
			return nil, false
		}
	}
	for _, e := range s.entries {
		keys = append(keys, e.key)
	}
	return keys, true
}

func (ini *Ini) Sections() []string {
	names := make([]string, 0, len(ini.sections))
	for _, s := range ini.sections {
		names = append(names, s.name)
	}
	return names
}

// Save writes the Ini to path, root keys first, then each section.
func (ini *Ini) Save(path string) error {
	return writeAtomic(path, []byte(ini.Text()))
}

// Text is the config as it is saved: comments kept with what follows
// them, a blank line before each section.
func (ini *Ini) Text() string {
	var b strings.Builder
	lines := func(ls []string) {
		for _, l := range ls {
			b.WriteString(l + "\n")
		}
	}
	lines(ini.head)
	gap := func() {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
	}
	if len(ini.head) > 0 && len(ini.root.entries) > 0 {
		gap()
	}
	for _, e := range ini.root.entries {
		lines(e.notes)
		fmt.Fprintf(&b, "%s=%s\n", e.key, e.value)
	}
	for _, s := range ini.sections {
		gap()
		lines(s.notes)
		fmt.Fprintf(&b, "[%s]\n", s.name)
		for _, e := range s.entries {
			lines(e.notes)
			fmt.Fprintf(&b, "%s=%s\n", e.key, e.value)
		}
	}
	if len(ini.tail) > 0 {
		gap()
		lines(ini.tail)
	}
	return b.String()
}

// writeAtomic writes data to a temporary file in path's folder, then
// renames it over path, so a program reading path never sees a partly
// written file. An existing file's permissions are kept.
func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ezconfig-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		// Best effort: some filesystems (e.g. Android shared storage)
		// don't support permissions, and that shouldn't block a save.
		os.Chmod(name, mode)
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}

// List returns "path=value" lines in file order: everything when sec is
// "", only the root keys when sec is the delimiter, otherwise only that
// section. ok is false if sec doesn't exist.
func (ini *Ini) List(sec string) (lines []string, ok bool) {
	add := func(prefix string, s *section) {
		for _, e := range s.entries {
			lines = append(lines, prefix+e.key+"="+e.value)
		}
	}
	switch sec {
	case "":
	case ini.delim:
		add("", &ini.root)
		return lines, true
	default:
		s := ini.section(sec)
		if s == nil {
			return nil, false
		}
		add(sec+ini.delim, s)
		return lines, true
	}
	add("", &ini.root)
	for _, s := range ini.sections {
		add(s.name+ini.delim, s)
	}
	return lines, true
}

// NamesContaining counts section and key names that contain s; such names
// cannot be addressed when s is the delimiter.
func (ini *Ini) NamesContaining(s string) int {
	n := 0
	check := func(sec *section) {
		for _, e := range sec.entries {
			if strings.Contains(e.key, s) {
				n++
			}
		}
	}
	check(&ini.root)
	for _, sec := range ini.sections {
		if strings.Contains(sec.name, s) {
			n++
		}
		check(sec)
	}
	return n
}

// KeyCount returns the total number of keys and the number of root keys.
func (ini *Ini) KeyCount() (total, root int) {
	root = len(ini.root.entries)
	total = root
	for _, s := range ini.sections {
		total += len(s.entries)
	}
	return total, root
}

// JSON renders the config, or one section, as a JSON object of strings:
// root keys at the top level, each section as an object. A root key and a
// section can share a name in INI but not in one JSON object, so such a
// root key is shown with "=" after its name ("net="), which no INI key can
// contain; renamed lists the root keys shown that way. A section's name can
// contain "=", though, so if that new name is already a section's, JSON
// can't hold both: err says so, and out is empty.
func (ini *Ini) JSON(sec string) (out string, ok bool, renamed []string, err error) {
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	if sec != "" {
		s := ini.sectionOrRoot(sec)
		if s == nil {
			return "", false, nil, nil
		}
		if len(s.entries) == 0 {
			return "{}\n", true, nil, nil
		}
		var members []string
		for _, e := range s.entries {
			members = append(members, "  "+q(e.key)+": "+q(e.value))
		}
		return "{\n" + strings.Join(members, ",\n") + "\n}\n", true, nil, nil
	}
	var members []string
	for _, e := range ini.root.entries {
		name := e.key
		if ini.section(e.key) != nil {
			name += "="
			if ini.section(name) != nil {
				return "", true, nil, fmt.Errorf("root key %q would be shown as %q, but [%s] is also a section; JSON can't hold both. Rename the section, or use emit %s and emit %s and emit %s separately",
					e.key, name, name, ini.delim, e.key, name)
			}
			renamed = append(renamed, e.key)
		}
		members = append(members, "  "+q(name)+": "+q(e.value))
	}
	for _, s := range ini.sections {
		if len(s.entries) == 0 {
			members = append(members, "  "+q(s.name)+": {}")
			continue
		}
		var inner []string
		for _, e := range s.entries {
			inner = append(inner, "    "+q(e.key)+": "+q(e.value))
		}
		members = append(members, "  "+q(s.name)+": {\n"+strings.Join(inner, ",\n")+"\n  }")
	}
	if len(members) == 0 {
		return "{}\n", true, renamed, nil
	}
	return "{\n" + strings.Join(members, ",\n") + "\n}\n", true, renamed, nil
}

// OddSections lists sections whose names contain "=" or brackets, as
// problems for check: other INI readers accept them, but ezconfig never
// creates them, and a name ending in "=" can be confused with a root key
// that emit renamed.
func (ini *Ini) OddSections() []string {
	var out []string
	for _, s := range ini.sections {
		if strings.ContainsAny(s.name, "=[]") {
			out = append(out, fmt.Sprintf("[%s]: a section name containing \"=\" or brackets can't always be told apart in emit or addressed by set; rename the section", s.name))
		}
	}
	return out
}

// Clashes lists root keys that share their name with a section, as
// problems for check: INI allows it, but emit has to rename the key.
func (ini *Ini) Clashes() []string {
	var out []string
	for _, e := range ini.root.entries {
		if ini.section(e.key) != nil {
			out = append(out, fmt.Sprintf("%s: a root key and a section share this name; emit shows the key as %s=", e.key, e.key))
		}
	}
	return out
}

// renameNote is the warning emit gives for a renamed root key.
func renameNote(key string) string {
	return fmt.Sprintf("emit: root key %q is shown as %q because [%s] is a section", key, key+"=", key)
}

// sortedCopy is a small helper for stable, predictable completion lists.
func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
