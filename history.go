package main

// history.go - opt-in command history, one file per config.
//
// Histories live in a hidden folder next to the configs, one file each:
//
//	config.ini
//	system.ini
//	.ezconfig/config.ini.hst
//	.ezconfig/system.ini.hst
//
// A config's history file is its switch: `history on` creates it (and the
// folder, hidden on Windows), `history off` deletes it after asking, and
// removes the folder once it is empty.
//
// The first line of each file records that config's delimiter:
//
//	#delim /
//
// and the rest are commands, one per line, appended as they are run.
// Because the delimiter is stored with the history, recalled commands
// always fit the session. Changing the delimiter (the `delim` command)
// erases the history, after asking. A file without a header is treated
// as using "."; a "#sep" header from 0.5 is read as "#delim".
//
// Earlier versions were called inirepl. 0.8 kept the same per-config files
// in a ".inirepl" folder; they are moved into ".ezconfig" automatically.
// Versions 0.3 to 0.7 kept one shared .inirepl.hst per folder. It can't
// be split by config, so on startup the shell offers to adopt it as this
// config's history, or to delete it.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	histDir    = ".ezconfig"    // hidden folder next to the configs
	histExt    = ".hst"         // histDir/<config file name>.hst
	oldHistDir = ".inirepl"     // the same folder in 0.8, under the old name
	legacyName = ".inirepl.hst" // the shared file used by 0.3 to 0.7
	histLimit  = 500            // commands kept; the file is trimmed at startup
	histHeader = "#delim "
	oldHeader  = "#sep " // written by 0.5
)

// headerDelim returns the delimiter named by a header line, if it is one.
func headerDelim(line string) (string, bool) {
	for _, h := range []string{histHeader, oldHeader} {
		if strings.HasPrefix(line, h) {
			return strings.TrimPrefix(line, h), true
		}
	}
	return "", false
}

// histLabel is the history file's path relative to the config's folder,
// e.g. ".ezconfig/config.ini.hst", for messages.
func (r *Repl) histLabel() string {
	name := filepath.Base(r.path)
	if runtime.GOOS == "windows" { // Config.ini and config.ini are one file there
		name = strings.ToLower(name)
	}
	return filepath.Join(histDir, name+histExt)
}

func (r *Repl) histFile() string {
	return filepath.Join(filepath.Dir(r.path), r.histLabel())
}

func (r *Repl) legacyFile() string {
	return filepath.Join(filepath.Dir(r.path), legacyName)
}

// ensureHistDir creates the hidden history folder if needed.
func (r *Repl) ensureHistDir() error {
	dir := filepath.Join(filepath.Dir(r.path), histDir)
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	hideDir(dir) // best effort: the dot already hides it outside Windows
	return nil
}

// removeHistDirIfEmpty tidies up after the last history is deleted.
func (r *Repl) removeHistDirIfEmpty() {
	dir := filepath.Join(filepath.Dir(r.path), histDir)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		os.Remove(dir)
	}
}

func (r *Repl) historyOn() bool {
	_, err := os.Stat(r.histFile())
	return err == nil
}

// readHistory returns the file's delimiter and its commands. A missing or
// unusable header means ".". Blank lines and other "#" lines are skipped.
func (r *Repl) readHistory() (sep string, lines []string, err error) {
	data, err := os.ReadFile(r.histFile())
	if err != nil {
		return "", nil, err
	}
	sep = "."
	for i, l := range strings.Split(string(data), "\n") {
		l = strings.TrimRight(l, "\r")
		if s, ok := headerDelim(l); i == 0 && ok {
			if checkDelim(s) == nil {
				sep = s
			} else {
				errorf("history: unusable delimiter %q in %s; using \".\"", s, r.histLabel())
			}
			continue
		}
		if l != "" && !strings.HasPrefix(l, "#") {
			lines = append(lines, l)
		}
	}
	return sep, lines, nil
}

// fileDelim reads only the header of the history file ("" if unreadable).
func (r *Repl) fileDelim() string {
	f, err := os.Open(r.histFile())
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if sc.Scan() {
		if s, ok := headerDelim(strings.TrimRight(sc.Text(), "\r")); ok {
			return s
		}
	}
	return "."
}

// writeHistory replaces the file's contents with a header and lines.
// With create set it refuses to overwrite an existing file.
func (r *Repl) writeHistory(sep string, lines []string, create bool) error {
	flags := os.O_WRONLY | os.O_TRUNC
	if create {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(r.histFile(), flags, 0o644)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(histHeader + sep + "\n")
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	_, err = f.WriteString(b.String())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// migrateOldDir runs at startup: a history this config had in 0.8's
// ".inirepl" folder is moved into ".ezconfig", and the old folder is
// removed once empty. Same format, so nothing needs asking.
func (r *Repl) migrateOldDir() {
	if !r.interactive || r.historyOn() {
		return
	}
	folder := filepath.Dir(r.path)
	old := filepath.Join(folder, oldHistDir, filepath.Base(r.histLabel()))
	if _, err := os.Stat(old); err != nil {
		return
	}
	if err := r.ensureHistDir(); err != nil {
		errorf("history: %v", err)
		return
	}
	if err := os.Rename(old, r.histFile()); err != nil {
		errorf("history: %v", err)
		return
	}
	fmt.Printf("moved history from %s to %s\n",
		filepath.Join(oldHistDir, filepath.Base(old)), r.histLabel())
	if entries, err := os.ReadDir(filepath.Join(folder, oldHistDir)); err == nil && len(entries) == 0 {
		os.Remove(filepath.Join(folder, oldHistDir))
	}
}

// offerLegacy runs at startup when the shared .inirepl.hst from 0.3 to
// 0.7 is present and this config has no history of its own: it offers to
// adopt the old file as this config's history, or else to delete it.
// Saying no to both leaves it, and the offer comes again next time.
func (r *Repl) offerLegacy() {
	if !r.interactive || r.historyOn() {
		return
	}
	old := r.legacyFile()
	data, err := os.ReadFile(old)
	if err != nil {
		return
	}
	n := 0
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			n++
		}
	}
	fmt.Printf("found %s from an older version (%s, shared by this folder)\n",
		legacyName, count(n, "entry", "entries"))
	if r.confirm("adopt it as the history for " + filepath.Base(r.path) + "?") {
		if err := r.ensureHistDir(); err != nil {
			errorf("history: %v", err)
			return
		}
		if err := os.Rename(old, r.histFile()); err != nil {
			errorf("history: %v", err)
			return
		}
		fmt.Printf("moved to %s\n", r.histLabel())
		return
	}
	if r.confirm("delete " + legacyName + "?") {
		if err := os.Remove(old); err != nil {
			errorf("history: %v", err)
			return
		}
		fmt.Println("deleted")
	}
}

// loadHistory runs at startup: if history is on, it adopts the config's
// delimiter, trims the file to histLimit (adding a header to older files),
// feeds the commands to the line editor and announces it.
func (r *Repl) loadHistory() {
	data, err := os.ReadFile(r.histFile())
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		errorf("history: %v", err)
		return
	}
	hasHeader := strings.HasPrefix(string(data), histHeader) // current form only
	sep, lines, _ := r.readHistory()
	if len(lines) > histLimit || !hasHeader {
		if len(lines) > histLimit {
			lines = lines[len(lines)-histLimit:]
		}
		if err := r.writeHistory(sep, lines, false); err != nil {
			errorf("history: %v", err)
		}
	}
	r.ini.delim = sep
	for _, l := range lines {
		r.ln.AppendHistory(l)
	}
	fmt.Printf("history: on (%s, %s, delimiter %q)\n",
		r.histLabel(), count(len(lines), "entry", "entries"), sep)
}

// record keeps a command for this session and, when history is on,
// appends it to the file. Piped (non-terminal) input is never recorded.
// If another session has changed this config's delimiter, this session
// stops recording rather than mix delimiters in one file.
func (r *Repl) record(line string) {
	if !r.interactive {
		return
	}
	r.session = append(r.session, line)
	if r.histStopped || !r.historyOn() {
		return
	}
	if s := r.fileDelim(); s != r.ini.delim {
		r.histStopped = true
		errorf("history: delimiter changed to %q by another session; not recording", s)
		return
	}
	f, err := os.OpenFile(r.histFile(), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		errorf("history: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		errorf("history: %v", err)
	}
}

// historyCmd implements: history, history on, history off.
func (r *Repl) historyCmd(args []string) {
	if len(args) > 1 {
		errorf("usage: history [on|off]")
		return
	}
	switch firstOr(args, "") {
	case "":
		if !r.historyOn() {
			fmt.Println("history: off")
			return
		}
		_, lines, _ := r.readHistory()
		fmt.Printf("history: on (%s, %s, delimiter %q)\n",
			r.histFile(), count(len(lines), "entry", "entries"), r.fileDelim())

	case "on":
		if r.historyOn() {
			fmt.Println("history is already on")
			return
		}
		// Start the file with this session's delimiter and commands so far.
		if err := r.ensureHistDir(); err != nil {
			errorf("history: cannot create %s: %v", histDir, err)
			return
		}
		if err := r.writeHistory(r.ini.delim, r.session, true); err != nil {
			errorf("history: cannot create %s: %v", r.histFile(), err)
			r.removeHistDirIfEmpty()
			return
		}
		r.histStopped = false
		fmt.Printf("history: on (%s, delimiter %q)\n", r.histFile(), r.ini.delim)

	case "off":
		if !r.historyOn() {
			fmt.Println("history is already off")
			return
		}
		_, lines, _ := r.readHistory()
		q := fmt.Sprintf("delete the history for %s (%s)?",
			filepath.Base(r.path), count(len(lines), "entry", "entries"))
		if !r.confirm(q) {
			fmt.Println("history stays on")
			return
		}
		if err := os.Remove(r.histFile()); err != nil {
			errorf("history: %v", err)
			return
		}
		r.removeHistDirIfEmpty()
		fmt.Println("history: off")
		if r.ini.delim != "." {
			fmt.Println(noteText(fmt.Sprintf("note: the delimiter %q is no longer remembered for this config", r.ini.delim)))
		}

	default:
		errorf("usage: history [on|off]")
	}
}

// delimCmd implements: delim (show) and delim CHAR (change). With history on,
// changing the delimiter erases this config's history, after asking.
func (r *Repl) delimCmd(args []string) {
	if len(args) > 1 {
		errorf("usage: delim [CHAR]")
		return
	}
	if len(args) == 0 {
		fmt.Println("delimiter:", r.ini.delim)
		return
	}
	sep := args[0]
	if err := checkDelim(sep); err != nil {
		errorf("delim: %v", err)
		return
	}
	if sep == r.ini.delim {
		fmt.Printf("the delimiter is already %q\n", sep)
		return
	}
	on := r.historyOn()
	if on {
		_, lines, _ := r.readHistory()
		q := fmt.Sprintf("change delimiter to %q? this erases history (%s)",
			sep, count(len(lines), "entry", "entries"))
		if len(lines) > 0 && !r.confirm(q) {
			return
		}
		if err := r.writeHistory(sep, nil, false); err != nil {
			errorf("history: %v", err)
			return
		}
		r.histStopped = false
	}
	r.ini.delim = sep
	r.ln.ClearHistory() // recalled lines were typed for the old delimiter
	r.session = nil
	if on {
		fmt.Printf("delimiter: %s (history erased; remembered for this config)\n", sep)
	} else {
		fmt.Printf("delimiter: %s (this session only; turn history on to remember it)\n", sep)
	}
	r.warnDelim()
}
