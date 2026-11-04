# ezconfig

A small cross-platform tool for reading and changing INI configs, meant to be
the single source of truth for a config.

- `ezconfig -c config.ini` opens an interactive shell with tab completion.
  Changes stay in memory until saved; an asterisk in the prompt indicates
  unsaved changes, and `changes` lists them.
- `ezconfig -c config.ini COMMAND ...` runs one command for scripts and other
  programs: `get`, `set`, `unset`, `keys`, `sections`, `list`, `emit`
  (JSON for programs). Values in `emit` are always strings, as in the
  config itself: with a schema they are checked and in standard spelling,
  so a program converts the ones it needs (decode into a plain map, then
  convert). `man ./docs/ezconfig.1` (or `docs/ezconfig.txt`) has short
  examples for PowerShell, Python and Go under USING FROM PROGRAMS.

An optional schema (`schema.ini` next to the config, or `-s FILE`) supplies
defaults and rules. Its ordinary sections are defaults, shaped like a config;
blocks starting with `@` describe the config:

    [net]
    port=8080

    [@section net]
    title=Network
    keys=port

    [@rule net.port]
    type=int
    min=1
    max=65535
    format=a number from 1 to 65535
    prompt=Which port should the player listen on?
    doc=Port the player listens on

`ask` is the one command meant for people (`set` never asks). On its own
it walks through the schema's sections asking for each value; `ask NAME...`
asks about the sections and keys named, so `ezconfig ask schedule.showtime`
asks one question. On the command line it then shows the changes and
asks: save, redo the questions (starting from the answers given), or
quit. That question has no default, so pressing Enter once too often
can't lose anything. In the shell the answers simply join the session's
unsaved changes, and one line says what changed: the shell collects
changes, the command line completes them. A named key works even without
a schema. An `[@ask]` block gives it a `title`, `doc`
and `sections=` (which sections, in order), and can say what the schema
is: `application=` (what it ships with) and `version=` (its own version),
shown when `ask` starts and in `status`. During `ask`, Enter can't leave
a required key unset or an invalid value in place. It works in the shell
and as `ezconfig ask` (which needs a terminal, so a program can run it on
first launch: exit 0 when saved, 6 when stopped or quit without saving; in the
shell, quitting keeps the answers unsaved in memory). While a
question is asked, the up arrow recalls the key's current value and its
default rather than the shell's history.

Rules have a type (string, int, float, bool, enum) with min, max, values,
pattern and required, or a built-in type: `x11color`, `hexcolor`, `date`,
`time`, `time12`, `duration`, `size`, `ipv4`, `ip`, `host`, `url`, `email`,
`timezone`, `port`. A built-in type checks the value properly (a date
against the calendar), accepts forgiving input and stores the standard
spelling (`steel blue` becomes `SteelBlue`, `7:30pm` becomes `7:30 PM`),
and describes itself, so it needs no format; `x11color` offers its names
to Tab. Every rule, of any type, can have `required`, `doc`, `prompt` and
`format` (how to write the value, in words; shown instead of a pattern,
and in place of a built-in type's own description). Sections have
`title`, `doc` and `keys` (the order to present them); `[@section .]` is
the root keys.
`ask` shows a rule's `prompt` (or its `doc`) with the format in
parentheses, so write formats without parentheses of their own:
`format=HH:MM, 24-hour, e.g. 07:30`.

With a schema, reading commands show what applications should see: defaults
for keys the config doesn't set, and only valid values (an invalid value is
replaced by its default, or left out, with a warning on stderr). `set`
refuses values that break a rule, `check` reports problems, and `-s none`
shows the config exactly as written. `explain` covers every key the schema
knows; `explain SECTION` one section; `explain KEY` one key in detail. A
bare name is a root key, and also the section of that name if there is
one; `NAME.` (with the delimiter) is only the section; `ask` reads names
the same way. See `examples/`.

A root key and a section may share a name in INI, but not in one JSON
object, so `emit` shows such a root key with `=` after its name
(`"net=": "wired"`), which no INI key can contain; the section keeps its
name. It says so on stderr and exits 7, so a program can tell; `emit .`
and `emit net` give each part without renaming, and `check` reports the
clash. Section names may contain `=` (ezconfig never creates them, and
`check` points them out), so if a `[net=]` section also exists, `emit`
refuses with exit 2 and writes nothing, rather than produce JSON with two
members of the same name.

In the shell and in `ask`, a little colour marks what the text already
says, as git does: `+` lines green, `-` red, `~` yellow, errors red,
notes yellow, section headings bold; in the shell, `list` keys are cyan and
`emit` colours JSON as jq does. It is used only on a terminal, never for
one-shot data or anything piped, and `NO_COLOR` (or `TERM=dumb`) turns it
off.

Comments in a config (lines starting with `;` or `#`) are kept when it is
saved, with the key or section that follows them; a comment block at the
top, set apart by a blank line, stays at the top.

A program's own setup script can hand the settings to `ezconfig ask` and
act on its exit status; see `examples/setup-script/`.

A manual page is in `docs/`: `ezconfig.1` for `man`, and `ezconfig.txt`
for reading anywhere (`man ./docs/ezconfig.1` shows it on Linux).

Run `ezconfig help` (or `help` inside the shell) for the full command list.

## Building

    go mod tidy
    go build                                # this platform
    GOOS=windows GOARCH=amd64 go build -trimpath    # Windows
    GOOS=linux GOARCH=arm64 go build        # Android (Termux)

The version is in `version.go`; a release updates it there and in
`winres/` (then rebuilds the .syso). To stamp a test build differently:
`go build -ldflags "-X main.version=0.22.1-test"`.

Windows builds include version details and a manifest from
`rsrc_windows_amd64.syso`; see `winres/` to rebuild it. Leave the symbols
in (no `-ldflags "-s -w"`): stripped, unsigned Go programs are flagged by
antivirus heuristics more often.

Earlier versions were called inirepl; tag v0.8.0 is the last of those.
