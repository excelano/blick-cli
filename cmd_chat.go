package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// runChat handles `blick chat <contact...> [--topic "..."]` from the
// shell. One recipient → 1:1 chat (idempotent — opens the existing
// thread if one already exists). Two or more → a fresh group chat,
// with an optional topic shown as the chat title. Body is read from
// stdin until a "." sentinel line (or EOF).
func runChat(client *GraphClient, args []string) {
	positional, topic, err := parseChatArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: blick chat <contact> [more contacts...] [--topic \"...\"]")
		os.Exit(1)
	}
	if err := composeAndSendChat(client, positional, topic, shellReadBody); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// replChat is the REPL-side entry. Same flow as the shell, but body input
// comes from the REPL's stdin channel so SIGINT cancels cleanly.
func replChat(client *GraphClient, args []string) {
	positional, topic, err := parseChatArgs(args)
	if err != nil {
		fmt.Printf("  %s%v%s\n", red, err, reset)
		return
	}
	if len(positional) == 0 {
		fmt.Printf("  Usage: %schat <contact> [more contacts...] [--topic \"...\"]%s\n", cyan, reset)
		return
	}
	if err := composeAndSendChat(client, positional, topic, replReadBody); err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
	}
}

// parseChatArgs splits chat args into recipients + topic. Mirrors
// parseEmailArgs so chat and email keep the same flag shape. --topic
// and -t both consume the next arg. Positional recipients are
// comma-tolerant: `alice bob`, `alice,bob`, and `alice, bob` all parse
// as two recipients.
func parseChatArgs(args []string) ([]string, string, error) {
	var topic string
	positional := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--topic", "-t":
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("--topic requires a value")
			}
			topic = args[i+1]
			i++
		default:
			for _, part := range strings.Split(args[i], ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					positional = append(positional, part)
				}
			}
		}
	}
	return positional, topic, nil
}

// composeAndSendChat is the shared flow for shell and REPL. Resolves
// each recipient against the address book, opens (1:1) or creates
// (group) the chat, reads the body via the caller-supplied reader, and
// sends. Returns nil on cancel (empty body or Ctrl-C) so neither caller
// treats cancel as an error.
func composeAndSendChat(client *GraphClient, recipients []string, topic string, readBody func() (string, bool)) error {
	store, err := LoadContacts()
	if err != nil {
		return err
	}

	type resolved struct {
		contact *Contact
		display string
	}
	contacts := make([]resolved, 0, len(recipients))
	for _, r := range recipients {
		c, err := store.Resolve(r)
		if err != nil {
			return err
		}
		d := c.Email
		if c.Name != "" && c.Name != c.Email {
			d = fmt.Sprintf("%s <%s>", c.Name, c.Email)
		}
		contacts = append(contacts, resolved{contact: c, display: d})
	}

	displays := make([]string, len(contacts))
	for i, c := range contacts {
		displays[i] = c.display
	}
	fmt.Printf("  %sTo:%s %s\n", bold, reset, strings.Join(displays, ", "))
	// Only group chats carry a topic. For 1:1, --topic is silently
	// ignored — Graph has nowhere to put it on oneOnOne chats.
	if topic != "" && len(contacts) > 1 {
		fmt.Printf("  %sTopic:%s %s\n", bold, reset, topic)
	}

	var chatID string
	if len(contacts) == 1 {
		c := contacts[0].contact
		chatID = c.ChatID
		if chatID == "" {
			fmt.Printf("  %sLooking up chat...%s\n", dim, reset)
			userID, err := client.LookupUserID(c.Email)
			if err != nil {
				return fmt.Errorf("looking up %s: %w", c.Email, err)
			}
			chatID, err = client.EnsureOneOnOneChat(userID)
			if err != nil {
				return fmt.Errorf("creating chat: %w", err)
			}
			if stored, ok := store.Contacts[c.Key]; ok {
				stored.ChatID = chatID
				if err := store.Save(); err != nil {
					fmt.Fprintf(os.Stderr, "  %sCould not cache chat ID: %v%s\n", dim, err, reset)
				}
			}
		}
	} else {
		fmt.Printf("  %sLooking up users and creating group chat...%s\n", dim, reset)
		userIDs := make([]string, 0, len(contacts))
		for _, c := range contacts {
			userID, err := client.LookupUserID(c.contact.Email)
			if err != nil {
				return fmt.Errorf("looking up %s: %w", c.contact.Email, err)
			}
			userIDs = append(userIDs, userID)
		}
		chatID, err = client.CreateGroupChat(userIDs, topic)
		if err != nil {
			return fmt.Errorf("creating group chat: %w", err)
		}
	}

	fmt.Printf("  %s(end with `.` on a line by itself, or Ctrl-C to cancel)%s\n", dim, reset)
	text, ok := readBody()
	if !ok {
		fmt.Println("  (cancelled)")
		return nil
	}
	text = strings.TrimRight(text, " \t\n")
	if text == "" {
		fmt.Println("  (empty — not sent)")
		return nil
	}

	if err := client.SendChatMessage(chatID, text); err != nil {
		return err
	}
	fmt.Printf("  %sSent.%s\n", green, reset)
	return nil
}

// shellReadBody reads body lines from os.Stdin until a "." sentinel.
// Ctrl-D / EOF is cancel — anything typed before it is discarded so the
// shell path matches the REPL's Ctrl-C semantics and the documented `.`
// submit protocol.
func shellReadBody() (string, bool) {
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for {
		fmt.Printf("  %s> %s", cyan, reset)
		if !scanner.Scan() {
			return "", false
		}
		line := scanner.Text()
		if line == "." {
			return strings.Join(lines, "\n"), true
		}
		lines = append(lines, line)
	}
}

// replReadBody reads body lines from the REPL's stdinLines channel so
// SIGINT routes to "cancel this reply" rather than killing the process.
// Returns ("", false) when SIGINT fires or the channel closes mid-input.
func replReadBody() (string, bool) {
	var lines []string
	for {
		fmt.Printf("  %s> %s", cyan, reset)
		select {
		case line, ok := <-stdinLines:
			if !ok {
				return "", false
			}
			if line == "." {
				return strings.Join(lines, "\n"), true
			}
			lines = append(lines, line)
		case <-sigCh:
			fmt.Println()
			return "", false
		}
	}
}
