package main

// oneshot.go - run a single command from the command line and exit.
//
//	inirepl [-c FILE] [-d CHAR] COMMAND [ARGS...]
//
// One-shot mode offers every command that reads, plus set and unset:
// get, keys, sections, dump, json, set, unset (and help, version).
// Everything else stays in the interactive shell.
//
// stdout carries only data; messages go to stderr. Writes are
// all-or-nothing and saved atomically, and only when something changed.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Exit codes, shared with validini's table.
const (
	exitOK       = 0
	exitSyntax   = 2 // INI syntax error in the config
	exitFile     = 3 // config unreadable (or unwritable)
	exitArgs     = 4 // bad arguments
	exitNotFound = 5 // key or section not found
)

// oneShotCommands are available from the command line; shellOnly are
// recognised so they can be refused with a helpful message.
var (
	oneShotCommands = map[string]bool{
		"get": true, "keys": true, "sections": true, "dump": true, "json": true,
		"set": true, "unset": true, "help": true, "version": true,
	}
	shellOnly = map[string]bool{
		"wipe": true, "copy": true, "status": true, "save": true, "revert": true,
		"history": true, "delim": true, "quit": true, "exit": true,
	}
)

const usage = `usage: inirepl [-c FILE] [-d CHAR] [COMMAND [ARGS...]]

Without a command, starts the interactive shell.

commands:
get KEY...         print values
set KEY=VALUE...   set values and save
unset KEY...       remove keys and save
keys [SECTION]     list root keys, or the keys of SECTION
sections           list section names
dump [SECTION]     print KEY=VALUE lines for everything, or for SECTION
json [SECTION]     print everything, or SECTION, as JSON
help               show this text
version            show the program version

options:
-c FILE            config to use (default: config.ini)
-d CHAR            delimiter between section and key (default: .)

exit status:
0  success
2  syntax error in the config
3  config missing or unreadable
4  bad arguments
5  key or section not found`

// oneShot runs cmd with args against the config at path and returns the
// exit status. Output goes to stdout, messages to stderr.
func oneShot(path, delim, cmd string, args []string, stdout, stderr io.Writer) int {
	fail := func(code int, format string, a ...any) int {
		fmt.Fprintf(stderr, "inirepl: "+format+"\n", a...)
		return code
	}

	switch cmd {
	case "help":
		fmt.Fprintln(stdout, usage)
		return exitOK
	case "version":
		fmt.Fprintln(stdout, "inirepl", version)
		return exitOK
	}

	// Load the config. Only set may start from a missing file.
	_, statErr := os.Stat(path)
	missing := os.IsNotExist(statErr)
	if missing && cmd != "set" {
		return fail(exitFile, "%s: no such file", path)
	}
	ini, err := LoadIni(path, delim)
	if err != nil {
		var se *SyntaxError
		if errors.As(err, &se) {
			return fail(exitSyntax, "%s: %v", path, err)
		}
		return fail(exitFile, "%s: %v", path, err)
	}

	// notFound reports missing names, with a hint when the delimiter
	// itself appears in names in this config.
	notFound := func(names []string) int {
		fmt.Fprintf(stderr, "inirepl: not found: %s\n", strings.Join(names, ", "))
		if n := ini.NamesContaining(delim); n > 0 {
			fmt.Fprintf(stderr, "inirepl: note: %s %q; try -d with another character\n",
				count(n, "name contains", "names contain"), delim)
		}
		return exitNotFound
	}

	switch cmd {
	case "get":
		if len(args) == 0 {
			return fail(exitArgs, "usage: get KEY...")
		}
		values := make([]string, 0, len(args))
		var missingKeys []string
		for _, p := range args {
			if v, ok := ini.Get(p); ok {
				values = append(values, v)
			} else {
				missingKeys = append(missingKeys, p)
			}
		}
		if len(missingKeys) > 0 { // all or nothing, so lines never shift
			return notFound(missingKeys)
		}
		for _, v := range values {
			fmt.Fprintln(stdout, v)
		}
		return exitOK

	case "keys", "dump":
		if len(args) > 1 {
			return fail(exitArgs, "usage: %s [SECTION]", cmd)
		}
		var lines []string
		var ok bool
		if cmd == "keys" {
			lines, ok = ini.Keys(firstOr(args, ""))
		} else {
			lines, ok = ini.List(firstOr(args, ""))
		}
		if !ok {
			return notFound(args)
		}
		for _, l := range lines {
			fmt.Fprintln(stdout, l)
		}
		return exitOK

	case "sections":
		if len(args) > 0 {
			return fail(exitArgs, "usage: sections")
		}
		for _, s := range ini.Sections() {
			fmt.Fprintln(stdout, s)
		}
		return exitOK

	case "json":
		if len(args) > 1 {
			return fail(exitArgs, "usage: json [SECTION]")
		}
		out, ok, err := ini.JSON(firstOr(args, ""))
		if !ok {
			return notFound(args)
		}
		if err != nil {
			return fail(exitSyntax, "json: %v", err)
		}
		fmt.Fprint(stdout, out)
		return exitOK

	case "set":
		if len(args) == 0 {
			return fail(exitArgs, "usage: set KEY=VALUE...")
		}
		// Check every pair before changing anything.
		type pair struct{ path, value string }
		pairs := make([]pair, 0, len(args))
		for _, a := range args {
			p, v, ok := strings.Cut(a, "=")
			p, v = strings.TrimSpace(p), strings.TrimSpace(v)
			if !ok || p == "" {
				return fail(exitArgs, "expected KEY=VALUE, got %q", a)
			}
			if err := ini.CheckSet(p, v); err != nil {
				return fail(exitArgs, "%v", err)
			}
			pairs = append(pairs, pair{p, v})
		}
		changed := missing // creating the file is itself a change
		for _, pr := range pairs {
			if old, ok := ini.Get(pr.path); !ok || old != pr.value {
				changed = true
			}
			ini.Set(pr.path, pr.value)
		}
		if !changed {
			return exitOK
		}
		if err := ini.Save(path); err != nil {
			return fail(exitFile, "save %s: %v", path, err)
		}
		if missing {
			fmt.Fprintf(stderr, "inirepl: created %s\n", path)
		}
		return exitOK

	case "unset":
		if len(args) == 0 {
			return fail(exitArgs, "usage: unset KEY...")
		}
		var missingKeys []string
		for _, p := range args {
			if _, ok := ini.Get(p); !ok {
				missingKeys = append(missingKeys, p)
			}
		}
		if len(missingKeys) > 0 { // nothing is removed unless all exist
			return notFound(missingKeys)
		}
		for _, p := range args {
			ini.Unset(p)
		}
		if err := ini.Save(path); err != nil {
			return fail(exitFile, "save %s: %v", path, err)
		}
		return exitOK
	}

	if shellOnly[cmd] {
		return fail(exitArgs, "%s is only available in the interactive shell (run inirepl without a command)", cmd)
	}
	return fail(exitArgs, "unknown command: %s (try inirepl help)", cmd)
}
