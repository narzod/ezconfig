package main

// ask.go - asking for values: set KEY without "=", and ask.
//
//	config.ini=> set app.locale
//	Language and region for dates and numbers
//	format: language code, optionally with region (e.g. en-US)
//	now: en-US (the default)
//	app.locale> fr-CA
//
// Enter alone keeps the current value; Ctrl+C or Ctrl+D cancels. Tab
// completes the allowed values, and the up arrow recalls the current value
// and the default, for editing. To set an empty value, use set KEY=.
//
// ask asks about each key of the schema's sections in turn, then shows
// the changes and asks whether to save them. It runs in the shell or as
// a one-shot command; either way the shell's history is set aside while
// questions are asked.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterh/liner"
)

// askOutcome is how one question ended.
type askOutcome int

const (
	askKept      askOutcome = iota // Enter alone, the same value, or not asked
	askChanged                     // a new value was set
	askCancelled                   // Ctrl+C or Ctrl+D
)

// reader asks one question. Tab offers choices; the up arrow offers
// recall, most recent first. It returns an error when the person cancels.
type reader func(prompt string, choices, recall []string) (string, error)

// askOpts adjusts how a question behaves.
type askOpts struct {
	insist   bool // ask: Enter can't leave a required key unset or an invalid value in place
	remember bool // set KEY: put the resulting set command in the shell's history
}

// askSet explains key path and asks for its value using read.
func (r *Repl) askSet(path string, read reader) askOutcome {
	sec, key := r.ini.splitPath(path)
	if strings.TrimSpace(key) == "" {
		errorf("usage: set KEY=VALUE, or set KEY to be asked")
		return askKept
	}
	if !r.interactive {
		errorf("set %s: asking for a value needs a terminal; use set %s=VALUE", path, path)
		return askKept
	}
	return r.ask(sec, key, read, askOpts{remember: true})
}

// ask explains one key and asks for its value.
func (r *Repl) ask(sec, key string, read reader, opts askOpts) askOutcome {
	addr := address(sec, key, r.ini.delim)
	rl := r.schema.ruleFor(sec, key)
	if rl != nil {
		switch {
		case rl.prompt != "":
			fmt.Println(rl.prompt)
		case rl.doc != "":
			fmt.Println(rl.doc)
		}
		fmt.Println("format:", rl.Hint())
	}
	fmt.Println("now:", r.nowText(sec, key, rl))

	var choices []string
	if rl != nil {
		choices = rl.choices()
	}
	old, had := valueAt(r.ini, sec, key)
	for {
		answer, err := read(addr+"> ", choices, r.recallFor(sec, key))
		if err != nil {
			fmt.Println("cancelled")
			return askCancelled
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			if why := r.needsAnswer(sec, key, rl); opts.insist && why != "" {
				errorf("%s; type a value (Ctrl+C stops)", why)
				continue
			}
			fmt.Println("unchanged")
			return askKept
		}
		value := answer
		if rl != nil {
			v, err := rl.check(answer)
			if err != nil {
				errorf("%v", err)
				continue
			}
			value = v
		}
		if err := r.ini.Set(addr, value); err != nil {
			errorf("%v", err)
			continue
		}
		if opts.remember {
			// Recall the exact command with the up arrow, and keep it in history.
			resolved := "set " + addr + "=" + value
			if r.ln != nil {
				r.ln.AppendHistory(resolved)
			}
			r.record(resolved)
		}
		if had && old == value {
			return askKept
		}
		return askChanged
	}
}

// needsAnswer says why Enter alone isn't enough during ask: the key is
// required and has no value, or its value breaks the rule. "" if it is.
func (r *Repl) needsAnswer(sec, key string, rl *rule) string {
	if rl == nil {
		return ""
	}
	if raw, ok := valueAt(r.ini, sec, key); ok {
		if _, err := rl.check(raw); err != nil {
			return "the current value is invalid"
		}
		return ""
	}
	if _, ok := valueAt(r.view(), sec, key); !ok && rl.required {
		return "a value is required"
	}
	return ""
}

// recallFor is what the up arrow offers: the current value, then the
// default when it differs. Empty values are left out.
func (r *Repl) recallFor(sec, key string) []string {
	var out []string
	if v, ok := valueAt(r.ini, sec, key); ok && v != "" {
		out = append(out, v)
	}
	if d, ok := r.schema.defaultFor(sec, key); ok && d != "" && (len(out) == 0 || out[0] != d) {
		out = append(out, d)
	}
	return out
}

// nowText describes a key's current value and where it comes from.
func (r *Repl) nowText(sec, key string, rl *rule) string {
	raw, inConfig := valueAt(r.ini, sec, key)
	eff, inEffect := valueAt(r.view(), sec, key)
	show := func(v string) string {
		if v == "" {
			return "(empty)"
		}
		return v
	}
	switch {
	case inConfig && rl != nil:
		if _, err := rl.check(raw); err != nil {
			return show(raw) + " (invalid: " + err.Error() + ")"
		}
		return show(eff)
	case inConfig:
		return show(raw)
	case inEffect:
		return show(eff) + " (the default)"
	}
	return "not set"
}

// readAnswer reads one answer with the line editor. Tab completes the
// given choices; the up arrow recalls only the given values, so the
// shell's history is set aside for the question and put back after.
func (r *Repl) readAnswer(prompt string, choices, recall []string) (string, error) {
	var saved bytes.Buffer
	r.ln.WriteHistory(&saved)
	r.ln.ClearHistory()
	for i := len(recall) - 1; i >= 0; i-- { // most recent last
		r.ln.AppendHistory(recall[i])
	}
	r.ln.SetCtrlCAborts(true)
	r.ln.SetWordCompleter(func(line string, pos int) (string, []string, string) {
		var out []string
		for _, c := range choices {
			if strings.HasPrefix(strings.ToLower(c), strings.ToLower(line[:pos])) {
				out = append(out, c)
			}
		}
		return "", out, line[pos:]
	})
	defer func() {
		r.ln.SetCtrlCAborts(false)
		r.ln.SetWordCompleter(r.complete)
		r.ln.ClearHistory()
		r.ln.ReadHistory(&saved)
	}()
	return r.ln.Prompt(prompt)
}

// confirmAnswer asks a yes/no question at the end of ask, without the
// shell's history on the up arrow; anything but y or yes means no.
func (r *Repl) confirmAnswer(question string) bool {
	a, err := r.readAnswer(question+" [y/N] ", []string{"yes", "no"}, nil)
	a = strings.ToLower(strings.TrimSpace(a))
	return err == nil && (a == "y" || a == "yes")
}

// askResult is how a ask run ended.
type askResult int

const (
	askDone       askResult = iota // saved, or nothing to change
	askUnsaved                     // shell: answers kept in memory, not saved
	askStopped                     // Ctrl+C, or no at the save question (one-shot)
	askRefused                     // no schema, bad arguments, or no terminal
	askSaveFailed                  // the save itself failed
)

// ask asks about each key of the schema's sections, or of one section,
// then shows the changes and asks whether to save them. save writes the
// config and reports whether it worked; oneShot changes what declining
// means: in the shell the answers stay unsaved in memory, otherwise they
// are dropped.
func (r *Repl) askAll(args []string, read reader, confirm func(string) bool, save func() bool, oneShot bool) askResult {
	switch {
	case r.schema == nil:
		errorf("no schema (put %s next to the config, or use -s FILE)", schemaName)
		return askRefused
	case len(args) > 1:
		errorf("usage: ask [SECTION]")
		return askRefused
	case !r.interactive:
		errorf("ask needs a terminal; use set KEY=VALUE in scripts")
		return askRefused
	}
	var sections []string
	if len(args) == 1 {
		name := args[0]
		if name == r.ini.delim {
			name = ""
		}
		if len(r.schema.AskKeys(r.ini, name)) == 0 {
			errorf("nothing to ask about in %s", args[0])
			return askRefused
		}
		sections = []string{name}
	} else {
		sections = r.schema.AskSections(r.ini)
		if su := r.schema.ask; su != nil {
			if su.title != "" {
				fmt.Println(su.title)
			}
			if su.doc != "" {
				fmt.Println(su.doc)
			}
		}
	}
	fmt.Println("Enter keeps a value; Ctrl+C stops.")

	before := r.ini.Clone()
	stopped := false
walk:
	for _, sec := range sections {
		fmt.Println()
		fmt.Println(r.schema.SectionHeading(sec))
		for _, key := range r.schema.AskKeys(r.ini, sec) {
			fmt.Println()
			if r.ask(sec, key, read, askOpts{insist: true}) == askCancelled {
				stopped = true
				break walk
			}
		}
	}

	fmt.Println()
	changes := diffIni(before, r.ini)
	switch {
	case stopped && oneShot:
		fmt.Println("stopped; nothing saved")
		return askStopped
	case stopped:
		if len(changes) > 0 {
			fmt.Printf("stopped; %s unsaved (type changes to see them)\n", count(len(changes), "answer is", "answers are"))
		} else {
			fmt.Println("stopped")
		}
		return askUnsaved
	case len(changes) == 0:
		fmt.Println("done: no changes")
		return askDone
	}
	for _, c := range changes {
		fmt.Println(c.line(r.ini.delim))
	}
	question := fmt.Sprintf("save %s to %s?", plural(len(changes), "change"), filepath.Base(r.path))
	if total := r.unsaved(); total > len(changes) { // the shell had unsaved changes already
		question = fmt.Sprintf("save %s to %s (%d made earlier)?",
			plural(total, "change"), filepath.Base(r.path), total-len(changes))
	}
	if confirm(question) {
		if save() {
			return askDone
		}
		return askSaveFailed
	}
	if oneShot {
		fmt.Println("not saved")
		return askStopped
	}
	fmt.Println("not saved; the answers stay in memory (type save, or revert)")
	return askUnsaved
}

// oneShotAsk runs ezconfig ask [SECTION]: the questions with the line
// editor, but no shell and no command history.
func oneShotAsk(path, delim, schemaOpt string, args []string) int {
	fail := func(code int, format string, a ...any) int {
		fmt.Fprintf(os.Stderr, "ezconfig: "+format+"\n", a...)
		return code
	}
	if !isTerminal(os.Stdin) {
		return fail(exitArgs, "ask needs a terminal; use set KEY=VALUE in scripts")
	}
	_, statErr := os.Stat(path)
	missing := os.IsNotExist(statErr)
	ini, err := LoadIni(path, delim) // a missing file starts empty
	if err != nil {
		var se *SyntaxError
		if errors.As(err, &se) {
			return fail(exitSyntax, "%s: %v", path, err)
		}
		return fail(exitFile, "%s: %v", path, err)
	}
	schemaPath, err := findSchema(path, schemaOpt)
	if err != nil {
		return fail(exitFile, "%v", err)
	}
	if schemaPath == "" {
		return fail(exitArgs, "no schema (put %s next to the config, or use -s FILE)", schemaName)
	}
	schema, err := LoadSchema(schemaPath)
	if err != nil {
		var se *SchemaError
		if errors.As(err, &se) {
			return fail(exitSyntax, "%s: %v", schemaPath, err)
		}
		return fail(exitFile, "%s: %v", schemaPath, err)
	}

	r := &Repl{ini: ini, saved: ini.Clone(), schema: schema, path: path, interactive: true}
	r.ln = liner.NewLiner()
	r.ln.SetTabCompletionStyle(liner.TabPrints)
	defer r.ln.Close()

	save := func() bool {
		if err := r.ini.Save(path); err != nil {
			fail(exitFile, "save %s: %v", path, err)
			return false
		}
		if missing {
			fmt.Printf("created %s\n", path)
		} else {
			fmt.Printf("saved %s\n", path)
		}
		return true
	}
	switch r.askAll(args, r.readAnswer, r.confirmAnswer, save, true) {
	case askStopped:
		return exitCancelled
	case askRefused:
		return exitArgs
	case askSaveFailed:
		return exitFile
	}
	if p := schema.Problems(r.ini); len(p) > 0 {
		for _, l := range p {
			fmt.Fprintln(os.Stderr, "ezconfig:", l)
		}
		return exitInvalid
	}
	return exitOK
}
