package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runEmail handles `blick email <contact...> [--subject "..."]` from the
// shell. Resolves recipients, opens $EDITOR with an RFC822-shape draft
// (Subject header + blank line + body), and sends on save.
func runEmail(client *GraphClient, args []string) {
	var subject string
	positional := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--subject", "-s":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--subject requires a value")
				os.Exit(1)
			}
			subject = args[i+1]
			i++
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: blick email <contact> [more contacts...] [--subject \"...\"]")
		os.Exit(1)
	}
	if err := composeAndSend(client, positional, subject); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// composeAndSend is the shared compose flow. The recipients slice is
// resolved against the address book; unknown handles are a hard error
// before the editor opens (so the user fixes a typo without losing the
// draft they haven't written yet).
func composeAndSend(client *GraphClient, recipients []string, subject string) error {
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

	draft, err := editDraft(subject, "")
	if err != nil {
		return err
	}

	finalSubject, body := splitDraft(draft)
	body = strings.TrimRight(body, " \t\n")
	if body == "" {
		fmt.Println("  (empty body — not sent)")
		return nil
	}

	if err := client.SendMail(addrs, finalSubject, body); err != nil {
		path, saveErr := saveDraftCopy(addrs, finalSubject, body)
		if saveErr == nil {
			fmt.Fprintf(os.Stderr, "  %sDraft saved to %s%s\n", dim, path, reset)
		}
		return err
	}
	fmt.Printf("  %sSent.%s\n", green, reset)
	return nil
}

// editDraft writes a Subject-header + body skeleton to a temp file, opens
// $EDITOR on it, and returns the post-edit contents. Falls back to vi
// then ved. Subject is pre-filled when the caller supplied --subject.
func editDraft(subject, body string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if _, err := exec.LookPath("vi"); err == nil {
			editor = "vi"
		} else if _, err := exec.LookPath("ved"); err == nil {
			editor = "ved"
		} else {
			return "", fmt.Errorf("no editor available — set $EDITOR")
		}
	}

	skeleton := fmt.Sprintf("Subject: %s\n\n%s", subject, body)
	tmp, err := os.CreateTemp("", "blick-email-*.txt")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(skeleton); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %s: %w", filepath.Base(editor), err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	_ = os.Remove(path)
	return string(data), nil
}

// splitDraft parses the post-edit buffer back into (subject, body).
// The first `Subject:` header line wins; everything after the first blank
// line (or after the header if no blank line is present) is the body.
// A buffer with no Subject header is treated as all-body, empty subject.
func splitDraft(s string) (string, string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")

	subject := ""
	bodyStart := 0
	if len(lines) > 0 && strings.HasPrefix(strings.ToLower(lines[0]), "subject:") {
		subject = strings.TrimSpace(lines[0][len("subject:"):])
		bodyStart = 1
		if bodyStart < len(lines) && strings.TrimSpace(lines[bodyStart]) == "" {
			bodyStart++
		}
	}
	body := strings.Join(lines[bodyStart:], "\n")
	return subject, body
}

// saveDraftCopy writes the unsent draft to ~/.config/blick/drafts/ with
// the recipient list and timestamp in the filename, so a transient Graph
// failure doesn't lose work. Returns the path written.
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

// replEmail is the REPL-side entry: takes an already-tokenized argument
// list (one or more recipient handles) and runs the compose flow. No
// --subject parsing here — REPL users get the Subject header inline in
// the editor buffer.
func replEmail(client *GraphClient, args []string) {
	if len(args) == 0 {
		fmt.Printf("  Usage: %semail <contact> [more contacts...]%s\n", cyan, reset)
		return
	}
	if err := composeAndSend(client, args, ""); err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
	}
}

