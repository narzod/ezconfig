# Trying the schema

Each folder holds a config and the `schema.ini` that ezconfig finds next to
it automatically. Run these from inside the folder. On Windows, `echo
%errorlevel%` (cmd) or `$LASTEXITCODE` (PowerShell) shows the exit status.

## signage

    ezconfig list                          defaults filled in where config.ini is silent
    ezconfig get display.width player.debug    1920 and false: a default, and "no" in its standard spelling
    ezconfig describe                      every key the schema knows, section by section
    ezconfig describe player               one section; site comes first, as keys= says
    ezconfig describe display.rotation     type, allowed values, default and current value
    ezconfig check                         no output, exit 0: config.ini is valid
    ezconfig -c broken.ini check           eight problems, exit 1
    ezconfig -c broken.ini list            only valid values: defaults replace the bad ones, warnings on stderr
    ezconfig -c broken.ini -s none list    the same config exactly as written
    ezconfig set display.rotation=45       refused, exit 1
    ezconfig set player.debug=YES          accepted, saved as true
    ezconfig -s none list                  the config alone, without the schema

Then open the shell with `ezconfig` and try:

- `set schedule.on` (no `=`): asks for the time, says what format it needs,
  and asks again after an answer like 7am; Enter alone keeps the value
- `set display.rotation`, then Tab twice at the question: lists 0 90 180 270
- `set display.rotation=` then Tab twice: lists 0 90 180 270
- `set display.` then Tab twice: lists the keys you can set, including scale
- `set player.` then Tab twice: includes site, which nothing has set yet
- `set display.background=r` then Tab: completes to red
- `get net.` then Tab: offers net.port even though config.ini never sets it
- `unset display.rotation`: reports that the default (0) now applies
- `describe player.site`, then `check`

## backup

Root keys (retries, verify) are described by `[@section .]`.

    ezconfig list                          mode shows as incremental, verify as true
    ezconfig get retries verify            root keys with defaults: 3 and true
    ezconfig set backup.mode=weekly        refused: not one of mirror, incremental, full
    ezconfig unset backup.source           allowed, but then `ezconfig check` reports it as required
    ezconfig describe .                    the root keys

## types

Rules for common kinds of value: colours, dates, 12- and 24-hour times, time
zones, window geometry, host names, IPv4 addresses, URLs, email addresses,
durations, sizes, UUIDs, version numbers and locales. Each pattern has a
`format`, so people see words instead of the regular expression.

    ezconfig describe                      every rule, with its format
    ezconfig check                         event.date 2026-02-30 passes: a pattern can't count days in a month
    ezconfig set event.open=19:30          refused: needs am or pm
    ezconfig set job.cache=512MB           accepted
