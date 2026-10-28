package main

import (
	"reflect"
	"strings"
	"testing"
)

func changeLines(old, cur *Ini) []string {
	var out []string
	for _, c := range diffIni(old, cur) {
		out = append(out, c.line(cur.delim))
	}
	return out
}

func TestDiffIni(t *testing.T) {
	r := newTestRepl(t)
	saved := r.ini.Clone()

	if got := changeLines(saved, r.ini); got != nil {
		t.Fatalf("no edits: got %q", got)
	}

	r.ini.Set("net.port", "9090")
	r.ini.Set("motd", "hello")
	r.ini.Unset("name")
	want := []string{"+ motd=hello", "- name=demo", "~ net.port=9090 (was 8080)"}
	if got := changeLines(saved, r.ini); !reflect.DeepEqual(got, want) {
		t.Errorf("edits: got %q, want %q", got, want)
	}

	// Setting a value back to its saved value is not a change.
	r.ini.Set("net.port", "8080")
	r.ini.Set("net.port", "8080")
	if got := changeLines(saved, r.ini); len(got) != 2 {
		t.Errorf("after setting back: got %q", got)
	}

	// The clone is independent of later edits.
	if v, _ := saved.Get("net.port"); v != "8080" {
		t.Errorf("clone changed: %s", v)
	}
}

func TestDiffSections(t *testing.T) {
	r := newTestRepl(t)
	saved := r.ini.Clone()

	r.ini.UnsetSection("display")
	r.ini.CopySection("net", "net2")
	r.ini.Set("empty.x", "1")
	r.ini.Unset("empty.x") // leaves an empty section behind
	want := []string{"+ net2.host=localhost", "+ net2.port=8080", "+ [empty]", "- display.width=1920"}
	if got := changeLines(saved, r.ini); !reflect.DeepEqual(got, want) {
		t.Errorf("sections: got %q, want %q", got, want)
	}

	// The current delimiter is used for display.
	r.ini.delim = "/"
	if got := changeLines(saved, r.ini); !strings.HasPrefix(got[0], "+ net2/host=") {
		t.Errorf("delimiter: got %q", got)
	}
}

func TestPromptLabel(t *testing.T) {
	// Run on Linux; the Windows forms (C:\SignLoop\config.ini ->
	// SignLoop\config.ini, C:\config.ini -> C:\config.ini) follow the same
	// rules through filepath.
	cases := map[string]string{
		"/home/jeff/signloop/config.ini": "signloop/config.ini",
		"/etc/config.ini":                "etc/config.ini",
		"/config.ini":                    "/config.ini",
	}
	for in, want := range cases {
		if got := promptLabel(in); got != want {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}
}

func TestMoveSection(t *testing.T) {
	r := newTestRepl(t)
	ini := r.ini

	// A rename keeps the section's position.
	if err := ini.MoveSection("net", "web"); err != nil {
		t.Fatal(err)
	}
	if got := ini.Sections(); !reflect.DeepEqual(got, []string{"web", "display"}) {
		t.Errorf("rename: sections %q", got)
	}
	if v, _ := ini.Get("web.port"); v != "8080" {
		t.Errorf("rename: web.port = %q", v)
	}

	// Refusals.
	for _, c := range []struct{ src, dst string }{
		{"nope", "x"}, {"web", "display"}, {"web", "name"}, {"web", "a.b"}, {"web", "web"}, {"web", "."},
	} {
		if err := ini.MoveSection(c.src, c.dst); err == nil {
			t.Errorf("move %s %s: expected an error", c.src, c.dst)
		}
	}

	// The root keys become a new section at the end, leaving the root empty.
	if err := ini.MoveSection(".", "globals"); err != nil {
		t.Fatal(err)
	}
	if got, _ := ini.List("."); got != nil {
		t.Errorf("root after move: %q", got)
	}
	if got := ini.Sections(); got[len(got)-1] != "globals" {
		t.Errorf("globals not last: %q", got)
	}

	// A section's keys can become the root keys while the root is empty,
	// even a key named after the section being moved away.
	ini.Set("legacy.legacy", "1")
	if err := ini.MoveSection("legacy", "."); err != nil {
		t.Fatal(err)
	}
	if v, _ := ini.Get("legacy"); v != "1" || ini.section("legacy") != nil {
		t.Errorf("move into root: value %q, section still there: %v", v, ini.section("legacy") != nil)
	}

	// Copying, by contrast, keeps the section, so that same name clashes.
	ini.ClearRoot()
	ini.Set("keep.keep", "1")
	if err := ini.CopySection("keep", "."); err == nil {
		t.Error("copy keep .: expected a name clash")
	}
}

func TestKeyPaths(t *testing.T) {
	r := newTestRepl(t)
	all, _ := r.ini.KeyPaths("")
	if !reflect.DeepEqual(all, []string{"name", "nightly", "net.host", "net.port", "display.width"}) {
		t.Errorf("all: %q", all)
	}
	if got, _ := r.ini.KeyPaths("net"); !reflect.DeepEqual(got, []string{"net.host", "net.port"}) {
		t.Errorf("net: %q", got)
	}
	if got, _ := r.ini.KeyPaths("."); !reflect.DeepEqual(got, []string{"name", "nightly"}) {
		t.Errorf("root: %q", got)
	}
	if _, ok := r.ini.KeyPaths("nope"); ok {
		t.Error("nope: expected not ok")
	}
}
