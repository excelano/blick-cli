package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempHome points HOME at a fresh tempdir for the duration of one test
// so configDir() reads and writes there. The blick/checkin migration logic
// in configDir() inspects ~/.config/{blick,checkin}; pointing HOME at an
// empty tempdir keeps tests isolated and fast.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestLoadContactsMissingFile(t *testing.T) {
	withTempHome(t)
	s, err := LoadContacts()
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(s.Contacts) != 0 {
		t.Fatalf("expected empty contacts, got %d", len(s.Contacts))
	}
	if s.Version != contactsFileVersion {
		t.Fatalf("expected version %d, got %d", contactsFileVersion, s.Version)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	withTempHome(t)
	s := &ContactStore{
		Version:  contactsFileVersion,
		Contacts: map[string]*Contact{},
	}
	s.Contacts["tony"] = &Contact{Key: "tony", Name: "Tony Stark", Email: "tony@stark.com"}
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	info, err := os.Stat(contactsPath())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected mode 0600, got %o", info.Mode().Perm())
	}

	loaded, err := LoadContacts()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	c, ok := loaded.Contacts["tony"]
	if !ok {
		t.Fatalf("tony not found")
	}
	if c.Key != "tony" || c.Name != "Tony Stark" || c.Email != "tony@stark.com" {
		t.Errorf("unexpected contact: %+v", c)
	}
}

func TestLoadContactsMalformed(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".config", "blick")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "contacts.json"), []byte("{not json"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadContacts()
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
	if !strings.Contains(err.Error(), "contacts.json") {
		t.Errorf("error should name the file, got: %v", err)
	}
}

func TestResolveExactMatch(t *testing.T) {
	s := &ContactStore{Contacts: map[string]*Contact{
		"tony": {Key: "tony", Name: "Tony Stark", Email: "tony@stark.com"},
	}}
	c, err := s.Resolve("tony")
	if err != nil {
		t.Fatalf("expected hit, got: %v", err)
	}
	if c.Email != "tony@stark.com" {
		t.Errorf("wrong contact: %+v", c)
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	s := &ContactStore{Contacts: map[string]*Contact{
		"tony": {Key: "tony", Name: "Tony Stark", Email: "tony@stark.com"},
	}}
	c, err := s.Resolve("TONY")
	if err != nil {
		t.Fatalf("expected case-insensitive hit, got: %v", err)
	}
	if c.Email != "tony@stark.com" {
		t.Errorf("wrong contact: %+v", c)
	}
}

func TestResolveEmailPassthrough(t *testing.T) {
	s := &ContactStore{Contacts: map[string]*Contact{}}
	c, err := s.Resolve("anyone@example.com")
	if err != nil {
		t.Fatalf("email-shaped input should pass through, got: %v", err)
	}
	if c.Email != "anyone@example.com" || c.Name != "anyone@example.com" {
		t.Errorf("expected synthetic contact, got: %+v", c)
	}
}

func TestResolveMiss(t *testing.T) {
	s := &ContactStore{Contacts: map[string]*Contact{}}
	_, err := s.Resolve("nobody")
	if err == nil {
		t.Fatal("expected miss to error")
	}
	if !strings.Contains(err.Error(), "no contact") {
		t.Errorf("error should mention 'no contact', got: %v", err)
	}
}

func TestResolveEmpty(t *testing.T) {
	s := &ContactStore{Contacts: map[string]*Contact{}}
	if _, err := s.Resolve(""); err == nil {
		t.Fatal("expected empty input to error")
	}
	if _, err := s.Resolve("   "); err == nil {
		t.Fatal("expected whitespace input to error")
	}
}

func TestDeriveKey(t *testing.T) {
	cases := []struct {
		name, want string
	}{
		{"Tony Stark", "tony"},
		{"tony", "tony"},
		{"O'Brien, Pat", "obrien"},
		{"Anne-Marie Whitfield", "annemarie"},
		{"  Spaced  Name  ", "spaced"},
		{"", ""},
	}
	for _, c := range cases {
		got := deriveKey(c.name)
		if got != c.want {
			t.Errorf("deriveKey(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDeriveKeyWithCollision(t *testing.T) {
	taken := map[string]bool{}

	k1 := deriveKeyWithCollision("Tony Stark", taken)
	if k1 != "tony" {
		t.Errorf("first Tony should be tony, got %q", k1)
	}
	taken[k1] = true

	k2 := deriveKeyWithCollision("Tony Soprano", taken)
	if k2 != "tony-s" {
		t.Errorf("second Tony with different last initial should be tony-s, got %q", k2)
	}
	taken[k2] = true

	// Third Tony with same last initial as the first one.
	k3 := deriveKeyWithCollision("Tony Stewart", taken)
	if k3 != "tony2" {
		t.Errorf("third Tony with colliding last initial should fall back to tony2, got %q", k3)
	}
	taken[k3] = true

	k4 := deriveKeyWithCollision("Tony Stark", taken)
	if k4 != "tony3" {
		t.Errorf("fourth Tony should be tony3, got %q", k4)
	}
}

func TestEmailLikeRegex(t *testing.T) {
	yes := []string{"a@b.co", "tony@stark.com", "first.last+tag@example.co.uk"}
	no := []string{"tony", "tony@", "@stark.com", "tony@stark", "two parts@x.com", "a@b@c.com"}
	for _, s := range yes {
		if !emailLike.MatchString(s) {
			t.Errorf("expected %q to match email-like", s)
		}
	}
	for _, s := range no {
		if emailLike.MatchString(s) {
			t.Errorf("expected %q to NOT match email-like", s)
		}
	}
}
