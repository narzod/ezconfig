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

// askSet explains key path and asks for its value using read, which
// returns an error when the person cancels.
func (r *Repl) askSet(path string, read func(prompt string, choices []string) (string, error)) {
	sec, key := r.ini.splitPath(path)
	addr := address(sec, key, r.ini.delim)
	if strings.TrimSpace(key) == "" {
		errorf("usage: set KEY=VALUE, or set KEY to be asked")
		return
	}
	if !r.interactive {
		errorf("set %s: asking for a value needs a terminal; use set %s=VALUE", path, path)
		return
	}
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
			return
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			fmt.Println("unchanged")
			return
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
		if err := r.ini.Set(path, value); err != nil {
			errorf("%v", err)
			continue
		}
		// Recall the exact command with the up arrow, and keep it in history.
		resolved := "set " + addr + "=" + value
		if r.ln != nil {
			r.ln.AppendHistory(resolved)
		}
		r.record(resolved)
		return
	}
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
