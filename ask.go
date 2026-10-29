package main

// ask.go - set KEY without "=": explain what the key needs, then ask for
// its value, asking again after an invalid answer.
//
//	config.ini=> set app.locale
//	Language and region for dates and numbers
//	format: language code, optionally with region (e.g. en-US)
//	now: en-US (the default)
//	app.locale> fr-CA
//
// Enter alone keeps the current value; Ctrl+C or Ctrl+D cancels. Tab
// completes the allowed values. To set an empty value, use set KEY=.

import (
	"fmt"
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

// reader asks one question; it returns an error when the person cancels.
type reader func(prompt string, choices []string) (string, error)

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
	return r.ask(sec, key, read)
}

// ask explains one key and asks for its value.
func (r *Repl) ask(sec, key string, read reader) askOutcome {
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
	for {
		answer, err := read(addr+"> ", choices)
		if err != nil {
			fmt.Println("cancelled")
			return askCancelled
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
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
		old, had := valueAt(r.ini, sec, key)
		if err := r.ini.Set(addr, value); err != nil {
			errorf("%v", err)
			continue
		}
		// Recall the exact command with the up arrow, and keep it in history.
		resolved := "set " + addr + "=" + value
		if r.ln != nil {
			r.ln.AppendHistory(resolved)
		}
		r.record(resolved)
		if had && old == value {
			return askKept
		}
		return askChanged
	}
}

// setup walks through the schema's sections, or just one, asking for each
// key in turn. Answers stay in memory like any other change.
func (r *Repl) setup(args []string, read reader) {
	switch {
	case r.schema == nil:
		errorf("no schema (put %s next to the config, or start with -s FILE)", schemaName)
		return
	case len(args) > 1:
		errorf("usage: setup [SECTION]")
		return
	case !r.interactive:
		errorf("setup needs a terminal")
		return
	}
	var sections []string
	if len(args) == 1 {
		name := args[0]
		if name == r.ini.delim {
			name = ""
		}
		if len(r.schema.SetupKeys(r.ini, name)) == 0 {
			errorf("nothing to set up in %s", args[0])
			return
		}
		sections = []string{name}
	} else {
		sections = r.schema.SetupSections(r.ini)
		if su := r.schema.setup; su != nil {
			if su.title != "" {
				fmt.Println(su.title)
			}
			if su.doc != "" {
				fmt.Println(su.doc)
			}
		}
	}
	fmt.Println("Enter keeps a value; Ctrl+C stops.")

	changed := 0
	for _, sec := range sections {
		fmt.Println()
		fmt.Println(r.schema.SectionHeading(sec))
		for _, key := range r.schema.SetupKeys(r.ini, sec) {
			fmt.Println()
			switch r.ask(sec, key, read) {
			case askChanged:
				changed++
			case askCancelled:
				fmt.Printf("setup stopped: %s\n", r.setupSummary(changed))
				return
			}
		}
	}
	fmt.Println()
	fmt.Printf("setup finished: %s\n", r.setupSummary(changed))
}

func (r *Repl) setupSummary(changed int) string {
	if changed == 0 {
		return "no changes"
	}
	return plural(changed, "change") + " (type save to keep them)"
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

// readAnswer reads one answer with the line editor: Tab completes the
// given choices, and Ctrl+C or Ctrl+D cancels.
func (r *Repl) readAnswer(prompt string, choices []string) (string, error) {
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
	}()
	answer, err := r.ln.Prompt(prompt)
	if err == liner.ErrPromptAborted {
		return "", err
	}
	return answer, err
}
