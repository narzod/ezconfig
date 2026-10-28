# ezconfig

A small cross-platform tool for reading and changing INI configs, meant to be
the single source of truth for a config.

- `ezconfig -c config.ini` opens an interactive shell with tab completion.
  Changes stay in memory until saved; an asterisk in the prompt indicates
  unsaved changes, and `changes` lists them.
- `ezconfig -c config.ini COMMAND ...` runs one command for scripts and other
  programs: `get`, `set`, `unset`, `keys`, `sections`, `list`, `dump`.

Run `ezconfig help` (or `help` inside the shell) for the full command list.

## Building

    go mod tidy
    go build                                # this platform
    GOOS=windows GOARCH=amd64 go build      # Windows
    GOOS=linux GOARCH=arm64 go build        # Android (Termux)

Earlier versions were called inirepl; tag v0.8.0 is the last of those.
