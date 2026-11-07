package main

import (
	"errors"
	"os"
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
	got, ok := s.ExplainName(cfg, "app")
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("explain app:\n got %q\nwant %q", got, want)
	}
	if got, _ := s.ExplainName(cfg, "."); !reflect.DeepEqual(got, []string{"name: Name of this player - text"}) {
		t.Errorf("explain .: %q", got)
	}
	if got, _ := s.ExplainName(cfg, "app.locale"); !reflect.DeepEqual(got[:3],
		[]string{"app.locale", "type: string matching [a-z]{2}(-[A-Z]{2})?", "format: language code, optionally with region (e.g. en-US)"}) {
		t.Errorf("explain app.locale: %q", got)
	}
	all := s.ExplainAll(cfg)
	if all[0] != "name: Name of this player - text" || all[1] != "" || all[2] != want[0] {
		t.Errorf("explain all: %q", all)
	}
	if _, ok := s.ExplainName(cfg, "nope"); ok {
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

func TestAskNames(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadSchema(writeFile(t, dir, "schema.ini", sectionSchema))
	if err != nil {
		t.Fatal(err)
	}
	ini := NewIni()
	ini.parse(strings.NewReader("[app]\nextra=1\n[other]\nx=2\n"))
	r := &Repl{ini: ini, saved: ini.Clone(), schema: s, path: filepath.Join(dir, "config.ini"), interactive: true}

	plan, res := r.askPlan([]string{"app.mode", "app", ".", "name", "other.x", "app.mode"})
	want := []askItem{
		{sec: "app", key: "mode"},
		{sec: "app", whole: true},
		{sec: "", whole: true},
		{sec: "", key: "name"},
		{sec: "other", key: "x"}, // set in the config, no rule: still a key ask can ask about
	}
	if res != askDone || !reflect.DeepEqual(plan, want) {
		t.Errorf("plan:\n got %v %v\nwant %v", res, plan, want)
	}
	// Unknown names are refused before anything is asked.
	if _, res := r.askPlan([]string{"app.mode", "app.nope", "nope."}); res != askNotFound {
		t.Errorf("unknown names: %v", res)
	}
	// other has no rules, so there is nothing to ask about it as a section.
	if _, res := r.askPlan([]string{"other"}); res != askNotFound {
		t.Errorf("section without rules: %v", res)
	}

	// One key: just the question, no heading, then the review.
	var prompts []string
	got := r.askAll([]string{"app.mode"}, func(p string, _, _ []string) (string, error) {
		prompts = append(prompts, promptKey(p))
		if p == "app.mode> " {
			return "kiosk", nil
		}
		return "s", nil
	}, func() bool { return true }, true)
	if got != askDone || !reflect.DeepEqual(prompts, []string{"app.mode> ", "REVIEW"}) {
		t.Errorf("ask app.mode: %v %q", got, prompts)
	}

	// Without a schema, named keys still work; sections and plain ask don't.
	bare := &Repl{ini: ini, saved: ini.Clone(), path: filepath.Join(dir, "config.ini"), interactive: true}
	if plan, res := bare.askPlan([]string{"other.x"}); res != askDone || len(plan) != 1 {
		t.Errorf("no schema, key: %v %v", res, plan)
	}
	if _, res := bare.askPlan([]string{"other."}); res != askRefused {
		t.Errorf("no schema, section: %v", res)
	}
	if got := bare.askAll(nil, nil, nil, true); got != askRefused {
		t.Errorf("no schema, plain ask: %v", got)
	}
	if _, res := bare.askPlan([]string{"other.newkey"}); res != askNotFound {
		t.Errorf("no schema, unknown key: %v", res)
	}

	// Without a terminal, nothing is asked.
	r.interactive = false
	if got := r.askAll([]string{"app.mode"}, nil, nil, true); got != askRefused {
		t.Errorf("no terminal: %v", got)
	}
}

func TestAsk(t *testing.T) {
	dir := t.TempDir()
	src := sectionSchema + `
[@ask]
title=Player questions
sections=app,.
`
	s, err := LoadSchema(writeFile(t, dir, "schema.ini", src))
	if err != nil {
		t.Fatal(err)
	}
	ini := NewIni()
	ini.parse(strings.NewReader("[app]\nextra=1\n"))
	r := &Repl{ini: ini, saved: ini.Clone(), schema: s, path: filepath.Join(dir, "config.ini"), interactive: true}

	if got := s.AskSections(ini); !reflect.DeepEqual(got, []string{"app", ""}) {
		t.Errorf("AskSections: %q", got)
	}
	// Keys with rules (or named by keys=), in KeysIn order; extra has no rule.
	if got := s.AskKeys(ini, "app"); !reflect.DeepEqual(got, []string{"locale", "mode", "retries"}) {
		t.Errorf("AskKeys: %q", got)
	}

	const review = "REVIEW" // promptKey's name for the review question, in either mode
	var asked []string
	var recalled [][]string
	answers := map[string][]string{
		"app.locale> ": {""}, "app.mode> ": {"desktop"},
		"app.retries> ": {"", "3"}, // required with no value: Enter is refused
		"name> ":        {"front"},
		review:          {"", "", "s"}, // extra Enter presses only ask again
	}
	read := func(p string, _, recall []string) (string, error) {
		p = promptKey(p)
		asked = append(asked, p)
		recalled = append(recalled, recall)
		a := answers[p][0]
		answers[p] = answers[p][1:]
		return a, nil
	}
	saves := 0
	save := func() bool { saves++; return true }
	if got := r.askAll(nil, read, save, true); got != askDone {
		t.Errorf("ask: result %d", got)
	}
	if !reflect.DeepEqual(asked, []string{"app.locale> ", "app.mode> ", "app.retries> ", "app.retries> ", "name> ",
		review, review, review}) {
		t.Errorf("asked: %q", asked)
	}
	if !reflect.DeepEqual(recalled[0], []string{"en-US"}) { // the default
		t.Errorf("recall for app.locale: %q", recalled[0])
	}
	if saves != 1 {
		t.Errorf("saves %d", saves)
	}
	for path, want := range map[string]string{"app.mode": "desktop", "app.retries": "3", "name": "front"} {
		if v, _ := ini.Get(path); v != want {
			t.Errorf("%s = %q, want %q", path, v, want)
		}
	}
	if _, ok := ini.Get("app.locale"); ok {
		t.Error("Enter wrote the default into the config")
	}
	if len(r.session) != 0 {
		t.Errorf("ask answers went into history: %q", r.session)
	}

	// The up arrow offers the current value, then the default.
	if got := r.recallFor("app", "locale", s.ruleFor("app", "locale")); !reflect.DeepEqual(got, []string{"en-US"}) {
		t.Errorf("recall, unset: %q", got)
	}
	ini.Set("app.locale", "fr-CA")
	if got := r.recallFor("app", "locale", s.ruleFor("app", "locale")); !reflect.DeepEqual(got, []string{"fr-CA", "en-US"}) {
		t.Errorf("recall, set: %q", got)
	}

	// ask SECTION walks one section; Ctrl+C stops the walk.
	asked = nil
	stop := func(p string, _, _ []string) (string, error) {
		p = promptKey(p)
		asked = append(asked, p)
		return "", errors.New("aborted")
	}
	if got := r.askAll([]string{"app"}, stop, save, true); got != askStopped || len(asked) != 1 {
		t.Errorf("one-shot cancel: result %d, asked %q", got, asked)
	}
	if got := r.askAll([]string{"app"}, stop, save, false); got != askDone {
		t.Errorf("shell cancel: result %d", got)
	}

	// script answers each prompt in turn from a list.
	script := func(steps ...[2]string) reader {
		return func(p string, _, _ []string) (string, error) {
			p = promptKey(p)
			for i, st := range steps {
				if st[0] == p {
					steps = append(steps[:i:i], steps[i+1:]...)
					return st[1], nil
				}
			}
			return "", nil // Enter
		}
	}

	// Redo: the questions again, starting from the answers given.
	ini.Set("app.mode", "desktop")
	before := saves
	var prompts []string
	steps := [][2]string{
		{"app.mode> ", "kiosk"},
		{review, "r"},
		{"app.mode> ", "desktop"}, // back to where it was: no changes left
	}
	got := r.askAll([]string{"app"}, func(p string, _, _ []string) (string, error) {
		prompts = append(prompts, p)
		p = promptKey(p)
		for i, st := range steps {
			if st[0] == p {
				steps = append(steps[:i:i], steps[i+1:]...)
				return st[1], nil
			}
		}
		return "", nil
	}, save, true)
	if got != askDone || saves != before {
		t.Errorf("redo back to the old value: result %d, saves %d", got, saves-before)
	}
	// The second pass shows the first pass's answer in the brackets.
	if !contains(prompts, "app.mode [kiosk]> ") {
		t.Errorf("redo prompts: %q", prompts)
	}
	got = r.askAll([]string{"app"}, script(
		[2]string{"app.mode> ", "kiosk"},
		[2]string{review, "redo"},
		[2]string{"app.retries> ", "7"}, // kiosk is kept with Enter
		[2]string{review, "save"},
	), save, true)
	if v, _ := ini.Get("app.retries"); got != askDone || v != "7" || saves != before+1 {
		t.Errorf("redo then save: result %d, retries %q", got, v)
	}
	if v, _ := ini.Get("app.mode"); v != "kiosk" {
		t.Errorf("redo lost the first answer: mode %q", v)
	}

	// Quit: dropped in one-shot, kept unsaved in memory in the shell.
	if got := r.askAll([]string{"app"}, script([2]string{"app.mode> ", "desktop"}, [2]string{review, "q"}), save, true); got != askStopped {
		t.Errorf("one-shot quit: result %d", got)
	}
	ini.Set("app.mode", "kiosk") // a real one-shot run would have exited, dropping desktop

	// In the shell there is no review: the answers join the session.
	var shellPrompts []string
	got = r.askAll([]string{"app"}, func(p string, _, _ []string) (string, error) {
		shellPrompts = append(shellPrompts, promptKey(p))
		if promptKey(p) == "app.mode> " {
			return "desktop", nil
		}
		return "", nil
	}, save, false)
	if v, _ := ini.Get("app.mode"); got != askUnsaved || v != "desktop" || saves != before+1 || contains(shellPrompts, review) {
		t.Errorf("shell ask: result %d, mode %q, saves %d, prompts %q", got, v, saves-before, shellPrompts)
	}
	// Ctrl+C in the shell keeps the answers already given.
	ini.Set("app.mode", "kiosk")
	got = r.askAll([]string{"app"}, func(p string, _, _ []string) (string, error) {
		switch promptKey(p) {
		case "app.locale> ":
			return "de-DE", nil
		case "app.mode> ":
			return "", errors.New("aborted")
		}
		return "", nil
	}, save, false)
	if v, _ := ini.Get("app.locale"); got != askUnsaved || v != "de-DE" {
		t.Errorf("shell Ctrl+C: result %d, locale %q", got, v)
	}
	// Ctrl+C at the review is a stop, not a save.
	ctrlC := func(p string, _, _ []string) (string, error) {
		p = promptKey(p)
		if p == review {
			return "", errors.New("aborted")
		}
		if p == "app.mode> " {
			return "desktop", nil // a change, so the review is reached
		}
		return "", nil
	}
	if got := r.askAll([]string{"app"}, ctrlC, save, true); got != askStopped || saves != before+1 {
		t.Errorf("Ctrl+C at the review: result %d", got)
	}

	// An invalid current value can't be kept with Enter during ask.
	ini.Set("app.mode", "tablet")
	if why := r.needsAnswer("app", "mode", s.ruleFor("app", "mode")); why == "" {
		t.Error("invalid value: Enter should be refused")
	}

	// Without [@ask] sections=, every section with rules, root first.
	plain, _ := LoadSchema(writeFile(t, dir, "plain.ini", sectionSchema))
	if got := plain.AskSections(ini); !reflect.DeepEqual(got, []string{"", "app"}) {
		t.Errorf("default sections: %q", got)
	}
}

func TestAskErrors(t *testing.T) {
	cases := map[string]string{
		"[@ask x]\ntitle=a\n":                 "takes no name",
		"[@ask]\ncolour=red\n":                "unknown field",
		"[@ask]\nsections=nope\n":             "nope, which has no rules",
		"[@rule a.b]\n[@ask]\nsections=a,.\n": "names ., which",
	}
	for src, want := range cases {
		_, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want an error containing %q", src, err, want)
		}
	}
}

func TestExplainSameName(t *testing.T) {
	src := `[@rule net]
doc=A root key named like a section

[@section net]
title=Network

[@rule net.port]
type=int
`
	s, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", src))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewIni()
	both, ok := s.ExplainName(cfg, "net")
	if !ok || both[0] != "net" || !contains(both, "") || !contains(both, "[net] Network") {
		t.Errorf("explain net: %q", both)
	}
	sec, ok := s.ExplainName(cfg, "net.")
	if !ok || sec[0] != "[net] Network" || len(sec) != 2 {
		t.Errorf("explain net.: %q", sec)
	}
	if _, ok := s.ExplainName(cfg, "nope."); ok {
		t.Error("explain nope.: expected not found")
	}
	if key, _ := s.ExplainName(cfg, "net.port"); key[0] != "net.port" {
		t.Errorf("explain net.port: %q", key)
	}
}

func contains(list []string, s string) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}

// promptKey drops the current value from a prompt: "a.b [x]> " is "a.b> ".
func promptKey(p string) string {
	if strings.HasPrefix(p, "save, redo, or ") {
		return "REVIEW"
	}
	if i := strings.Index(p, " ["); i >= 0 && strings.HasSuffix(p, "]> ") {
		return p[:i] + "> "
	}
	return p
}

func TestQuestionLayout(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadSchema(writeFile(t, dir, "schema.ini", sectionSchema))
	if err != nil {
		t.Fatal(err)
	}
	if got := questionLine(s.ruleFor("app", "locale")); got != "Which language and region? (language code, optionally with region (e.g. en-US))" {
		t.Errorf("prompt and format: %q", got)
	}
	if got := questionLine(s.ruleFor("", "name")); got != "Name of this player" {
		t.Errorf("plain string, doc only: %q", got)
	}
	if got := questionLine(s.ruleFor("app", "retries")); got != "a whole number from 0 to 9" {
		t.Errorf("no prompt or doc: %q", got)
	}

	ini := NewIni()
	ini.parse(strings.NewReader("[app]\nmode=tablet\nretries=\n"))
	r := &Repl{ini: ini, saved: ini.Clone(), schema: s, path: filepath.Join(dir, "config.ini"), interactive: true}
	var prompts []string
	read := func(p string, _, _ []string) (string, error) {
		prompts = append(prompts, p)
		return "", errors.New("stop")
	}
	r.ask("app", "locale", read, askOpts{})  // a default
	r.ask("app", "mode", read, askOpts{})    // invalid: no brackets
	r.ask("app", "retries", read, askOpts{}) // set, but empty: invalid for an int
	r.ask("", "name", read, askOpts{})       // nothing at all
	want := []string{"app.locale [en-US]> ", "app.mode> ", "app.retries> ", "name> "}
	if !reflect.DeepEqual(prompts, want) {
		t.Errorf("prompts:\n got %q\nwant %q", prompts, want)
	}
	if got := r.recallFor("app", "mode", s.ruleFor("app", "mode")); !reflect.DeepEqual(got, []string{"tablet"}) {
		t.Errorf("recall for an invalid value: %q", got)
	}
}

func TestSchemaIdentity(t *testing.T) {
	cases := map[string]string{
		"[@ask]\napplication=SignLoop 2.4\nversion=3\n": "version 3, for SignLoop 2.4",
		"[@ask]\nversion=3\n":                           "version 3",
		"[@ask]\napplication=SignLoop\n":                "for SignLoop",
		"[@ask]\ntitle=x\n":                             "",
		"a=1\n":                                         "", // no [@ask] at all
	}
	for src, want := range cases {
		s, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", src))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Identity(); got != want {
			t.Errorf("%q: got %q, want %q", src, got, want)
		}
	}
	var none *Schema
	if none.Identity() != "" {
		t.Error("nil schema")
	}
}

func TestShellExportNeedsSave(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.ini", "name=demo\n")
	ini, _ := LoadIni(cfg, ".")
	r := &Repl{ini: ini, saved: ini.Clone(), path: cfg, disk: statFile(cfg)}
	target := filepath.Join(dir, "config.json")

	r.exec("set name=changed")
	r.exec("export")
	if _, err := os.Stat(target); err == nil {
		t.Fatal("exported with unsaved changes")
	}
	r.exec("save")
	r.exec("export")
	got, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(got), `"name": "changed"`) {
		t.Errorf("export after save: %v %q", err, got)
	}
}
