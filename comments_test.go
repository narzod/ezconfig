package main

import (
	"strings"
	"testing"
)

func TestCommentsKept(t *testing.T) {
	src := `; Settings for the player.
; Run setup to change them.

; the name
name=a
# hash comments too
debug=on

; about the network
[net]
; the port
port=1
host=x

; trailing remark
`
	ini := NewIni()
	if err := ini.parse(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	ini.Set("net.port", "2")
	ini.Set("net.timeout", "30")
	ini.Unset("debug") // its comment goes with it
	want := `; Settings for the player.
; Run setup to change them.

; the name
name=a

; about the network
[net]
; the port
port=2
host=x
timeout=30

; trailing remark
`
	if got := ini.Text(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	// A file that is only a comment keeps it at the top as keys are added.
	only := NewIni()
	only.parse(strings.NewReader("; Run setup to change these.\n"))
	only.Set("player.site", "0142")
	if got := only.Text(); got != "; Run setup to change these.\n\n[player]\nsite=0142\n" {
		t.Errorf("comment-only file:\n%s", got)
	}

	// A comment right above the first key, with no blank line, is that key's.
	k := NewIni()
	k.parse(strings.NewReader("; the name\nname=a\n"))
	k.Unset("name")
	if got := k.Text(); got != "" {
		t.Errorf("key comment kept after unset: %q", got)
	}

	// Renaming a section keeps its comment; the clone keeps them all.
	ini.MoveSection("net", "web")
	if c := ini.Clone().Text(); !strings.Contains(c, "; about the network\n[web]\n; the port\nport=2") {
		t.Errorf("after move, clone:\n%s", c)
	}
}
