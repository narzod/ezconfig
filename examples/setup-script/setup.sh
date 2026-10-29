#!/bin/sh
# setup.sh - set up the signage player on Linux (and Termux).
# ezconfig asks for the settings; this script does the system parts, in
# the same style. Keep ezconfig, config.ini and schema.ini next to this
# file, or ezconfig on the PATH.
here=$(cd "$(dirname "$0")" && pwd)
ez="$here/ezconfig"
[ -x "$ez" ] || ez=ezconfig
autostart="${XDG_CONFIG_HOME:-$HOME/.config}/autostart"
launcher="$autostart/signage-player.desktop"

# The settings. ezconfig exits 0 when saved (or nothing changed), 1 when
# saved but a value still breaks the schema, 6 when the person quit or
# stopped, and 2 to 5 when something is wrong with the files.
"$ez" -c "$here/config.ini" -s "$here/schema.ini" ask
case $? in
0) ;;
1) echo "some settings still break the schema; run ezconfig check" ;;
6) echo "setup stopped; nothing else was changed"; exit 6 ;;
*) echo "setup could not finish (see the message above)"; exit 2 ;;
esac

# The system parts.
echo
echo "[system] System: How the player starts"
echo
now=n
[ -f "$launcher" ] && now=y
while :; do
	echo "Start the player when this user signs in? (y or n)"
	printf 'system.startup [%s]> ' "$now"
	read -r answer || exit 6
	case $answer in
	"") answer=$now; break ;;
	[yY] | [yY][eE][sS]) answer=y; break ;;
	[nN] | [nN][oO]) answer=n; break ;;
	*) echo "type y or n" ;;
	esac
done

echo
echo "----------------------------------------"
if [ "$answer" = y ]; then
	mkdir -p "$autostart"
	printf '[Desktop Entry]\nType=Application\nName=Signage player\nExec=%s\n' "$here/player" >"$launcher"
	echo "the player starts at sign-in"
else
	rm -f "$launcher"
	echo "the player doesn't start at sign-in"
fi
echo "setup finished"
