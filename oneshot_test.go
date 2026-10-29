package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// run executes a one-shot command against path and returns its exit
// status, stdout and stderr.
func run(t *testing.T, path, delim, cmd string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := oneShot(path, delim, "", cmd, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func tempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.ini")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOneShotRead(t *testing.T) {
	p := tempConfig(t, sample)
	cases := []struct {
		cmd  string
		args []string
		code int
		out  string
	}{
		{"get", []string{"net.port"}, 0, "8080\n"},
		{"get", []string{"name", "net.host"}, 0, "demo\nlocalhost\n"},
		{"get", []string{"name", "net.nope", "net.host"}, 5, ""}, // all or nothing
		{"get", nil, 4, ""},
		{"keys", nil, 0, "name\nnightly\nnet.host\nnet.port\ndisplay.width\n"},
		{"keys", []string{"net"}, 0, "net.host\nnet.port\n"},
		{"keys", []string{"."}, 0, "name\nnightly\n"},
		{"keys", []string{"nope"}, 5, ""},
		{"sections", nil, 0, "net\ndisplay\n"},
		{"sections", []string{"x"}, 4, ""},
		{"list", []string{"net"}, 0, "net.host=localhost\nnet.port=8080\n"},
		{"list", []string{"."}, 0, "name=demo\nnightly=true\n"},
		{"list", []string{"nope"}, 5, ""},
		{"list", []string{"a", "b"}, 4, ""},
		{"emit", nil, 0, "{\n  \"name\": \"demo\","},
		{"emit", []string{"net"}, 0, "{\n  \"host\": \"localhost\",\n  \"port\": \"8080\"\n}\n"},
		{"emit", []string{"."}, 0, "{\n  \"name\": \"demo\",\n  \"nightly\": \"true\"\n}\n"},
		{"emit", []string{"nope"}, 5, ""},
		{"json", nil, 4, ""},
		{"wipe", []string{"net"}, 4, ""},
		{"move", []string{"net", "web"}, 4, ""},
		{"bogus", nil, 4, ""},
	}
	for _, c := range cases {
		code, out, _ := run(t, p, ".", c.cmd, c.args...)
		if code != c.code || !strings.HasPrefix(out, c.out) || (c.out == "" && out != "") {
			t.Errorf("%s %q: got code %d out %q, want %d %q", c.cmd, c.args, code, out, c.code, c.out)
		}
	}
	if readFile(t, p) != sample {
		t.Error("read-only commands changed the file")
	}
}

func TestOneShotSet(t *testing.T) {
	p := tempConfig(t, sample)
	if code, out, errOut := run(t, p, ".", "set", "net.port=9090", "motd=hi there", "new.key=1"); code != 0 || out != "" || errOut != "" {
		t.Fatalf("set: code %d out %q err %q", code, out, errOut)
	}
	_, out, _ := run(t, p, ".", "get", "net.port", "motd", "new.key")
	if out != "9090\nhi there\n1\n" {
		t.Errorf("after set: %q", out)
	}

	// A bad pair changes nothing.
	before := readFile(t, p)
	if code, _, _ := run(t, p, ".", "set", "net.port=1", "oops"); code != 4 {
		t.Errorf("bad pair: code %d", code)
	}
	if code, _, _ := run(t, p, ".", "set", "net.port=1", "[x]=1"); code != 4 {
		t.Errorf("bracket key: code %d", code)
	}
	if readFile(t, p) != before {
		t.Error("failed set changed the file")
	}

	// Setting a value it already has leaves the file untouched.
	fi1, _ := os.Stat(p)
	os.Chtimes(p, fi1.ModTime().Add(-1e9*60), fi1.ModTime().Add(-1e9*60))
	fi1, _ = os.Stat(p)
	run(t, p, ".", "set", "net.port=9090")
	fi2, _ := os.Stat(p)
	if !fi2.ModTime().Equal(fi1.ModTime()) {
		t.Error("no-op set rewrote the file")
	}

	// No temporary files are left behind.
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Errorf("leftover files: %v", entries)
	}
}

func TestOneShotUnset(t *testing.T) {
	p := tempConfig(t, sample)
	if code, _, _ := run(t, p, ".", "unset", "name", "net.nope"); code != 5 {
		t.Errorf("unset with a missing key: code %d", code)
	}
	if readFile(t, p) != sample {
		t.Error("failed unset changed the file")
	}
	if code, _, _ := run(t, p, ".", "unset", "name", "net.host"); code != 0 {
		t.Errorf("unset: code %d", code)
	}
	_, out, _ := run(t, p, ".", "list")
	if out != "nightly=true\nnet.port=8080\ndisplay.width=1920\n" {
		t.Errorf("after unset: %q", out)
	}
}

func TestOneShotFiles(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "new.ini")
	if code, _, _ := run(t, missing, ".", "get", "a"); code != 3 {
		t.Errorf("get on missing file: code %d", code)
	}
	code, _, errOut := run(t, missing, ".", "set", "a=1")
	if code != 0 || !strings.Contains(errOut, "created") {
		t.Errorf("set on missing file: code %d err %q", code, errOut)
	}
	if readFile(t, missing) != "a=1\n" {
		t.Errorf("created file: %q", readFile(t, missing))
	}
	bad := tempConfig(t, "a=1\n[broken\n")
	if code, _, _ := run(t, bad, ".", "get", "a"); code != 2 {
		t.Errorf("syntax error: code %d", code)
	}
}

func TestOneShotDelim(t *testing.T) {
	p := tempConfig(t, "[boot.description]\nkeyboard.typ=US\n")
	code, _, errOut := run(t, p, ".", "get", "boot.description.keyboard.typ")
	if code != 5 || !strings.Contains(errOut, "-d") {
		t.Errorf("dotted names with '.': code %d err %q", code, errOut)
	}
	if _, out, _ := run(t, p, "/", "get", "boot.description/keyboard.typ"); out != "US\n" {
		t.Errorf("with -d /: %q", out)
	}
}

func TestParseArgs(t *testing.T) {
	cases := []struct {
		argv []string
		want cmdLine
	}{
		{nil, cmdLine{path: "config.ini", delim: "."}},
		{[]string{"-c", "x.ini"}, cmdLine{path: "x.ini", delim: "."}},
		{[]string{"-c", "x.ini", "-d", "/", "get", "a/b", "-c"},
			cmdLine{path: "x.ini", delim: "/", delimSet: true, cmd: "get", args: []string{"a/b", "-c"}}},
		{[]string{"--config=y.ini", "--delim=:", "list"},
			cmdLine{path: "y.ini", delim: ":", delimSet: true, cmd: "list", args: []string{}}},
		{[]string{"--", "get", "-odd"}, cmdLine{path: "config.ini", delim: ".", cmd: "get", args: []string{"-odd"}}},
		{[]string{"--version"}, cmdLine{path: "config.ini", delim: ".", early: "version"}},
	}
	for _, c := range cases {
		got, err := parseArgs(c.argv)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %+v, %v", c.argv, got, err)
		}
	}
	for _, bad := range [][]string{{"-c"}, {"-x"}, {"-d"}} {
		if _, err := parseArgs(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}
