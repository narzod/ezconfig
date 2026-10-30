# Roadmap

What we've decided to build next, what's for later, and what we've set
aside. Updated whenever a decision is made; each released version is a
tag (`git tag`), and its commit message says what changed.

## Next

- **Windows text files.** Accept a UTF-8 byte-order mark at the start of
  a config or schema (some Windows editors add one; today such a file
  fails with "line 1: expected key=value") and keep it on save. Keep the
  file's line endings (CRLF or LF) on save; a new file gets the
  platform's. Recognise a UTF-16 file and say so plainly instead of
  reporting a syntax error.

## Later

- **More built-in types**, as needs come up: window geometry, UUID,
  version number, locale (today these are patterns with a format; see
  examples/types). Possibly min and max for date, time and port.
- **Named types**, `[@type NAME]`: a schema defines a type once (type,
  min, max, values, pattern, format, doc) and rules use it by name and
  may override any of it; prompt and required stay with the rule.
- **Blank lines kept on save.** Comments are kept; blank lines between
  keys are not (a save puts one blank line before each section).
- **Values with leading or trailing spaces**, which are trimmed today.
- **Schema built into the executable**, so a program ships one file.
  Revisit only if a schema file next to the program proves awkward.

## Set aside

- **Renaming the executable to change what it does** (e.g. `setup.exe`
  running `ask`). "setup" belongs to real setup scripts on Windows, and a
  schema named after the program invites people to edit it as the
  config. Instead, a program's setup script calls `ezconfig ask`; see
  examples/setup-script.
- **The schema naming its config** (`config=` in `[@ask]`). The config
  finds its schema, or has none; the tool also works without a schema.
- **Editing a single answer at the end of ask.** ask offers redo instead,
  as setup scripts usually do; editing one key is what `set KEY` is for.
- **A leading delimiter for root keys** (`.name`). Root keys are the
  common case and stay bare; `NAME.` is the section when both exist.
- **Colour in output.** Plain text throughout.

## Notes

- Windows builds keep their symbols and carry version details and a
  manifest (winres/): a stripped, unsigned build was flagged by
  Defender. The manifest also keeps Windows from treating the program as
  an installer.
- A release updates `version.go` and `winres/` together.
