// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEditorCommandPrefersVisual(t *testing.T) {
	t.Setenv("VISUAL", "nved --wait")
	t.Setenv("EDITOR", "nano")
	cmd, err := editorCommand()
	if err != nil || strings.Join(cmd, " ") != "nved --wait" {
		t.Errorf("editorCommand = %v, %v; want $VISUAL with its argument", cmd, err)
	}
	t.Setenv("VISUAL", "")
	cmd, err = editorCommand()
	if err != nil || strings.Join(cmd, " ") != "nano" {
		t.Errorf("editorCommand = %v, %v; want $EDITOR once $VISUAL is unset", cmd, err)
	}
}

func TestValidateContacts(t *testing.T) {
	good := &ContactStore{Contacts: map[string]*Contact{"tony": {Name: "Tony Stark", Email: "tony@example.com"}}}
	if err := validateContacts(good); err != nil {
		t.Errorf("a complete contact was refused: %v", err)
	}
	for label, s := range map[string]*ContactStore{
		"empty key": {Contacts: map[string]*Contact{"": {Name: "Tony", Email: "tony@example.com"}}},
		"no name":   {Contacts: map[string]*Contact{"tony": {Email: "tony@example.com"}}},
		"bad email": {Contacts: map[string]*Contact{"tony": {Name: "Tony", Email: "not an address"}}},
	} {
		if validateContacts(s) == nil {
			t.Errorf("%s was accepted", label)
		}
	}
}

func TestParseContactsRefusesEmptyEntry(t *testing.T) {
	if _, err := parseContacts([]byte(`{"version":1,"contacts":{"tony":null}}`)); err == nil {
		t.Error("a null contact was accepted")
	}
}

// contactsEdit runs through a stand-in editor that overwrites the copy it is
// handed, the way a real editor would after the user saved.
func TestContactsEditSavesWhatTheEditorWrote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in editor is a shell script")
	}
	home := withTempHome(t)
	script := filepath.Join(home, "editor.sh")
	body := "#!/bin/sh\ncat > \"$1\" <<'JSON'\n{\"version\":1,\"contacts\":{\"tony\":{\"name\":\"Tony Stark\",\"email\":\"tony@example.com\"}}}\nJSON\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", script)

	contactsEdit()

	store, err := LoadContacts()
	if err != nil {
		t.Fatalf("LoadContacts after edit: %v", err)
	}
	c, ok := store.Contacts["tony"]
	if !ok || c.Email != "tony@example.com" || c.Key != "tony" {
		t.Errorf("after edit, contacts = %v; want tony as the editor wrote him", store.Contacts)
	}
	leftovers, _ := filepath.Glob(filepath.Join(configDir(), "contacts-edit-*.json"))
	if len(leftovers) != 0 {
		t.Errorf("the plaintext copy was left behind: %v", leftovers)
	}
}
