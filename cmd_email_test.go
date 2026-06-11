package main

import (
	"os"
	"testing"
)

func TestSplitDraft(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantSubject string
		wantBody    string
	}{
		{
			"standard header + blank + body",
			"Subject: Hello\n\nHi there.\n",
			"Hello",
			"Hi there.\n",
		},
		{
			"empty subject value preserved",
			"Subject: \n\nNo subject here.\n",
			"",
			"No subject here.\n",
		},
		{
			"case-insensitive subject header",
			"subject: lowercased\n\nbody\n",
			"lowercased",
			"body\n",
		},
		{
			"no blank separator — body starts on line 2",
			"Subject: terse\nimmediate body\n",
			"terse",
			"immediate body\n",
		},
		{
			"no subject header — entire buffer is body",
			"just a body\nwith two lines\n",
			"",
			"just a body\nwith two lines\n",
		},
		{
			"CRLF line endings normalized",
			"Subject: cr\r\n\r\ncrlf body\r\n",
			"cr",
			"crlf body\n",
		},
		{
			"multi-line body preserved",
			"Subject: long\n\npara one\n\npara two\n",
			"long",
			"para one\n\npara two\n",
		},
		{
			"subject value trimmed",
			"Subject:    padded   \n\nbody\n",
			"padded",
			"body\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSubject, gotBody := splitDraft(tc.in)
			if gotSubject != tc.wantSubject {
				t.Errorf("subject: got %q, want %q", gotSubject, tc.wantSubject)
			}
			if gotBody != tc.wantBody {
				t.Errorf("body: got %q, want %q", gotBody, tc.wantBody)
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
