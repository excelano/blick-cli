package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// runChat handles `blick chat <contact>` from the shell. One-shot: takes
// the recipient, reads the message body from stdin until a "." sentinel
// line (or EOF), and sends.
func runChat(client *GraphClient, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: blick chat <contact>")
		os.Exit(1)
	}
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "Only one recipient is supported (1:1 chats only — group chats land in a later release).")
		os.Exit(1)
	}
	if err := composeAndSendChat(client, args[0], shellReadBody); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// replChat is the REPL-side entry. Same flow as the shell, but body input
// comes from the REPL's stdin channel so SIGINT cancels cleanly.
func replChat(client *GraphClient, args []string) {
	if len(args) == 0 {
		fmt.Printf("  Usage: %schat <contact>%s\n", cyan, reset)
		return
	}
	if len(args) > 1 {
		fmt.Printf("  Only one recipient is supported.\n")
		return
	}
	if err := composeAndSendChat(client, args[0], replReadBody); err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
	}
}

// composeAndSendChat is the shared flow. Resolves the recipient against
// the address book, ensures a 1:1 chat ID (cached when the contact is in
// the store), reads the body via the caller-supplied reader, sends.
// Returns nil on cancel (empty body) so neither caller treats cancel as
// an error.
func composeAndSendChat(client *GraphClient, who string, readBody func() (string, bool)) error {
	store, err := LoadContacts()
	if err != nil {
		return err
	}

	c, err := store.Resolve(who)
	if err != nil {
		return err
	}

	display := c.Email
	if c.Name != "" && c.Name != c.Email {
		display = fmt.Sprintf("%s <%s>", c.Name, c.Email)
	}
	fmt.Printf("  %sTo:%s %s\n", bold, reset, display)

	chatID := c.ChatID
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
