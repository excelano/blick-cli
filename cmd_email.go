package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runEmail handles `blick email <contact...> [--subject "..."]` from the
// shell. Reads Subject + body inline ed-style — same `.`-sentinel as
// `reply N` and `chat`. --subject pre-fills the subject so quick
// one-liners can skip that prompt.
func runEmail(client *GraphClient, args []string) {
	positional, subject, err := parseEmailArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: blick email <contact> [more contacts...] [--subject \"...\"]")
		os.Exit(1)
	}
	if err := composeAndSend(client, positional, subject, newShellComposeReader()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// replEmail is the REPL-side entry. Accepts the same --subject/-s flag
// as the shell so muscle memory from one mode doesn't break the other.
func replEmail(client *GraphClient, args []string) {
	positional, subject, err := parseEmailArgs(args)
	if err != nil {
		fmt.Printf("  %s%v%s\n", red, err, reset)
		return
	}
	if len(positional) == 0 {
		fmt.Printf("  Usage: %semail <contact> [more contacts...] [--subject \"...\"]%s\n", cyan, reset)
		return
	}
	if err := composeAndSend(client, positional, subject, replComposeReader{}); err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
	}
}

// parseEmailArgs splits compose args into recipients + subject. Shared
// between shell and REPL entry points so flag handling stays consistent.
// --subject and -s both consume the next arg as the subject value.
func parseEmailArgs(args []string) ([]string, string, error) {
	var subject string
	positional := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--subject", "-s":
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("--subject requires a value")
			}
			subject = args[i+1]
			i++
		default:
			positional = append(positional, args[i])
		}
	}
	return positional, subject, nil
}

// composeReader abstracts the subject + body input for shell vs. REPL
// callers. Shell shares a single bufio.Scanner across both reads so the
// two reads don't fight over terminal line buffering; REPL routes
// through the existing stdinLines + sigCh plumbing so Ctrl-C cancels
// cleanly instead of killing the process.
type composeReader interface {
	readLine(prompt string) (string, bool) // ok=false means cancel/EOF
	readBody() (string, bool)              // ok=false means cancel
}

// composeAndSend is the shared compose flow. Resolves recipients,
// gathers Subject and body via the injected reader, and sends.
// Unknown handles fail before any prompt opens so the user fixes a typo
// without losing a draft they haven't typed yet.
func composeAndSend(client *GraphClient, recipients []string, subject string, reader composeReader) error {
	store, err := LoadContacts()
	if err != nil {
		return err
	}

	addrs := make([]string, 0, len(recipients))
	display := make([]string, 0, len(recipients))
	for _, r := range recipients {
		c, err := store.Resolve(r)
		if err != nil {
			return err
		}
		addrs = append(addrs, c.Email)
		if c.Name != "" && c.Name != c.Email {
			display = append(display, fmt.Sprintf("%s <%s>", c.Name, c.Email))
		} else {
			display = append(display, c.Email)
		}
	}

	fmt.Printf("  %sTo:%s %s\n", bold, reset, strings.Join(display, ", "))

	if subject == "" {
		s, ok := reader.readLine(fmt.Sprintf("  %sSubject:%s ", bold, reset))
		if !ok {
			fmt.Println("  (cancelled)")
			return nil
		}
		subject = strings.TrimSpace(s)
	} else {
		fmt.Printf("  %sSubject:%s %s\n", bold, reset, subject)
	}

	fmt.Printf("  %s(end with `.` on a line by itself, or Ctrl-C to cancel)%s\n", dim, reset)
	body, ok := reader.readBody()
	if !ok {
		fmt.Println("  (cancelled)")
		return nil
	}
	body = strings.TrimRight(body, " \t\n")
	if body == "" {
		fmt.Println("  (empty body — not sent)")
		return nil
	}

	if err := client.SendMail(addrs, subject, body); err != nil {
		path, saveErr := saveDraftCopy(addrs, subject, body)
		if saveErr == nil {
			fmt.Fprintf(os.Stderr, "  %sDraft saved to %s%s\n", dim, path, reset)
		}
		return err
	}
	fmt.Printf("  %sSent.%s\n", green, reset)
	return nil
}

// shellComposeReader reuses one bufio.Scanner across subject + body
// reads. Two scanners on os.Stdin can race on terminal line buffers
// (the first reads ahead, the second sees nothing) — sharing avoids
// that.
type shellComposeReader struct {
	scanner *bufio.Scanner
}

func newShellComposeReader() *shellComposeReader {
	return &shellComposeReader{scanner: bufio.NewScanner(os.Stdin)}
}

func (r *shellComposeReader) readLine(prompt string) (string, bool) {
	fmt.Print(prompt)
	if !r.scanner.Scan() {
		return "", false
	}
	return r.scanner.Text(), true
}

func (r *shellComposeReader) readBody() (string, bool) {
	var lines []string
	for {
		fmt.Printf("  %s> %s", cyan, reset)
		if !r.scanner.Scan() {
			// Ctrl-D / EOF mid-body is cancel, not submit. Anything
			// the user typed is discarded — matches the REPL's
			// Ctrl-C path and the documented `.` submit protocol.
			return "", false
		}
		line := r.scanner.Text()
		if line == "." {
			return strings.Join(lines, "\n"), true
		}
		lines = append(lines, line)
	}
}

// replComposeReader routes subject + body through the REPL's shared
// stdinLines + sigCh so Ctrl-C cancels the compose rather than killing
// the process.
type replComposeReader struct{}

func (replComposeReader) readLine(prompt string) (string, bool) {
	fmt.Print(prompt)
	select {
	case line, ok := <-stdinLines:
		if !ok {
			return "", false
		}
		return line, true
	case <-sigCh:
		fmt.Println()
		return "", false
	}
}

func (replComposeReader) readBody() (string, bool) {
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

// saveDraftCopy writes the unsent draft to ~/.config/blick/drafts/ with
// a timestamped filename so a transient Graph failure doesn't lose
// work. Returns the path written.
func saveDraftCopy(to []string, subject, body string) (string, error) {
	dir := filepath.Join(configDir(), "drafts")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	name := fmt.Sprintf("%s.eml", stamp)
	path := filepath.Join(dir, name)
	contents := fmt.Sprintf("To: %s\nSubject: %s\n\n%s\n", strings.Join(to, ", "), subject, body)
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		return "", err
	}
	return path, nil
}
