package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
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

// LoadContacts reads contacts.json. A missing file returns an empty store —
// the first add or seed creates it. A malformed file is a hard error so the
// caller surfaces the path and the user can fix it by hand; we never
// silently rewrite a file we couldn't parse.
func LoadContacts() (*ContactStore, error) {
	path := contactsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ContactStore{Version: contactsFileVersion, Contacts: map[string]*Contact{}}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var s ContactStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	if s.Contacts == nil {
		s.Contacts = map[string]*Contact{}
	}
	for k, c := range s.Contacts {
		c.Key = k
	}
	return &s, nil
}

// Save writes the store atomically (write-then-rename) at mode 0600 so a
// crash mid-write can't truncate the file. The config dir is created with
// mode 0700 the same way the token cache does it.
func (s *ContactStore) Save() error {
	if s.Version == 0 {
		s.Version = contactsFileVersion
	}
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := contactsPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
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
