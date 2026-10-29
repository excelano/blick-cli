// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

//go:build linux

package main

import (
	"bytes"
	"os"
	"testing"
)

func TestAutostartYesThenNo(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := setAutostart("yes"); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(autostartPath())
	if err != nil || !bytes.Equal(written, trayDesktopEntry) {
		t.Fatalf("autostart entry not written: %v", err)
	}
	if !bytes.Contains(trayDesktopEntry, []byte("Exec=blick tray --foreground")) {
		t.Error("the embedded entry does not start the tray")
	}
	for range 2 {
		if err := setAutostart("no"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(autostartPath()); !os.IsNotExist(err) {
		t.Fatalf("autostart entry still there: %v", err)
	}
	if err := setAutostart("maybe"); err == nil {
		t.Error("an unknown choice was accepted")
	}
}
