package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseList(t *testing.T) {
	cases := []struct {
		args        []string
		format, sec string
		bad         bool
	}{
		{nil, "flat", "", false},
		{[]string{"json"}, "json", "", false},
		{[]string{"ini", "net"}, "ini", "net", false},
		{[]string{"net"}, "flat", "net", false},
		{[]string{"net."}, "flat", "net", false},
		{[]string{"json."}, "flat", "json", false},         // the section named json
		{[]string{"json", "json."}, "json", "json", false}, // that section, as JSON
		{[]string{"json", "json"}, "json", "json", false},  // after a format, a section
		{[]string{"."}, "flat", ".", false},
		{[]string{"json", "net", "x"}, "", "", true},
		{[]string{"net", "x"}, "", "", true},
	}
	for _, c := range cases {
		format, sec, err := parseList(c.args, ".")
		if (err != nil) != c.bad || (!c.bad && (format != c.format || sec != c.sec)) {
			t.Errorf("%q: got %q %q %v, want %q %q bad=%v", c.args, format, sec, err, c.format, c.sec, c.bad)
		}
	}
}

func TestINIFormat(t *testing.T) {
	ini := NewIni()
	ini.parse(strings.NewReader("; a comment\nname=demo\n[net]\n; port\nport=1\n[empty]\n"))
	if got, _ := ini.INI(""); got != "name=demo\n\n[net]\nport=1\n\n[empty]\n" {
		t.Errorf("all: %q", got)
	}
	if got, _ := ini.INI("."); got != "name=demo\n" {
		t.Errorf("root: %q", got)
	}
	if _, ok := ini.INI("nope"); ok {
		t.Error("nope: expected not found")
	}
	// A root key and a section with the same name are fine in INI.
	ini.Set("net", "wired")
	if got, _ := ini.INI(""); !strings.Contains(got, "net=wired\n") || !strings.Contains(got, "[net]\nport=1") {
		t.Errorf("clash in ini: %q", got)
	}
}

func TestOneShotListFormats(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.ini", "name=demo\n[json]\nx=1\n[net]\nport=1\n")
	if code, out, _ := run(t, cfg, ".", "list", "json."); code != 0 || out != "json.x=1\n" {
		t.Errorf("list json.: %d %q", code, out)
	}
	if code, out, _ := run(t, cfg, ".", "list", "json", "json."); code != 0 || out != "{\n  \"x\": \"1\"\n}\n" {
		t.Errorf("list json json.: %d %q", code, out)
	}
	if code, out, _ := run(t, cfg, ".", "list", "ini"); code != 0 || !strings.HasPrefix(out, "name=demo\n\n[json]\nx=1\n") {
		t.Errorf("list ini: %d %q", code, out)
	}
	if code, _, errOut := run(t, cfg, ".", "emit"); code != exitArgs || !strings.Contains(errOut, "emit is now list json") {
		t.Errorf("emit: %d %q", code, errOut)
	}
}

func TestExport(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "signage.ini", "name=demo\n[net]\nport=1\n")
	writeFile(t, dir, "schema.ini", "[net]\nhost=localhost\n")

	// The default file is named after the config, next to it.
	code, out, errOut := run(t, cfg, ".", "export")
	target := filepath.Join(dir, "signage.json")
	if code != 0 || out != "" || !strings.Contains(errOut, "exported 3 keys to "+target) {
		t.Errorf("export: %d %q %q", code, out, errOut)
	}
	got, _ := os.ReadFile(target)
	if _, want, _ := run(t, cfg, ".", "list", "json"); string(got) != want {
		t.Errorf("exported file differs from list json:\n%s\n%s", got, want)
	}

	// A file can be named.
	other := filepath.Join(dir, "out", "settings.json")
	os.Mkdir(filepath.Join(dir, "out"), 0o755)
	if code, _, _ := run(t, cfg, ".", "export", other); code != 0 {
		t.Errorf("export FILE: %d", code)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("export FILE: %v", err)
	}

	// Never over the config or the schema.
	for _, f := range []string{cfg, filepath.Join(dir, "schema.ini")} {
		before, _ := os.ReadFile(f)
		if code, _, errOut := run(t, cfg, ".", "export", f); code != exitArgs || !strings.Contains(errOut, "choose another file") {
			t.Errorf("export over %s: %d %q", f, code, errOut)
		}
		if after, _ := os.ReadFile(f); string(after) != string(before) {
			t.Errorf("%s was changed", f)
		}
	}

	// A renamed root key: written, exit 7. A double clash: nothing written, exit 2.
	writeFile(t, dir, "signage.ini", "net=wired\n[net]\nport=1\n")
	if code, _, errOut := run(t, cfg, ".", "export"); code != exitRenamed || !strings.Contains(errOut, `shown as "net="`) {
		t.Errorf("export with a clash: %d %q", code, errOut)
	}
	os.Remove(target)
	writeFile(t, dir, "signage.ini", "net=wired\n[net]\nport=1\n[net=]\nx=1\n")
	if code, _, _ := run(t, cfg, ".", "export"); code != exitSyntax {
		t.Errorf("export with a double clash: %d", code)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("export wrote a file it couldn't represent")
	}
}

func TestExportTarget(t *testing.T) {
	if got := exportTarget(filepath.FromSlash("/a/config.ini"), ""); got != filepath.FromSlash("/a/config.json") {
		t.Errorf("default: %q", got)
	}
	if got := exportTarget(filepath.FromSlash("/a/settings"), ""); got != filepath.FromSlash("/a/settings.json") {
		t.Errorf("no extension: %q", got)
	}
}
