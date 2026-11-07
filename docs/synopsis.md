# ezconfig: a synopsis for engineers

November 2026; describes ezconfig 0.27.0. Kept here until it finds a
home in the README or elsewhere; the original is a shared doc.

## What it is

ezconfig is a single-file command-line tool, written in Go, that is the one place an application's settings are read and changed, on Windows, Linux and Android (Termux). Settings live in two ordinary INI files with different owners: `config.ini` belongs to the user and holds only what they chose to change, and `schema.ini` ships with the application and holds its defaults and rules. ezconfig combines the two, so an upgrade can bring new defaults without ever touching the user's choices.

## What problems it solves

The same two files carry a project from its first sketch to a product with clients.

- **Prototyping:** a developer writes a quick settings file and lets it grow as the project needs. The configuration is usable before the application exists: it can be read, checked and changed from day one.
- **Real users and clients:** as the project grows, the schema adds rules, so people can customize their settings while conforming to them. Bad input is refused or set aside, and new defaults reach every install without overwriting anyone's choices.
- **Setup:** a setup script can call `ezconfig ask` to collect the settings it needs, for the whole application, for one component (`ask schedule`) or down to a single setting (`ask schedule.showtime`). The questions, formats and allowed values all come from the schema, so the script never repeats them.

## How programs use it

Programs don't parse INI themselves; they call ezconfig. `ezconfig list json` prints the whole merged config as JSON, with defaults filled in, invalid values left out and every value in standard spelling, and `ezconfig get net.port` prints single values. Data goes to standard output, messages to standard error, and documented exit codes say what happened, so any language that can run a process and read JSON (Go, Python, PowerShell, Free Pascal) gets the same validated settings. Writes made with `ezconfig set` are checked against the schema and saved atomically, so a program never reads a half-written file.

## The schema

The schema is optional, and it is where the strictness lives: each key can have a type, a range, a list of allowed values, a pattern, or a built-in type such as a calendar-checked date, a time, an IP address, a URL, an email address, a time zone or an X11 colour name. Input is forgiving and output is standard, so `debug=YES` is stored and reported as `true`, and `steel blue` as `SteelBlue`. The same file describes each key for people, with a question to ask, a format in words and a short description.

## For people, and why INI

People get an interactive shell with Tab completion, unsaved-change tracking (`changes`, `save`, `revert`), `explain` for what each key means and `check` for problems. `ezconfig ask` walks someone through the settings one question at a time and ends by asking whether to save, which suits setup scripts and talking a client through a change on the phone. INI was chosen because people can read and repair it in Notepad, while the strict side faces programs: the schema enforces the rules, and programs receive JSON.
