package main

import (
	"strings"
	"testing"
)

func TestBuiltinTypes(t *testing.T) {
	cases := []struct {
		typ, in, want string // want "" means invalid
	}{
		{"x11color", "SteelBlue", "SteelBlue"},
		{"x11color", "steel blue", "SteelBlue"},
		{"x11color", "LIGHTGOLDENRODYELLOW", "LightGoldenrodYellow"},
		{"x11color", "grey", "Grey"},
		{"x11color", "Steel Blu", ""},
		{"x11color", "#4682B4", ""},
		{"hexcolor", "#4682b4", "#4682B4"},
		{"hexcolor", "4682b4", ""},
		{"hexcolor", "#4682b", ""},
		{"date", "2026-11-05", "2026-11-05"},
		{"date", "2026-2-3", "2026-02-03"},
		{"date", "2028-02-29", "2028-02-29"}, // leap year
		{"date", "2026-02-30", ""},
		{"date", "2026-13-01", ""},
		{"date", "11/05/2026", ""},
		{"time", "07:30", "07:30"},
		{"time", "7:30", "07:30"},
		{"time", "23:59", "23:59"},
		{"time", "24:00", ""},
		{"time", "7am", ""},
		{"time12", "7:30 PM", "7:30 PM"},
		{"time12", "07:30pm", "7:30 PM"},
		{"time12", "12:05 a.m.", "12:05 AM"},
		{"time12", "13:00 pm", ""},
		{"time12", "19:30", ""},
		{"duration", "1h30m", "1h30m"},
		{"duration", "90 S", "90s"},
		{"duration", "2d", "2d"},
		{"duration", "1.5h", ""},
		{"duration", "ten minutes", ""},
		{"size", "512mb", "512MB"},
		{"size", "2 GB", "2GB"},
		{"size", "100b", "100B"},
		{"size", "1.5GB", ""},
		{"size", "2 gigs", ""},
		{"ipv4", "192.168.1.20", "192.168.1.20"},
		{"ipv4", "192.168.1.256", ""},
		{"ipv4", "::1", ""},
		{"ip", "FE80::0001", "fe80::1"},
		{"ip", "10.0.0.1", "10.0.0.1"},
		{"ip", "fe80::1%eth0", ""},
		{"host", "Signage-07.Example.com", "signage-07.example.com"},
		{"host", "10.0.0.5", "10.0.0.5"},
		{"host", "bad_host", ""},
		{"host", "-lead.example.com", ""},
		{"url", "HTTPS://Prices.Example.com/API?x=1", "https://prices.example.com/API?x=1"},
		{"url", "ftp://example.com", ""},
		{"url", "example.com", ""},
		{"email", "IT@Example.COM", "IT@example.com"},
		{"email", "it@example", ""},
		{"email", "it at example.com", ""},
		{"timezone", "America/New_York", "America/New_York"},
		{"timezone", "UTC", "UTC"},
		{"timezone", "Mars/Olympus", ""},
		{"timezone", "Local", ""},
		{"port", "8080", "8080"},
		{"port", "08080", "8080"},
		{"port", "0", ""},
		{"port", "70000", ""},
	}
	for _, c := range cases {
		b := builtinType(c.typ)
		if b == nil {
			t.Fatalf("no built-in type %s", c.typ)
		}
		got, err := b.check(c.in)
		switch {
		case c.want == "" && err == nil:
			t.Errorf("%s %q: expected an error, got %q", c.typ, c.in, got)
		case c.want != "" && (err != nil || got != c.want):
			t.Errorf("%s %q: got %q, %v; want %q", c.typ, c.in, got, err, c.want)
		}
	}
}

func TestX11Suggestion(t *testing.T) {
	_, err := builtinType("x11color").check("SteelBlu")
	if err == nil || !strings.Contains(err.Error(), "did you mean SteelBlue?") {
		t.Errorf("near miss: %v", err)
	}
	for in, want := range map[string]string{"Tomatoe": "Tomato", "dodger blu": "DodgerBlue", "lightgoldenrodyello": "LightGoldenrodYellow", "gery": "Grey"} {
		if got := nearestColor(in); got != want {
			t.Errorf("nearest to %q: %q, want %q", in, got, want)
		}
	}
	_, err = builtinType("x11color").check("Zebra")
	if err == nil || strings.Contains(err.Error(), "did you mean") {
		t.Errorf("far miss: %v", err)
	}
	if n := len(x11Colors); n < 140 || n != len(x11ByKey) {
		t.Errorf("%d names, %d distinct", n, len(x11ByKey))
	}
}

func TestBuiltinInSchema(t *testing.T) {
	src := `[look]
background=steel blue

[@rule look.background]
type=x11color
doc=Color behind the playlist

[@rule look.accent]
type=hexcolor
format=a hex color, e.g. #4682B4

[@rule event.date]
type=date
`
	s, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", src))
	if err != nil {
		t.Fatal(err)
	}
	bg := s.ruleFor("look", "background")
	if bg.Hint() != "an X11 color name, e.g. SteelBlue; Tab lists them" {
		t.Errorf("hint: %q", bg.Hint())
	}
	if len(bg.choices()) != len(x11Colors) {
		t.Errorf("choices: %d", len(bg.choices()))
	}
	if got := s.ruleFor("look", "accent").Hint(); got != "a hex color, e.g. #4682B4" {
		t.Errorf("format wins over the type's hint: %q", got)
	}
	// The default is checked and reported in standard spelling.
	cfg := NewIni()
	if v, _ := s.Effective(cfg).Get("look.background"); v != "SteelBlue" {
		t.Errorf("default: %q", v)
	}
	// Lines in explain.
	lines, _ := s.Explain(cfg, "event.date")
	if lines[1] != "type: date (built in): YYYY-MM-DD, e.g. 2026-11-05" {
		t.Errorf("explain: %q", lines)
	}

	for src, want := range map[string]string{
		"[@rule a]\ntype=date\nmin=2026-01-01\n":      "min doesn't apply to type date",
		"[@rule a]\ntype=x11color\nvalues=red\n":      "values doesn't apply",
		"[@rule a]\ntype=colour\n":                    "built in: date, duration",
		"a=Purpleish\n[@rule a]\ntype=x11color\n":     "breaks its own rule",
		"[@rule a]\ntype=x11color\npattern=[A-Z].*\n": "pattern doesn't apply",
		"[@rule a]\ntype=Date\n":                      "", // type names ignore case
	} {
		_, err := LoadSchema(writeFile(t, t.TempDir(), "schema.ini", src))
		switch {
		case want == "" && err != nil:
			t.Errorf("%q: %v", src, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%q: got %v, want an error containing %q", src, err, want)
		}
	}
}
