# ezconfig

A small cross-platform tool for reading and changing INI configs, meant to be
the single source of truth for a config.

- `ezconfig -c config.ini` opens an interactive shell with tab completion.
  Changes stay in memory until saved; an asterisk in the prompt indicates
  unsaved changes, and `changes` lists them.
- `ezconfig -c config.ini COMMAND ...` runs one command for scripts and other
  programs: `get`, `set`, `unset`, `keys`, `sections`, `list`, `emit`
  (JSON for programs).

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

`ask` walks through the schema's sections asking for each value, as
`set KEY` does, then shows the changes and asks: save, edit an answer, or
quit. That question has no default, so pressing Enter once too often
can't lose anything. `ask SECTION` does one section. An `[@ask]` block gives it a `title`, `doc`
and `sections=` (which sections, in order). During `ask`, Enter can't leave
a required key unset or an invalid value in place. It works in the shell
and as `ezconfig ask` (which needs a terminal, so a program can run it on
first launch: exit 0 when saved, 6 when stopped or quit without saving; in the
shell, quitting keeps the answers unsaved in memory). While a
question is asked, the up arrow recalls the key's current value and its
default rather than the shell's history.

Rules have a type (string, int, float, bool, enum) with min, max, values,
pattern and required, plus `format` (how to write the value, in words, shown
instead of the pattern), `doc` and `prompt`. Sections have `title`, `doc` and
`keys` (the order to present them); `[@section .]` is the root keys.
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
one; `NAME.` (with the delimiter) is only the section. In
the shell, `set KEY` without `=` explains the key and asks for its value
(Enter keeps it; `set KEY=` sets it empty). See `examples/`.

Run `ezconfig help` (or `help` inside the shell) for the full command list.

## Building

    go mod tidy
    go build                                # this platform
    GOOS=windows GOARCH=amd64 go build -trimpath    # Windows
    GOOS=linux GOARCH=arm64 go build        # Android (Termux)

Windows builds include version details and a manifest from
`rsrc_windows_amd64.syso`; see `winres/` to rebuild it. Leave the symbols
in (no `-ldflags "-s -w"`): stripped, unsigned Go programs are flagged by
antivirus heuristics more often.

Earlier versions were called inirepl; tag v0.8.0 is the last of those.
