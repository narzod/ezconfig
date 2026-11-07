# Roadmap

What we've decided to build, in order, then what's for later, open for
discussion, or set aside. Updated whenever a decision is made; each
released version is a tag (`git tag`), and its commit message says what
changed.

## Stage 1: on main (done in 0.26.2)

- Windows text files: a UTF-8 byte-order mark and CRLF line endings are
  read and kept; a new file gets the platform's line endings; a UTF-16
  file is refused with a plain message (exit 2).
- `name.` means a section in every command that takes one.
- "Why this design?" at the top of the README, which also links
  `docs/synopsis.md`. Open: whether the synopsis stays its own page or is
  folded in (it now says `list json`).

## Stage 2: on a branch, `list-formats` (built as 0.27.0-rc1)

Tried on the branch before it reaches main; merged and tagged `v0.27.0`
once tried, or dropped if it doesn't work out.

- `list [FORMAT] [SECTION]`: `flat` (default), `ini` or `json`; a format
  word comes first; a section named like a format is written `name.`.
- `emit` is gone; `list json` replaces it with the same JSON, exit codes
  and colour, and `ezconfig emit` says so.
- `export [FILE]` writes the JSON, atomically, by default to the config's
  name with `.json`; never over the config or schema; exit codes as for
  `list json`; its message goes to standard error.
- No options: formats are words.
- rc3: `export` publishes the saved config only; in the shell it refuses
  while there are unsaved changes ("save first, or revert").
- rc2: Tab offers sections as `name.` in every command (the one spelling
  that always means a section), and a typed `list json` notes it when a
  `[json]` section exists too.

## Stage 3: toward 1.0

- **Freeze the contract** that projects depend on: command names, the
  JSON shape, exit codes and the schema syntax. From 1.0 on, 1.x only
  adds; a breaking change would be 2.0.
- **Client helpers** in `clients/`: a short, tested `LoadConfig` for
  Free Pascal, Go and Python (and PowerShell), each finding ezconfig next
  to the program first, then on the PATH, checking `ezconfig version`
  (1.0 or later), and reading `list json`.
- **Push the version tags to GitHub** (v0.15.0 to v0.27.0-rc3). They are
  in Jeff's WSL clone (fetched from `ezconfig-tags.bundle`) and need one
  `git push origin --tags` from there once GitHub sign-in works on that
  machine; Claude's workspace can push branches but not tags. Needed
  before GitHub releases and `go install ...@v1.0.0`.
- **A licence file.** Open: which licence (MIT is the common short one).
- **Publish on GitHub with releases**: one file per platform
  (`ezconfig-windows-amd64.exe`, `ezconfig-linux-amd64`,
  `ezconfig-linux-arm64` for Termux), each with its SHA-256.
- **Install scripts** (`install.sh`, `install.ps1`) for operators: detect
  the platform, verify the checksum, install without administrator
  rights, safe to re-run, a version can be pinned. The manual route stays
  documented beside them.
- **How projects declare it**: a README "Requirements" line ("ezconfig
  1.0 or later, next to the program or on the PATH"); clients get it in
  the application's folder, operators on their PATH.

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

## For discussion

- **A locale type** (`type=locale`): language and region checked against
  ISO 639 and ISO 3166, forgiving input (`en_us` stored as `en-US`), Tab
  in two steps (languages, then regions after the dash), suggestions for
  near misses (`en-UK`, did you mean `en-GB`?). Open: any valid pairing or
  only common ones; scripts such as `zh-Hant-TW`; whether a Linux-style
  `en_US.UTF-8` form is ever needed.
- **`values=` on an int**: "a number, but only one of these", e.g. a
  rotation of 0, 90, 180 or 270. Today that is an enum, whose values are
  names.
- **More built-in types, a shortlist**: `list=yes` on any rule (a
  comma-separated list, each item checked), `timerange` (07:00-22:00,
  across midnight), `weekdays` (mon-fri,sun), `slug`, `path` and
  `folder`, `resolution` (1920x1080, with common sizes on Tab),
  `hostport`. Cheap whenever needed: `mac`, `uuid`, `geometry`, `version`,
  `percent`, `cidr`, `country`, `currency`.
- **Typed JSON from `list json`** (numbers and
  true/false where the schema says so). Decided against for now: it
  stays strings. It is the
  config in JSON's shape, the same shape with or without a schema and
  the same text `get` and `list` give; the schema has already checked and
  standardized each value, so the program's conversion can't fail. A
  typed form would also need a flag (`list json --typed`, failing without a
  schema), the first option belonging to one command.

- **Tab completion for `ask`.** It offers keys and sections as `name.`,
  like `explain`, including sections with no rules, which `ask` then
  refuses. Jeff has a view on this to come back to; nothing changes until
  then.

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
- **`set KEY` asking for a value.** One-shot commands are for programs and
  never ask; `ask` is the one command for people, in the shell too. `ask
  KEY` asks one question and ends with the same review, so a consultant
  on the phone or a setup script can use it.
- **Metadata in the JSON** of `list json` (formerly `emit`), a `_meta`
  member describing renames, and **refusing to produce JSON** when a root
  key and a section share a name. Instead
  `list json` shows the root key as `NAME=` (no INI key can contain `=`, so it
  can't collide), warns on stderr and exits 7; a program that checks the
  code can read it, and one that doesn't expect it stops on the non-zero
  status. A reserved name such as `""`, `default` or `_meta` would only
  bring its own clashes.
- **One set of names for root keys and sections.** INI allows the same
  name for both; ezconfig keeps that freedom and handles it in `list json`.

## Notes

- **Colour, git-style.** Only on a terminal, only in the shell and `ask`,
  only on what the text already marks (`+ - ~`, errors, notes, headings,
  keys in shell `list`, JSON as jq colours it), never on one-shot data; `NO_COLOR` and `TERM=dumb` turn it off. These are the
  only environment variables ezconfig reads, because they are shared
  conventions rather than settings of its own.
- **The shell collects changes; the command line completes them.** In
  the shell, `set` and `ask` add to the session and `save` writes it all;
  on the command line, `set` saves at once and `ask` ends with a review
  (save, redo or quit) and an exit status.
- Windows builds keep their symbols and carry version details and a
  manifest (winres/): a stripped, unsigned build was flagged by
  Defender. The manifest also keeps Windows from treating the program as
  an installer.
- A release updates `version.go` and `winres/` together.
