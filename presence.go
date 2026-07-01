package main

import (
	"encoding/json"
	"fmt"
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
// elapses. Microsoft aggregates across sessions with the precedence
// DoNotDisturb > Busy > Available > Away, so our Available beats Teams'
// idle-driven Away while user-preferred state still wins overall.
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

// maybeHeartbeatPresence implements the "nudge Away → Available" behavior. If
// presence_heartbeat is on and the user is currently Away, register an
// Available session for one hour. Best-effort: any failure is reported but
// does not block the rest of the dashboard.
func maybeHeartbeatPresence(client *GraphClient, cfg Config) {
	if !cfg.PresenceHeartbeat {
		return
	}
	p, err := client.getPresence()
	if err != nil {
		fmt.Printf("  %s(presence: could not read current state: %v)%s\n", dim, err, reset)
		return
	}
	if p.Availability != "Away" {
		return
	}
	if err := client.setPresenceSession(cfg.ClientID, "Available", "Available", "PT1H"); err != nil {
		fmt.Printf("  %s(presence: could not nudge to Available: %v)%s\n", dim, err, reset)
		return
	}
	// "nudged", not "switched": a manually set preferred Away takes precedence
	// over this session, so we can't confirm the effective status actually
	// moved — only that the Available session was registered.
	fmt.Printf("  %s(presence: nudged Away → Available for 1h)%s\n", dim, reset)
}
