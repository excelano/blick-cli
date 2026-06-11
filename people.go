package main

import (
	"encoding/json"
	"net/url"
)

// PersonCandidate is a thin projection of /me/people response entries —
// just the bits we need to propose contact rows. Multiple email addresses
// per person collapse to the first one returned by Graph (typically the
// primary work address).
type PersonCandidate struct {
	DisplayName string
	Email       string
}

// RelevantPeople returns the user's relevance-ranked people list from
// Microsoft Graph: contacts you interact with across mail, calendar, and
// Teams. Limited to 50 so the seed preview is scannable in one screen.
// Entries without an email address (room mailboxes, distribution lists
// without SMTP, unresolved attendees) are filtered out.
func (g *GraphClient) RelevantPeople() ([]PersonCandidate, error) {
	query := url.Values{
		"$top":    {"50"},
		"$select": {"displayName,scoredEmailAddresses"},
	}
	data, err := g.get("/me/people", query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
			DisplayName           string `json:"displayName"`
			ScoredEmailAddresses []struct {
				Address string `json:"address"`
			} `json:"scoredEmailAddresses"`
		} `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	out := make([]PersonCandidate, 0, len(result.Value))
	for _, p := range result.Value {
		if p.DisplayName == "" || len(p.ScoredEmailAddresses) == 0 {
			continue
		}
		email := p.ScoredEmailAddresses[0].Address
		if email == "" {
			continue
		}
		out = append(out, PersonCandidate{
			DisplayName: p.DisplayName,
			Email:       email,
		})
	}
	return out, nil
}
