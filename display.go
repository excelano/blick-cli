package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	bold   = "\033[1m"
	dim    = "\033[2m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	green  = "\033[32m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

func relativeTime(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func untilTime(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return "now"
	}

	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "in 1 min"
		}
		return fmt.Sprintf("in %d min", m)
	default:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if h == 0 {
			return fmt.Sprintf("in %d min", m)
		}
		if m == 0 {
			if h == 1 {
				return "in 1 hour"
			}
			return fmt.Sprintf("in %d hours", h)
		}
		return fmt.Sprintf("in %dh%dm", h, m)
	}
}

func truncate(s string, maxLen int) string {
	// Replace newlines with spaces for single-line display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func renderDashboard(meeting *Meeting, emails []Email, emailErr error, chats []ChatMessage, chatErr error, enableTeams bool) {
	fmt.Println()

	// Meeting
	if meeting != nil {
		timeColor := green
		until := untilTime(meeting.Start)
		if time.Until(meeting.Start) < 15*time.Minute {
			timeColor = yellow
		}
		if time.Until(meeting.Start) < 0 {
			timeColor = red
			until = "now"
		}

		location := ""
		if meeting.Location != "" {
			location = " — " + meeting.Location
		} else if meeting.IsOnline {
			location = " — Online"
		}

		fmt.Printf("  📅 %s%s%s — %s%s%s%s\n\n",
			bold, meeting.Subject, reset,
			timeColor, until, reset,
			dim+location+reset)
	} else {
		fmt.Printf("  📅 %sNo upcoming meetings%s\n\n", dim, reset)
	}

	// Emails
	if emailErr != nil {
		fmt.Printf("  📧 %sCould not load emails: %v%s\n\n", red, emailErr, reset)
	} else if len(emails) == 0 {
		fmt.Printf("  📧 %sNo unread emails%s\n\n", dim, reset)
	} else {
		fmt.Printf("  📧 %sunread emails (%d):%s\n", bold, len(emails), reset)
		for i, e := range emails {
			fmt.Printf("    %s%d.%s %s — %q  %s(%s)%s\n",
				cyan, i+1, reset,
				e.From,
				truncate(e.Subject, 50),
				dim, relativeTime(e.Received), reset)
		}
		fmt.Println()
	}

	// Chats
	if !enableTeams {
		fmt.Printf("  💬 %sTeams disabled (set \"enable_teams\": true in config after admin consent)%s\n\n", dim, reset)
	} else if chatErr != nil {
		fmt.Printf("  💬 %sCould not load chats: %v%s\n\n", red, chatErr, reset)
	} else if len(chats) == 0 {
		fmt.Printf("  💬 %sNo pending chats%s\n\n", dim, reset)
	} else {
		offset := len(emails)
		fmt.Printf("  💬 %spending chats (%d):%s\n", bold, len(chats), reset)
		for i, c := range chats {
			preview := truncate(c.Preview, 40)
			fmt.Printf("    %s%d.%s %s — %q  %s(%s)%s\n",
				cyan, offset+i+1, reset,
				c.Topic,
				preview,
				dim, relativeTime(c.Sent), reset)
		}
		fmt.Println()
	}
}

func renderHelp() {
	fmt.Printf("  %sCommands:%s\n", bold, reset)
	fmt.Printf("    %s<N>%s      view message      %sr<N>%s     reply\n", cyan, reset, cyan, reset)
	fmt.Printf("    %sd<N>%s     mark read (done)   %sx%s       mark all read & quit\n", cyan, reset, cyan, reset)
	fmt.Printf("    %sr%s        refresh            %sq%s       quit\n", cyan, reset, cyan, reset)
	fmt.Println()
}
