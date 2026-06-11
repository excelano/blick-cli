package main

import "testing"

// TestContactChatIDRoundTrip covers the cache write-back path used by
// composeAndSendChat: setting ChatID on a stored contact, calling Save,
// and confirming the value survives a reload. The rest of the chat flow
// is Graph API calls — exercised manually, not unit-tested.
func TestContactChatIDRoundTrip(t *testing.T) {
	withTempHome(t)
	s := &ContactStore{
		Version:  contactsFileVersion,
		Contacts: map[string]*Contact{},
	}
	s.Contacts["tony"] = &Contact{Key: "tony", Name: "Tony Stark", Email: "tony@stark.com"}
	if err := s.Save(); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	loaded, err := LoadContacts()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	loaded.Contacts["tony"].ChatID = "19:abc@thread.v2"
	if err := loaded.Save(); err != nil {
		t.Fatalf("save with chat id: %v", err)
	}

	reloaded, err := LoadContacts()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Contacts["tony"].ChatID; got != "19:abc@thread.v2" {
		t.Errorf("ChatID not persisted: got %q", got)
	}
	if got := reloaded.Contacts["tony"].Email; got != "tony@stark.com" {
		t.Errorf("Email lost across roundtrip: got %q", got)
	}
}
