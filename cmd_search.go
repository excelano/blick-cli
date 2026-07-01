package main

import (
	"fmt"
	"os"
	"strings"
)

// searchEmailTop caps how many matches a search pulls. Graph ranks $search by
// relevance, so the cap keeps the most relevant; the view notes when it's hit.
const searchEmailTop = 25

const searchUsage = "search [--from X] [--subject X] [--text X] [words...]"

// searchQuery is the parsed form of a `search` command line: optional
// from/subject restrictions plus free-text terms.
type searchQuery struct {
	from    string
	subject string
	text    []string
}

// parseSearchArgs reads --from/--subject/--text (with -f/-s/-t short forms);
// bare words become free-text terms. Each flag consumes the next arg.
func parseSearchArgs(args []string) (searchQuery, error) {
	var sq searchQuery
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from", "-f":
			if i+1 >= len(args) {
				return searchQuery{}, fmt.Errorf("--from requires a value")
			}
			sq.from = args[i+1]
			i++
		case "--subject", "-s":
			if i+1 >= len(args) {
				return searchQuery{}, fmt.Errorf("--subject requires a value")
			}
			sq.subject = args[i+1]
			i++
		case "--text", "-t":
			if i+1 >= len(args) {
				return searchQuery{}, fmt.Errorf("--text requires a value")
			}
			sq.text = append(sq.text, args[i+1])
			i++
		default:
			sq.text = append(sq.text, args[i])
		}
	}
	return sq, nil
}

// kqlTerm formats one KQL term. A value with whitespace is phrase-quoted so it
// stays a single term; an empty prop yields a bare free-text term. The whole
// KQL string is later wrapped in double quotes for $search, so any backslash
// or double quote in the value is escaped first — otherwise an inner quote
// closes the $search string early and corrupts (or breaks) the query.
func kqlTerm(prop, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	phrase := strings.ContainsAny(value, " \t")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	if phrase {
		value = `"` + value + `"`
	}
	if prop == "" {
		return value
	}
	return prop + ":" + value
}

// kql builds the space-joined KQL expression Graph's $search takes. Terms AND
// implicitly, so `from:bob invoice` finds mail from bob mentioning invoice.
func (sq searchQuery) kql() string {
	var terms []string
	if t := kqlTerm("from", sq.from); t != "" {
		terms = append(terms, t)
	}
	if t := kqlTerm("subject", sq.subject); t != "" {
		terms = append(terms, t)
	}
	for _, x := range sq.text {
		if t := kqlTerm("", x); t != "" {
			terms = append(terms, t)
		}
	}
	return strings.Join(terms, " ")
}

func (sq searchQuery) isEmpty() bool {
	return sq.kql() == ""
}

// runSearch handles the one-shot `blick search ...`: render matches and exit.
func runSearch(client *GraphClient, args []string) {
	sq, err := parseSearchArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if sq.isEmpty() {
		fmt.Fprintf(os.Stderr, "Usage: blick %s\n", searchUsage)
		os.Exit(1)
	}
	fetchSearch(client, sq)
}

// replSearch is the REPL-side `search ...`. Like the inbox, it returns the
// matched items so view/reply/done/attach/forward target them; a bad or empty
// query prints and returns prev unchanged so the current numbering holds.
func replSearch(client *GraphClient, args []string, prev []Item) []Item {
	sq, err := parseSearchArgs(args)
	if err != nil {
		fmt.Printf("  %s%v%s\n", red, err, reset)
		return prev
	}
	if sq.isEmpty() {
		fmt.Printf("  Usage: %s%s%s\n", cyan, searchUsage, reset)
		return prev
	}
	return fetchSearch(client, sq)
}

// fetchSearch runs the query, renders the matches, and returns them as items.
func fetchSearch(client *GraphClient, sq searchQuery) []Item {
	emails, err := client.SearchEmails(sq.kql())
	renderSearch(sq.kql(), emails, err)
	items := make([]Item, 0, len(emails))
	for i := range emails {
		items = append(items, Item{Kind: "email", Email: &emails[i]})
	}
	return items
}
