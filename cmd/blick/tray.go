// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/oauth2"
)

const trayPollInterval = 2 * time.Minute

// trayBusName is held on the session bus while a tray runs, so a second
// `blick tray`, as from autostart plus a manual start, exits instead of adding
// a second icon.
const trayBusName = "com.excelano.blick.Tray"

// runTray handles `blick tray`: a StatusNotifierItem showing the unread count
// and next meeting, polled every trayPollInterval. On GNOME it needs the
// AppIndicator extension to be seen; without a host it runs and waits for one.
// It starts itself in the background and returns, unless --foreground keeps
// it attached to the terminal.
func runTray(cfg Config, args []string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: the tray needs a desktop session: %v\n", err)
		os.Exit(1)
	}
	if len(args) > 0 && args[0] == "--foreground" {
		serveTray(cfg, conn)
		return
	}
	var running bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, trayBusName).Store(&running); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if running {
		fmt.Fprintln(os.Stderr, "blick tray is already running.")
		os.Exit(1)
	}
	logPath, err := startTrayInBackground()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: starting the tray: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("blick tray started. Log: %s\n", logPath)
}

// startTrayInBackground runs `blick tray --foreground` in its own session, so
// closing the terminal does not end it, with its output in a log that each
// start begins afresh.
func startTrayInBackground() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "blick")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	logPath := filepath.Join(dir, "tray.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer logFile.Close()
	cmd := exec.Command(self, "tray", "--foreground")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	return logPath, cmd.Process.Release()
}

func serveTray(cfg Config, conn *dbus.Conn) {
	reply, err := conn.RequestName(trayBusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		fmt.Fprintln(os.Stderr, "blick tray is already running.")
		os.Exit(1)
	}
	item, err := newStatusItem(conn, "blick", "blick")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	t := &tray{
		cfg:        cfg,
		item:       item,
		state:      trayState{look: trayStale, status: "Checking…"},
		refreshNow: make(chan struct{}, 1),
	}
	t.run()
}

// The menu's entries, by index.
const (
	entryStatus = iota
	entryMeeting
	entryAfterMeeting
	entryOpen
	entryJoin
	entryRefresh
	entryBeforeQuit
	entryQuit
)

// trayState is everything the icon and menu show. meeting is empty when there
// is no meeting line to show, and joinURL when there is nothing to join.
type trayState struct {
	look    trayLook
	status  string
	meeting string
	joinURL string
}

type tray struct {
	cfg        Config
	item       *statusItem
	state      trayState
	refreshNow chan struct{}
}

// run owns the tray's state: it redraws for each poll and answers each click,
// while poll fetches in the background so a click never waits on Graph.
func (t *tray) run() {
	states := make(chan trayState)
	go poll(t.cfg, states, t.refreshNow)
	t.redraw()
	for {
		select {
		case s := <-states:
			t.state = s
			t.redraw()
		case entry := <-t.item.clicks:
			switch entry {
			case entryOpen:
				t.openDashboard()
			case entryJoin:
				if err := openURL(t.state.joinURL); err != nil {
					t.note("Couldn't open the browser", err)
				}
			case entryRefresh:
				select {
				case t.refreshNow <- struct{}{}:
				default:
				}
			case entryQuit:
				return
			}
		}
	}
}

func (t *tray) redraw() {
	s := t.state
	t.item.setIcon(s.look)
	t.item.setToolTip(s.status)
	t.item.setMenu([]menuEntry{
		entryStatus:       {label: s.status},
		entryMeeting:      {label: s.meeting, hidden: s.meeting == ""},
		entryAfterMeeting: {separator: true},
		entryOpen:         {label: "Open blick", enabled: true},
		entryJoin:         {label: "Join meeting", enabled: s.joinURL != ""},
		entryRefresh:      {label: "Refresh", enabled: true},
		entryBeforeQuit:   {separator: true},
		entryQuit:         {label: "Quit", enabled: true},
	})
}

// note puts summary in place of the counts until the next poll. The error
// itself goes to stderr, which lands in the log or, under autostart, the
// journal.
func (t *tray) note(summary string, err error) {
	fmt.Fprintf(os.Stderr, "blick tray: %v\n", err)
	t.state.status = summary
	t.redraw()
}

// openDashboard starts the dashboard in $TERMINAL, or Debian's
// x-terminal-emulator when that is unset.
func (t *tray) openDashboard() {
	self, err := os.Executable()
	if err != nil {
		t.note("Couldn't find blick", err)
		return
	}
	term := os.Getenv("TERMINAL")
	if term == "" {
		term = "x-terminal-emulator"
	}
	cmd := exec.Command(term, "-e", self)
	if err := cmd.Start(); err != nil {
		t.note("Couldn't open a terminal", err)
		return
	}
	go cmd.Wait()
}

// poll sends a fresh trayState every trayPollInterval, and at once on
// refreshNow.
func poll(cfg Config, states chan<- trayState, refreshNow <-chan struct{}) {
	p := trayPoller{cfg: cfg}
	ticker := time.NewTicker(trayPollInterval)
	for {
		states <- p.fetch()
		select {
		case <-ticker.C:
		case <-refreshNow:
		}
	}
}

type trayPoller struct {
	cfg    Config
	client *GraphClient // nil until signed in, and again after a failed poll
}

// fetch reads the counts and next meeting. A failure fades the icon and drops
// the client, so the next poll starts again from the cached token, which picks
// up a sign-in made since in a terminal.
func (p *trayPoller) fetch() trayState {
	if p.client == nil {
		client, err := trayClient(p.cfg)
		if err != nil {
			return problemState(err)
		}
		p.client = client
	}

	d := fetchDashboard(p.client, p.cfg.EnableTeams, false)
	if err := errors.Join(d.emailErr, d.chatErr); err != nil {
		p.client = nil
		return problemState(err)
	}

	s := trayState{
		look:    trayIdle,
		status:  trayStatus(len(d.emails), len(d.chats)),
		meeting: trayMeetingLine(d.meeting),
	}
	if len(d.emails)+len(d.chats) > 0 {
		s.look = trayUnread
	}
	if d.meeting != nil {
		s.joinURL = d.meeting.JoinURL
	}
	return s
}

func problemState(err error) trayState {
	fmt.Fprintf(os.Stderr, "blick tray: %v\n", err)
	return trayState{look: trayStale, status: trayProblem(err)}
}

// trayClient signs in from the cached token alone, since the tray has no
// terminal for the device-code flow.
func trayClient(cfg Config) (*GraphClient, error) {
	tok, err := refreshCachedToken(cfg)
	if err != nil {
		return nil, err
	}
	return NewGraphClient(cfg, tok)
}

// trayProblem names a failed poll for the menu: no cached token, or one
// Microsoft refused, needs a sign-in; anything else is taken as unreachable.
func trayProblem(err error) string {
	var refused *oauth2.RetrieveError
	if errors.Is(err, os.ErrNotExist) || errors.As(err, &refused) {
		return "Not signed in — open blick to sign in"
	}
	return "Couldn't reach Microsoft — retrying"
}

// trayStatus is the menu's count line, e.g. "3 unread emails · 1 unread chat".
func trayStatus(emails, chats int) string {
	var parts []string
	if emails > 0 {
		parts = append(parts, plural(emails, "unread email"))
	}
	if chats > 0 {
		parts = append(parts, plural(chats, "unread chat"))
	}
	if len(parts) == 0 {
		return "Nothing unread"
	}
	return strings.Join(parts, " · ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// trayMeetingLine words the next meeting the way the dashboard does.
func trayMeetingLine(m *Meeting) string {
	if m == nil {
		return "No upcoming meetings"
	}
	until := untilTime(m.Start)
	d := time.Until(m.Start)
	if d < 3*time.Minute && d >= 0 {
		until = "Starting soon"
	}
	if d < 0 {
		until = "now"
	}
	return fmt.Sprintf("%s — %s · %s", truncate(m.Subject, 40), until, absoluteTime(m.Start))
}
