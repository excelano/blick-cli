package main

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		in      string
		wantCmd string
		wantN   int
	}{
		// Bare number = view
		{"5", "view", 5},
		{"view 5", "view", 5},

		// ed-style canonical: address-then-letter
		{"5r", "reply", 5},
		{"5d", "done", 5},
		{"10r", "reply", 10},

		// Legacy letter-then-number still works
		{"r5", "reply", 5},
		{"d5", "done", 5},
		{"r 5", "reply", 5},
		{"d 5", "done", 5},
		{"reply 5", "reply", 5},
		{"done 5", "done", 5},

		// No-arg verbs
		{"r", "refresh", -1},
		{"refresh", "refresh", -1},
		{"q", "quit", -1},
		{"quit", "quit", -1},
		{"x", "exit", -1},
		{"exit", "exit", -1},
		{"H", "help", -1},
		{"help", "help", -1},
		{"t", "today", -1},
		{"today", "today", -1},
		{"j", "join", -1},
		{"join", "join", -1},

		// Unparseable
		{"", "unknown", -1},
		{"foo", "unknown", -1},
		{"rr", "unknown", -1},
		{"5x", "unknown", -1},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			gotCmd, gotN := parseCommand(tc.in)
			if gotCmd != tc.wantCmd || gotN != tc.wantN {
				t.Errorf("parseCommand(%q) = (%q, %d), want (%q, %d)",
					tc.in, gotCmd, gotN, tc.wantCmd, tc.wantN)
			}
		})
	}
}
