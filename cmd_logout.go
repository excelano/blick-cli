package main

import (
	"fmt"
	"os"
)

// runLogout deletes the cached OAuth token so the next `blick` run
// triggers the device-code flow again. Config (client_id, tenant_id,
// feature flags) and the address book are left alone — this is a
// session reset, not a full wipe.
func runLogout() {
	path := tokenPath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: could not remove %s: %v\n", path, err)
		os.Exit(1)
	}
	if os.IsNotExist(err) {
		fmt.Println("Already logged out.")
		return
	}
	fmt.Println("Logged out. The next `blick` run will prompt for the device code again.")
}
