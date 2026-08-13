package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// inboxMaxDaysBack caps how far back the window reaches so a fat-fingered
// `inbox 9999` doesn't ask Graph for the whole mailbox — the view truncates
// at inboxEmailTop anyway.
const inboxMaxDaysBack = 30

// windowStart returns local midnight `daysBack` days before today, always
// inclusive of today: daysBack=0 is midnight this morning, daysBack=2 is
// midnight two mornings ago. The result stays in the local zone; callers
// convert to UTC for Graph.
func windowStart(daysBack int) time.Time {
	now := time.Now().Local()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return midnight.AddDate(0, 0, -daysBack)
}

// parseInboxArgs reads the optional day count — how many days back beyond
// today to include. Bare `inbox` is today only (0 back); `inbox 1` reaches
// yesterday, `inbox N` reaches N days back. Rejects non-numbers, N < 0, and
// N past the cap.
func parseInboxArgs(args []string) (int, error) {
	if len(args) == 0 {
		return 0, nil
	}
	if len(args) > 1 {
		return 0, fmt.Errorf("inbox takes at most one argument: a number of days back")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("not a number of days: %q", args[0])
	}
	if n < 0 {
		return 0, fmt.Errorf("days back cannot be negative")
	}
	if n > inboxMaxDaysBack {
		return 0, fmt.Errorf("days back must be %d or fewer", inboxMaxDaysBack)
	}
	return n, nil
}

// replInbox handles the REPL `inbox`/`i` verb: render the history view and
// return the item list so the numbered verbs (view/reply/done/attach) target
// it, exactly like the dashboard. On a bad argument it prints the reason and
// returns prev unchanged so the current numbering holds.
func replInbox(client *GraphClient, enableTeams bool, args []string, prev []Item) []Item {
	daysBack, err := parseInboxArgs(args)
	if err != nil {
		fmt.Printf("  %s%v%s\n", red, err, reset)
		return prev
	}
	return fetchInbox(client, enableTeams, daysBack)
}

// showInbox handles the one-shot `blick inbox [N]` command: render the view
// and exit. No verbs follow, so the item list is discarded. A bad argument
// prints to stderr and exits non-zero.
func showInbox(client *GraphClient, enableTeams bool, args []string) {
	daysBack, err := parseInboxArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fetchInbox(client, enableTeams, daysBack)
}

// fetchInbox pulls the window's chats and emails in parallel, renders the
// inbox view, and returns the item list in dashboard order (chats first).
func fetchInbox(client *GraphClient, enableTeams bool, daysBack int) []Item {
	since := windowStart(daysBack)

	var (
		emails   []Email
		chats    []ChatMessage
		emailErr error
		chatErr  error
	)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		emails, emailErr = client.EmailsSince(since)
	}()

	if enableTeams {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chats, chatErr = client.ChatsSince(since)
		}()
	}

	wg.Wait()

	emailsTruncated := emailErr == nil && len(emails) >= inboxEmailTop
	chatsTruncated := chatErr == nil && len(chats) >= chatPageSize
	renderInbox(daysBack, emails, emailErr, chats, chatErr, enableTeams, emailsTruncated, chatsTruncated)

	// Chats numbered ahead of emails, matching renderInbox's order and the
	// dashboard's convention.
	var items []Item
	for i := range chats {
		items = append(items, Item{Kind: "chat", Chat: &chats[i]})
	}
	for i := range emails {
		items = append(items, Item{Kind: "email", Email: &emails[i]})
	}
	return items
}
