package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// proposedContact is the seed-preview row type. Lives at file scope so
// editProposed can pass it through $EDITOR round-trip.
type proposedContact struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// needsGraphForContacts returns true for the one contacts subcommand that
// hits Microsoft Graph (`seed`). main.go uses it to skip the device-code
// flow for the offline-friendly subcommands.
func needsGraphForContacts(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return args[0] == "seed"
}

// runContacts dispatches `blick contacts ...`. Args is everything after
// "contacts" on the command line. The dispatcher itself never touches
// Graph — only `seed` does — so loading and listing work even when the
// app registration hasn't been granted People.Read consent yet.
func runContacts(client *GraphClient, args []string) {
	sub := "list"
	rest := []string{}
	if len(args) > 0 {
		sub = args[0]
		rest = args[1:]
	}

	store, err := LoadContacts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch sub {
	case "list", "ls":
		contactsList(store)
	case "add":
		contactsAdd(store, rest)
	case "remove", "rm":
		contactsRemove(store, rest)
	case "show":
		contactsShow(store, rest)
	case "seed":
		contactsSeed(client, store)
	default:
		fmt.Fprintf(os.Stderr, "Unknown contacts subcommand: %s\n", sub)
		fmt.Fprintln(os.Stderr, "Usage: blick contacts {list|add|remove|show|seed}")
		os.Exit(1)
	}
}

func contactsList(store *ContactStore) {
	rows := store.Sorted()
	if len(rows) == 0 {
		fmt.Printf("  %sNo contacts. Add one with `blick contacts add <key> <email>`.%s\n", dim, reset)
		return
	}
	fmt.Println()
	fmt.Printf("  %s%-12s  %-28s  %s%s\n", dim, "Key", "Name", "Email", reset)
	for _, c := range rows {
		chatMark := ""
		if c.ChatID != "" {
			chatMark = " " + dim + "(chat cached)" + reset
		}
		fmt.Printf("  %s%-12s%s  %-28s  %s%s\n",
			cyan, c.Key, reset, truncate(c.Name, 28), c.Email, chatMark)
	}
	fmt.Println()
}

func contactsShow(store *ContactStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: blick contacts show <key>")
		os.Exit(1)
	}
	c, ok := store.Contacts[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "No contact %q\n", args[0])
		os.Exit(1)
	}
	fmt.Println()
	fmt.Printf("  %sKey:%s    %s\n", bold, reset, c.Key)
	fmt.Printf("  %sName:%s   %s\n", bold, reset, c.Name)
	fmt.Printf("  %sEmail:%s  %s\n", bold, reset, c.Email)
	if c.ChatID != "" {
		fmt.Printf("  %sChatID:%s %s\n", bold, reset, c.ChatID)
	}
	fmt.Println()
}

// contactsAdd: `blick contacts add <key> <email> [--name "Display Name"]`
// Rejects duplicate keys. With no --name, the local-part before the @ is
// the display name — Tony writes `blick contacts add tony tony@stark.com`
// and gets back name "tony", which he can edit in the JSON later if he
// cares.
func contactsAdd(store *ContactStore, args []string) {
	var key, email, name string
	positional := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--name requires a value")
				os.Exit(1)
			}
			name = args[i+1]
			i++
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: blick contacts add <key> <email> [--name \"Display Name\"]")
		os.Exit(1)
	}
	key = positional[0]
	email = positional[1]

	if !emailLike.MatchString(email) {
		fmt.Fprintf(os.Stderr, "Not a valid email: %s\n", email)
		os.Exit(1)
	}
	if _, exists := store.Contacts[key]; exists {
		fmt.Fprintf(os.Stderr, "Contact %q already exists. Remove it first or pick a different key.\n", key)
		os.Exit(1)
	}
	if name == "" {
		at := strings.IndexByte(email, '@')
		if at > 0 {
			name = email[:at]
		} else {
			name = email
		}
	}
	store.Contacts[key] = &Contact{Key: key, Name: name, Email: email}
	if err := store.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving contacts: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  %sAdded %s — %s <%s>%s\n", green, key, name, email, reset)
}

func contactsRemove(store *ContactStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: blick contacts remove <key>")
		os.Exit(1)
	}
	key := args[0]
	c, ok := store.Contacts[key]
	if !ok {
		fmt.Fprintf(os.Stderr, "No contact %q\n", key)
		os.Exit(1)
	}
	fmt.Printf("Remove %s (%s <%s>)? [y/N] ", key, c.Name, c.Email)
	reader := bufio.NewReader(os.Stdin)
	reply, _ := reader.ReadString('\n')
	reply = strings.TrimSpace(strings.ToLower(reply))
	if reply != "y" && reply != "yes" {
		fmt.Println("Cancelled.")
		return
	}
	delete(store.Contacts, key)
	if err := store.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving contacts: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  %sRemoved %s%s\n", green, key, reset)
}

// contactsSeed pulls /me/people, proposes new contact rows (skipping any
// whose email already lives in the store), shows a preview, and writes on
// confirmation. The "e" branch drops the proposed JSON into $EDITOR so the
// user can prune or tweak keys before the write.
func contactsSeed(client *GraphClient, store *ContactStore) {
	if client == nil {
		fmt.Fprintln(os.Stderr, "Seed requires authentication. Run `blick` once to sign in, then re-run.")
		os.Exit(1)
	}
	fmt.Printf("  Fetching relevant people from Microsoft Graph...\n")
	people, err := client.RelevantPeople()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "If this is a People.Read consent error, sign out and back in to re-prompt for permissions.")
		os.Exit(1)
	}

	existingEmails := map[string]bool{}
	takenKeys := map[string]bool{}
	for _, c := range store.Contacts {
		existingEmails[strings.ToLower(c.Email)] = true
		takenKeys[c.Key] = true
	}

	var props []proposedContact
	for _, p := range people {
		if existingEmails[strings.ToLower(p.Email)] {
			continue
		}
		key := deriveKeyWithCollision(p.DisplayName, takenKeys)
		takenKeys[key] = true
		props = append(props, proposedContact{Key: key, Name: p.DisplayName, Email: p.Email})
	}

	if len(props) == 0 {
		fmt.Printf("  %sNo new people to add — every relevant contact is already in the address book.%s\n", dim, reset)
		return
	}

	fmt.Printf("\n  %sProposed contacts (%d):%s\n\n", bold, len(props), reset)
	fmt.Printf("    %s%-12s  %-28s  %s%s\n", dim, "Key", "Name", "Email", reset)
	for _, p := range props {
		fmt.Printf("    %s%-12s%s  %-28s  %s\n", cyan, p.Key, reset, truncate(p.Name, 28), p.Email)
	}
	fmt.Println()

	fmt.Printf("Write all? [Y/n/e=edit] ")
	reader := bufio.NewReader(os.Stdin)
	reply, _ := reader.ReadString('\n')
	reply = strings.TrimSpace(strings.ToLower(reply))

	switch reply {
	case "", "y", "yes":
		// accept as-is
	case "n", "no":
		fmt.Println("Cancelled.")
		return
	case "e", "edit":
		edited, err := editProposed(props)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Edit failed: %v\n", err)
			os.Exit(1)
		}
		props = edited
	default:
		fmt.Println("Cancelled.")
		return
	}

	added := 0
	for _, p := range props {
		if p.Key == "" || p.Email == "" {
			continue
		}
		if _, exists := store.Contacts[p.Key]; exists {
			fmt.Fprintf(os.Stderr, "  skipping %s — key already exists\n", p.Key)
			continue
		}
		store.Contacts[p.Key] = &Contact{Key: p.Key, Name: p.Name, Email: p.Email}
		added++
	}

	if err := store.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving contacts: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  %sAdded %d contacts.%s\n", green, added, reset)
}

// editorFallback returns the first available editor in the OS-specific
// fallback chain when $EDITOR is unset: nved → ved → nano → vim on
// Unix/macOS, edit → notepad on Windows. Returns "" if none are on PATH.
func editorFallback() string {
	var chain []string
	if runtime.GOOS == "windows" {
		chain = []string{"edit", "notepad"}
	} else {
		chain = []string{"nved", "ved", "nano", "vim"}
	}
	for _, e := range chain {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	return ""
}

// editProposed writes the proposed list to a temp JSON file, opens it in
// $EDITOR (or the OS-specific fallback chain when $EDITOR is unset), and
// parses the result back. Lets the user prune rows or rename keys before
// commit. The temp file is left on disk after a failed parse so the user
// can recover their edits.
func editProposed(props []proposedContact) ([]proposedContact, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = editorFallback()
		if editor == "" {
			return nil, fmt.Errorf("no editor available — set $EDITOR")
		}
	}

	data, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "blick-seed-*.json")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("editor %s: %w", filepath.Base(editor), err)
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []proposedContact
	if err := json.Unmarshal(edited, &out); err != nil {
		return nil, fmt.Errorf("parsing edited JSON (%s left on disk): %w", path, err)
	}
	_ = os.Remove(path)
	return out, nil
}
