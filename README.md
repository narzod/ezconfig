# ezconfig

A small cross-platform tool for reading and changing INI configs, meant to be
the single source of truth for a config.

- `ezconfig -c config.ini` opens an interactive shell with tab completion.
  Changes stay in memory until saved; an asterisk in the prompt indicates
  unsaved changes, and `changes` lists them.
- `ezconfig -c config.ini COMMAND ...` runs one command for scripts and other
  programs: `get`, `set`, `unset`, `keys`, `sections`, `list`, `dump`.

An optional schema (`schema.ini` next to the config, or `-s FILE`) supplies
defaults and rules: types string, int, float, bool and enum, with min, max,
values, pattern and required. With a schema, reading commands show what
applications should see: defaults for keys the config doesn't set, and only
valid values (an invalid value is replaced by its default, or left out, with
a warning on stderr). `set` refuses values that break a rule, `check`
reports problems, `describe KEY` explains a key, and `-s none` shows the
config exactly as written. See `examples/`.

Run `ezconfig help` (or `help` inside the shell) for the full command list.

## Building

    go mod tidy
    go build                                # this platform
    GOOS=windows GOARCH=amd64 go build      # Windows
    GOOS=linux GOARCH=arm64 go build        # Android (Termux)

Earlier versions were called inirepl; tag v0.8.0 is the last of those.
