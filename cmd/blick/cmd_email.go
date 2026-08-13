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
	ca, err := parseEmailArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(ca.Recipients) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: blick email <contact> [more contacts...] [--cc ...] [--bcc ...] [--subject \"...\"] [--attach file]")
		os.Exit(1)
	}
	if err := composeAndSend(client, ca, newShellComposeReader()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// replEmail is the REPL-side entry. Accepts the same --subject/-s flag
// as the shell so muscle memory from one mode doesn't break the other.
func replEmail(client *GraphClient, args []string) {
	ca, err := parseEmailArgs(args)
	if err != nil {
		fmt.Printf("  %s%v%s\n", red, err, reset)
		return
	}
	if len(ca.Recipients) == 0 {
		fmt.Printf("  Usage: %semail <contact> [more contacts...] [--cc ...] [--bcc ...] [--subject \"...\"] [--attach file]%s\n", cyan, reset)
		return
	}
	if err := composeAndSend(client, ca, replComposeReader{}); err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
	}
}

// composeArgs is the parsed form of an `email` command line: the To
// recipients, optional Cc/Bcc, subject, and attachment paths. A struct keeps
// the growing set of recipient lists named rather than positional.
type composeArgs struct {
	Recipients []string
	Cc         []string
	Bcc        []string
	Subject    string
	Attach     []string
}

// parseEmailArgs splits compose args into To recipients + Cc/Bcc + subject +
// attachment paths. Shared between shell and REPL entry points so flag
// handling stays consistent. --subject/-s consumes the next arg as the
// subject; --attach/-a a file path (repeatable); --cc/--bcc a recipient list
// (repeatable, and comma-tolerant like the positional To). Positional
// recipients and --cc/--bcc values are comma-tolerant: `alice bob`,
// `alice,bob`, and `alice, bob` all parse as two recipients.
func parseEmailArgs(args []string) (composeArgs, error) {
	var ca composeArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--subject", "-s":
			if i+1 >= len(args) {
				return composeArgs{}, fmt.Errorf("--subject requires a value")
			}
			ca.Subject = args[i+1]
			i++
		case "--cc":
			if i+1 >= len(args) {
				return composeArgs{}, fmt.Errorf("--cc requires a value")
			}
			ca.Cc = append(ca.Cc, splitRecipients(args[i+1:i+2])...)
			i++
		case "--bcc":
			if i+1 >= len(args) {
				return composeArgs{}, fmt.Errorf("--bcc requires a value")
			}
			ca.Bcc = append(ca.Bcc, splitRecipients(args[i+1:i+2])...)
			i++
		case "--attach", "-a":
			if i+1 >= len(args) {
				return composeArgs{}, fmt.Errorf("--attach requires a file path")
			}
			ca.Attach = append(ca.Attach, args[i+1])
			i++
		default:
			ca.Recipients = append(ca.Recipients, splitRecipients(args[i:i+1])...)
		}
	}
	return ca, nil
}

// splitRecipients expands raw tokens into contact handles, splitting each on
// commas and dropping blanks — so "alice bob", "alice,bob", and "alice, bob"
// all yield the same handles. Shared by email compose and forward.
func splitRecipients(tokens []string) []string {
	out := []string{}
	for _, t := range tokens {
		for _, part := range strings.Split(t, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// resolveRecipients maps contact handles to addresses through the address
// book, returning parallel slices of SMTP addresses and display strings
// ("Name <addr>", or just the address when no distinct name). Fails on the
// first unknown handle so callers abort before prompting for a body.
func resolveRecipients(store *ContactStore, handles []string) (addrs, display []string, err error) {
	addrs = make([]string, 0, len(handles))
	display = make([]string, 0, len(handles))
	for _, h := range handles {
		c, err := store.Resolve(h)
		if err != nil {
			return nil, nil, err
		}
		addrs = append(addrs, c.Email)
		if c.Name != "" && c.Name != c.Email {
			display = append(display, fmt.Sprintf("%s <%s>", c.Name, c.Email))
		} else {
			display = append(display, c.Email)
		}
	}
	return addrs, display, nil
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
func composeAndSend(client *GraphClient, ca composeArgs, reader composeReader) error {
	store, err := LoadContacts()
	if err != nil {
		return err
	}

	toAddrs, toDisplay, err := resolveRecipients(store, ca.Recipients)
	if err != nil {
		return err
	}
	ccAddrs, ccDisplay, err := resolveRecipients(store, ca.Cc)
	if err != nil {
		return err
	}
	bccAddrs, bccDisplay, err := resolveRecipients(store, ca.Bcc)
	if err != nil {
		return err
	}

	// Read attachments up front so a bad path or oversized file fails
	// before the user types a message they'd otherwise lose.
	attachments, err := readOutgoingAttachments(ca.Attach)
	if err != nil {
		return err
	}

	fmt.Printf("  %sTo:%s %s\n", bold, reset, strings.Join(toDisplay, ", "))
	if len(ccDisplay) > 0 {
		fmt.Printf("  %sCc:%s %s\n", bold, reset, strings.Join(ccDisplay, ", "))
	}
	if len(bccDisplay) > 0 {
		fmt.Printf("  %sBcc:%s %s\n", bold, reset, strings.Join(bccDisplay, ", "))
	}
	for _, a := range attachments {
		fmt.Printf("  %sAttach:%s %s %s(%s)%s\n", bold, reset, a.Name, dim, humanSize(len(a.Content)), reset)
	}

	subject := ca.Subject
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

	if err := client.SendMail(toAddrs, ccAddrs, bccAddrs, subject, body, attachments); err != nil {
		path, saveErr := saveDraftCopy(toAddrs, ccAddrs, bccAddrs, subject, body)
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

// replComposeReader routes subject + body through the shared
// readline.Instance in body-mode config: no history persistence (drafts
// shouldn't show up in `history`) and no autocompletion (Tab inserting
// a verb mid-message is a bug, not a feature). Ctrl-C / EOF return
// readline.ErrInterrupt / io.EOF which we collapse to (_, false) =
// "cancel the compose".
type replComposeReader struct{}

func (replComposeReader) readLine(prompt string) (string, bool) {
	enterBodyMode()
	defer exitBodyMode()
	rl.SetPrompt(prompt)
	line, err := rl.Readline()
	if err != nil {
		return "", false
	}
	return line, true
}

func (replComposeReader) readBody() (string, bool) {
	enterBodyMode()
	defer exitBodyMode()
	return readBodyDraft()
}

// saveDraftCopy writes the unsent draft to ~/.config/blick/drafts/ with
// a timestamped filename so a transient Graph failure doesn't lose
// work. Cc/Bcc headers are written only when non-empty. Returns the path
// written.
func saveDraftCopy(to, cc, bcc []string, subject, body string) (string, error) {
	dir := filepath.Join(configDir(), "drafts")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	name := fmt.Sprintf("%s.eml", stamp)
	path := filepath.Join(dir, name)

	var h strings.Builder
	fmt.Fprintf(&h, "To: %s\n", strings.Join(to, ", "))
	if len(cc) > 0 {
		fmt.Fprintf(&h, "Cc: %s\n", strings.Join(cc, ", "))
	}
	if len(bcc) > 0 {
		fmt.Fprintf(&h, "Bcc: %s\n", strings.Join(bcc, ", "))
	}
	fmt.Fprintf(&h, "Subject: %s\n\n%s\n", subject, body)

	if err := os.WriteFile(path, []byte(h.String()), 0600); err != nil {
		return "", err
	}
	return path, nil
}
