// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import "testing"

func TestUnreadEmailsHeader(t *testing.T) {
	tests := []struct {
		shown, total int
		want         string
	}{
		{3, 3, "unread emails (3)"},
		{10, 40, "unread emails (40, showing 10)"},
		{3, 2, "unread emails (3)"},
	}
	for _, tt := range tests {
		if got := unreadEmailsHeader(tt.shown, tt.total); got != tt.want {
			t.Errorf("unreadEmailsHeader(%d, %d) = %q, want %q", tt.shown, tt.total, got, tt.want)
		}
	}
}
