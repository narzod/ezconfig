package main

import (
	"reflect"
	"strings"
	"testing"
)

const sample = `name=demo
nightly=true
[net]
host=localhost
port=8080
[display]
width=1920
`

func newTestRepl(t *testing.T) *Repl {
	ini := NewIni()
	if err := ini.parse(strings.NewReader(sample)); err != nil {
		t.Fatal(err)
	}
	return &Repl{ini: ini}
}

func TestComplete(t *testing.T) {
	r := newTestRepl(t)
	cases := []struct {
		line string
		want []string
	}{
		{"", []string{"changes ", "copy ", "delim ", "dump ", "get ", "help ", "history ", "keys ", "list ", "move ", "path ", "quit ",
			"revert ", "save ", "sections ", "set ", "status ", "unset ", "version ", "wipe "}},
		{"se", []string{"sections ", "set "}},
		{"delim ", []string{"/", ":", "|"}},
		{"d", []string{"delim ", "dump "}},
		{"du", []string{"dump "}},
		{"w", []string{"wipe "}},
		{"h", []string{"help ", "history "}},
		{"copy ", []string{". ", "display ", "net "}},
		{"copy n", []string{"net "}},
		{"copy net ", nil},
		{"history ", []string{"off", "on"}},
		{"history o", []string{"off", "on"}},
		{"history on ", nil},
		{"get ", []string{"name ", "nightly ", "display.", "net."}},
		{"get n", []string{"name ", "nightly ", "net."}},
		{"get net.", []string{"net.host ", "net.port "}},
		{"get net.p", []string{"net.port "}},
		{"get name net.h", []string{"net.host "}},
		{"set n", []string{"name=", "nightly=", "net."}},
		{"set net.po", []string{"net.port="}},
		{"set net.port=", []string{"net.port=8080"}},
		{"set net.port=1 ", nil},
		{"keys ", []string{".", "display", "net"}},
		{"get .", nil},
		{"keys d", []string{"display"}},
		{"unset display.", []string{"display.width "}},
		{"unset n", []string{"name ", "nightly ", "net."}},
		{"wipe ", []string{". ", "display ", "net "}},
		{"wipe net n", []string{"net "}},
		{"list ", []string{".", "display", "net"}},
		{"list net ", nil},
		{"dump ", []string{".", "display", "net"}},
		{"l", []string{"list "}},
		{"c", []string{"changes ", "copy "}},
		{"m", []string{"move "}},
		{"move ", []string{". ", "display ", "net "}},
		{"move net ", nil},
		{"ch", []string{"changes "}},
		{"changes ", []string{".", "display", "net"}},
		{"j", nil},
		{"status ", nil},
		{"sections ", nil},
	}
	for _, c := range cases {
		head, got, _ := r.complete(c.line, len(c.line))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %q, want %q", c.line, got, c.want)
		}
		if !strings.HasPrefix(c.line, head) {
			t.Errorf("%q: head %q is not a prefix", c.line, head)
		}
	}
}

func TestList(t *testing.T) {
	r := newTestRepl(t)
	all, _ := r.ini.List("")
	want := []string{"name=demo", "nightly=true", "net.host=localhost", "net.port=8080", "display.width=1920"}
	if !reflect.DeepEqual(all, want) {
		t.Errorf("list: got %q", all)
	}
	root, _ := r.ini.List(".")
	if !reflect.DeepEqual(root, want[:2]) {
		t.Errorf("list .: got %q", root)
	}
	if keys, ok := r.ini.Keys("."); !ok || !reflect.DeepEqual(keys, []string{"name", "nightly"}) {
		t.Errorf("keys .: got %q", keys)
	}
	net, _ := r.ini.List("net")
	if !reflect.DeepEqual(net, want[2:4]) {
		t.Errorf("list net: got %q", net)
	}
	if _, ok := r.ini.List("nope"); ok {
		t.Error("list nope: expected not ok")
	}
}

func TestCopySection(t *testing.T) {
	r := newTestRepl(t)
	if err := r.ini.CopySection("net", "net2"); err != nil {
		t.Fatal(err)
	}
	got, _ := r.ini.List("net2")
	if !reflect.DeepEqual(got, []string{"net2.host=localhost", "net2.port=8080"}) {
		t.Errorf("copy: got %q", got)
	}
	// The copy is independent of the original.
	r.ini.Set("net2.port", "9")
	if v, _ := r.ini.Get("net.port"); v != "8080" {
		t.Errorf("original changed: %s", v)
	}
	if secs := r.ini.Sections(); secs[len(secs)-1] != "net2" {
		t.Errorf("copy not appended last: %q", secs)
	}
	for _, c := range []struct{ src, dst string }{
		{"nope", "x"}, {"net", "display"}, {"net", "name"}, {"net", "a.b"}, {"net", ""}, {"net", "a b"},
	} {
		if err := r.ini.CopySection(c.src, c.dst); err == nil {
			t.Errorf("copy %q %q: expected error", c.src, c.dst)
		}
	}
}

func TestCopyRoot(t *testing.T) {
	r := newTestRepl(t)
	// root -> new section
	if err := r.ini.CopySection(".", "globals"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.ini.List("globals"); !reflect.DeepEqual(got, []string{"globals.name=demo", "globals.nightly=true"}) {
		t.Errorf("copy . globals: got %q", got)
	}
	// section -> root refused while the root has keys
	if err := r.ini.CopySection("net", "."); err == nil {
		t.Error("copy net .: expected refusal (root not empty)")
	}
	if err := r.ini.CopySection(".", "."); err == nil {
		t.Error("copy . .: expected refusal")
	}
	// after wiping the root, section -> root works
	r.ini.ClearRoot()
	if err := r.ini.CopySection("net", "."); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.ini.List("."); !reflect.DeepEqual(got, []string{"host=localhost", "port=8080"}) {
		t.Errorf("copy net .: got %q", got)
	}
	// a key that would share its name with a section is refused
	r.ini.ClearRoot()
	r.ini.Set("x.display", "1")
	if err := r.ini.CopySection("x", "."); err == nil {
		t.Error("copy x .: expected clash refusal")
	}
}

func TestDelimiter(t *testing.T) {
	ini := NewIni()
	ini.delim = "/"
	src := "php.version=8\n[boot.description]\nkeyboard.typ=US\n[net]\nport=80\n"
	if err := ini.parse(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	r := &Repl{ini: ini}
	if v, ok := ini.Get("boot.description/keyboard.typ"); !ok || v != "US" {
		t.Errorf("get with /: got %q %v", v, ok)
	}
	if v, ok := ini.Get("php.version"); !ok || v != "8" {
		t.Errorf("dotted root key: got %q %v", v, ok)
	}
	if got, _ := ini.List("/"); !reflect.DeepEqual(got, []string{"php.version=8"}) {
		t.Errorf("list /: got %q", got)
	}
	if got, _ := ini.List("net"); !reflect.DeepEqual(got, []string{"net/port=80"}) {
		t.Errorf("list net: got %q", got)
	}
	_, cands, _ := r.complete("get boot.description/k", len("get boot.description/k"))
	if !reflect.DeepEqual(cands, []string{"boot.description/keyboard.typ "}) {
		t.Errorf("complete with /: got %q", cands)
	}
	_, cands, _ = r.complete("list ", len("list "))
	if !reflect.DeepEqual(cands, []string{"/", "boot.description", "net"}) {
		t.Errorf("list completion with /: got %q", cands)
	}
	if n := ini.NamesContaining("."); n != 3 {
		t.Errorf("NamesContaining(.): got %d, want 3", n)
	}
	if err := ini.CopySection("net", "a/b"); err == nil {
		t.Error("copy to name containing delimiter: expected error")
	}
	if err := ini.CopySection("net", "net.v2"); err != nil {
		t.Errorf("copy to dotted name with / delimiter: %v", err)
	}
}

func TestCheckDelim(t *testing.T) {
	for _, ok := range []string{".", "/", ":", "|", "!"} {
		if err := checkDelim(ok); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
	for _, bad := range []string{"", "//", "=", "[", "]", ";", "#", " ", "\t"} {
		if err := checkDelim(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestJSON(t *testing.T) {
	r := newTestRepl(t)
	got, _, err := r.ini.JSON("")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "name": "demo",
  "nightly": "true",
  "net": {
    "host": "localhost",
    "port": "8080"
  },
  "display": {
    "width": "1920"
  }
}
`
	if got != want {
		t.Errorf("dump:\n%s", got)
	}
	r.ini.Set("net", "clash")
	if _, _, err := r.ini.JSON(""); err == nil {
		t.Error("expected clash error")
	}
	if empty, _, _ := NewIni().JSON(""); empty != "{}\n" {
		t.Errorf("empty dump: %q", empty)
	}
}

func TestMidLineCompletion(t *testing.T) {
	r := newTestRepl(t)
	line := "get net.h display.width"
	head, got, tail := r.complete(line, len("get net.h"))
	if head != "get " || tail != " display.width" || !reflect.DeepEqual(got, []string{"net.host "}) {
		t.Errorf("got head=%q cands=%q tail=%q", head, got, tail)
	}
}
