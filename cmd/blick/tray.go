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
	"sync"
	"syscall"
	"time"

	"fyne.io/systray"
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
	// The menu is not served until onReady returns, so the loops run apart
	// from it.
	systray.Run(func() { go newTray(cfg).run() }, nil)
}

type tray struct {
	cfg    Config
	client *GraphClient // nil until signed in, and again after a failed poll

	status  *systray.MenuItem
	meeting *systray.MenuItem
	open    *systray.MenuItem
	join    *systray.MenuItem
	refresh *systray.MenuItem
	quit    *systray.MenuItem

	refreshNow chan struct{}

	mu      sync.Mutex
	joinURL string
}

func newTray(cfg Config) *tray {
	systray.SetIcon(renderTrayIcon(false, true))
	systray.SetTitle("blick")
	t := &tray{
		cfg:        cfg,
		status:     systray.AddMenuItem("Checking…", ""),
		meeting:    systray.AddMenuItem("", ""),
		refreshNow: make(chan struct{}, 1),
	}
	t.status.Disable()
	t.meeting.Disable()
	t.meeting.Hide()
	systray.AddSeparator()
	t.open = systray.AddMenuItem("Open blick", "")
	t.join = systray.AddMenuItem("Join meeting", "")
	t.join.Disable()
	t.refresh = systray.AddMenuItem("Refresh", "")
	systray.AddSeparator()
	t.quit = systray.AddMenuItem("Quit", "")
	return t
}

// run answers menu clicks while poll fetches in the background. The menu
// drops a click that nothing is waiting for, so this loop never blocks on
// Graph.
func (t *tray) run() {
	go t.poll()
	for {
		select {
		case <-t.refresh.ClickedCh:
			select {
			case t.refreshNow <- struct{}{}:
			default:
			}
		case <-t.open.ClickedCh:
			t.openDashboard()
		case <-t.join.ClickedCh:
			t.mu.Lock()
			url := t.joinURL
			t.mu.Unlock()
			if err := openURL(url); err != nil {
				t.showProblem("Couldn't open the browser", err)
			}
		case <-t.quit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (t *tray) poll() {
	ticker := time.NewTicker(trayPollInterval)
	for {
		t.update()
		select {
		case <-ticker.C:
		case <-t.refreshNow:
		}
	}
}

// update fetches the counts and next meeting and redraws. A failure greys the
// icon and drops the client, so the next poll starts again from the cached
// token, which picks up a sign-in made since in a terminal.
func (t *tray) update() {
	if t.client == nil {
		client, err := trayClient(t.cfg)
		if err != nil {
			t.showProblem(trayProblem(err), err)
			return
		}
		t.client = client
	}

	d := fetchDashboard(t.client, t.cfg.EnableTeams, false)
	if err := errors.Join(d.emailErr, d.chatErr); err != nil {
		t.client = nil
		t.showProblem(trayProblem(err), err)
		return
	}

	status := trayStatus(len(d.emails), len(d.chats))
	systray.SetIcon(renderTrayIcon(len(d.emails)+len(d.chats) > 0, false))
	systray.SetTooltip(status)
	t.status.SetTitle(status)
	t.meeting.SetTitle(trayMeetingLine(d.meeting))
	t.meeting.Show()

	t.mu.Lock()
	t.joinURL = ""
	if d.meeting != nil {
		t.joinURL = d.meeting.JoinURL
	}
	if t.joinURL == "" {
		t.join.Disable()
	} else {
		t.join.Enable()
	}
	t.mu.Unlock()
}

// showProblem greys the icon and puts summary in place of the counts. The
// error itself goes to stderr, which lands in the journal under autostart.
func (t *tray) showProblem(summary string, err error) {
	fmt.Fprintf(os.Stderr, "blick tray: %v\n", err)
	systray.SetIcon(renderTrayIcon(false, true))
	systray.SetTooltip(summary)
	t.status.SetTitle(summary)
}

// openDashboard starts the dashboard in $TERMINAL, or Debian's
// x-terminal-emulator when that is unset.
func (t *tray) openDashboard() {
	self, err := os.Executable()
	if err != nil {
		t.showProblem("Couldn't find blick", err)
		return
	}
	term := os.Getenv("TERMINAL")
	if term == "" {
		term = "x-terminal-emulator"
	}
	cmd := exec.Command(term, "-e", self)
	if err := cmd.Start(); err != nil {
		t.showProblem("Couldn't open a terminal", err)
		return
	}
	go cmd.Wait()
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
