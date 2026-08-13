package main

import "testing"

func TestLookupPresence(t *testing.T) {
	// Every advertised keyword resolves to a non-empty Graph pair.
	for _, o := range presenceOptions {
		got, ok := lookupPresence(o.key)
		if !ok {
			t.Errorf("lookupPresence(%q) not found", o.key)
			continue
		}
		if got.availability == "" || got.activity == "" {
			t.Errorf("%q maps to empty pair: %+v", o.key, got)
		}
	}

	if _, ok := lookupPresence("bogus"); ok {
		t.Error("lookupPresence(\"bogus\") should not be found")
	}
	// Case is normalized by the caller, so lookup itself is exact-match.
	if _, ok := lookupPresence("Busy"); ok {
		t.Error("lookupPresence is exact-match; \"Busy\" should miss")
	}
}

func TestPresenceOptionsCoverExpectedStates(t *testing.T) {
	want := []string{"available", "busy", "dnd", "brb", "away", "offline"}
	if len(presenceOptions) != len(want) {
		t.Fatalf("got %d options, want %d", len(presenceOptions), len(want))
	}
	for i, w := range want {
		if presenceOptions[i].key != w {
			t.Errorf("option[%d] = %q, want %q", i, presenceOptions[i].key, w)
		}
	}
}

func TestPresenceLabel(t *testing.T) {
	cases := []struct {
		name string
		in   presenceState
		want string
	}{
		{"activity same as availability", presenceState{"Available", "Available"}, "Available"},
		{"activity adds info", presenceState{"Busy", "InAMeeting"}, "Busy (InAMeeting)"},
		{"no activity", presenceState{"Away", ""}, "Away"},
		{"empty is unknown", presenceState{"", ""}, "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := presenceLabel(c.in); got != c.want {
				t.Errorf("presenceLabel(%+v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
