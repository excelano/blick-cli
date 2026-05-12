package main

import (
	"bufio"
	"fmt"
	"os"
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		debug = true
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("Usage: checkin")
		fmt.Println()
		fmt.Println("Check unread Outlook emails, Teams chats, and your next meeting.")
		fmt.Println()
		fmt.Println("Config: ~/.config/checkin/config.json")
		fmt.Println("  {\"client_id\": \"...\", \"tenant_id\": \"...\"}")
		fmt.Println()
		fmt.Println("See README.md for Azure AD app registration.")
		os.Exit(0)
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

	items := fetchAndDisplay(client, cfg.EnableTeams)
	if items == nil {
		return
	}

	renderHelp()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%scheckin>%s ", bold, reset)
		if !scanner.Scan() {
			fmt.Println()
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch {
		case input == "q" || input == "quit":
			return

		case input == "r":
			items = fetchAndDisplay(client, cfg.EnableTeams)
			if items == nil {
				return
			}
			renderHelp()

		case input == "x":
			markAllRead(client, items)
			return

		case strings.HasPrefix(input, "d"):
			n, err := strconv.Atoi(input[1:])
			if err != nil || n < 1 || n > len(items) {
				fmt.Printf("  Invalid item: %s\n", input)
				continue
			}
			markRead(client, items[n-1])

		case strings.HasPrefix(input, "r"):
			n, err := strconv.Atoi(input[1:])
			if err != nil || n < 1 || n > len(items) {
				fmt.Printf("  Invalid item: %s\n", input)
				continue
			}
			replyTo(client, items[n-1], scanner)

		default:
			n, err := strconv.Atoi(input)
			if err != nil || n < 1 || n > len(items) {
				fmt.Printf("  Unknown command. Type %sq%s to quit.\n", cyan, reset)
				continue
			}
			viewItem(client, items[n-1])
		}
	}
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
			chats, chatErr = client.PendingChats()
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
		// No Graph API to mark Teams chats as read
		fmt.Printf("  %sNote: Teams chats can't be marked as read via API. Open in Teams to clear.%s\n", dim, reset)
	}
}

func markAllRead(client *GraphClient, items []Item) {
	hasChats := false
	for _, item := range items {
		if item.Kind == "email" {
			if err := client.MarkEmailRead(item.Email.ID); err != nil {
				fmt.Printf("  %sError marking %q: %v%s\n", red, item.Email.Subject, err, reset)
			}
		} else {
			hasChats = true
		}
	}
	fmt.Printf("  %sAll emails marked as read.%s\n", green, reset)
	if hasChats {
		fmt.Printf("  %sNote: Teams chats can't be marked as read via API.%s\n", dim, reset)
	}
}

func replyTo(client *GraphClient, item Item, scanner *bufio.Scanner) {
	switch item.Kind {
	case "email":
		fmt.Printf("  Reply to %s — %q:\n", item.Email.From, truncate(item.Email.Subject, 40))
	case "chat":
		fmt.Printf("  Reply in %s:\n", item.Chat.Topic)
	}

	fmt.Printf("  %s> %s", cyan, reset)
	if !scanner.Scan() {
		return
	}
	text := strings.TrimSpace(scanner.Text())
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
