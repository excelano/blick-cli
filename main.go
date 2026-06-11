package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
)

type Item struct {
	Kind    string // "email" or "chat"
	Email   *Email
	Chat    *ChatMessage
}

var debug bool

// Populated at build time via -ldflags by goreleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Interactive REPL plumbing. One goroutine owns the stdin scanner and feeds
// stdinLines; sigCh receives SIGINT so we can route Ctrl-C to "cancel the
// reply" instead of letting it kill the process. Initialized in main() before
// the REPL starts.
var (
	stdinLines chan string
	sigCh      chan os.Signal
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		debug = true
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Printf("blick %s (%s, %s)\n", version, commit, date)
		os.Exit(0)
	}
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("Usage: blick [command]")
		fmt.Println()
		fmt.Println("Check unread Outlook emails, Teams chats, and your next meeting.")
		fmt.Println("With no command, opens the interactive dashboard.")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  today                          Show today's calendar and exit")
		fmt.Println("  contacts ...                   Manage the address book (list, add, remove, show, seed)")
		fmt.Println("  email <contact> [--subject]    Compose and send a message")
		fmt.Println("  chat <contact>                 Send a Teams chat message")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  -h, --help      Show this help")
		fmt.Println("  -V, --version   Show version")
		fmt.Println("      --debug     Verbose Graph request logging")
		fmt.Println()
		fmt.Println("Config:   ~/.config/blick/config.json")
		fmt.Println("          {\"client_id\": \"...\", \"tenant_id\": \"...\"}")
		fmt.Println("Contacts: ~/.config/blick/contacts.json")
		fmt.Println()
		fmt.Println("See README.md for Azure AD app registration.")
		os.Exit(0)
	}

	// Local-only contacts commands (everything except `seed`) don't need
	// Graph or auth — handle them before the device-code flow so the user
	// can curate the address book offline or pre-auth.
	if len(os.Args) > 1 && os.Args[1] == "contacts" {
		if needsGraphForContacts(os.Args[2:]) {
			// fall through to auth, dispatch happens below
		} else {
			runContacts(nil, os.Args[2:])
			return
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	tok, err := authenticate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Save refreshed token
	_ = saveCachedToken(tok)

	client, err := NewGraphClient(cfg, tok)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	maybeHeartbeatPresence(client, cfg)

	if len(os.Args) > 1 && os.Args[1] == "today" {
		showToday(client)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "contacts" {
		runContacts(client, os.Args[2:])
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "email" {
		runEmail(client, os.Args[2:])
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "chat" {
		if !cfg.EnableTeams {
			fmt.Fprintln(os.Stderr, "Teams is disabled in your config (\"enable_teams\": false). Set it to true and re-authenticate to use chat.")
			os.Exit(1)
		}
		runChat(client, os.Args[2:])
		return
	}

	items := fetchAndDisplay(client, cfg.EnableTeams)
	if items == nil {
		return
	}

	renderHelp()

	stdinLines = make(chan string)
	sigCh = make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			stdinLines <- scanner.Text()
		}
		close(stdinLines)
	}()

	for {
		fmt.Printf("%sblick>%s ", bold, reset)
		var input string
		select {
		case line, ok := <-stdinLines:
			if !ok {
				fmt.Println()
				return
			}
			input = strings.TrimSpace(line)
		case <-sigCh:
			fmt.Println()
			return
		}

		if input == "" {
			continue
		}

		// String-argument commands sit outside parseCommand's (cmd, int)
		// shape — peel them off before the int-arg dispatcher.
		fields := strings.Fields(input)
		if len(fields) > 0 && fields[0] == "email" {
			replEmail(client, fields[1:])
			continue
		}
		if len(fields) > 0 && fields[0] == "chat" {
			if !cfg.EnableTeams {
				fmt.Printf("  %sTeams is disabled in your config.%s\n", dim, reset)
				continue
			}
			replChat(client, fields[1:])
			continue
		}

		cmd, n := parseCommand(input)

		switch cmd {
		case "quit":
			return

		case "refresh":
			items = fetchAndDisplay(client, cfg.EnableTeams)
			if items == nil {
				return
			}
			renderHelp()

		case "exit":
			markAllRead(client, items)
			return

		case "help":
			renderFullHelp()

		case "today":
			showToday(client)

		case "view":
			if n < 1 || n > len(items) {
				fmt.Printf("  Invalid item: %s\n", input)
				continue
			}
			viewItem(client, items[n-1])

		case "reply":
			if n < 1 || n > len(items) {
				fmt.Printf("  Invalid item: %s\n", input)
				continue
			}
			replyTo(client, items[n-1])

		case "done":
			if n < 1 || n > len(items) {
				fmt.Printf("  Invalid item: %s\n", input)
				continue
			}
			markRead(client, items[n-1])

		default:
			fmt.Printf("  Unknown command. Type %sH%s for help.\n", cyan, reset)
		}
	}
}

// parseCommand normalizes REPL input into a (command, item-number) pair.
// Item-number is -1 when the command doesn't take one. Returns
// ("unknown", -1) when the input doesn't map to any command.
//
// Short forms map to their long-form equivalents so the dispatcher only
// has to handle one name per action:
//   r5, r 5, reply 5  -> ("reply", 5)
//   d3, d 3, done 3   -> ("done",  3)
//   7, view 7         -> ("view",  7)
//   r, refresh        -> ("refresh", -1)
//   x, exit           -> ("exit",   -1)
//   q, quit           -> ("quit",   -1)
//   H, help           -> ("help",   -1)
//   t, today          -> ("today",  -1)
func parseCommand(input string) (string, int) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return "unknown", -1
	}
	first := fields[0]
	rest := fields[1:]

	if num, err := strconv.Atoi(first); err == nil {
		return "view", num
	}

	if len(first) > 1 {
		if num, err := strconv.Atoi(first[1:]); err == nil {
			switch first[0] {
			case 'r':
				return "reply", num
			case 'd':
				return "done", num
			}
		}
	}

	switch first {
	case "q":
		return "quit", -1
	case "x":
		return "exit", -1
	case "H":
		return "help", -1
	case "t":
		return "today", -1
	case "r":
		if len(rest) == 0 {
			return "refresh", -1
		}
		if num, err := strconv.Atoi(rest[0]); err == nil {
			return "reply", num
		}
		return "unknown", -1
	case "d":
		if len(rest) == 0 {
			return "unknown", -1
		}
		if num, err := strconv.Atoi(rest[0]); err == nil {
			return "done", num
		}
		return "unknown", -1
	}

	switch first {
	case "view", "reply", "done":
		if len(rest) == 0 {
			return "unknown", -1
		}
		num, err := strconv.Atoi(rest[0])
		if err != nil {
			return "unknown", -1
		}
		return first, num
	case "today", "refresh", "exit", "help", "quit":
		return first, -1
	}

	return "unknown", -1
}

func showToday(client *GraphClient) {
	events, err := client.TodaysMeetings()
	if err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
		return
	}
	renderToday(events)
}

func fetchAndDisplay(client *GraphClient, enableTeams bool) []Item {
	var (
		meeting    *Meeting
		emails     []Email
		chats      []ChatMessage
		meetingErr error
		emailErr   error
		chatErr    error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		meeting, meetingErr = client.NextMeeting()
		if meetingErr != nil {
			meeting = nil
		}
	}()

	go func() {
		defer wg.Done()
		emails, emailErr = client.UnreadEmails()
	}()

	if enableTeams {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chats, chatErr = client.UnreadChats()
		}()
	}

	wg.Wait()

	renderDashboard(meeting, emails, emailErr, chats, chatErr, enableTeams)

	var items []Item
	for i := range emails {
		items = append(items, Item{Kind: "email", Email: &emails[i]})
	}
	for i := range chats {
		items = append(items, Item{Kind: "chat", Chat: &chats[i]})
	}

	if len(items) == 0 {
		fmt.Printf("  %sNothing to do!%s\n\n", dim, reset)
		return nil
	}

	return items
}

func markRead(client *GraphClient, item Item) {
	switch item.Kind {
	case "email":
		if err := client.MarkEmailRead(item.Email.ID); err != nil {
			fmt.Printf("  %sError: %v%s\n", red, err, reset)
			return
		}
		fmt.Printf("  Marked as read: %s\n", truncate(item.Email.Subject, 50))
	case "chat":
		if err := client.MarkChatRead(item.Chat.ChatID); err != nil {
			fmt.Printf("  %sError: %v%s\n", red, err, reset)
			return
		}
		fmt.Printf("  Marked as read: %s\n", truncate(item.Chat.Topic, 50))
	}
}

func markAllRead(client *GraphClient, items []Item) {
	for _, item := range items {
		switch item.Kind {
		case "email":
			if err := client.MarkEmailRead(item.Email.ID); err != nil {
				fmt.Printf("  %sError marking %q: %v%s\n", red, item.Email.Subject, err, reset)
			}
		case "chat":
			if err := client.MarkChatRead(item.Chat.ChatID); err != nil {
				fmt.Printf("  %sError marking %q: %v%s\n", red, item.Chat.Topic, err, reset)
			}
		}
	}
	fmt.Printf("  %sAll marked as read.%s\n", green, reset)
}

func replyTo(client *GraphClient, item Item) {
	switch item.Kind {
	case "email":
		fmt.Printf("  Reply to %s — %q:\n", item.Email.From, truncate(item.Email.Subject, 40))
	case "chat":
		fmt.Printf("  Reply in %s:\n", item.Chat.Topic)
	}
	fmt.Printf("  %s(end with `.` on a line by itself, or Ctrl-C to cancel)%s\n", dim, reset)

	var lines []string
	done := false
	for !done {
		fmt.Printf("  %s> %s", cyan, reset)
		select {
		case line, ok := <-stdinLines:
			if !ok {
				return
			}
			if line == "." {
				done = true
			} else {
				lines = append(lines, line)
			}
		case <-sigCh:
			fmt.Println()
			fmt.Println("  (cancelled)")
			return
		}
	}
	text := strings.TrimRight(strings.Join(lines, "\n"), " \t\n")
	if text == "" {
		fmt.Println("  (cancelled)")
		return
	}

	switch item.Kind {
	case "email":
		if err := client.ReplyToEmail(item.Email.ID, text); err != nil {
			fmt.Printf("  %sError: %v%s\n", red, err, reset)
			return
		}
		_ = client.MarkEmailRead(item.Email.ID)
		fmt.Printf("  %sSent & marked as read.%s\n", green, reset)
	case "chat":
		if err := client.SendChatMessage(item.Chat.ChatID, text); err != nil {
			fmt.Printf("  %sError: %v%s\n", red, err, reset)
			return
		}
		fmt.Printf("  %sSent.%s\n", green, reset)
	}
}

func viewItem(client *GraphClient, item Item) {
	switch item.Kind {
	case "email":
		fmt.Printf("\n  %sFrom:%s %s\n", bold, reset, item.Email.From)
		fmt.Printf("  %sSubject:%s %s\n", bold, reset, item.Email.Subject)
		fmt.Printf("  %sReceived:%s %s\n\n", bold, reset, item.Email.Received.Local().Format("Mon Jan 2 3:04 PM"))

		body, err := client.GetEmailBody(item.Email.ID)
		if err != nil {
			fmt.Printf("  %sError loading body: %v%s\n", red, err, reset)
			return
		}

		// Indent body for readability
		for _, line := range strings.Split(body, "\n") {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println()

	case "chat":
		fmt.Printf("\n  %sChat:%s %s\n\n", bold, reset, item.Chat.Topic)

		messages, err := client.GetChatMessages(item.Chat.ChatID, 5)
		if err != nil {
			fmt.Printf("  %sError loading messages: %v%s\n", red, err, reset)
			return
		}

		// Show messages oldest-first
		for i := len(messages) - 1; i >= 0; i-- {
			m := messages[i]
			fmt.Printf("  %s%s%s %s(%s)%s\n", bold, m.From, reset, dim, relativeTime(m.Sent), reset)
			for _, line := range strings.Split(m.Preview, "\n") {
				fmt.Printf("    %s\n", line)
			}
			fmt.Println()
		}
	}
}
