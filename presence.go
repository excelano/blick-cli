package main

import (
	"encoding/json"
)

type presenceState struct {
	Availability string `json:"availability"`
	Activity     string `json:"activity"`
}

// getPresence reads the current /me/presence — both the user's effective
// availability and activity.
func (g *GraphClient) getPresence() (presenceState, error) {
	data, err := g.get("/me/presence", nil)
	if err != nil {
		return presenceState{}, err
	}
	var p presenceState
	if err := json.Unmarshal(data, &p); err != nil {
		return presenceState{}, err
	}
	return p, nil
}

// setPresenceSession registers our app as an active presence session, pinning
// availability/activity to the chosen values until the expirationDuration
// elapses. A session is what makes a user-preferred presence actually show
// (preferred presence with no session reads as Offline), so the `presence`
// command pairs this with setUserPreferredPresence for the online states.
//
// sessionId must be the app's client ID per the Graph spec.
func (g *GraphClient) setPresenceSession(sessionID, availability, activity, expirationISO8601 string) error {
	body := map[string]string{
		"sessionId":          sessionID,
		"availability":       availability,
		"activity":           activity,
		"expirationDuration": expirationISO8601,
	}
	_, err := g.post("/me/presence/setPresence", body)
	return err
}

// setUserPreferredPresence sets the user's manually chosen presence — the same
// override the Teams status picker writes. It takes precedence over
// session/app-computed presence and persists (until it expires or is changed)
// even with no Teams client running, which is why the manual `presence` command
// uses it rather than a per-session setPresence. An empty expiration omits the
// duration (Graph applies its default); otherwise expiration is an ISO 8601
// duration like "PT8H". Requires Presence.ReadWrite.
func (g *GraphClient) setUserPreferredPresence(availability, activity, expiration string) error {
	body := map[string]string{
		"availability": availability,
		"activity":     activity,
	}
	if expiration != "" {
		body["expirationDuration"] = expiration
	}
	_, err := g.post("/me/presence/setUserPreferredPresence", body)
	return err
}
