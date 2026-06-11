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

func absoluteTime(t time.Time) string {
	now := time.Now()
	local := t.Local()

	if now.Year() == local.Year() && now.YearDay() == local.YearDay() {
		return local.Format("3:04 PM")
	}

	diff := local.Sub(now)
	if diff < 0 {
		diff = -diff
	}
	if diff < 7*24*time.Hour {
		return local.Format("Mon 3:04 PM")
	}
	return local.Format("Jan 2")
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
		return fmt.Sprintf("in %dh %dm", h, m)
	}
}

// flatten collapses a multi-line string onto one line by splitting on
// newlines, trimming each segment, and rejoining with sep. Outlook returns
// addresses in location.displayName as "Street\nCity, ST Zip\nCountry";
// passing sep=", " produces a properly punctuated single-line address.
// Use sep=" " for free-form text like meeting titles.
func flatten(s, sep string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var out []string
	for _, p := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, sep)
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

		fmt.Printf("  📅 %s%s%s — %s%s%s %s· %s%s%s\n\n",
			bold, meeting.Subject, reset,
			timeColor, until, reset,
			dim, absoluteTime(meeting.Start), location, reset)
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
			fmt.Printf("    %s%d.%s %s — %q  %s(%s · %s)%s\n",
				cyan, i+1, reset,
				e.From,
				truncate(e.Subject, 50),
				dim, relativeTime(e.Received), absoluteTime(e.Received), reset)
		}
		fmt.Println()
	}

	// Chats
	if !enableTeams {
		fmt.Printf("  💬 %sTeams disabled (set \"enable_teams\": true in config to enable)%s\n\n", dim, reset)
	} else if chatErr != nil {
		fmt.Printf("  💬 %sCould not load chats: %v%s\n\n", red, chatErr, reset)
	} else if len(chats) == 0 {
		fmt.Printf("  💬 %sNo unread chats%s\n\n", dim, reset)
	} else {
		offset := len(emails)
		fmt.Printf("  💬 %sunread chats (%d):%s\n", bold, len(chats), reset)
		for i, c := range chats {
			preview := truncate(c.Preview, 40)
			fmt.Printf("    %s%d.%s %s — %q  %s(%s · %s)%s\n",
				cyan, offset+i+1, reset,
				c.Topic,
				preview,
				dim, relativeTime(c.Sent), absoluteTime(c.Sent), reset)
		}
		fmt.Println()
	}
}

func renderHelp() {
	rows := [][4]string{
		{"<N>", "view", "r<N>", "reply"},
		{"d<N>", "done", "r", "refresh"},
		{"t", "show today", "x", "exit (mark all read)"},
		{"H", "help", "q", "quit"},
	}
	fmt.Printf("  %sCommands:%s\n", bold, reset)
	for _, r := range rows {
		fmt.Printf("    %s%-8s%s %-18s %s%-8s%s %s\n",
			cyan, r[0], reset, r[1],
			cyan, r[2], reset, r[3])
	}
	fmt.Println()
}

func renderFullHelp() {
	fmt.Println()
	fmt.Printf("  %sCommands:%s\n\n", bold, reset)
	fmt.Printf("    %s%-8s  %-13s  %s%s\n", dim, "Short", "Long", "What it does", reset)
	rows := []struct{ short, long, desc string }{
		{"<N>", "view N", "Open the Nth item from the list"},
		{"r<N>", "reply N", "Reply to the Nth item (ed-style editor)"},
		{"d<N>", "done N", "Mark the Nth item as read"},
		{"", "email <c>", "Compose a new message to contact <c>"},
		{"", "chat <c>", "Send a Teams chat to contact <c>"},
		{"r", "refresh", "Reload the dashboard"},
		{"t", "today", "Show today's calendar"},
		{"x", "exit", "Mark all items as read & quit"},
		{"H", "help", "Show this help"},
		{"q", "quit", "Quit"},
	}
	for _, r := range rows {
		fmt.Printf("    %s%-8s%s  %s%-13s%s  %s\n",
			cyan, r.short, reset,
			cyan, r.long, reset,
			r.desc)
	}
	fmt.Println()
}

func renderToday(events []Meeting) {
	fmt.Println()
	today := time.Now().Local()
	fmt.Printf("  %s%s%s\n\n", bold, today.Format("Monday, January 2, 2006"), reset)

	if len(events) == 0 {
		fmt.Printf("  %sNo meetings today%s\n\n", dim, reset)
		return
	}

	var allDay, timed []Meeting
	for _, e := range events {
		if e.IsAllDay {
			allDay = append(allDay, e)
		} else {
			timed = append(timed, e)
		}
	}

	for _, e := range allDay {
		fmt.Printf("    %sall day%s              %s\n", dim, reset, e.Subject)
	}

	now := time.Now()
	var totalDuration time.Duration

	for _, e := range timed {
		startStr := e.Start.Local().Format("3:04 PM")
		endStr := e.End.Local().Format("3:04 PM")
		timeRange := fmt.Sprintf("%8s – %-8s", startStr, endStr)

		subject := e.Subject
		var style string
		switch {
		case e.End.Before(now):
			style = dim
		case e.Start.Before(now) || e.Start.Equal(now):
			style = bold
			subject = subject + " · now"
		}

		loc := ""
		if e.IsOnline {
			loc = "Online"
		} else if e.Location != "" {
			loc = e.Location
		}

		line := fmt.Sprintf("    %-19s  %-30s  %s", timeRange, subject, loc)
		line = strings.TrimRight(line, " ")

		if style != "" {
			fmt.Printf("%s%s%s\n", style, line, reset)
		} else {
			fmt.Println(line)
		}

		totalDuration += e.End.Sub(e.Start)
	}

	noun := "events"
	if len(events) == 1 {
		noun = "event"
	}
	fmt.Printf("\n  %s%d %s · %s scheduled%s\n\n",
		dim, len(events), noun, formatDuration(totalDuration), reset)
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
