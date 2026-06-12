package main

import (
	"os"
	"reflect"
	"testing"
)

func TestParseEmailArgs(t *testing.T) {
	tests := []struct {
		name         string
		in           []string
		wantContacts []string
		wantSubject  string
		wantErr      bool
	}{
		{"single recipient, no subject", []string{"alice"}, []string{"alice"}, "", false},
		{"multi recipient, no subject", []string{"alice", "bob"}, []string{"alice", "bob"}, "", false},
		{"long flag", []string{"alice", "--subject", "Hello"}, []string{"alice"}, "Hello", false},
		{"short flag", []string{"alice", "-s", "Hello"}, []string{"alice"}, "Hello", false},
		{"flag interleaved with recipients", []string{"alice", "--subject", "Hi", "bob"}, []string{"alice", "bob"}, "Hi", false},
		{"flag before recipients", []string{"--subject", "Hi", "alice"}, []string{"alice"}, "Hi", false},
		{"trailing --subject errors", []string{"alice", "--subject"}, nil, "", true},
		{"trailing -s errors", []string{"alice", "-s"}, nil, "", true},
		{"empty input", []string{}, []string{}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotContacts, gotSubject, err := parseEmailArgs(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (contacts=%v subject=%q)", gotContacts, gotSubject)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(gotContacts, tc.wantContacts) {
				t.Errorf("contacts: got %v, want %v", gotContacts, tc.wantContacts)
			}
			if gotSubject != tc.wantSubject {
				t.Errorf("subject: got %q, want %q", gotSubject, tc.wantSubject)
			}
		})
	}
}

func TestSaveDraftCopy(t *testing.T) {
	withTempHome(t)
	path, err := saveDraftCopy([]string{"a@example.com", "b@example.com"}, "Subj", "Body text")
	if err != nil {
		t.Fatalf("saveDraftCopy: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	want := "To: a@example.com, b@example.com\nSubject: Subj\n\nBody text\n"
	if string(data) != want {
		t.Errorf("draft contents:\n got %q\nwant %q", string(data), want)
	}
}
