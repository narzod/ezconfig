package main

// ezconfig - an experimental cross-platform REPL for editing an INI file
// in memory, with tab completion of commands, sections and keys, plus a
// one-shot mode for scripts and other programs (see oneshot.go).
//
// usage: ezconfig [-c FILE] [-s FILE] [-d CHAR] [COMMAND [ARGS...]]

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterh/liner"
)

// version is set at build time: go build -ldflags "-X main.version=1.0.0"
var version = "dev"

const helpText = `commands:
set KEY[=VALUE]    set a value (asks for it when "=" is left out)
get KEY...         print values
unset KEY...       remove keys
wipe SECTION...    remove sections (asks first)
copy SRC DEST      copy a section to a new one
move SRC DEST      move or rename a section
keys [SECTION]     list key names for everything, or for SECTION
sections           list section names
list [SECTION]     print KEY=VALUE lines for everything, or for SECTION
dump [SECTION]     print everything, or SECTION, as JSON
check              report values that break the schema
describe [NAME...] explain keys, or list a section's keys
status             show file details and session settings
changes [SECTION]  show unsaved changes
path               show the config's full path
save [FILE]        write changes (to FILE if given)
revert             discard unsaved changes
history [on|off]   show or switch command history
delim [CHAR]       show or change the delimiter
version            show the program version
help               show this text
quit               leave

KEY is "name" or "section.name"; "." alone means the root keys.
Changes stay in memory until saved.
An asterisk in the prompt indicates unsaved changes.
Tab completes commands and names.
Most reading commands, set and unset also work as ezconfig COMMAND.`

// diskState is what we know about the file on disk at load or save.
type diskState struct {
	exists  bool
	size    int64
	modTime time.Time
}

func statFile(path string) diskState {
	fi, err := os.Stat(path)
	if err != nil {
		return diskState{}
	}
	return diskState{true, fi.Size(), fi.ModTime()}
}

type Repl struct {
	ini    *Ini
	path   string // absolute path of the loaded file
	disk   diskState
	saved  *Ini    // the config as loaded or last saved; unsaved changes are the difference
	schema *Schema // nil without a schema
	ln     *liner.State

	interactive bool     // stdin is a terminal; only then is history recorded
	session     []string // commands typed this session (for `history on`)
	histStopped bool     // another session changed the folder's delimiter
}

// cmdLine is the parsed command line.
type cmdLine struct {
	path     string   // -c FILE (default config.ini)
	delim    string   // -d CHAR (default ".")
	delimSet bool     // -d was given
	schema   string   // -s FILE, "none", or "" for schema.ini next to the config
	cmd      string   // one-shot command, "" for the shell
	args     []string // the command's arguments
	early    string   // "help" or "version" from --help / --version
}

// parseArgs reads options up to the first word that isn't one; that word
// is the command and everything after it belongs to the command.
func parseArgs(argv []string) (cmdLine, error) {
	cl := cmdLine{path: "config.ini", delim: "."}
	value := func(i *int, name string) (string, error) {
		if *i+1 >= len(argv) {
			return "", fmt.Errorf("%s needs a value", name)
		}
		*i++
		return argv[*i], nil
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		var err error
		switch {
		case a == "--":
			if i+1 < len(argv) {
				cl.cmd, cl.args = argv[i+1], argv[i+2:]
			}
			return cl, nil
		case a == "-c" || a == "--config":
			cl.path, err = value(&i, a)
		case strings.HasPrefix(a, "--config="):
			cl.path = strings.TrimPrefix(a, "--config=")
		case a == "-s" || a == "--schema":
			cl.schema, err = value(&i, a)
		case strings.HasPrefix(a, "--schema="):
			cl.schema = strings.TrimPrefix(a, "--schema=")
		case a == "-d" || a == "--delim":
			cl.delim, err = value(&i, a)
			cl.delimSet = true
		case strings.HasPrefix(a, "--delim="):
			cl.delim, cl.delimSet = strings.TrimPrefix(a, "--delim="), true
		case a == "-h" || a == "--help":
			cl.early = "help"
		case a == "--version":
			cl.early = "version"
		case strings.HasPrefix(a, "-") && a != "-":
			return cl, fmt.Errorf("unknown option: %s", a)
		default:
			cl.cmd, cl.args = a, argv[i+1:]
			return cl, nil
		}
		if err != nil {
			return cl, err
		}
	}
	return cl, nil
}

func main() {
	cl, err := parseArgs(os.Args[1:])
	if err != nil {
		die(exitArgs, "%v (try ezconfig help)", err)
	}
	if cl.early != "" {
		os.Exit(oneShot("", ".", "", cl.early, nil, os.Stdout, os.Stderr))
	}
	if err := checkDelim(cl.delim); err != nil {
		die(exitArgs, "-d: %v", err)
	}
	if cl.cmd != "" {
		if !oneShotCommands[cl.cmd] && !shellOnly[cl.cmd] &&
			(strings.HasSuffix(strings.ToLower(cl.cmd), ".ini") || fileExists(cl.cmd)) {
			die(exitArgs, "unknown command: %s (to open a file, use -c %s)", cl.cmd, cl.cmd)
		}
		path := cl.path // messages name the full path, so it's clear which file
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		os.Exit(oneShot(path, cl.delim, cl.schema, cl.cmd, cl.args, os.Stdout, os.Stderr))
	}
	if cl.delimSet {
		die(exitArgs, "-d applies to one-shot commands; in the shell, use the delim command")
	}

	path := cl.path
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	// The delimiter starts as "."; loadHistory adopts the folder's own.
	ini, err := LoadIni(path, ".")
	if err != nil {
		var se *SyntaxError
		if errors.As(err, &se) {
			die(exitSyntax, "%s: %v", path, err)
		}
		die(exitFile, "%s: %v", path, err)
	}
	schemaPath, err := findSchema(path, cl.schema)
	if err != nil {
		die(exitFile, "%v", err)
	}
	var schema *Schema
	if schemaPath != "" {
		if schema, err = LoadSchema(schemaPath); err != nil {
			var se *SchemaError
			if errors.As(err, &se) {
				die(exitSyntax, "%s: %v", schemaPath, err)
			}
			die(exitFile, "%s: %v", schemaPath, err)
		}
	}
	r := &Repl{ini: ini, saved: ini.Clone(), schema: schema, path: path, disk: statFile(path),
		interactive: isTerminal(os.Stdin)}

	if r.disk.exists {
		total, _ := ini.KeyCount()
		fmt.Printf("loaded %s (%s, %s)\n", path,
			plural(len(ini.Sections()), "section"), plural(total, "key"))
	} else {
		fmt.Printf("new file: %s (created on save)\n", path)
	}
	if schema != nil {
		fmt.Printf("schema: %s (%s)\n", schemaLabel(schemaPath, path), plural(schema.Rules(), "rule"))
		if p := schema.Problems(ini); len(p) > 0 {
			fmt.Printf("note: %s; type check for details\n", count(len(p), "value breaks the schema", "values break the schema"))
		}
	}

	r.ln = liner.NewLiner()
	defer r.ln.Close()
	r.ln.SetTabCompletionStyle(liner.TabPrints)
	r.ln.SetWordCompleter(r.complete)
	r.migrateOldDir()
	r.offerLegacy()
	r.loadHistory()
	r.warnDelim()

	for {
		line, err := r.ln.Prompt(r.prompt())
		if errors.Is(err, io.EOF) {
			if n := r.unsaved(); n > 0 {
				errorf("%s discarded", plural(n, "unsaved change"))
			}
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "ezconfig:", err)
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r.ln.AppendHistory(line)
		r.record(line)
		if r.exec(line) {
			return
		}
	}
}

// exec runs one command line. It returns true when the REPL should exit.
func (r *Repl) exec(line string) bool {
	cmd, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)
	args := strings.Fields(rest)

	switch cmd {
	case "set":
		path, value, ok := strings.Cut(rest, "=")
		path = strings.TrimSpace(path)
		if !ok && len(args) == 1 { // set KEY: ask for the value
			r.askSet(path, r.readAnswer)
			break
		}
		if !ok || path == "" {
			errorf("usage: set KEY=VALUE, or set KEY to be asked")
			break
		}
		value = strings.TrimSpace(value)
		if rl := r.schema.ruleFor(r.ini.splitPath(path)); rl != nil {
			v, err := rl.check(value)
			if err != nil {
				errorf("%s: %v", path, err)
				break
			}
			value = v
		}
		if err := r.ini.Set(path, value); err != nil {
			errorf("%v", err)
		}

	case "get":
		if len(args) == 0 {
			errorf("usage: get KEY...")
			break
		}
		view, notes := r.schema.EffectiveNotes(r.ini)
		for _, p := range args {
			sec, key := r.ini.splitPath(p)
			for _, n := range notesFor(notes, [][2]string{{sec, key}}) {
				errorf("%s", n)
			}
			if v, ok := view.Get(p); ok {
				fmt.Println(v)
			} else if _, inConfig := r.ini.Get(p); !inConfig {
				errorf("not found: %s", p)
			}
		}

	case "unset":
		if len(args) == 0 {
			errorf("usage: unset KEY...")
			break
		}
		for _, p := range args {
			_, hasDef := r.schema.defaultFor(r.ini.splitPath(p))
			removed := r.ini.Unset(p)
			def, _ := r.view().Get(p) // the default, in its standard spelling
			switch {
			case removed && hasDef:
				fmt.Printf("%s now uses the default (%s)\n", p, def)
			case hasDef:
				errorf("%s isn't set in the config; it already uses the default (%s)", p, def)
			case !removed:
				errorf("not found: %s", p)
			}
		}

	case "wipe":
		if len(args) == 0 {
			errorf("usage: wipe SECTION...")
			break
		}
		for _, name := range args {
			keys, ok := r.ini.Keys(name)
			if !ok {
				errorf("no such section: %s", name)
				continue
			}
			if name == r.ini.delim {
				if len(keys) == 0 {
					fmt.Println("the root has no keys")
					continue
				}
				if r.confirm(fmt.Sprintf("wipe root keys (%s)?", plural(len(keys), "key"))) {
					r.ini.ClearRoot()
				}
				continue
			}
			if !r.confirm(fmt.Sprintf("wipe section %s (%s)?", name, plural(len(keys), "key"))) {
				continue
			}
			r.ini.UnsetSection(name)
		}

	case "copy":
		if len(args) != 2 {
			errorf("usage: copy SRC DEST")
			break
		}
		if err := r.ini.CopySection(args[0], args[1]); err != nil {
			errorf("copy: %v", err)
			break
		}
		keys, _ := r.ini.Keys(args[1])
		fmt.Printf("copied %s to %s (%s)\n", r.sectionLabel(args[0]), r.sectionLabel(args[1]),
			plural(len(keys), "key"))

	case "move":
		if len(args) != 2 {
			errorf("usage: move SRC DEST")
			break
		}
		keys, _ := r.ini.Keys(args[0])
		if err := r.ini.MoveSection(args[0], args[1]); err != nil {
			errorf("move: %v", err)
			break
		}
		fmt.Printf("moved %s to %s (%s)\n", r.sectionLabel(args[0]), r.sectionLabel(args[1]),
			plural(len(keys), "key"))

	case "changes":
		r.changesCmd(args)

	case "path":
		if len(args) > 0 {
			errorf("usage: path")
			break
		}
		fmt.Println(r.path)

	case "history":
		r.historyCmd(args)

	case "delim":
		r.delimCmd(args)

	case "keys":
		if len(args) > 1 {
			errorf("usage: keys [SECTION]")
			break
		}
		keys, ok := r.view().KeyPaths(firstOr(args, ""))
		if !ok {
			errorf("no such section: %s", args[0])
			break
		}
		r.warnScope(firstOr(args, ""))
		printLines(keys)

	case "sections":
		printLines(r.view().Sections())

	case "list":
		if len(args) > 1 {
			errorf("usage: list [SECTION]")
			break
		}
		lines, ok := r.view().List(firstOr(args, ""))
		if !ok {
			errorf("no such section: %s", args[0])
			break
		}
		r.warnScope(firstOr(args, ""))
		printLines(lines)

	case "dump":
		if len(args) > 1 {
			errorf("usage: dump [SECTION]")
			break
		}
		out, ok, err := r.view().JSON(firstOr(args, ""))
		if !ok {
			errorf("no such section: %s", args[0])
			break
		}
		if err != nil {
			errorf("dump: %v", err)
			break
		}
		r.warnScope(firstOr(args, ""))
		fmt.Print(out)

	case "check":
		if len(args) > 0 {
			errorf("usage: check")
			break
		}
		if r.schema == nil {
			errorf("no schema (put %s next to the config, or start with -s FILE)", schemaName)
			break
		}
		problems := r.schema.Problems(r.ini)
		printLines(problems)
		if len(problems) == 0 {
			fmt.Println("no problems found")
		}

	case "describe":
		if r.schema == nil {
			errorf("no schema (put %s next to the config, or start with -s FILE)", schemaName)
			break
		}
		if len(args) == 0 {
			printLines(r.schema.DescribeAll(r.ini))
			break
		}
		for i, p := range args {
			lines, ok := r.schema.DescribeName(r.ini, p)
			if !ok {
				errorf("not found: %s", p)
				continue
			}
			if i > 0 {
				fmt.Println()
			}
			printLines(lines)
		}

	case "status":
		r.status()

	case "save":
		if len(args) > 1 {
			errorf("usage: save [FILE]")
			break
		}
		r.save(firstOr(args, ""))

	case "revert":
		if n := r.unsaved(); n > 0 && !r.confirm("discard "+plural(n, "unsaved change")+" and reload?") {
			break
		}
		ini, err := LoadIni(r.path, r.ini.delim)
		if err != nil {
			errorf("revert: %v", err)
			break
		}
		r.ini, r.saved, r.disk = ini, ini.Clone(), statFile(r.path)
		fmt.Printf("reloaded %s\n", r.path)
		r.warnDelim()

	case "version":
		fmt.Println("ezconfig", version)

	case "help", "?":
		fmt.Println(helpText)
		if r.ini.delim != "." {
			fmt.Printf("\nThe delimiter in this session is %q: use it wherever \".\" appears above.\n", r.ini.delim)
		}

	case "quit", "exit":
		if n := r.unsaved(); n > 0 && !r.confirm("discard "+plural(n, "unsaved change")+" and quit?") {
			break
		}
		return true

	default:
		errorf("unknown command: %s (try help)", cmd)
	}
	return false
}

// checkDelim rejects delimiters that can't be told apart from INI syntax
// or spacing. A delimiter is exactly one character.
func checkDelim(sep string) error {
	if len([]rune(sep)) != 1 {
		return fmt.Errorf("the delimiter must be a single character, not %q", sep)
	}
	if strings.ContainsAny(sep, "=[];#\"' \t") {
		return fmt.Errorf("%q can't be used as a delimiter; try / or : or |", sep)
	}
	return nil
}

// warnDelim notes names containing the delimiter, which can't be addressed,
// and suggests an alternative the config doesn't use.
func (r *Repl) warnDelim() {
	n := r.ini.NamesContaining(r.ini.delim)
	if n == 0 {
		return
	}
	msg := fmt.Sprintf("note: %s %q and can't be addressed",
		count(n, "name contains", "names contain"), r.ini.delim)
	for _, alt := range []string{".", "/", ":", "|", "!"} {
		if alt != r.ini.delim && r.ini.NamesContaining(alt) == 0 {
			msg += fmt.Sprintf("; type delim %s to switch (or another character)", alt)
			break
		}
	}
	fmt.Println(msg)
}

// sectionLabel names a section in messages, spelling out the root.
func (r *Repl) sectionLabel(name string) string {
	if name == r.ini.delim {
		return "the root"
	}
	return name
}

// save writes to target, or to the loaded file when target is "".
func (r *Repl) save(target string) {
	toLoaded := target == ""
	if toLoaded {
		target = r.path
	} else if abs, err := filepath.Abs(target); err == nil && abs == r.path {
		toLoaded = true
	}
	if toLoaded && r.changedOnDisk() &&
		!r.confirm(filepath.Base(r.path)+" changed on disk since load; overwrite?") {
		return
	}
	if err := r.ini.Save(target); err != nil {
		errorf("save: %v", err)
		return
	}
	if !toLoaded {
		fmt.Printf("saved a copy to %s\n", target)
		return
	}
	fmt.Printf("saved %s to %s\n", plural(r.unsaved(), "change"), filepath.Base(r.path))
	r.saved, r.disk = r.ini.Clone(), statFile(r.path)
}

func (r *Repl) changedOnDisk() bool {
	now := statFile(r.path)
	return now.exists != r.disk.exists || now.size != r.disk.size ||
		!now.modTime.Equal(r.disk.modTime)
}

func (r *Repl) status() {
	fmt.Println("file:", r.path)
	if r.disk.exists {
		fmt.Printf("on disk: yes, %s, modified %s\n",
			humanSize(r.disk.size), r.disk.modTime.Format("2006-01-02 15:04"))
	} else {
		fmt.Println("on disk: no, will be created on save")
	}
	fmt.Println("changed on disk since load:", yesNo(r.changedOnDisk()))
	if n := r.unsaved(); n > 0 {
		fmt.Printf("unsaved changes: yes (%d)\n", n)
	} else {
		fmt.Println("unsaved changes: no")
	}
	fmt.Println("writable:", yesNo(writable(r.path)))
	total, root := r.ini.KeyCount()
	fmt.Println("sections:", len(r.ini.Sections()))
	if r.schema != nil {
		inEffect, _ := r.view().KeyCount()
		fmt.Printf("keys: %d set in the config (%d root), %d in effect\n", total, root, inEffect)
	} else {
		fmt.Printf("keys: %d (%d root)\n", total, root)
	}
	if r.schema != nil {
		fmt.Printf("schema: %s (%s)\n", r.schema.path, plural(r.schema.Rules(), "rule"))
	} else {
		fmt.Println("schema: none")
	}
	switch {
	case !r.historyOn():
		fmt.Println("delimiter:", r.ini.delim, "(this session only)")
		fmt.Println("history: off")
	case r.histStopped:
		fmt.Println("delimiter:", r.ini.delim)
		fmt.Printf("history: on (%s), not recording: another session changed the delimiter\n", r.histLabel())
	default:
		fmt.Println("delimiter:", r.ini.delim, "(remembered for this config)")
		fmt.Printf("history: on (%s)\n", r.histLabel())
	}
}

// writable reports whether path could be saved: an existing file must open
// for writing (without truncating); a new file needs a writable directory.
func writable(path string) bool {
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		f.Close()
		return true
	} else if !os.IsNotExist(err) {
		return false
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ezconfig-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}

// confirm asks a yes/no question; anything but y or yes means no.
func (r *Repl) confirm(question string) bool {
	answer, err := r.ln.Prompt(question + " [y/N] ")
	if err != nil {
		return false
	}
	a := strings.ToLower(strings.TrimSpace(answer))
	return a == "y" || a == "yes"
}

func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d bytes", n)
	}
	return fmt.Sprintf("%.1f KB", float64(n)/1024)
}

func plural(n int, word string) string { return count(n, word, word+"s") }

func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func firstOr(args []string, def string) string {
	if len(args) > 0 {
		return args[0]
	}
	return def
}

func printLines(lines []string) {
	for _, l := range lines {
		fmt.Println(l)
	}
}

func errorf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// die reports a startup error and exits with code.
func die(code int, format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ezconfig: "+format+"\n", a...)
	os.Exit(code)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// view is what applications would see if the session were saved: the
// config with the schema's defaults and standard spellings applied.
func (r *Repl) view() *Ini {
	return r.schema.Effective(r.ini)
}

// warnScope reports invalid values that reading commands replaced by a
// default or left out, within a SECTION argument ("" for all).
func (r *Repl) warnScope(arg string) {
	_, notes := r.schema.EffectiveNotes(r.ini)
	var texts []string
	switch arg {
	case "":
		texts = notesIn(notes, "", true)
	case r.ini.delim:
		texts = notesIn(notes, "", false)
	default:
		texts = notesIn(notes, arg, false)
	}
	for _, t := range texts {
		errorf("%s", t)
	}
}
