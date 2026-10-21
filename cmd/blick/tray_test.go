// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestTrayStatus(t *testing.T) {
	tests := []struct {
		emails, chats int
		want          string
	}{
		{0, 0, "Nothing unread"},
		{1, 0, "1 unread email"},
		{0, 2, "2 unread chats"},
		{3, 1, "3 unread emails · 1 unread chat"},
	}
	for _, tt := range tests {
		if got := trayStatus(tt.emails, tt.chats); got != tt.want {
			t.Errorf("trayStatus(%d, %d) = %q, want %q", tt.emails, tt.chats, got, tt.want)
		}
	}
}

func TestTrayMeetingLine(t *testing.T) {
	if got := trayMeetingLine(nil); got != "No upcoming meetings" {
		t.Errorf("nil meeting = %q", got)
	}
	tests := []struct {
		start time.Duration
		want  string
	}{
		{-10 * time.Minute, "now"},
		{time.Minute, "Starting soon"},
		{47*time.Minute + 30*time.Second, "in 47 min"},
	}
	for _, tt := range tests {
		m := &Meeting{Subject: "Standup", Start: time.Now().Add(tt.start)}
		got := trayMeetingLine(m)
		if !strings.HasPrefix(got, "Standup — "+tt.want+" · ") {
			t.Errorf("start %v: got %q, want it to begin %q", tt.start, got, "Standup — "+tt.want)
		}
	}
}

func TestTrayProblem(t *testing.T) {
	signIn := "Not signed in — open blick to sign in"
	unreachable := "Couldn't reach Microsoft — retrying"
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"no cache", fmt.Errorf("reading: %w", os.ErrNotExist), signIn},
		{"refused refresh", fmt.Errorf("getting token: %w", &oauth2.RetrieveError{ErrorCode: "invalid_grant"}), signIn},
		{"network", errors.New("dial tcp: no route to host"), unreachable},
	}
	for _, tt := range tests {
		if got := trayProblem(tt.err); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
