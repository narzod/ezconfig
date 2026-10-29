package main

// schema.go - an optional schema: defaults and rules for a config.
//
// A schema is itself an INI file with two parts. Ordinary sections and
// root keys are defaults, shaped exactly like a config. Sections named
// schema.<address> are rules, one per key:
//
//	[net]
//	port=8080
//
//	[schema.net.port]
//	type=int
//	min=1
//	max=65535
//	doc=Port the player listens on
//
// The address after "schema." is split at its first dot: schema.name is
// the root key "name", schema.net.port is key "port" in section "net".
//
// Rule fields: type (string, int, float, bool, enum; default string),
// min and max (int, float), values (enum; or a bool's own true,false
// pair), pattern (string; must match the whole value), required, doc.
//
// Input is forgiving and output is standard: bools accept true/false,
// yes/no and 1/0 in any case, enums match ignoring case, and both are
// reported and saved in their standard spelling.
//
// Without a schema, nothing here changes how ezconfig behaves.

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// schemaName is looked for next to the config when -s isn't given.
const schemaName = "schema.ini"

// SchemaError reports a problem in the schema file itself.
type SchemaError struct{ Msg string }

func (e *SchemaError) Error() string { return e.Msg }

type rule struct {
	sec, key    string // sec is "" for a root key
	typ         string // string, int, float, bool, enum
	min, max    string // as written; checked to parse for the type
	values      []string
	pattern     *regexp.Regexp
	patternText string
	required    bool
	doc         string
}

// Schema holds the defaults and rules from a schema file.
type Schema struct {
	path     string
	defaults *Ini
	rules    []*rule
}

var (
	ruleFields = map[string]bool{"type": true, "min": true, "max": true, "values": true,
		"pattern": true, "required": true, "doc": true}
	ruleTypes = map[string]bool{"string": true, "int": true, "float": true, "bool": true, "enum": true}
)

// LoadSchema reads and checks a schema file.
func LoadSchema(path string) (*Schema, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	raw, err := LoadIni(path, ".")
	if err != nil {
		var se *SyntaxError
		if errors.As(err, &se) {
			return nil, &SchemaError{se.Error()}
		}
		return nil, err
	}
	s := &Schema{path: path, defaults: NewIni()}
	s.defaults.root.entries = append([]entry(nil), raw.root.entries...)
	for _, sec := range raw.sections {
		if !strings.HasPrefix(sec.name, "schema.") {
			s.defaults.sections = append(s.defaults.sections,
				&section{name: sec.name, entries: append([]entry(nil), sec.entries...)})
			continue
		}
		r, err := parseRule(sec)
		if err != nil {
			return nil, &SchemaError{fmt.Sprintf("[%s]: %v", sec.name, err)}
		}
		s.rules = append(s.rules, r)
	}
	// Defaults must follow their own rules.
	for _, r := range s.rules {
		if v, ok := valueAt(s.defaults, r.sec, r.key); ok {
			if _, err := r.check(v); err != nil {
				return nil, &SchemaError{fmt.Sprintf("default %s=%s breaks its own rule: %v",
					address(r.sec, r.key, "."), v, err)}
			}
		}
	}
	return s, nil
}

func parseRule(sec *section) (*rule, error) {
	addr := strings.TrimPrefix(sec.name, "schema.")
	rsec, rkey, ok := strings.Cut(addr, ".")
	if !ok {
		rsec, rkey = "", addr
	}
	if rkey == "" || rsec == "" && ok {
		return nil, fmt.Errorf("expected [schema.KEY] or [schema.SECTION.KEY]")
	}
	r := &rule{sec: rsec, key: rkey, typ: "string"}
	for _, e := range sec.entries {
		if !ruleFields[e.key] {
			return nil, fmt.Errorf("unknown field %q (fields: type, min, max, values, pattern, required, doc)", e.key)
		}
		switch e.key {
		case "type":
			r.typ = strings.ToLower(e.value)
		case "min":
			r.min = e.value
		case "max":
			r.max = e.value
		case "values":
			for _, v := range strings.Split(e.value, ",") {
				if v = strings.TrimSpace(v); v != "" {
					r.values = append(r.values, v)
				}
			}
		case "pattern":
			re, err := regexp.Compile("^(?:" + e.value + ")$")
			if err != nil {
				return nil, fmt.Errorf("pattern: %v", err)
			}
			r.pattern, r.patternText = re, e.value
		case "required":
			b, ok := boolWord(e.value)
			if !ok {
				return nil, fmt.Errorf("required must be true or false, not %q", e.value)
			}
			r.required = b
		case "doc":
			r.doc = e.value
		}
	}
	if !ruleTypes[r.typ] {
		return nil, fmt.Errorf("unknown type %q (types: string, int, float, bool, enum)", r.typ)
	}
	only := func(field string, set bool, types ...string) error {
		if !set {
			return nil
		}
		for _, t := range types {
			if r.typ == t {
				return nil
			}
		}
		return fmt.Errorf("%s doesn't apply to type %s", field, r.typ)
	}
	for _, err := range []error{
		only("min", r.min != "", "int", "float"),
		only("max", r.max != "", "int", "float"),
		only("values", r.values != nil, "enum", "bool"),
		only("pattern", r.pattern != nil, "string"),
	} {
		if err != nil {
			return nil, err
		}
	}
	for field, v := range map[string]string{"min": r.min, "max": r.max} {
		if v == "" {
			continue
		}
		if _, err := r.number(v); err != nil {
			return nil, fmt.Errorf("%s: %v", field, err)
		}
	}
	if r.min != "" && r.max != "" {
		lo, _ := r.number(r.min)
		hi, _ := r.number(r.max)
		if lo > hi {
			return nil, fmt.Errorf("min %s is above max %s", r.min, r.max)
		}
	}
	switch {
	case r.typ == "enum" && len(r.values) == 0:
		return nil, fmt.Errorf("an enum needs values=, e.g. values=red,green,blue")
	case r.typ == "bool" && r.values != nil && len(r.values) != 2:
		return nil, fmt.Errorf("a bool's values= is a pair: the true word, then the false word (e.g. on,off)")
	}
	return r, nil
}

// boolWord reads the six accepted bool spellings, in any case.
func boolWord(v string) (value, ok bool) {
	switch strings.ToLower(v) {
	case "true", "yes", "1":
		return true, true
	case "false", "no", "0":
		return false, true
	}
	return false, false
}

// number parses v as this rule's numeric type.
func (r *rule) number(v string) (float64, error) {
	if r.typ == "int" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s is not a whole number", v)
		}
		return float64(n), nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%s is not a number", v)
	}
	return f, nil
}

// check validates v and returns its standard spelling: the value itself,
// except for bools and enums, which come back as the rule spells them.
func (r *rule) check(v string) (string, error) {
	if v == "" && r.typ != "string" {
		return "", errors.New("the value is empty (unset the key to use the default)")
	}
	switch r.typ {
	case "string":
		if r.pattern != nil && !r.pattern.MatchString(v) {
			return "", fmt.Errorf("%q doesn't match the pattern %s", v, r.patternText)
		}
	case "int", "float":
		n, err := r.number(v)
		if err != nil {
			return "", err
		}
		if lo, _ := r.number(r.min); r.min != "" && n < lo {
			return "", fmt.Errorf("%s is below the minimum (%s)", v, r.min)
		}
		if hi, _ := r.number(r.max); r.max != "" && n > hi {
			return "", fmt.Errorf("%s is above the maximum (%s)", v, r.max)
		}
	case "bool":
		t, f := r.boolPair()
		switch b, ok := boolWord(v); {
		case ok && b, strings.EqualFold(v, t):
			return t, nil
		case ok && !b, strings.EqualFold(v, f):
			return f, nil
		}
		return "", fmt.Errorf("%s is not a yes/no value (%s)", v, r.boolWords())
	case "enum":
		for _, x := range r.values {
			if strings.EqualFold(x, v) {
				return x, nil
			}
		}
		return "", fmt.Errorf("%s is not one of %s", v, strings.Join(r.values, ", "))
	}
	return v, nil
}

// boolPair is the standard true and false spelling.
func (r *rule) boolPair() (string, string) {
	if len(r.values) == 2 {
		return r.values[0], r.values[1]
	}
	return "true", "false"
}

func (r *rule) boolWords() string {
	t, f := r.boolPair()
	if t == "true" && f == "false" {
		return "true/false, yes/no, 1/0"
	}
	return t + "/" + f + ", true/false, yes/no, 1/0"
}

// choices are the values Tab offers after "set KEY=".
func (r *rule) choices() []string {
	switch r.typ {
	case "enum":
		return r.values
	case "bool":
		t, f := r.boolPair()
		return []string{t, f}
	}
	return nil
}

// summary describes the type in one line, for describe.
func (r *rule) summary() string {
	switch r.typ {
	case "string":
		if r.pattern != nil {
			return "string matching " + r.patternText
		}
		return "string"
	case "int", "float":
		switch {
		case r.min != "" && r.max != "":
			return fmt.Sprintf("%s, %s to %s", r.typ, r.min, r.max)
		case r.min != "":
			return fmt.Sprintf("%s, at least %s", r.typ, r.min)
		case r.max != "":
			return fmt.Sprintf("%s, at most %s", r.typ, r.max)
		}
		return r.typ
	case "bool":
		t, f := r.boolPair()
		return fmt.Sprintf("bool, %s/%s (also accepts %s)", t, f, r.boolWords())
	}
	return "enum, one of " + strings.Join(r.values, ", ")
}

// ruleFor finds the rule for a key; nil if there is none or no schema.
func (s *Schema) ruleFor(sec, key string) *rule {
	if s == nil {
		return nil
	}
	for _, r := range s.rules {
		if r.sec == sec && r.key == key {
			return r
		}
	}
	return nil
}

func (s *Schema) defaultFor(sec, key string) (string, bool) {
	if s == nil {
		return "", false
	}
	return valueAt(s.defaults, sec, key)
}

// Effective is what applications see: the config with the schema's
// defaults filled in, bools and enums in their standard spelling, and
// only valid values. A value that breaks its rule is replaced by the
// default, or left out when there is no default; each such substitution
// is described in the returned notes. The config's own order comes first;
// keys and sections that only the defaults have follow. Without a schema
// it is the config itself.
func (s *Schema) Effective(cfg *Ini) *Ini {
	eff, _ := s.EffectiveNotes(cfg)
	return eff
}

// note explains why the effective value of a key differs from the config.
type note struct {
	sec, key string
	text     string
}

// EffectiveNotes is Effective, plus a note for each invalid value that was
// replaced by its default or left out.
func (s *Schema) EffectiveNotes(cfg *Ini) (*Ini, []note) {
	if s == nil {
		return cfg, nil
	}
	out := cfg.Clone()
	var notes []note
	fix := func(name string, sec *section) {
		kept := sec.entries[:0]
		for _, e := range sec.entries {
			r := s.ruleFor(name, e.key)
			if r == nil {
				kept = append(kept, e)
				continue
			}
			v, err := r.check(e.value)
			if err == nil {
				kept = append(kept, entry{e.key, v})
				continue
			}
			addr := address(name, e.key, cfg.delim)
			if def, ok := s.defaultFor(name, e.key); ok {
				def, _ = r.check(def) // defaults are checked when the schema loads
				kept = append(kept, entry{e.key, def})
				notes = append(notes, note{name, e.key, fmt.Sprintf("%s: %v; using the default (%s)", addr, err, def)})
			} else {
				notes = append(notes, note{name, e.key, fmt.Sprintf("%s: %v; left out (no default)", addr, err)})
			}
		}
		sec.entries = kept
	}
	fix("", &out.root)
	for _, sec := range out.sections {
		fix(sec.name, sec)
	}
	add := func(dst *section, name string, entries []entry) {
		for _, e := range entries {
			if dst.find(e.key) < 0 {
				if r := s.ruleFor(name, e.key); r != nil {
					e.value, _ = r.check(e.value)
				}
				dst.entries = append(dst.entries, e)
			}
		}
	}
	add(&out.root, "", s.defaults.root.entries)
	for _, ds := range s.defaults.sections {
		add(out.ensureSection(ds.name), ds.name, ds.entries)
	}
	return out, notes
}

// Problems lists values in cfg that break the schema, and required keys
// that have no value, as "address: reason" lines.
func (s *Schema) Problems(cfg *Ini) []string {
	if s == nil {
		return nil
	}
	var out []string
	invalid := map[[2]string]bool{}
	walk := func(name string, sec *section) {
		for _, e := range sec.entries {
			if r := s.ruleFor(name, e.key); r != nil {
				if _, err := r.check(e.value); err != nil {
					out = append(out, address(name, e.key, cfg.delim)+": "+err.Error())
					invalid[[2]string{name, e.key}] = true
				}
			}
		}
	}
	walk("", &cfg.root)
	for _, sec := range cfg.sections {
		walk(sec.name, sec)
	}
	eff := s.Effective(cfg)
	for _, r := range s.rules {
		if invalid[[2]string{r.sec, r.key}] {
			continue // already reported as invalid
		}
		if v, ok := valueAt(eff, r.sec, r.key); r.required && (!ok || v == "") {
			out = append(out, address(r.sec, r.key, cfg.delim)+": required, but not set")
		}
	}
	return out
}

// Describe explains one key: its rule, default and current value, and
// where that value comes from. ok is false if neither the schema nor the
// config knows the key.
func (s *Schema) Describe(cfg *Ini, path string) (lines []string, ok bool) {
	sec, key := cfg.splitPath(path)
	r := s.ruleFor(sec, key)
	def, hasDef := s.defaultFor(sec, key)
	raw, inConfig := valueAt(cfg, sec, key)
	if r == nil && !hasDef && !inConfig {
		return nil, false
	}
	lines = append(lines, address(sec, key, cfg.delim))
	if r != nil {
		lines = append(lines, "type: "+r.summary())
		if r.required {
			lines = append(lines, "required: yes")
		}
	} else {
		lines = append(lines, "type: no rule in the schema")
	}
	if hasDef {
		lines = append(lines, "default: "+def)
	} else {
		lines = append(lines, "default: none")
	}
	v, inEffective := valueAt(s.Effective(cfg), sec, key)
	var err error
	if inConfig && r != nil {
		_, err = r.check(raw)
	}
	switch {
	case err != nil && inEffective:
		lines = append(lines, fmt.Sprintf("value: %s (the default, because the config's value is invalid: %v)", v, err))
	case err != nil:
		lines = append(lines, fmt.Sprintf("value: none (the config's value is invalid: %v; there is no default)", err))
	case inConfig:
		lines = append(lines, "value: "+v+" (set in the config)")
	case hasDef:
		lines = append(lines, "value: "+v+" (the default)")
	default:
		lines = append(lines, "value: not set")
	}
	if r != nil && r.doc != "" {
		lines = append(lines, "doc: "+r.doc)
	}
	return lines, true
}

// notesFor picks the notes about the given keys (sec, key pairs).
func notesFor(notes []note, keys [][2]string) []string {
	var out []string
	for _, n := range notes {
		for _, k := range keys {
			if n.sec == k[0] && n.key == k[1] {
				out = append(out, n.text)
				break
			}
		}
	}
	return out
}

// notesIn picks the notes about one section ("" with all set means every
// note; sec "" with all false means the root keys).
func notesIn(notes []note, sec string, all bool) []string {
	var out []string
	for _, n := range notes {
		if all || n.sec == sec {
			out = append(out, n.text)
		}
	}
	return out
}

// Rules counts the schema's rules.
func (s *Schema) Rules() int {
	if s == nil {
		return 0
	}
	return len(s.rules)
}

// valueAt reads a value by section ("" for the root) and key.
func valueAt(ini *Ini, sec, key string) (string, bool) {
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

func address(sec, key, delim string) string {
	if sec == "" {
		return key
	}
	return sec + delim + key
}

// findSchema decides which schema applies to a config. opt is -s: a file,
// "none", or "" to look for schema.ini next to the config. It returns ""
// when there is no schema; an explicitly named file must exist.
func findSchema(configPath, opt string) (string, error) {
	switch opt {
	case "none":
		return "", nil
	case "":
		cand := filepath.Join(filepath.Dir(configPath), schemaName)
		ci, err1 := os.Stat(cand)
		if err1 != nil {
			return "", nil
		}
		if fi, err2 := os.Stat(configPath); err2 == nil && os.SameFile(ci, fi) {
			return "", nil // the config is itself named schema.ini
		}
		return cand, nil
	}
	abs, err := filepath.Abs(opt)
	if err != nil {
		abs = opt
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("schema %s: no such file", abs)
	}
	return abs, nil
}

// schemaLabel names a schema briefly: just its file name when it sits
// next to the config, otherwise its full path.
func schemaLabel(schemaPath, configPath string) string {
	if filepath.Dir(schemaPath) == filepath.Dir(configPath) {
		return filepath.Base(schemaPath)
	}
	return schemaPath
}
