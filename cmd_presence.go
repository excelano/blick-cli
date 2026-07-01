package main

import (
	"fmt"
	"os"
	"strings"
)

// presenceExpiration bounds how long a manually set presence holds before
// Graph reverts it — long enough to cover a workday, short enough that a
// forgotten "dnd" clears itself by evening. ISO 8601 duration.
const presenceExpiration = "PT8H"

// presenceOption maps a blick presence keyword to the Graph availability +
// activity pair it writes. Order here is the order shown in usage and errors.
type presenceOption struct {
	key          string
	availability string
	activity     string
}

var presenceOptions = []presenceOption{
	{"available", "Available", "Available"},
	{"busy", "Busy", "Busy"},
	{"dnd", "DoNotDisturb", "DoNotDisturb"},
	{"brb", "BeRightBack", "BeRightBack"},
	{"away", "Away", "Away"},
	{"offline", "Offline", "OffWork"},
}

func lookupPresence(key string) (presenceOption, bool) {
	for _, o := range presenceOptions {
		if o.key == key {
			return o, true
		}
	}
	return presenceOption{}, false
}

// presenceKeys is the "available | busy | ..." list for usage messages.
func presenceKeys() string {
	keys := make([]string, len(presenceOptions))
	for i, o := range presenceOptions {
		keys[i] = o.key
	}
	return strings.Join(keys, " | ")
}

// presenceColor maps an availability to the dashboard status-dot color. The
// text label always carries the meaning; the color is a secondary cue (and
// non-color-safe, so it never stands alone).
func presenceColor(availability string) string {
	switch availability {
	case "Available", "AvailableIdle":
		return green
	case "Busy", "BusyIdle", "DoNotDisturb":
		return red
	case "Away", "BeRightBack":
		return yellow
	default: // Offline, PresenceUnknown, and anything unexpected
		return dim
	}
}

// presenceLabel renders a read presence for display, appending the activity
// only when it adds information beyond the availability.
func presenceLabel(p presenceState) string {
	if p.Activity != "" && p.Activity != p.Availability {
		return fmt.Sprintf("%s (%s)", p.Availability, p.Activity)
	}
	if p.Availability == "" {
		return "unknown"
	}
	return p.Availability
}

// presenceSessionExpiration is how long the registered presence session lasts.
// Preferred presence only shows while a session exists, so this bounds how long
// a manually set status holds with no Teams client running. Re-run the command
// (or launch blick) to refresh it. Kept within setPresence's accepted range;
// verify the maximum against a live tenant.
const presenceSessionExpiration = "PT4H"

// presenceScopeHint tells the user how to recover from a missing-scope 403 —
// which an existing token predating the always-on Presence.ReadWrite scope
// will hit, since OAuth refresh doesn't re-request scopes.
const presenceScopeHint = "This usually means the Presence.ReadWrite permission isn't on your token yet. Run `blick logout`, then re-run to grant it."

// setPresenceState applies a manual presence: the preferred override (durable,
// the primary set) plus — for the online states — a session so the status
// shows even with no Teams client signed in. Offline is preferred-only, since
// appearing offline means having no active session. A session failure is
// non-fatal: preferred presence still applies whenever a Teams client is
// signed in, so it comes back as a warning, not an error.
func setPresenceState(client *GraphClient, opt presenceOption) (warning string, err error) {
	if err := client.setUserPreferredPresence(opt.availability, opt.activity, presenceExpiration); err != nil {
		return "", err
	}
	if opt.availability != "Offline" {
		if serr := client.setPresenceSession(client.clientID, opt.availability, opt.activity, presenceSessionExpiration); serr != nil {
			return fmt.Sprintf("status set, but registering a presence session failed (%v) — it may not show unless a Teams client is signed in", serr), nil
		}
	}
	return "", nil
}

// isPresenceScopeError reports whether err is a Graph 403, i.e. the permission
// isn't granted, so the caller can point the user at re-authentication.
func isPresenceScopeError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "returned 403")
}

// runPresence handles the one-shot `blick presence [state]`. With no state it
// reports the current presence (the no-arg-reports-state convention); with a
// state it sets it. Bad input exits non-zero.
func runPresence(client *GraphClient, args []string) {
	if len(args) == 0 {
		p, err := client.getPresence()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Presence: %s\n", presenceLabel(p))
		return
	}
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Usage: blick presence [%s]\n", presenceKeys())
		os.Exit(1)
	}
	opt, ok := lookupPresence(strings.ToLower(args[0]))
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown presence %q. Choose one of: %s\n", args[0], presenceKeys())
		os.Exit(1)
	}
	warn, err := setPresenceState(client, opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if isPresenceScopeError(err) {
			fmt.Fprintln(os.Stderr, presenceScopeHint)
		}
		os.Exit(1)
	}
	if warn != "" {
		fmt.Fprintf(os.Stderr, "Note: %s\n", warn)
	}
	fmt.Printf("Presence set to %s.\n", opt.key)
}

// replPresence is the REPL-side `presence`/`p [state]`, matching runPresence
// but printing in the dashboard style instead of exiting.
func replPresence(client *GraphClient, args []string) {
	if len(args) == 0 {
		p, err := client.getPresence()
		if err != nil {
			fmt.Printf("  %sError: %v%s\n", red, err, reset)
			return
		}
		fmt.Printf("  %sPresence:%s %s\n", bold, reset, presenceLabel(p))
		return
	}
	if len(args) > 1 {
		fmt.Printf("  Usage: %spresence [%s]%s\n", cyan, presenceKeys(), reset)
		return
	}
	opt, ok := lookupPresence(strings.ToLower(args[0]))
	if !ok {
		fmt.Printf("  %sUnknown presence %q. Choose one of: %s%s\n", red, args[0], presenceKeys(), reset)
		return
	}
	warn, err := setPresenceState(client, opt)
	if err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
		if isPresenceScopeError(err) {
			fmt.Printf("  %s%s%s\n", dim, presenceScopeHint, reset)
		}
		return
	}
	if warn != "" {
		fmt.Printf("  %s(%s)%s\n", dim, warn, reset)
	}
	fmt.Printf("  %sPresence set to %s.%s\n", green, opt.key, reset)
}
