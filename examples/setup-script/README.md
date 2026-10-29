# A setup script around ezconfig

`setup.cmd` (Windows) and `setup.sh` (Linux, Termux) show how a program's
own setup script can hand the settings to `ezconfig ask` and do the
system parts itself, in the same style: a `[section]` heading, a question
with its format in parentheses, a `name [current]>` prompt, and a line of
dashes before the result.

Copy ezconfig (ezconfig.exe on Windows) into this folder, then run the
script. ezconfig's exit status decides what happens next: 0 or 1 carry on,
6 (the person quit or stopped) ends the script with nothing else changed.

The schema's `[@ask]` block names what it ships with:

    application=Signage player 1.0
    version=1

ezconfig shows these when `ask` starts and in `status`.

`config.ini` starts with only a comment; `ask` fills it in. The startup
step is a stand-in: it starts a `player.exe` (or `player`) next to the
script at sign-in.
