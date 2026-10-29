package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testSchema = `[player]
name=player
debug=no

[display]
width=1920
rotation=0
background=Black

[net]
port=8080

[schema.player.name]
pattern=[a-z0-9-]+
doc=Name shown in the console

[schema.player.site]
required=true

[schema.player.debug]
type=bool

[schema.player.autostart]
type=bool
values=on,off

[schema.display.width]
type=int
min=320
max=7680

[schema.display.rotation]
type=enum
values=0,90,180,270

[schema.display.background]
type=enum
values=black,white,red,green,blue,yellow

[schema.display.scale]
type=float
min=0.5
max=2

[schema.net.port]
type=int
min=1
max=65535
doc=Port the player listens on
`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func loadTestSchema(t *testing.T) *Schema {
	t.Helper()
	s, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", testSchema))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRuleChecks(t *testing.T) {
	s := loadTestSchema(t)
	cases := []struct {
		sec, key, in, want string // want "" means invalid
	}{
		{"net", "port", "9090", "9090"},
		{"net", "port", "0", ""},
		{"net", "port", "70000", ""},
		{"net", "port", "80a", ""},
		{"net", "port", "", ""},
		{"display", "scale", "1.25", "1.25"},
		{"display", "scale", "3", ""},
		{"display", "scale", "NaN", ""},
		{"player", "debug", "Yes", "true"},
		{"player", "debug", "1", "true"},
		{"player", "debug", "FALSE", "false"},
		{"player", "debug", "0", "false"},
		{"player", "debug", "maybe", ""},
		{"player", "autostart", "ON", "on"},
		{"player", "autostart", "yes", "on"},
		{"player", "autostart", "0", "off"},
		{"display", "rotation", "90", "90"},
		{"display", "rotation", "45", ""},
		{"display", "background", "RED", "red"},
		{"display", "background", "purple", ""},
		{"player", "name", "lobby-east", "lobby-east"},
		{"player", "name", "Lobby East", ""},
		{"player", "site", "", ""}, // string with no pattern: empty is allowed
	}
	for _, c := range cases {
		r := s.ruleFor(c.sec, c.key)
		got, err := r.check(c.in)
		if c.want == "" && c.key == "site" {
			if err != nil {
				t.Errorf("%s=%q: unexpected error %v", c.key, c.in, err)
			}
			continue
		}
		if c.want == "" && err == nil {
			t.Errorf("%s.%s=%q: expected an error, got %q", c.sec, c.key, c.in, got)
		}
		if c.want != "" && (err != nil || got != c.want) {
			t.Errorf("%s.%s=%q: got %q, %v; want %q", c.sec, c.key, c.in, got, err, c.want)
		}
	}
}

func TestEffectiveAndProblems(t *testing.T) {
	s := loadTestSchema(t)
	cfg := NewIni()
	cfg.parse(strings.NewReader("[player]\nname=lobby-east\ndebug=YES\n[net]\nport=70000\n[extra]\nfoo=bar\n"))

	eff := s.Effective(cfg)
	for path, want := range map[string]string{
		"player.name":        "lobby-east", // from the config
		"player.debug":       "true",       // standard spelling
		"display.width":      "1920",       // default
		"display.background": "black",      // default, standard spelling
		"extra.foo":          "bar",        // not in the schema: kept
	} {
		if got, _ := eff.Get(path); got != want {
			t.Errorf("effective %s: got %q, want %q", path, got, want)
		}
	}
	// The config itself is untouched.
	if v, _ := cfg.Get("player.debug"); v != "YES" {
		t.Errorf("config changed: %q", v)
	}
	if _, ok := cfg.Get("display.width"); ok {
		t.Error("defaults leaked into the config")
	}
	// Config order first, then sections only the defaults have.
	if got := eff.Sections(); !reflect.DeepEqual(got, []string{"player", "net", "extra", "display"}) {
		t.Errorf("sections: %q", got)
	}

	want := []string{
		"net.port: 70000 is above the maximum (65535)",
		"player.site: required, but not set",
	}
	if got := s.Problems(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("problems:\n got %q\nwant %q", got, want)
	}
}

func TestDescribe(t *testing.T) {
	s := loadTestSchema(t)
	cfg := NewIni()
	cfg.parse(strings.NewReader("[net]\nport=9090\n"))

	got, ok := s.Describe(cfg, "net.port")
	want := []string{"net.port", "type: int, 1 to 65535", "default: 8080",
		"value: 9090 (set in the config)", "doc: Port the player listens on"}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("net.port:\n got %q\nwant %q", got, want)
	}
	got, _ = s.Describe(cfg, "player.site")
	if !reflect.DeepEqual(got, []string{"player.site", "type: string", "required: yes", "default: none", "value: not set"}) {
		t.Errorf("player.site: %q", got)
	}
	got, _ = s.Describe(cfg, "display.background")
	if got[len(got)-1] != "value: black (the default)" {
		t.Errorf("display.background: %q", got)
	}
	if _, ok := s.Describe(cfg, "nope.nothing"); ok {
		t.Error("nope.nothing: expected not found")
	}
}

func TestSchemaErrors(t *testing.T) {
	cases := map[string]string{
		"[schema.a]\ntype=prot\n":                    "unknown type",
		"[schema.a]\ntype=int\nmin=x\n":              "not a whole number",
		"[schema.a]\ntype=int\nmin=5\nmax=1\n":       "above max",
		"[schema.a]\ntype=enum\n":                    "needs values",
		"[schema.a]\ntype=bool\nvalues=on\n":         "pair",
		"[schema.a]\nmax=3\n":                        "doesn't apply",
		"[schema.a]\ncolour=red\n":                   "unknown field",
		"[schema.a]\npattern=(\n":                    "pattern",
		"[schema.a]\nrequired=maybe\n":               "required",
		"a=45\n[schema.a]\ntype=enum\nvalues=0,90\n": "breaks its own rule",
		"[schema.]\ntype=int\n":                      "expected",
		"[schema.a]\ntype=int\n[broken\n":            "unterminated",
	}
	for src, want := range cases {
		_, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want an error containing %q", src, err, want)
		}
	}
}

func TestFindSchema(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.ini", "a=1\n")
	if p, _ := findSchema(cfg, ""); p != "" {
		t.Errorf("no schema.ini: got %q", p)
	}
	sp := writeFile(t, dir, "schema.ini", "a=1\n")
	if p, _ := findSchema(cfg, ""); p != sp {
		t.Errorf("schema.ini next to the config: got %q", p)
	}
	if p, _ := findSchema(cfg, "none"); p != "" {
		t.Errorf("-s none: got %q", p)
	}
	if p, _ := findSchema(sp, ""); p != "" {
		t.Errorf("the config is schema.ini itself: got %q", p)
	}
	if _, err := findSchema(cfg, filepath.Join(dir, "missing.ini")); err == nil {
		t.Error("-s missing.ini: expected an error")
	}
}

func TestOneShotSchema(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.ini", "[player]\nname=lobby-east\nsite=0142\n")
	writeFile(t, dir, "schema.ini", testSchema)

	if code, out, _ := run(t, cfg, ".", "get", "net.port", "player.debug"); code != 0 || out != "8080\nfalse\n" {
		t.Errorf("get with defaults: %d %q", code, out)
	}
	if code, _, errOut := run(t, cfg, ".", "set", "net.port=70000"); code != 1 || !strings.Contains(errOut, "maximum") {
		t.Errorf("invalid set: %d %q", code, errOut)
	}
	if code, _, _ := run(t, cfg, ".", "set", "player.debug=YES", "display.background=Red"); code != 0 {
		t.Errorf("valid set: %d", code)
	}
	if got := readFile(t, cfg); !strings.Contains(got, "debug=true") || !strings.Contains(got, "background=red") {
		t.Errorf("stored spelling:\n%s", got)
	}
	if strings.Contains(readFile(t, cfg), "port=") {
		t.Error("defaults were written into the config")
	}
	if code, out, _ := run(t, cfg, ".", "check"); code != 0 || out != "" {
		t.Errorf("check on a valid config: %d %q", code, out)
	}
	writeFile(t, dir, "config.ini", "[net]\nport=0\n")
	if code, out, _ := run(t, cfg, ".", "check"); code != 1 || !strings.Contains(out, "player.site: required") {
		t.Errorf("check on an invalid config: %d %q", code, out)
	}
	if code, out, _ := run(t, cfg, ".", "describe", "net.port"); code != 0 || !strings.HasPrefix(out, "net.port\ntype: int") {
		t.Errorf("describe: %d %q", code, out)
	}
	writeFile(t, dir, "schema.ini", "[schema.a]\ntype=prot\n")
	if code, _, _ := run(t, cfg, ".", "list"); code != 2 {
		t.Errorf("broken schema: code %d, want 2", code)
	}
}

func TestCompleteWithSchema(t *testing.T) {
	ini := NewIni()
	ini.parse(strings.NewReader("[player]\nname=lobby-east\n"))
	r := &Repl{ini: ini, schema: loadTestSchema(t)}
	cases := []struct {
		line string
		want []string
	}{
		{"set display.rotation=", []string{"display.rotation=0", "display.rotation=90",
			"display.rotation=180", "display.rotation=270"}},
		{"set display.background=r", []string{"display.background=red"}},
		{"set player.autostart=", []string{"player.autostart=on", "player.autostart=off"}},
		{"get net.", []string{"net.port "}},        // a schema default the config never sets
		{"unset net.", nil},                        // only what the config sets
		{"set player.n", []string{"player.name="}}, // config key
		{"describe display.ro", []string{"display.rotation "}},
	}
	for _, c := range cases {
		head, cands, _ := r.complete(c.line, len(c.line))
		if got := completedWords(c.line, head, cands); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %q, want %q", c.line, got, c.want)
		}
	}

	// What liner lists is only what remains to choose.
	for line, want := range map[string][]string{
		"set display.rotation=":   {"0", "90", "180", "270"},
		"set display.":            {"background=", "rotation=", "scale=", "width="}, // scale: a rule, no default
		"set player.":             {"autostart=", "debug=", "name=", "site="},       // site: required, no default
		"set display.background=": {"black", "white", "red", "green", "blue", "yellow"},
	} {
		if _, cands, _ := r.complete(line, len(line)); !reflect.DeepEqual(cands, want) {
			t.Errorf("%q lists %q, want %q", line, cands, want)
		}
	}
}

func TestValidOnly(t *testing.T) {
	s := loadTestSchema(t)
	cfg := NewIni()
	cfg.parse(strings.NewReader("[player]\nname=Lobby East\nsite=0142\ndebug=maybe\n[display]\nscale=3\n[net]\nport=70000\n"))

	eff, notes := s.EffectiveNotes(cfg)
	for path, want := range map[string]string{
		"player.name":  "player", // invalid: the default
		"player.debug": "false",  // invalid: the default, in its standard spelling
		"net.port":     "8080",   // invalid: the default
		"player.site":  "0142",   // valid: kept
	} {
		if got, _ := eff.Get(path); got != want {
			t.Errorf("%s: got %q, want %q", path, got, want)
		}
	}
	if _, ok := eff.Get("display.scale"); ok {
		t.Error("display.scale: invalid with no default, should be left out")
	}
	var texts []string
	for _, n := range notes {
		texts = append(texts, n.text)
	}
	want := []string{
		`player.name: "Lobby East" doesn't match the pattern [a-z0-9-]+; using the default (player)`,
		"player.debug: maybe is not a yes/no value (true/false, yes/no, 1/0); using the default (false)",
		"display.scale: 3 is above the maximum (2); left out (no default)",
		"net.port: 70000 is above the maximum (65535); using the default (8080)",
	}
	if !reflect.DeepEqual(texts, want) {
		t.Errorf("notes:\n got %q\nwant %q", texts, want)
	}

	// describe says what applications get, and why.
	lines, _ := s.Describe(cfg, "net.port")
	if got := lines[3]; got != "value: 8080 (the default, because the config's value is invalid: 70000 is above the maximum (65535))" {
		t.Errorf("describe net.port: %q", got)
	}
	lines, _ = s.Describe(cfg, "display.scale")
	if got := lines[3]; !strings.HasPrefix(got, "value: none (the config's value is invalid") {
		t.Errorf("describe display.scale: %q", got)
	}

	// An invalid value isn't also reported as missing when required.
	s.ruleFor("player", "site").required = true
	cfg.Set("player.site", "")
	for _, p := range s.Problems(cfg) {
		if strings.HasPrefix(p, "player.name: required") {
			t.Errorf("duplicate problem: %q", p)
		}
	}
}

func TestOneShotValidOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.ini", "[player]\nname=lobby-east\nsite=0142\n[display]\nscale=3\n[net]\nport=70000\n")
	writeFile(t, dir, "schema.ini", testSchema)

	code, out, errOut := run(t, cfg, ".", "get", "net.port")
	if code != 0 || out != "8080\n" || !strings.Contains(errOut, "using the default (8080)") {
		t.Errorf("get invalid with default: %d %q %q", code, out, errOut)
	}
	code, out, errOut = run(t, cfg, ".", "get", "net.port", "display.scale")
	if code != 1 || out != "" || !strings.Contains(errOut, "left out (no default)") {
		t.Errorf("get invalid without default: %d %q %q", code, out, errOut)
	}
	code, out, errOut = run(t, cfg, ".", "get", "display.scale", "nope.key")
	if code != 5 || out != "" {
		t.Errorf("get invalid and missing: %d %q %q", code, out, errOut)
	}
	code, out, errOut = run(t, cfg, ".", "list", "net")
	if code != 0 || out != "net.port=8080\n" || !strings.Contains(errOut, "net.port: 70000") {
		t.Errorf("list net: %d %q %q", code, out, errOut)
	}
	code, out, errOut = run(t, cfg, ".", "list", "player")
	if code != 0 || errOut != "" {
		t.Errorf("list player warns about other sections: %d %q", code, errOut)
	}
	code, out, _ = run(t, cfg, ".", "dump", "display")
	if code != 0 || strings.Contains(out, "scale") {
		t.Errorf("dump display includes the invalid scale: %d %q", code, out)
	}
	// -s none shows exactly what the config says, valid or not.
	var buf, errBuf bytes.Buffer
	if code := oneShot(cfg, ".", "none", "list", []string{"net"}, &buf, &errBuf); code != 0 || buf.String() != "net.port=70000\n" {
		t.Errorf("-s none: %d %q", code, buf.String())
	}
}
