# Trying the schema

Each folder holds a config and the `schema.ini` that ezconfig finds next to
it automatically. Run these from inside the folder. On Windows, `echo
%errorlevel%` (cmd) or `$LASTEXITCODE` (PowerShell) shows the exit status.

## signage

    ezconfig list                          defaults filled in where config.ini is silent
    ezconfig get display.width player.debug    1920 and false: a default, and "no" in its standard spelling
    ezconfig describe display.rotation     type, allowed values, default and current value
    ezconfig check                         no output, exit 0: config.ini is valid
    ezconfig -c broken.ini check           eight problems, exit 1
    ezconfig -c broken.ini list            only valid values: defaults replace the bad ones, warnings on stderr
    ezconfig -c broken.ini -s none list    the same config exactly as written
    ezconfig set display.rotation=45       refused, exit 1
    ezconfig set player.debug=YES          accepted, saved as true
    ezconfig -s none list                  the config alone, without the schema

Then open the shell with `ezconfig` and try:

- `set display.rotation=` then Tab twice: lists 0 90 180 270
- `set display.` then Tab twice: lists the keys you can set, including scale
- `set player.` then Tab twice: includes site, which nothing has set yet
- `set display.background=r` then Tab: completes to red
- `get net.` then Tab: offers net.port even though config.ini never sets it
- `unset display.rotation`: reports that the default (0) now applies
- `describe player.site`, then `check`

## backup

    ezconfig list                          mode shows as incremental, verify as true
    ezconfig get retries verify            root keys with defaults: 3 and true
    ezconfig set backup.mode=weekly        refused: not one of mirror, incremental, full
    ezconfig unset backup.source           allowed, but then `ezconfig check` reports it as required
