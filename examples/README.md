# Trying the schema

Each folder holds a config and the `schema.ini` that ezconfig finds next to
it automatically. Run these from inside the folder. On Windows, `echo
%errorlevel%` (cmd) or `$LASTEXITCODE` (PowerShell) shows the exit status.

## signage

    ezconfig list                          defaults filled in where config.ini is silent
    ezconfig get display.width player.debug    1920 and false: a default, and "no" in its standard spelling
    ezconfig explain                      every key the schema knows, section by section
    ezconfig explain player               one section; site comes first, as keys= says
    ezconfig explain display.rotation     type, allowed values, default and current value
    ezconfig check                         no output, exit 0: config.ini is valid
    ezconfig -c broken.ini check           eight problems, exit 1
    ezconfig -c broken.ini list            only valid values: defaults replace the bad ones, warnings on stderr
    ezconfig -c broken.ini -s none list    the same config exactly as written
    ezconfig set display.rotation=45       refused, exit 1
    ezconfig set player.debug=YES          accepted, saved as true
    ezconfig -s none list                  the config alone, without the schema

Then open the shell with `ezconfig` and try:

- `ask`: asks for each player, schedule and display value in the order
  `[@ask] sections=` gives, then shows the changes and asks: save, redo,
  or quit; Enter keeps a value (but not a missing site), and at
  the end only s, e or q will do
- `ask schedule`, then the up arrow at a question: recalls the current
  value and the default for editing
- `set schedule.on` (no `=`): asks for the time, says what format it needs,
  and asks again after an answer like 7am; Enter alone keeps the value
- `set display.rotation`, then Tab twice at the question: lists 0 90 180 270
- `set display.rotation=` then Tab twice: lists 0 90 180 270
- `set display.` then Tab twice: lists the keys you can set, including scale
- `set player.` then Tab twice: includes site, which nothing has set yet
- `set display.background=r` then Tab: completes to red
- `get net.` then Tab: offers net.port even though config.ini never sets it
- `unset display.rotation`: reports that the default (0) now applies
- `explain player.site`, then `check`

## backup

Root keys (retries, verify) are described by `[@section .]`. Its `[@ask]`
has no `sections=`, so `ask` walks every section with rules, root first.

    ezconfig list                          mode shows as incremental, verify as true
    ezconfig get retries verify            root keys with defaults: 3 and true
    ezconfig set backup.mode=weekly        refused: not one of mirror, incremental, full
    ezconfig unset backup.source           allowed, but then `ezconfig check` reports it as required
    ezconfig explain .                    the root keys
    ezconfig ask                           asks for each value, then save, redo or quit; exit 0, or 6 if not saved

## types

Common kinds of value, mostly with built-in types: X11 color names, hex
colors, dates (checked against the calendar), 12- and 24-hour times, time
zones, host names, IPv4 addresses, ports, URLs, email addresses, durations
and sizes. Window geometry, UUIDs, version numbers and locales have no
built-in type yet, so they use a pattern with a `format`.

    ezconfig list look                     steel blue in the config comes out as SteelBlue
    ezconfig get event.open                7:30pm is stored as 7:30 PM
    ezconfig set event.date=2026-02-30     refused: not a real date
    ezconfig set look.background=Tomatoe   refused: did you mean Tomato?
    ezconfig explain look                  each type describes itself; no list of colors

In the shell, `set look.background=` then Tab twice offers all the color
names (after asking), and `set look.background=dark` then Tab lists the
Dark... ones.
