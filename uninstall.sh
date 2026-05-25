#!/bin/sh
# checkin uninstaller — finds and removes the checkin binary, with an
# optional follow-up step to remove the token cache and config at
# ~/.config/checkin/. POSIX sh, no bash extensions.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/excelano/checkin-cli/main/uninstall.sh | sh
#
# Environment variables:
#   CHECKIN_UNINSTALL_YES=1  Skip the interactive confirmation (assume yes)
#   CHECKIN_PURGE=1          Also remove ~/.config/checkin/ (token cache and config)

set -eu

BIN="checkin"

say() { printf '%s\n' "$*" >&2; }
err() { say "error: $*"; exit 1; }

read_yes() {
	prompt="$1"
	if [ "${CHECKIN_UNINSTALL_YES:-0}" = "1" ]; then
		return 0
	fi
	if [ ! -t 0 ] && [ ! -e /dev/tty ]; then
		err "no terminal available for confirmation; re-run with CHECKIN_UNINSTALL_YES=1 to skip the prompt"
	fi
	printf '%s [y/N]: ' "$prompt" >&2
	if [ -e /dev/tty ]; then
		read ans </dev/tty
	else
		read ans
	fi
	case "$ans" in
		y|Y|yes|YES) return 0 ;;
		*) return 1 ;;
	esac
}

if ! command -v "$BIN" >/dev/null 2>&1; then
	say "$BIN is not on PATH; nothing to uninstall."
	say "If you installed to a custom location, remove it manually:"
	say "    rm /path/to/$BIN"
	exit 0
fi

TARGET=$(command -v "$BIN")
say "Found $BIN at $TARGET"

if [ ! -w "$TARGET" ] && [ ! -w "$(dirname "$TARGET")" ]; then
	err "$TARGET is not writable; re-run with sudo to remove it"
fi

if ! read_yes "Remove $TARGET?"; then
	say "Aborted."
	exit 1
fi

rm -f "$TARGET" || err "could not remove $TARGET"
say "Removed $TARGET"

hash -r 2>/dev/null || true

LEFTOVER=$(command -v "$BIN" 2>/dev/null || true)
if [ -n "$LEFTOVER" ]; then
	say ""
	say "Note: another $BIN binary is still on PATH at $LEFTOVER"
	say "Re-run this uninstaller to remove it, or remove it manually."
fi

# Optional state cleanup. Token cache and config live in ~/.config/checkin/.
# Only remove if CHECKIN_PURGE=1 was passed or the user confirms — re-running
# checkin after a fresh install with the same config keeps the token, which
# saves a device-code round-trip.
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/checkin"
if [ -d "$CONFIG_DIR" ]; then
	if [ "${CHECKIN_PURGE:-0}" = "1" ] || read_yes "Also remove $CONFIG_DIR (token cache and config)?"; then
		rm -rf "$CONFIG_DIR"
		say "Removed $CONFIG_DIR"
	else
		say "Kept $CONFIG_DIR"
	fi
fi

say ""
say "Done."
