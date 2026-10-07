// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/excelano/atrest"
)

// sealForDisk seals plain under name unless atrest's key would not survive a
// reboot, as on a machine with no unlocked Secret Service. There the file is
// stored as plaintext at the caller's mode, because a sealed one would cost an
// interactive sign-in at every boot.
func sealForDisk(name string, plain []byte) ([]byte, error) {
	if !atrest.Persistent() {
		return plain, nil
	}
	return atrest.Seal(name, plain)
}

// unlockPromptWait bounds how long the keyring's prompt stays up unanswered
// before blick carries on without the file.
const unlockPromptWait = 2 * time.Minute

// unlockDeclined remembers that the keyring's prompt was dismissed or could not
// be shown, so the next sealed file this run opens does not raise it again.
var unlockDeclined bool

// openFromDisk opens what sealForDisk wrote. A file sealed under the login
// keyring while it was unlocked cannot be opened once the keyring is locked
// again, as after a reboot on a machine that signs in without a password. Then
// the user is told which file is waiting on the keyring, the desktop's own
// unlock prompt is raised, and the file is opened once it is answered. A prompt
// that is dismissed or cannot be shown leaves the file unopened, with an error
// that still reports atrest.ErrLocked for the caller to explain.
func openFromDisk(name, what string, stored []byte) (plain []byte, sealed bool, err error) {
	plain, sealed, err = atrest.Open(name, stored)
	if !errors.Is(err, atrest.ErrLocked) || unlockDeclined {
		return plain, sealed, err
	}
	fmt.Fprintf(os.Stderr, "Your %s is sealed under the login keyring, which is locked.\n", what)
	ctx, cancel := context.WithTimeout(context.Background(), unlockPromptWait)
	defer cancel()
	if unlockErr := atrest.Unlock(ctx); unlockErr != nil {
		unlockDeclined = true
		return nil, false, fmt.Errorf("%w (%v)", err, unlockErr)
	}
	return atrest.Open(name, stored)
}
