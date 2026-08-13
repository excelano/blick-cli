package main

import (
	"reflect"
	"testing"
)

func TestParseChatArgs(t *testing.T) {
	tests := []struct {
		name         string
		in           []string
		wantContacts []string
		wantTopic    string
		wantErr      bool
	}{
		{"single recipient, no topic", []string{"alice"}, []string{"alice"}, "", false},
		{"multi recipient, no topic", []string{"alice", "bob"}, []string{"alice", "bob"}, "", false},
		{"long flag", []string{"alice", "bob", "--topic", "Planning"}, []string{"alice", "bob"}, "Planning", false},
		{"short flag", []string{"alice", "bob", "-t", "Planning"}, []string{"alice", "bob"}, "Planning", false},
		{"flag interleaved with recipients", []string{"alice", "--topic", "Plan", "bob"}, []string{"alice", "bob"}, "Plan", false},
		{"flag before recipients", []string{"--topic", "Plan", "alice", "bob"}, []string{"alice", "bob"}, "Plan", false},
		{"trailing --topic errors", []string{"alice", "--topic"}, nil, "", true},
		{"trailing -t errors", []string{"alice", "-t"}, nil, "", true},
		{"empty input", []string{}, []string{}, "", false},
		{"comma-separated single arg", []string{"alice,bob"}, []string{"alice", "bob"}, "", false},
		{"comma-separated with spaces", []string{"alice,", "bob,", "carol"}, []string{"alice", "bob", "carol"}, "", false},
		{"mixed comma and space", []string{"alice,bob", "carol"}, []string{"alice", "bob", "carol"}, "", false},
		{"empty comma parts dropped", []string{"alice,,bob"}, []string{"alice", "bob"}, "", false},
		{"trailing comma dropped", []string{"alice,"}, []string{"alice"}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotContacts, gotTopic, err := parseChatArgs(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (contacts=%v topic=%q)", gotContacts, gotTopic)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(gotContacts, tc.wantContacts) {
				t.Errorf("contacts: got %v, want %v", gotContacts, tc.wantContacts)
			}
			if gotTopic != tc.wantTopic {
				t.Errorf("topic: got %q, want %q", gotTopic, tc.wantTopic)
			}
		})
	}
}

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
