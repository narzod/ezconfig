package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const sectionSchema = `[app]
locale=en-US

[@section app]
title=Application
doc=How the program behaves
keys=locale,mode

[@rule app.mode]
type=enum
values=kiosk,desktop
doc=How the window is shown

[@rule app.locale]
pattern=[a-z]{2}(-[A-Z]{2})?
format=language code, optionally with region (e.g. en-US)
prompt=Which language and region?
doc=Language for dates and numbers

[@rule app.retries]
type=int
min=0
max=9
required=true

[@rule name]
doc=Name of this player
`

func TestSections(t *testing.T) {
	s, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", sectionSchema))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewIni()
	cfg.parse(strings.NewReader("[app]\nextra=1\n[other]\nx=2\n"))

	// keys= first, then rules in order, then defaults, then the config.
	if got := s.KeysIn(cfg, "app"); !reflect.DeepEqual(got, []string{"locale", "mode", "retries", "extra"}) {
		t.Errorf("KeysIn app: %q", got)
	}
	if got := s.SectionsKnown(cfg); !reflect.DeepEqual(got, []string{"", "app", "other"}) {
		t.Errorf("SectionsKnown: %q", got)
	}

	want := []string{
		"[app] Application: How the program behaves",
		"app.locale: Language for dates and numbers - language code, optionally with region (e.g. en-US)",
		"app.mode: How the window is shown - one of kiosk, desktop",
		"app.retries - required, a whole number from 0 to 9",
		"app.extra - no rule",
	}
	got, ok := s.DescribeName(cfg, "app")
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("describe app:\n got %q\nwant %q", got, want)
	}
	if got, _ := s.DescribeName(cfg, "."); !reflect.DeepEqual(got, []string{"name: Name of this player - text"}) {
		t.Errorf("describe .: %q", got)
	}
	if got, _ := s.DescribeName(cfg, "app.locale"); !reflect.DeepEqual(got[:3],
		[]string{"app.locale", "type: string matching [a-z]{2}(-[A-Z]{2})?", "format: language code, optionally with region (e.g. en-US)"}) {
		t.Errorf("describe app.locale: %q", got)
	}
	all := s.DescribeAll(cfg)
	if all[0] != "name: Name of this player - text" || all[1] != "" || all[2] != want[0] {
		t.Errorf("describe all: %q", all)
	}
	if _, ok := s.DescribeName(cfg, "nope"); ok {
		t.Error("nope: expected not found")
	}

	// The format replaces the pattern in errors and hints.
	r := s.ruleFor("app", "locale")
	if _, err := r.check("French"); err == nil || !strings.Contains(err.Error(), "isn't in the format: language code") {
		t.Errorf("format error: %v", err)
	}
	if h := s.ruleFor("app", "mode").Hint(); h != "one of kiosk, desktop" {
		t.Errorf("hint: %q", h)
	}
	if h := s.ruleFor("app", "retries").Hint(); !strings.Contains(h, "0") || !strings.Contains(h, "9") {
		t.Errorf("hint retries: %q", h)
	}
}

func TestAskSet(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadSchema(writeFile(t, dir, "schema.ini", sectionSchema))
	if err != nil {
		t.Fatal(err)
	}
	ini := NewIni()
	r := &Repl{ini: ini, schema: s, path: filepath.Join(dir, "config.ini"), interactive: true}

	// An invalid answer is asked again; the valid one is stored in standard spelling.
	answers := []string{"Kiosk Mode", "KIOSK"}
	var prompts []string
	var offered []string
	r.askSet("app.mode", func(p string, choices []string) (string, error) {
		prompts = append(prompts, p)
		offered = choices
		a := answers[0]
		answers = answers[1:]
		return a, nil
	})
	if v, _ := ini.Get("app.mode"); v != "kiosk" {
		t.Errorf("app.mode = %q", v)
	}
	if len(prompts) != 2 || prompts[0] != "app.mode> " {
		t.Errorf("prompts: %q", prompts)
	}
	if !reflect.DeepEqual(offered, []string{"kiosk", "desktop"}) {
		t.Errorf("choices: %q", offered)
	}
	if last := r.session[len(r.session)-1]; last != "set app.mode=kiosk" {
		t.Errorf("recorded: %q", last)
	}

	// Enter keeps the value; cancelling changes nothing.
	r.askSet("app.mode", func(string, []string) (string, error) { return "", nil })
	r.askSet("app.mode", func(string, []string) (string, error) { return "desktop", errors.New("aborted") })
	if v, _ := ini.Get("app.mode"); v != "kiosk" {
		t.Errorf("after Enter and cancel: %q", v)
	}

	// Without a terminal, nothing is asked.
	r.interactive = false
	r.askSet("app.locale", func(string, []string) (string, error) {
		t.Error("asked without a terminal")
		return "", nil
	})
}

func TestSetup(t *testing.T) {
	dir := t.TempDir()
	src := sectionSchema + `
[@setup]
title=Player setup
sections=app,.
`
	s, err := LoadSchema(writeFile(t, dir, "schema.ini", src))
	if err != nil {
		t.Fatal(err)
	}
	ini := NewIni()
	ini.parse(strings.NewReader("[app]\nextra=1\n"))
	r := &Repl{ini: ini, schema: s, path: filepath.Join(dir, "config.ini"), interactive: true}

	if got := s.SetupSections(ini); !reflect.DeepEqual(got, []string{"app", ""}) {
		t.Errorf("SetupSections: %q", got)
	}
	// Keys with rules (or named by keys=), in KeysIn order; extra has no rule.
	if got := s.SetupKeys(ini, "app"); !reflect.DeepEqual(got, []string{"locale", "mode", "retries"}) {
		t.Errorf("SetupKeys: %q", got)
	}

	var asked []string
	answers := map[string]string{"app.locale> ": "", "app.mode> ": "desktop", "app.retries> ": "3", "name> ": "front"}
	r.setup(nil, func(p string, _ []string) (string, error) {
		asked = append(asked, p)
		return answers[p], nil
	})
	if !reflect.DeepEqual(asked, []string{"app.locale> ", "app.mode> ", "app.retries> ", "name> "}) {
		t.Errorf("asked: %q", asked)
	}
	for path, want := range map[string]string{"app.mode": "desktop", "app.retries": "3", "name": "front"} {
		if v, _ := ini.Get(path); v != want {
			t.Errorf("%s = %q, want %q", path, v, want)
		}
	}
	if _, ok := ini.Get("app.locale"); ok {
		t.Error("Enter wrote the default into the config")
	}

	// setup SECTION walks one section; Ctrl+C stops the walk.
	asked = nil
	r.setup([]string{"app"}, func(p string, _ []string) (string, error) {
		asked = append(asked, p)
		return "", errors.New("aborted")
	})
	if len(asked) != 1 {
		t.Errorf("after cancel: asked %q", asked)
	}

	// Without [@setup] sections=, every section with rules, root first.
	plain, _ := LoadSchema(writeFile(t, dir, "plain.ini", sectionSchema))
	if got := plain.SetupSections(ini); !reflect.DeepEqual(got, []string{"", "app"}) {
		t.Errorf("default sections: %q", got)
	}
}

func TestSetupErrors(t *testing.T) {
	cases := map[string]string{
		"[@setup x]\ntitle=a\n":                 "takes no name",
		"[@setup]\ncolour=red\n":                "unknown field",
		"[@setup]\nsections=nope\n":             "nope, which has no rules",
		"[@rule a.b]\n[@setup]\nsections=a,.\n": "names ., which",
	}
	for src, want := range cases {
		_, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want an error containing %q", src, err, want)
		}
	}
}
