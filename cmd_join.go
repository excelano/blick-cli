package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// runJoin handles `blick join` from the shell.
func runJoin(client *GraphClient) {
	if err := joinMeeting(client, shellJoinReport); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// replJoin is the REPL-side entry.
func replJoin(client *GraphClient) {
	if err := joinMeeting(client, replJoinReport); err != nil {
		fmt.Printf("  %sError: %v%s\n", red, err, reset)
	}
}

// joinReporter abstracts the user-facing output for shell vs REPL so the
// shared flow can println / Printf with the right prefix.
type joinReporter func(format string, args ...interface{})

func shellJoinReport(format string, args ...interface{}) { fmt.Printf(format+"\n", args...) }
func replJoinReport(format string, args ...interface{})  { fmt.Printf("  "+format+"\n", args...) }

// joinMeeting fetches today's meetings, picks the current one (in
// progress, online, with a join URL) or the next upcoming one matching
// the same predicate, and opens the join URL in the system browser.
// Returns nil on success or on user-facing "nothing to join" cases
// (already reported); returns an error only for unexpected failures.
func joinMeeting(client *GraphClient, report joinReporter) error {
	meetings, err := client.TodaysMeetings()
	if err != nil {
		return err
	}

	m := pickJoinableMeeting(meetings, time.Now())
	if m == nil {
		report("No online meeting to join in the rest of today.")
		return nil
	}

	report("Opening: %s — %s", m.Subject, m.Start.Format("3:04 PM"))
	if err := openURL(m.JoinURL); err != nil {
		report("Could not open browser: %v", err)
		report("URL: %s", m.JoinURL)
	}
	return nil
}

// pickJoinableMeeting prefers a meeting that is currently in progress
// (start <= now < end) over an upcoming one. Within each group it picks
// the earliest by start time. Only considers meetings with IsOnline +
// JoinURL set, since those are the only ones we can route to.
func pickJoinableMeeting(meetings []Meeting, now time.Time) *Meeting {
	var current *Meeting
	var next *Meeting
	for i := range meetings {
		m := &meetings[i]
		if !m.IsOnline || m.JoinURL == "" {
			continue
		}
		if !m.Start.After(now) && m.End.After(now) {
			if current == nil || m.Start.Before(current.Start) {
				current = m
			}
			continue
		}
		if m.Start.After(now) {
			if next == nil || m.Start.Before(next.Start) {
				next = m
			}
		}
	}
	if current != nil {
		return current
	}
	return next
}

// openURL hands the URL to the OS's browser launcher: rundll32 on
// Windows, `open` on macOS, `xdg-open` everywhere else. Caller is
// expected to print the URL on error so the user can paste it manually.
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
