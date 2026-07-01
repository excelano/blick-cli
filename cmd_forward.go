package main

import (
	"fmt"
	"strconv"
	"strings"
)

// replForward handles the REPL `forward N [contact...]` verb: forward the Nth
// item (email only) to new recipients with an optional note. Recipients may be
// passed inline after N or entered at a prompt. REPL-only, like reply and
// attach — it needs the dashboard's item numbering.
//
// Graph's /forward builds the "Fwd:" subject and quotes the original body
// server-side, so the flow only collects recipients and an optional comment.
// The original is left unread: forwarding for input doesn't mean you're done
// triaging it — mark it with `done N` when you are.
func replForward(client *GraphClient, items []Item, args []string) {
	if len(args) == 0 {
		fmt.Printf("  Usage: %sforward N [contact...]%s — forward the Nth email\n", cyan, reset)
		return
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Printf("  %sNot an item number: %q%s\n", red, args[0], reset)
		return
	}
	if n < 1 || n > len(items) {
		fmt.Printf("  %sInvalid item: %d%s\n", red, n, reset)
		return
	}
	item := items[n-1]
	if item.Kind != "email" {
		fmt.Printf("  %sItem %d is a chat — forwarding is for emails only.%s\n", red, n, reset)
		return
	}

	store, err := LoadContacts()
	if err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
		return
	}

	// Recipients: inline after N, else prompt for them.
	handles := splitRecipients(args[1:])
	if len(handles) == 0 {
		line, ok := replComposeReader{}.readLine(fmt.Sprintf("  %sForward to:%s ", bold, reset))
		if !ok {
			fmt.Println("  (cancelled)")
			return
		}
		handles = splitRecipients(strings.Fields(line))
	}
	if len(handles) == 0 {
		fmt.Println("  (no recipients — not forwarded)")
		return
	}

	addrs, display, err := resolveRecipients(store, handles)
	if err != nil {
		fmt.Printf("  %s%v%s\n", red, err, reset)
		return
	}

	fmt.Printf("  %sForwarding:%s %s — %q\n", bold, reset, item.Email.From, truncate(item.Email.Subject, 40))
	fmt.Printf("  %sTo:%s %s\n", bold, reset, strings.Join(display, ", "))
	fmt.Printf("  %s(optional note above the forwarded message; `.` alone forwards as-is, Ctrl-C cancels)%s\n", dim, reset)

	enterBodyMode()
	comment, ok := readBodyDraft()
	exitBodyMode()
	if !ok {
		fmt.Println("  (cancelled)")
		return
	}
	comment = strings.TrimRight(comment, " \t\n")

	if err := client.ForwardEmail(item.Email.ID, comment, addrs); err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
		return
	}
	fmt.Printf("  %sForwarded.%s\n", green, reset)
}
