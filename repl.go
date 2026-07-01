package main

import (
	"path/filepath"

	"github.com/chzyer/readline"
)

// Single readline.Instance for the whole interactive session. Two configs
// swap in via rl.SetConfig: promptConfig at the top-level "blick> " prompt
// (history file + autocompleter), bodyConfig for the inline .-sentinel
// composers in replyTo / replEmail / replChat (no history, no completion).
// The body-mode switch keeps quoted bodies and one-off drafts out of the
// persisted history file, which the user explicitly wants.
var (
	rl           *readline.Instance
	promptConfig *readline.Config
	bodyConfig   *readline.Config
)

// identityPainter is a no-op Painter for bodyConfig. readline's NewEx
// auto-installs a default painter when its initial config has none, but
// SetConfig does not — so a config that arrives only via SetConfig keeps
// a nil Painter and RuneBuffer.output crashes on first keystroke. We
// supply our own so the body config is whole on arrival.
type identityPainter struct{}

func (identityPainter) Paint(line []rune, _ int) []rune { return line }

// replVerbs is the verb table for tab completion at REPL position 0.
// Single-letter aliases are included so e/c/r/t/j/x/H/q complete the
// same way as their long forms. Numeric forms (<N>, <N>r, <N>d) are
// deliberately excluded — completion can't usefully suggest live item
// numbers from the dashboard.
var replVerbs = []string{
	"view", "reply", "done",
	"attach",
	"inbox", "i",
	"email", "e",
	"chat", "c",
	"refresh", "r",
	"today", "t",
	"join", "j",
	"exit", "x",
	"help", "H",
	"quit", "q",
}

// setupReadline builds the singleton readline.Instance with the prompt
// config and pre-builds the body config. contactKeys is snapshotted once
// at REPL start; later `contacts add` in this session won't show in
// completion until the next launch — confirmed acceptable.
func setupReadline(historyPath string, contactKeys []string) error {
	promptConfig = &readline.Config{
		Prompt:            bold + "blick> " + reset,
		HistoryFile:       historyPath,
		HistoryLimit:      1000,
		HistorySearchFold: true,
		AutoComplete:      newReplCompleter(contactKeys),
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
	}
	bodyConfig = &readline.Config{
		Prompt:                 "  " + cyan + "> " + reset,
		DisableAutoSaveHistory: true,
		AutoComplete:           nil,
		InterruptPrompt:        "^C",
		Painter:                identityPainter{},
	}
	var err error
	rl, err = readline.NewEx(promptConfig)
	return err
}

// enterBodyMode swaps the singleton readline.Instance to body config.
// Callers must defer exitBodyMode to restore the top-level prompt.
func enterBodyMode() {
	_ = rl.SetConfig(bodyConfig)
}

func exitBodyMode() {
	_ = rl.SetConfig(promptConfig)
}

// readBodyDraft loops on rl.Readline() under body-mode config until the
// user types "." on a line by itself. Returns the joined body (un-
// trimmed; trimming is the caller's choice) and true. Returns "" and
// false on Ctrl-C or EOF — caller renders "(cancelled)" or similar.
//
// Body mode must already be active before calling. The .-sentinel
// protocol matches the legacy stdinLines-fed loop byte for byte: each
// line is appended; "." closes the draft; mid-draft Ctrl-C/EOF cancels.
func readBodyDraft() (string, bool) {
	var lines []string
	for {
		line, err := rl.Readline()
		if err != nil {
			return "", false
		}
		if line == "." {
			return joinLines(lines), true
		}
		lines = append(lines, line)
	}
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for _, l := range lines[1:] {
		out += "\n" + l
	}
	return out
}

// newReplCompleter wires verb completion at position 0 and contact
// handle completion at position 1 for the email/chat verbs (full forms
// and aliases). Numeric/short item-keyed forms get no completion —
// they'd flood the suggestion list with no useful filtering.
func newReplCompleter(contactKeys []string) readline.AutoCompleter {
	contactPC := make([]readline.PrefixCompleterInterface, len(contactKeys))
	for i, k := range contactKeys {
		contactPC[i] = readline.PcItem(k)
	}
	composeVerbs := map[string]bool{
		"email": true, "e": true,
		"chat": true, "c": true,
	}
	items := make([]readline.PrefixCompleterInterface, 0, len(replVerbs))
	for _, v := range replVerbs {
		if composeVerbs[v] {
			items = append(items, readline.PcItem(v, contactPC...))
		} else {
			items = append(items, readline.PcItem(v))
		}
	}
	return readline.NewPrefixCompleter(items...)
}

// loadContactKeys snapshots address-book keys for tab completion.
// A failure to load is non-fatal: the REPL still boots, completion just
// doesn't include contacts. The user can always type the handle in full
// or fix contacts.json by hand.
func loadContactKeys() []string {
	store, err := LoadContacts()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(store.Contacts))
	for _, c := range store.Sorted() {
		out = append(out, c.Key)
	}
	return out
}

func replHistoryPath() string {
	return filepath.Join(configDir(), "history")
}
