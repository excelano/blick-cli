package main

import (
	"strings"
	"testing"
)

func TestSplitQuotedHistory_NoQuote(t *testing.T) {
	body := "Hi David,\n\nLet me know when you're free.\n\nThanks,\nAlice"
	visible, quoted, n := splitQuotedHistory(body)
	if quoted != "" || n != 0 {
		t.Fatalf("expected no quoted section, got %q (%d lines)", quoted, n)
	}
	if !strings.Contains(visible, "Let me know") {
		t.Fatalf("visible missing body text: %q", visible)
	}
}

func TestSplitQuotedHistory_GreaterPrefix(t *testing.T) {
	body := "Sounds good.\n\n> Original idea here\n> with two lines"
	visible, quoted, n := splitQuotedHistory(body)
	if visible != "Sounds good." {
		t.Fatalf("visible = %q, want %q", visible, "Sounds good.")
	}
	if !strings.HasPrefix(quoted, "> Original idea here") {
		t.Fatalf("quoted = %q", quoted)
	}
	if n != 2 {
		t.Fatalf("quotedLines = %d, want 2", n)
	}
}

func TestSplitQuotedHistory_WroteAttributionSingleLine(t *testing.T) {
	body := "Sure, attached.\n\nDavid wrote:\nDid you finish the doc?"
	visible, _, n := splitQuotedHistory(body)
	if visible != "Sure, attached." {
		t.Fatalf("visible = %q", visible)
	}
	if n < 2 {
		t.Fatalf("quotedLines = %d, want ≥2", n)
	}
}

func TestSplitQuotedHistory_WroteAttributionMultiline(t *testing.T) {
	// Multi-line attribution: the "On ..." starts the block; the boundary
	// must be the "On" line so the whole attribution folds together.
	body := "Got it, thanks.\n\nOn Mon, Jun 9, 2026,\nDavid Anderson <david@x.com> wrote:\n> earlier message"
	visible, quoted, _ := splitQuotedHistory(body)
	if visible != "Got it, thanks." {
		t.Fatalf("visible = %q", visible)
	}
	if !strings.HasPrefix(quoted, "On Mon, Jun 9, 2026,") {
		t.Fatalf("quoted should start at the 'On' line, got %q", quoted)
	}
}

func TestSplitQuotedHistory_OutlookHeaderBlock(t *testing.T) {
	body := "Confirming for Thursday.\n\nFrom: Alice <alice@x.com>\nSent: Monday, June 9, 2026 3:14 PM\nTo: David Anderson\nSubject: Status\n\nOriginal body."
	visible, quoted, _ := splitQuotedHistory(body)
	if visible != "Confirming for Thursday." {
		t.Fatalf("visible = %q", visible)
	}
	if !strings.HasPrefix(quoted, "From: Alice") {
		t.Fatalf("quoted should start at the From: header, got %q", quoted)
	}
}

func TestSplitQuotedHistory_OutlookHeaderBlock_RequiresTwoLabels(t *testing.T) {
	// A sentence starting with "From:" but with no other header labels
	// nearby must not trigger the Outlook header detector.
	body := "From: the desk of Alice Andersen, a quick note —\nthe migration is on track."
	_, quoted, _ := splitQuotedHistory(body)
	if quoted != "" {
		t.Fatalf("expected no quote for 'From:' prose, got %q", quoted)
	}
}

func TestSplitQuotedHistory_OriginalMessageSeparator(t *testing.T) {
	body := "FYI.\n\n-----Original Message-----\nFrom somebody"
	visible, quoted, _ := splitQuotedHistory(body)
	if visible != "FYI." {
		t.Fatalf("visible = %q", visible)
	}
	if !strings.HasPrefix(quoted, "-----Original Message-----") {
		t.Fatalf("quoted = %q", quoted)
	}
}

func TestSplitQuotedHistory_ForwardedMessageSeparator(t *testing.T) {
	body := "Passing this along.\n\n---------- Forwarded message ----------\nFrom: someone"
	visible, quoted, _ := splitQuotedHistory(body)
	if visible != "Passing this along." {
		t.Fatalf("visible = %q", visible)
	}
	if !strings.HasPrefix(quoted, "---------- Forwarded message") {
		t.Fatalf("quoted = %q", quoted)
	}
}

func TestSplitQuotedHistory_BeginForwardedMessage(t *testing.T) {
	body := "FYI.\n\nBegin forwarded message:\nFrom: Alice"
	visible, quoted, _ := splitQuotedHistory(body)
	if visible != "FYI." {
		t.Fatalf("visible = %q", visible)
	}
	if !strings.HasPrefix(quoted, "Begin forwarded message:") {
		t.Fatalf("quoted = %q", quoted)
	}
}

func TestSplitQuotedHistory_EarliestMarkerWins(t *testing.T) {
	// A `>` line above a "wrote:" line — the `>` boundary should win
	// because it comes first.
	body := "Top.\n\n> early quote\n\nOn Mon, Jun 9, David wrote:\n> later"
	visible, quoted, _ := splitQuotedHistory(body)
	if visible != "Top." {
		t.Fatalf("visible = %q", visible)
	}
	if !strings.HasPrefix(quoted, "> early quote") {
		t.Fatalf("quoted should start with the earliest marker, got %q", quoted)
	}
}

func TestSplitQuotedHistory_CRLFNormalized(t *testing.T) {
	body := "Reply.\r\n\r\n> quoted line\r\n> another"
	visible, _, n := splitQuotedHistory(body)
	if visible != "Reply." {
		t.Fatalf("visible = %q", visible)
	}
	if n != 2 {
		t.Fatalf("quotedLines = %d, want 2", n)
	}
}

func TestSplitQuotedHistory_BareForward(t *testing.T) {
	// A body that opens with the quote marker has no visible portion.
	body := "> only a quote\n> nothing above"
	visible, quoted, n := splitQuotedHistory(body)
	if visible != "" {
		t.Fatalf("visible should be empty, got %q", visible)
	}
	if n != 2 {
		t.Fatalf("quotedLines = %d, want 2", n)
	}
	if !strings.HasPrefix(quoted, "> only a quote") {
		t.Fatalf("quoted = %q", quoted)
	}
}
