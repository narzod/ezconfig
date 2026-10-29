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

// ask explains one key and asks for its value. The question (the rule's
// prompt, or its doc) comes with the format in parentheses, and the
// current value sits in the prompt:
//
//	What time should the screen turn on? (HH:MM, 24-hour, e.g. 07:30)
//	schedule.on [07:00]>
//
// A value that breaks the rule gets a line of its own instead.
func (r *Repl) ask(sec, key string, read reader, opts askOpts) askOutcome {
	addr := address(sec, key, r.ini.delim)
	rl := r.schema.ruleFor(sec, key)
	if line := questionLine(rl); line != "" {
		fmt.Println(line)
	}
	cur, shown, invalid := r.current(sec, key, rl)
	if invalid != nil {
		fmt.Println("current value is invalid:", invalid)
	}

	var choices []string
	if rl != nil {
		choices = rl.choices()
	}
	old, had := valueAt(r.ini, sec, key)
	for {
		prompt := addr + "> "
		if shown {
			prompt = addr + " [" + cur + "]> "
		}
		answer, err := read(prompt, choices, r.recallFor(sec, key, rl))
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

// questionLine is the line above the prompt: the rule's prompt (or doc)
// with its format in parentheses, or just one of them.
func questionLine(rl *rule) string {
	if rl == nil {
		return ""
	}
	text := rl.prompt
	if text == "" {
		text = rl.doc
	}
	hint := rl.Hint()
	if rl.typ == "string" && rl.pattern == nil && rl.format == "" {
		hint = "" // "text" says nothing a question doesn't
	}
	switch {
	case text == "":
		return hint
	case hint == "":
		return text
	default:
		return text + " (" + hint + ")"
	}
}

// current is the value shown in the prompt's brackets: what applications
// see now, set in the config or a default, in its standard spelling.
// shown is false when there is no value, or when the config's value is
// invalid; invalid then says why.
func (r *Repl) current(sec, key string, rl *rule) (value string, shown bool, invalid error) {
	if raw, ok := valueAt(r.ini, sec, key); ok && rl != nil {
		if _, err := rl.check(raw); err != nil {
			return "", false, err
		}
	}
	v, ok := valueAt(r.view(), sec, key)
	return v, ok, nil
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

// recallFor is what the up arrow offers: the current value (as the
// prompt shows it, or as the config spells it when it is invalid), then
// the default when it differs. Empty values are left out.
func (r *Repl) recallFor(sec, key string, rl *rule) []string {
	var out []string
	if v, shown, _ := r.current(sec, key, rl); shown && v != "" {
		out = append(out, v)
	} else if raw, ok := valueAt(r.ini, sec, key); ok && raw != "" {
		out = append(out, raw) // invalid: offered for fixing
	}
	if d, ok := r.schema.defaultFor(sec, key); ok && d != "" {
		if rl != nil {
			if std, err := rl.check(d); err == nil {
				d = std
			}
		}
		if len(out) == 0 || out[0] != d {
			out = append(out, d)
		}
	}
	return out
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

// askResult is how an ask run ended.
type askResult int

const (
	askDone       askResult = iota // saved, or nothing to change
	askUnsaved                     // shell: answers kept in memory, not saved
	askStopped                     // one-shot: Ctrl+C, or quit at the review
	askRefused                     // no schema, bad arguments, or no terminal
	askSaveFailed                  // the save itself failed
)

// askAll asks about each key of the schema's sections, or of one
// section, then shows the changes and asks whether to save, redo the
// questions (starting from these answers), or quit. That question has no default: Enter alone asks again,
// so pressing Enter once too often can't lose or save anything. save
// writes the config and reports whether it worked. In the shell, quitting
// leaves the answers unsaved in memory; one-shot, it drops them.
func (r *Repl) askAll(args []string, read reader, save func() bool, oneShot bool) askResult {
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
	quitWord := "quit without saving"
	if !oneShot {
		quitWord = "quit and keep the answers unsaved"
	}
	question := "save, redo, or " + quitWord + "? [s/r/q] "
	for {
		if !r.askWalk(sections, read) {
			fmt.Println()
			return r.askLeave(diffIni(before, r.ini), oneShot, "stopped")
		}

		// The review: set apart from the questions above it.
		fmt.Println()
		fmt.Println(strings.Repeat("-", 40))
		changes := diffIni(before, r.ini)
		if len(changes) == 0 {
			fmt.Println("no changes")
			return askDone
		}
		fmt.Printf("changes to %s:\n", filepath.Base(r.path))
		for _, c := range changes {
			fmt.Println(c.line(r.ini.delim))
		}
		if total := r.unsaved(); total > len(changes) { // the shell had unsaved changes already
			fmt.Printf("(saving also writes %s made earlier)\n", plural(total-len(changes), "change"))
		}
		fmt.Println()
		choice := ""
		for choice == "" {
			a, err := read(question, []string{"save", "redo", "quit"}, nil)
			if err != nil { // Ctrl+C here is a deliberate stop
				fmt.Println()
				return r.askLeave(changes, oneShot, "stopped")
			}
			switch strings.ToLower(strings.TrimSpace(a)) {
			case "s", "save":
				choice = "save"
			case "r", "redo":
				choice = "redo"
			case "q", "quit":
				choice = "quit"
			default:
				fmt.Println("type s to save, r to go through the questions again, or q to quit")
			}
		}
		switch choice {
		case "save":
			if save() {
				return askDone
			}
			return askSaveFailed
		case "quit":
			return r.askLeave(changes, oneShot, "quit")
		}
		// redo: the same questions again, starting from these answers.
	}
}

// askWalk asks every key of the given sections in turn. It returns false
// when the person stops with Ctrl+C.
func (r *Repl) askWalk(sections []string, read reader) bool {
	for _, sec := range sections {
		fmt.Println()
		fmt.Println(r.schema.SectionHeading(sec))
		for _, key := range r.schema.AskKeys(r.ini, sec) {
			fmt.Println()
			if r.ask(sec, key, read, askOpts{insist: true}) == askCancelled {
				return false
			}
		}
	}
	return true
}

// askLeave ends an ask run without saving: one-shot drops the answers,
// the shell keeps them unsaved in memory.
func (r *Repl) askLeave(changes []change, oneShot bool, how string) askResult {
	switch {
	case oneShot:
		fmt.Println(how + "; nothing saved")
		return askStopped
	case len(changes) > 0:
		fmt.Printf("%s; %s unsaved (type changes to see them, save to keep them)\n",
			how, count(len(changes), "answer is", "answers are"))
	default:
		fmt.Println(how)
	}
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
	switch r.askAll(args, r.readAnswer, save, true) {
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
