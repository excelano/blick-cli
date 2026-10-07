// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/excelano/atrest"
)

// Contact is a single address-book entry. Key is the lookup handle (e.g.
// "tony"); Email is the canonical address used by send-mail and start-chat
// flows; ChatID caches the 1:1 Teams chat ID once one has been discovered
// (left empty until the chat slice fills it in).
type Contact struct {
	Key    string `json:"-"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	ChatID string `json:"chatId,omitempty"`
}

// ContactStore mirrors the on-disk shape of contacts.json. Map-keyed by Key
// so the file is naturally hand-editable and lookups are O(1).
type ContactStore struct {
	Version  int                 `json:"version"`
	Contacts map[string]*Contact `json:"contacts"`
}

const contactsFileVersion = 1

func contactsPath() string {
	return filepath.Join(configDir(), "contacts.json")
}

// contactsSealName binds contacts.json to atrest's protection, distinct from
// the token's so the two files never open under each other's key.
const contactsSealName = "excelano/blick-contacts"

// readContacts returns the plaintext of contacts.json and whether it was
// sealed. A missing file, or one this machine cannot open — sealed on another
// machine or by another user — reads as no file, the same as a fresh install.
// A file sealed under a keyring that stays locked is an error rather than no
// file, so a later save cannot overwrite the address book it holds.
func readContacts() (plain []byte, sealed bool, err error) {
	path := contactsPath()
	stored, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("reading %s: %w", path, err)
	}
	plain, sealed, err = openFromDisk(contactsSealName, "address book", stored)
	if errors.Is(err, atrest.ErrCannotOpen) && !errors.Is(err, atrest.ErrLocked) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("opening %s: %w", path, err)
	}
	return plain, sealed, nil
}

// parseContacts reads the on-disk shape. A plaintext file from before sealing
// existed has the same shape, so it parses the same way.
func parseContacts(plain []byte) (*ContactStore, error) {
	var s ContactStore
	if err := json.Unmarshal(plain, &s); err != nil {
		return nil, err
	}
	if s.Contacts == nil {
		s.Contacts = map[string]*Contact{}
	}
	for k, c := range s.Contacts {
		if c == nil {
			return nil, fmt.Errorf("contact %q is empty", k)
		}
		c.Key = k
	}
	return &s, nil
}

// validateContacts refuses an address book a later command could not use: a
// contact with no key, no name, or an address that is not one.
func validateContacts(s *ContactStore) error {
	for k, c := range s.Contacts {
		switch {
		case strings.TrimSpace(k) == "":
			return fmt.Errorf("a contact has an empty key")
		case strings.TrimSpace(c.Name) == "":
			return fmt.Errorf("contact %q has no name", k)
		case !emailLike.MatchString(c.Email):
			return fmt.Errorf("contact %q: %q is not an email address", k, c.Email)
		}
	}
	return nil
}

func emptyContacts() *ContactStore {
	return &ContactStore{Version: contactsFileVersion, Contacts: map[string]*Contact{}}
}

// LoadContacts returns the address book, empty where there is no file to read.
// A file that does not parse is a hard error naming the path, never silently
// rewritten; blick contacts edit opens it for repair.
func LoadContacts() (*ContactStore, error) {
	plain, sealed, err := readContacts()
	if err != nil {
		return nil, err
	}
	if plain == nil {
		return emptyContacts(), nil
	}
	s, err := parseContacts(plain)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w (blick contacts edit opens it for repair)", contactsPath(), err)
	}
	if !sealed && atrest.Persistent() {
		// Best-effort: a failure to reseal here costs nothing but leaving
		// the file plaintext a little longer, and the caller already has
		// what it asked for.
		_ = s.Save()
	}
	return s, nil
}

// Save seals the store with sealForDisk and writes it atomically (write-then-
// rename) at mode 0600 so a crash mid-write can't truncate the file. The
// config dir is created with mode 0700 the same way the token cache does it.
func (s *ContactStore) Save() error {
	if s.Version == 0 {
		s.Version = contactsFileVersion
	}
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	plain, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	stored, err := sealForDisk(contactsSealName, plain)
	if err != nil {
		return fmt.Errorf("sealing %s: %w", contactsPath(), err)
	}
	tmp := contactsPath() + ".tmp"
	if err := os.WriteFile(tmp, stored, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, contactsPath())
}

// Sorted returns contacts in stable key order so list output and seed
// previews don't shuffle between runs.
func (s *ContactStore) Sorted() []*Contact {
	out := make([]*Contact, 0, len(s.Contacts))
	for _, c := range s.Contacts {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

var emailLike = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Resolve looks up a contact by key, or — when input looks like an email
// address — returns a synthetic contact pointing at that address. Used by
// the (forthcoming) email and chat compose commands so they don't need to
// know about ContactStore's shape.
func (s *ContactStore) Resolve(input string) (*Contact, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty contact")
	}
	if emailLike.MatchString(input) {
		return &Contact{Name: input, Email: input}, nil
	}
	if c, ok := s.Contacts[input]; ok {
		return c, nil
	}
	lower := strings.ToLower(input)
	for k, c := range s.Contacts {
		if strings.ToLower(k) == lower {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no contact %q — try: blick contacts list", input)
}

// deriveKey turns a display name into a lowercase, ASCII-only short handle.
// Strips punctuation and whitespace, keeps letters and digits, lowercases.
// "Tony Stark" -> "tony", "O'Brien, Pat" -> "obrien" (first whitespace-
// separated word wins). Caller handles collisions.
func deriveKey(displayName string) string {
	first := strings.Fields(displayName)
	if len(first) == 0 {
		return ""
	}
	var b strings.Builder
	for _, r := range first[0] {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// deriveKeyWithCollision returns a unique key not already present in
// `taken`. First attempt is deriveKey(name); if taken, append the
// lowercased last-name initial; if still taken, append digits 2, 3, ...
// "Tony Stark" then "Tony Soprano" -> "tony", "tony-s" (different last
// initial), or "tony", "tony2" if last initials collide too.
func deriveKeyWithCollision(displayName string, taken map[string]bool) string {
	base := deriveKey(displayName)
	if base == "" {
		base = "contact"
	}
	if !taken[base] {
		return base
	}

	fields := strings.Fields(displayName)
	if len(fields) > 1 {
		lastInitial := ""
		for _, r := range fields[len(fields)-1] {
			if unicode.IsLetter(r) {
				lastInitial = string(unicode.ToLower(r))
				break
			}
		}
		if lastInitial != "" {
			candidate := base + "-" + lastInitial
			if !taken[candidate] {
				return candidate
			}
		}
	}

	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s%d", base, n)
		if !taken[candidate] {
			return candidate
		}
	}
}
