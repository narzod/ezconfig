package main

import (
	"os"
	"strings"
	"testing"
)

func TestColor(t *testing.T) {
	c := change{mark: '+', sec: "net", key: "port", val: "8080"}
	if got := changeText(c, "."); got != "+ net.port=8080" {
		t.Errorf("colour off: %q", got)
	}
	colorOut = true
	defer func() { colorOut = false }()
	if got := changeText(c, "."); got != "\x1b[32m+ net.port=8080\x1b[0m" {
		t.Errorf("added: %q", got)
	}
	if got := changeText(change{mark: '-', key: "name", val: "a"}, "."); !strings.HasPrefix(got, "\x1b[31m-") {
		t.Errorf("removed: %q", got)
	}
	if got := noteText("note: something"); got != "\x1b[33mnote:\x1b[0m something" {
		t.Errorf("note: %q", got)
	}
	if got := out(styleBold, ""); got != "" {
		t.Errorf("empty text gets no codes: %q", got)
	}
}

func TestNoColorConventions(t *testing.T) {
	for _, env := range [][2]string{{"NO_COLOR", "1"}, {"TERM", "dumb"}} {
		old, had := os.LookupEnv(env[0])
		os.Setenv(env[0], env[1])
		colorOut, colorErr = false, false
		setupColor() // returns before even looking for a terminal
		if colorOut || colorErr {
			t.Errorf("%s=%s: colour on", env[0], env[1])
		}
		if had {
			os.Setenv(env[0], old)
		} else {
			os.Unsetenv(env[0])
		}
	}
}
