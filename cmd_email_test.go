package main

import (
	"os"
	"testing"
)

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
