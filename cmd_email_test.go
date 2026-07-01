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
		wantAttach   []string
		wantErr      bool
	}{
		{"single recipient, no subject", []string{"alice"}, []string{"alice"}, "", []string{}, false},
		{"multi recipient, no subject", []string{"alice", "bob"}, []string{"alice", "bob"}, "", []string{}, false},
		{"long flag", []string{"alice", "--subject", "Hello"}, []string{"alice"}, "Hello", []string{}, false},
		{"short flag", []string{"alice", "-s", "Hello"}, []string{"alice"}, "Hello", []string{}, false},
		{"flag interleaved with recipients", []string{"alice", "--subject", "Hi", "bob"}, []string{"alice", "bob"}, "Hi", []string{}, false},
		{"flag before recipients", []string{"--subject", "Hi", "alice"}, []string{"alice"}, "Hi", []string{}, false},
		{"trailing --subject errors", []string{"alice", "--subject"}, nil, "", nil, true},
		{"trailing -s errors", []string{"alice", "-s"}, nil, "", nil, true},
		{"empty input", []string{}, []string{}, "", []string{}, false},
		{"comma-separated single arg", []string{"alice,bob"}, []string{"alice", "bob"}, "", []string{}, false},
		{"comma-separated with spaces", []string{"alice,", "bob,", "carol"}, []string{"alice", "bob", "carol"}, "", []string{}, false},
		{"mixed comma and space", []string{"alice,bob", "carol"}, []string{"alice", "bob", "carol"}, "", []string{}, false},
		{"empty comma parts dropped", []string{"alice,,bob"}, []string{"alice", "bob"}, "", []string{}, false},
		{"trailing comma dropped", []string{"alice,"}, []string{"alice"}, "", []string{}, false},
		{"single attach long flag", []string{"alice", "--attach", "f.pdf"}, []string{"alice"}, "", []string{"f.pdf"}, false},
		{"single attach short flag", []string{"alice", "-a", "f.pdf"}, []string{"alice"}, "", []string{"f.pdf"}, false},
		{"multiple attach flags", []string{"alice", "--attach", "a.pdf", "--attach", "b.png"}, []string{"alice"}, "", []string{"a.pdf", "b.png"}, false},
		{"attach with subject and recipients", []string{"alice", "-s", "Hi", "bob", "--attach", "f.pdf"}, []string{"alice", "bob"}, "Hi", []string{"f.pdf"}, false},
		{"trailing --attach errors", []string{"alice", "--attach"}, nil, "", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotContacts, gotSubject, gotAttach, err := parseEmailArgs(tc.in)
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
			if !reflect.DeepEqual(gotAttach, tc.wantAttach) {
				t.Errorf("attach: got %v, want %v", gotAttach, tc.wantAttach)
			}
		})
	}
}

func TestSplitRecipients(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"space separated", []string{"alice", "bob"}, []string{"alice", "bob"}},
		{"comma in one token", []string{"alice,bob"}, []string{"alice", "bob"}},
		{"comma with spaces", []string{"alice,", "bob"}, []string{"alice", "bob"}},
		{"empty parts dropped", []string{"alice,,bob"}, []string{"alice", "bob"}},
		{"trailing comma dropped", []string{"alice,"}, []string{"alice"}},
		{"blanks trimmed", []string{" alice ", "  ", "bob"}, []string{"alice", "bob"}},
		{"empty input", []string{}, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitRecipients(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitRecipients(%v) = %v, want %v", tc.in, got, tc.want)
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
