package main

import (
	"os"
	"reflect"
	"testing"
)

func TestParseEmailArgs(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    composeArgs
		wantErr bool
	}{
		{"single recipient, no subject", []string{"alice"}, composeArgs{Recipients: []string{"alice"}}, false},
		{"multi recipient, no subject", []string{"alice", "bob"}, composeArgs{Recipients: []string{"alice", "bob"}}, false},
		{"long flag", []string{"alice", "--subject", "Hello"}, composeArgs{Recipients: []string{"alice"}, Subject: "Hello"}, false},
		{"short flag", []string{"alice", "-s", "Hello"}, composeArgs{Recipients: []string{"alice"}, Subject: "Hello"}, false},
		{"flag interleaved with recipients", []string{"alice", "--subject", "Hi", "bob"}, composeArgs{Recipients: []string{"alice", "bob"}, Subject: "Hi"}, false},
		{"flag before recipients", []string{"--subject", "Hi", "alice"}, composeArgs{Recipients: []string{"alice"}, Subject: "Hi"}, false},
		{"trailing --subject errors", []string{"alice", "--subject"}, composeArgs{}, true},
		{"trailing -s errors", []string{"alice", "-s"}, composeArgs{}, true},
		{"empty input", []string{}, composeArgs{}, false},
		{"comma-separated single arg", []string{"alice,bob"}, composeArgs{Recipients: []string{"alice", "bob"}}, false},
		{"comma-separated with spaces", []string{"alice,", "bob,", "carol"}, composeArgs{Recipients: []string{"alice", "bob", "carol"}}, false},
		{"mixed comma and space", []string{"alice,bob", "carol"}, composeArgs{Recipients: []string{"alice", "bob", "carol"}}, false},
		{"empty comma parts dropped", []string{"alice,,bob"}, composeArgs{Recipients: []string{"alice", "bob"}}, false},
		{"trailing comma dropped", []string{"alice,"}, composeArgs{Recipients: []string{"alice"}}, false},
		{"single attach long flag", []string{"alice", "--attach", "f.pdf"}, composeArgs{Recipients: []string{"alice"}, Attach: []string{"f.pdf"}}, false},
		{"single attach short flag", []string{"alice", "-a", "f.pdf"}, composeArgs{Recipients: []string{"alice"}, Attach: []string{"f.pdf"}}, false},
		{"multiple attach flags", []string{"alice", "--attach", "a.pdf", "--attach", "b.png"}, composeArgs{Recipients: []string{"alice"}, Attach: []string{"a.pdf", "b.png"}}, false},
		{"attach with subject and recipients", []string{"alice", "-s", "Hi", "bob", "--attach", "f.pdf"}, composeArgs{Recipients: []string{"alice", "bob"}, Subject: "Hi", Attach: []string{"f.pdf"}}, false},
		{"trailing --attach errors", []string{"alice", "--attach"}, composeArgs{}, true},
		{"cc single", []string{"alice", "--cc", "bob"}, composeArgs{Recipients: []string{"alice"}, Cc: []string{"bob"}}, false},
		{"cc comma list", []string{"alice", "--cc", "bob,carol"}, composeArgs{Recipients: []string{"alice"}, Cc: []string{"bob", "carol"}}, false},
		{"cc repeated accumulates", []string{"alice", "--cc", "bob", "--cc", "carol"}, composeArgs{Recipients: []string{"alice"}, Cc: []string{"bob", "carol"}}, false},
		{"bcc single", []string{"alice", "--bcc", "carol"}, composeArgs{Recipients: []string{"alice"}, Bcc: []string{"carol"}}, false},
		{"cc and bcc together", []string{"alice", "--cc", "bob", "--bcc", "carol,dave"}, composeArgs{Recipients: []string{"alice"}, Cc: []string{"bob"}, Bcc: []string{"carol", "dave"}}, false},
		{"trailing --cc errors", []string{"alice", "--cc"}, composeArgs{}, true},
		{"trailing --bcc errors", []string{"alice", "--bcc"}, composeArgs{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEmailArgs(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (%+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
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
	path, err := saveDraftCopy([]string{"a@example.com", "b@example.com"}, nil, nil, "Subj", "Body text")
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

func TestSaveDraftCopyWithCcBcc(t *testing.T) {
	withTempHome(t)
	path, err := saveDraftCopy([]string{"a@example.com"}, []string{"c@example.com"}, []string{"d@example.com"}, "Subj", "Body")
	if err != nil {
		t.Fatalf("saveDraftCopy: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	want := "To: a@example.com\nCc: c@example.com\nBcc: d@example.com\nSubject: Subj\n\nBody\n"
	if string(data) != want {
		t.Errorf("draft contents:\n got %q\nwant %q", string(data), want)
	}
}
