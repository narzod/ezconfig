# Roadmap

What we've decided to build, in order, then what's for later, open for
discussion, or set aside. Updated whenever a decision is made; each
released version is a tag (`git tag`), and its commit message says what
changed.

## Stage 1: on main, next

Small, independent changes, each released as usual.

- **Windows text files.** Accept a UTF-8 byte-order mark at the start of
  a config or schema (some Windows editors add one; today such a file
  fails with "line 1: expected key=value") and keep it on save. Keep the
  file's line endings (CRLF or LF) on save; a new file gets the
  platform's. Recognise a UTF-16 file and say so plainly instead of
  reporting a syntax error.
- **`name.` means a section in every command that takes one**: `keys`,
  `changes`, `wipe`, `copy`, `move` and `list`, as `explain` and `ask`
  already allow. Bare names keep working as now. The rule then has no
  exceptions: wherever a section goes, `name.` works.
- **"Why this design?" at the top of the README**: the two files and
  who owns each (the user's `config.ini`, the application's
  `schema.ini`, ezconfig combining them); why INI rather than TOML; and
  that ezconfig ships with the application and is how people and
  programs use the config, while the file stays plain INI anyone can
  read, compare and keep.

## Stage 2: on a branch, `list-formats`

One breaking change, tried on a branch before it reaches main. Exes
built from the branch report a pre-release version (`0.27.0-rc1`).
Merged and tagged `v0.27.0` once tried; dropped if it doesn't work out.

- **`list [FORMAT] [SECTION]`** prints the effective config as `flat`
  (`KEY=VALUE` lines, the default), `ini` or `json`. If the first word is
  a format it's the format, and what follows is the section; a section
  named `flat`, `ini` or `json` is written `name.` (`list json json.`).
- **`emit` goes away**: `list json` replaces it, with the same JSON, the
  same exit codes (7 renamed root key, 2 refused) and the same colour in
  the shell. Docs, man page, examples and the setup script change to
  `list json`.
- **`export [FILE]`** writes the JSON to a file, by default `config.json`
  next to the config (named after it: `signage.ini` gives
  `signage.json`). It is written atomically, says what it wrote, has the
  same exit codes as `list json`, and refuses to write over the config or
  the schema. JSON only: other formats go to a file by redirecting `list`
  (`ezconfig list ini > effective.ini`). It is the exception for programs
  that can't run ezconfig; reading `list json` is still the way programs
  should read their settings.
- **No options.** Formats are words, not `--format=`.

## Stage 3: toward 1.0

- **Freeze the contract** that projects depend on: command names, the
  JSON shape, exit codes and the schema syntax. From 1.0 on, 1.x only
  adds; a breaking change would be 2.0.
- **Client helpers** in `clients/`: a short, tested `LoadConfig` for
  Free Pascal, Go and Python (and PowerShell), each finding ezconfig next
  to the program first, then on the PATH, checking `ezconfig version`
  (1.0 or later), and reading `list json`.
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
- **Typed JSON from `emit`** (`list json` after stage 2: numbers and
  true/false where the schema says so). Decided against for now: it
  stays strings. It is the
  config in JSON's shape, the same shape with or without a schema and
  the same text `get` and `list` give; the schema has already checked and
  standardized each value, so the program's conversion can't fail. A
  typed form would also need a flag (`emit --typed`, failing without a
  schema), the first option belonging to one command.

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
- **Metadata in `emit`'s JSON** (a `_meta` member describing renames) and
  **refusing to emit** when a root key and a section share a name. Instead
  `emit` shows the root key as `NAME=` (no INI key can contain `=`, so it
  can't collide), warns on stderr and exits 7; a program that checks the
  code can read it, and one that doesn't expect it stops on the non-zero
  status. A reserved name such as `""`, `default` or `_meta` would only
  bring its own clashes.
- **One set of names for root keys and sections.** INI allows the same
  name for both; ezconfig keeps that freedom and handles it in `emit`.

## Notes

- **Colour, git-style.** Only on a terminal, only in the shell and `ask`,
  only on what the text already marks (`+ - ~`, errors, notes, headings,
  keys in shell `list` and `emit`), never on one-shot data; `NO_COLOR` and `TERM=dumb` turn it off. These are the
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
