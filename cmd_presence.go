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
	if err := client.setUserPreferredPresence(opt.availability, opt.activity, presenceExpiration); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
	if err := client.setUserPreferredPresence(opt.availability, opt.activity, presenceExpiration); err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
		return
	}
	fmt.Printf("  %sPresence set to %s.%s\n", green, opt.key, reset)
}
