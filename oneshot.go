package main

// oneshot.go - run a single command from the command line and exit.
//
//	ezconfig [-c FILE] [-s FILE] [-d CHAR] COMMAND [ARGS...]
//
// One-shot mode offers every command that reads, plus set and unset:
// get, keys, sections, list, dump, check, explain, set, unset (and help,
// version). With a schema, reading commands see the effective config
// (defaults filled in) and set refuses values that break it.
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
	exitOK        = 0
	exitInvalid   = 1 // a value breaks the schema
	exitSyntax    = 2 // INI syntax error in the config or schema
	exitFile      = 3 // config or schema unreadable (or config unwritable)
	exitArgs      = 4 // bad arguments
	exitNotFound  = 5 // key or section not found
	exitCancelled = 6 // ask stopped or not saved; nothing written
	exitRenamed   = 7 // emit renamed a root key that clashed with a section; the output is complete
)

// oneShotCommands are available from the command line; shellOnly are
// recognised so they can be refused with a helpful message.
var (
	oneShotCommands = map[string]bool{
		"get": true, "keys": true, "sections": true, "list": true, "emit": true,
		"check": true, "explain": true, "ask": true, "set": true, "unset": true, "help": true, "version": true,
	}
	shellOnly = map[string]bool{
		"wipe": true, "copy": true, "move": true, "status": true, "save": true, "revert": true,
		"history": true, "delim": true, "changes": true, "path": true, "quit": true, "exit": true,
	}
)

const usage = `usage: ezconfig [-c FILE] [-s FILE] [-d CHAR] [COMMAND [ARGS...]]

Without a command, starts the interactive shell.

commands:
get KEY...         print values
set KEY=VALUE...   set values and save
unset KEY...       remove keys and save
keys [SECTION]     list key names for everything, or for SECTION
sections           list section names
list [SECTION]     print KEY=VALUE lines for everything, or for SECTION
emit [SECTION]     print everything, or SECTION, as JSON for programs
check              report problems: values that break the schema, clashing names
explain [NAME...]  explain keys and sections, or everything
ask [NAME...]      ask for values (all, or those named), then save, redo or quit
                   (the one command for people; needs a terminal)
help               show this text
version            show the program version

options:
-c FILE            config to use (default: config.ini)
-s FILE            schema to use, or none (default: schema.ini next to the config)
-d CHAR            delimiter between section and key (default: .)

exit status:
0  success
1  a value breaks the schema
2  syntax error in the config or schema
3  config or schema missing or unreadable
4  bad arguments
5  key or section not found
6  ask stopped or quit; nothing saved
7  emit renamed a root key that shares its name with a section (the output
   is complete: the key is shown as NAME=)`

// oneShot runs cmd with args against the config at path and returns the
// exit status. Output goes to stdout, messages to stderr.
func oneShot(path, delim, schemaOpt, cmd string, args []string, stdout, stderr io.Writer) int {
	fail := func(code int, format string, a ...any) int {
		fmt.Fprintf(stderr, "ezconfig: "+format+"\n", a...)
		return code
	}

	switch cmd {
	case "help":
		fmt.Fprintln(stdout, usage)
		return exitOK
	case "version":
		fmt.Fprintln(stdout, "ezconfig", version)
		return exitOK
	}

	// Refuse commands that don't belong here before touching the file.
	if !oneShotCommands[cmd] {
		if shellOnly[cmd] {
			return fail(exitArgs, "%s is only available in the interactive shell (run ezconfig without a command)", cmd)
		}
		return fail(exitArgs, "unknown command: %s (try ezconfig help)", cmd)
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

	// Load the schema, if there is one.
	schemaPath, err := findSchema(path, schemaOpt)
	if err != nil {
		return fail(exitFile, "%v", err)
	}
	var schema *Schema
	if schemaPath != "" {
		if schema, err = LoadSchema(schemaPath); err != nil {
			var se *SchemaError
			if errors.As(err, &se) {
				return fail(exitSyntax, "%s: %v", schemaPath, err)
			}
			return fail(exitFile, "%s: %v", schemaPath, err)
		}
	}
	// What applications see: defaults filled in, only valid values. Each
	// invalid value replaced or left out has a note, shown on stderr.
	view, notes := schema.EffectiveNotes(ini)
	warn := func(texts []string) {
		for _, t := range texts {
			fmt.Fprintf(stderr, "ezconfig: %s\n", t)
		}
	}
	// warnScope warns about notes within a SECTION argument ("" for all).
	warnScope := func(arg string) {
		switch arg {
		case "":
			warn(notesIn(notes, "", true))
		case delim:
			warn(notesIn(notes, "", false))
		default:
			warn(notesIn(notes, arg, false))
		}
	}
	needSchema := func() int {
		return fail(exitArgs, "no schema (put %s next to the config, or use -s FILE)", schemaName)
	}

	// notFound reports missing names, with a hint when the delimiter
	// itself appears in names in this config.
	notFound := func(names []string) int {
		fmt.Fprintf(stderr, "ezconfig: not found: %s\n", strings.Join(names, ", "))
		if n := ini.NamesContaining(delim); n > 0 {
			fmt.Fprintf(stderr, "ezconfig: note: %s %q; try -d with another character\n",
				count(n, "name contains", "names contain"), delim)
		}
		return exitNotFound
	}

	if sectionCommands[cmd] {
		args = sectionArgs(args, delim)
	}

	switch cmd {
	case "ask": // reached only without a terminal; main runs it otherwise
		return fail(exitArgs, "ask needs a terminal; use set KEY=VALUE in scripts")

	case "get":
		if len(args) == 0 {
			return fail(exitArgs, "usage: get KEY...")
		}
		values := make([]string, 0, len(args))
		var missingKeys, invalidKeys []string
		var asked [][2]string
		for _, p := range args {
			sec, key := ini.splitPath(p)
			asked = append(asked, [2]string{sec, key})
			if v, ok := view.Get(p); ok {
				values = append(values, v)
			} else if _, inConfig := ini.Get(p); inConfig {
				invalidKeys = append(invalidKeys, p) // invalid, and no default
			} else {
				missingKeys = append(missingKeys, p)
			}
		}
		warn(notesFor(notes, asked))
		switch { // all or nothing, so lines never shift
		case len(missingKeys) > 0:
			return notFound(missingKeys)
		case len(invalidKeys) > 0:
			return exitInvalid
		}
		for _, v := range values {
			fmt.Fprintln(stdout, v)
		}
		return exitOK

	case "keys", "list":
		if len(args) > 1 {
			return fail(exitArgs, "usage: %s [SECTION]", cmd)
		}
		var lines []string
		var ok bool
		if cmd == "keys" {
			lines, ok = view.KeyPaths(firstOr(args, ""))
		} else {
			lines, ok = view.List(firstOr(args, ""))
		}
		if !ok {
			return notFound(args)
		}
		warnScope(firstOr(args, ""))
		for _, l := range lines {
			fmt.Fprintln(stdout, l)
		}
		return exitOK

	case "sections":
		if len(args) > 0 {
			return fail(exitArgs, "usage: sections")
		}
		for _, s := range view.Sections() {
			fmt.Fprintln(stdout, s)
		}
		return exitOK

	case "check":
		if len(args) > 0 {
			return fail(exitArgs, "usage: check")
		}
		problems := append(view.Clashes(), view.OddSections()...) // names as emit sees them, defaults included
		problems = append(problems, schema.Problems(ini)...)
		for _, p := range problems {
			fmt.Fprintln(stdout, p)
		}
		if len(problems) > 0 {
			return exitInvalid
		}
		return exitOK

	case "explain":
		if schema == nil {
			return needSchema()
		}
		if len(args) == 0 {
			for _, l := range schema.ExplainAll(ini) {
				fmt.Fprintln(stdout, l)
			}
			return exitOK
		}
		var blocks [][]string
		var unknown []string
		for _, p := range args {
			if lines, ok := schema.ExplainName(ini, p); ok {
				blocks = append(blocks, lines)
			} else {
				unknown = append(unknown, p)
			}
		}
		if len(unknown) > 0 {
			return notFound(unknown)
		}
		for i, b := range blocks {
			if i > 0 {
				fmt.Fprintln(stdout)
			}
			for _, l := range b {
				fmt.Fprintln(stdout, l)
			}
		}
		return exitOK

	case "emit":
		if len(args) > 1 {
			return fail(exitArgs, "usage: emit [SECTION]")
		}
		out, ok, renamed, err := view.JSON(firstOr(args, ""))
		if !ok {
			return notFound(args)
		}
		if err != nil { // nothing on stdout: half an answer would look whole
			return fail(exitSyntax, "emit: %v", err)
		}
		warnScope(firstOr(args, ""))
		fmt.Fprint(stdout, out)
		for _, k := range renamed {
			fmt.Fprintln(stderr, "ezconfig: "+renameNote(k))
		}
		if len(renamed) > 0 {
			return exitRenamed
		}
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
			if r := schema.ruleFor(ini.splitPath(p)); r != nil {
				std, err := r.check(v)
				if err != nil {
					return fail(exitInvalid, "%s: %v", p, err)
				}
				v = std
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
			fmt.Fprintf(stderr, "ezconfig: created %s\n", path)
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
		return fail(exitArgs, "%s is only available in the interactive shell (run ezconfig without a command)", cmd)
	}
	return fail(exitArgs, "unknown command: %s (try ezconfig help)", cmd)
}
