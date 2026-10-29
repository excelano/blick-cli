// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

//go:build linux

package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed blick-tray.desktop
var trayDesktopEntry []byte

func autostartPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "autostart", "blick-tray.desktop")
}

func setAutostart(choice string) error {
	switch choice {
	case "yes":
		if err := os.MkdirAll(filepath.Dir(autostartPath()), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(autostartPath(), trayDesktopEntry, 0644); err != nil {
			return err
		}
		fmt.Println("blick tray will start at login.")
	case "no":
		if err := os.Remove(autostartPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fmt.Println("blick tray will no longer start at login.")
	default:
		return errors.New("use blick tray --autostart yes, or --autostart no")
	}
	return nil
}
