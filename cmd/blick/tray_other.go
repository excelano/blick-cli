// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

//go:build !linux

package main

import (
	"fmt"
	"os"
)

func runTray(Config, []string) {
	fmt.Fprintln(os.Stderr, "blick tray is available on Linux only.")
	os.Exit(1)
}
