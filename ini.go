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
}

type section struct {
	name    string
	entries []entry
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
	cur := &ini.root
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			if !strings.HasSuffix(line, "]") {
				return &SyntaxError{n, "unterminated section header"}
			}
			cur = ini.ensureSection(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return &SyntaxError{n, "expected key=value"}
		}
		setIn(cur, strings.TrimSpace(k), strings.TrimSpace(v))
	}
	return sc.Err()
}

func setIn(s *section, key, value string) {
	if i := s.find(key); i >= 0 {
		s.entries[i].value = value
		return
	}
	s.entries = append(s.entries, entry{key, value})
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
		if len(ini.root.entries) > 0 {
			return fmt.Errorf("the root is not empty (wipe %s first)", ini.delim)
		}
		for _, e := range s.entries {
			if ini.section(e.key) != nil {
				return fmt.Errorf("key %s would share its name with section %s", e.key, e.key)
			}
		}
		ini.root.entries = append([]entry(nil), s.entries...)
		return nil
	}
	if dst == "" || strings.Contains(dst, ini.delim) || strings.ContainsAny(dst, "[]= \t") {
		return fmt.Errorf("invalid section name: %s", dst)
	}
	if ini.section(dst) != nil {
		return fmt.Errorf("section exists: %s", dst)
	}
	if _, ok := ini.Get(dst); ok {
		return fmt.Errorf("a root key named %s exists", dst)
	}
	entries := append([]entry(nil), s.entries...)
	ini.sections = append(ini.sections, &section{name: dst, entries: entries})
	return nil
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
	var b strings.Builder
	for _, e := range ini.root.entries {
		fmt.Fprintf(&b, "%s=%s\n", e.key, e.value)
	}
	for _, s := range ini.sections {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[%s]\n", s.name)
		for _, e := range s.entries {
			fmt.Fprintf(&b, "%s=%s\n", e.key, e.value)
		}
	}
	return writeAtomic(path, []byte(b.String()))
}

// writeAtomic writes data to a temporary file in path's folder, then
// renames it over path, so a program reading path never sees a partly
// written file. An existing file's permissions are kept.
func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".inirepl-*.tmp")
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

// JSON renders the Ini as JSON in file order: root keys as string
// members, sections as nested objects. All values are strings. A root
// key with the same name as a section is reported as an error.
//
// With sec set, only that section is rendered, as a flat object; the
// delimiter on its own renders only the root keys. ok is false if the
// section doesn't exist.
func (ini *Ini) JSON(sec string) (out string, ok bool, err error) {
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	if sec != "" {
		s := ini.sectionOrRoot(sec)
		if s == nil {
			return "", false, nil
		}
		if len(s.entries) == 0 {
			return "{}\n", true, nil
		}
		var members []string
		for _, e := range s.entries {
			members = append(members, "  "+q(e.key)+": "+q(e.value))
		}
		return "{\n" + strings.Join(members, ",\n") + "\n}\n", true, nil
	}
	for _, e := range ini.root.entries {
		if ini.section(e.key) != nil {
			return "", true, fmt.Errorf("%q is both a root key and a section", e.key)
		}
	}
	var members []string
	for _, e := range ini.root.entries {
		members = append(members, "  "+q(e.key)+": "+q(e.value))
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
		return "{}\n", true, nil
	}
	return "{\n" + strings.Join(members, ",\n") + "\n}\n", true, nil
}

// sortedCopy is a small helper for stable, predictable completion lists.
func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
