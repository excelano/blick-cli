package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOutgoingAttachments(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(small, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("reads files and labels them", func(t *testing.T) {
		got, err := readOutgoingAttachments([]string{small})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d attachments, want 1", len(got))
		}
		if got[0].Name != "note.txt" {
			t.Errorf("name = %q, want note.txt", got[0].Name)
		}
		if string(got[0].Content) != "hello" {
			t.Errorf("content = %q, want hello", got[0].Content)
		}
		if got[0].ContentType == "" {
			t.Error("content type should not be empty")
		}
	})

	t.Run("empty list is fine", func(t *testing.T) {
		got, err := readOutgoingAttachments(nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("got (%v, %v), want (empty, nil)", got, err)
		}
	})

	t.Run("missing file errors", func(t *testing.T) {
		if _, err := readOutgoingAttachments([]string{filepath.Join(dir, "nope.pdf")}); err == nil {
			t.Fatal("want error for missing file")
		}
	})

	t.Run("directory errors", func(t *testing.T) {
		if _, err := readOutgoingAttachments([]string{dir}); err == nil {
			t.Fatal("want error for directory")
		}
	})

	t.Run("over the size cap errors", func(t *testing.T) {
		big := filepath.Join(dir, "big.bin")
		if err := os.WriteFile(big, make([]byte, maxSendAttachmentBytes+1), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := readOutgoingAttachments([]string{big})
		if err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("want size-limit error, got %v", err)
		}
	})
}
