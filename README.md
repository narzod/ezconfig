# ezconfig

A small cross-platform tool for reading and changing INI configs, meant to be
the single source of truth for a config.

- `ezconfig -c config.ini` opens an interactive shell with tab completion.
  Changes stay in memory until saved; an asterisk in the prompt indicates
  unsaved changes, and `changes` lists them.
- `ezconfig -c config.ini COMMAND ...` runs one command for scripts and other
  programs: `get`, `set`, `unset`, `keys`, `sections`, `list`, `dump`.

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

A `[@setup]` block (with `title`, `doc` and `sections=`) sets up the
`setup` command in the shell, which walks through the sections asking for
each value, as `set KEY` does; `setup SECTION` does one section.

Rules have a type (string, int, float, bool, enum) with min, max, values,
pattern and required, plus `format` (how to write the value, in words, shown
instead of the pattern), `doc` and `prompt`. Sections have `title`, `doc` and
`keys` (the order to present them); `[@section .]` is the root keys.

With a schema, reading commands show what applications should see: defaults
for keys the config doesn't set, and only valid values (an invalid value is
replaced by its default, or left out, with a warning on stderr). `set`
refuses values that break a rule, `check` reports problems, and `-s none`
shows the config exactly as written. `describe` lists every key the schema
knows; `describe SECTION` one section; `describe KEY` one key in detail. In
the shell, `set KEY` without `=` explains the key and asks for its value
(Enter keeps it; `set KEY=` sets it empty). See `examples/`.

Run `ezconfig help` (or `help` inside the shell) for the full command list.

## Building

    go mod tidy
    go build                                # this platform
    GOOS=windows GOARCH=amd64 go build      # Windows
    GOOS=linux GOARCH=arm64 go build        # Android (Termux)

Earlier versions were called inirepl; tag v0.8.0 is the last of those.
