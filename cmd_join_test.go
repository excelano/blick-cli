package main

import (
	"testing"
	"time"
)

func TestPickJoinableMeeting(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	at := func(h, m int) time.Time { return time.Date(2026, 6, 12, h, m, 0, 0, time.UTC) }

	mk := func(subject string, start, end time.Time, online bool, url string) Meeting {
		return Meeting{Subject: subject, Start: start, End: end, IsOnline: online, JoinURL: url}
	}

	tests := []struct {
		name string
		in   []Meeting
		want string // subject of expected pick, or "" for nil
	}{
		{"empty list", nil, ""},
		{
			"current in-progress online meeting wins over upcoming",
			[]Meeting{
				mk("standup", at(9, 30), at(10, 15), true, "https://teams/standup"),
				mk("design review", at(10, 30), at(11, 0), true, "https://teams/design"),
			},
			"standup",
		},
		{
			"no current, pick next upcoming online",
			[]Meeting{
				mk("past", at(8, 0), at(9, 0), true, "https://teams/past"),
				mk("design review", at(10, 30), at(11, 0), true, "https://teams/design"),
				mk("late", at(15, 0), at(16, 0), true, "https://teams/late"),
			},
			"design review",
		},
		{
			"skip non-online meetings even if current",
			[]Meeting{
				mk("in-person", at(9, 30), at(10, 30), false, ""),
				mk("design review", at(10, 30), at(11, 0), true, "https://teams/design"),
			},
			"design review",
		},
		{
			"skip online meetings with no join URL",
			[]Meeting{
				mk("brokered", at(9, 30), at(10, 30), true, ""),
				mk("design review", at(10, 30), at(11, 0), true, "https://teams/design"),
			},
			"design review",
		},
		{
			"no joinable meeting at all",
			[]Meeting{
				mk("in-person past", at(8, 0), at(9, 0), false, ""),
				mk("in-person future", at(14, 0), at(15, 0), false, ""),
			},
			"",
		},
		{
			"earliest current wins on tie",
			[]Meeting{
				mk("later start", at(9, 45), at(10, 30), true, "https://teams/b"),
				mk("earlier start", at(9, 30), at(10, 30), true, "https://teams/a"),
			},
			"earlier start",
		},
		{
			"meeting ending exactly at now is not current",
			[]Meeting{
				mk("just ended", at(9, 0), at(10, 0), true, "https://teams/just"),
				mk("next", at(10, 30), at(11, 0), true, "https://teams/next"),
			},
			"next",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pickJoinableMeeting(tc.in, now)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("expected nil, got %q", got.Subject)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %q, got nil", tc.want)
			}
			if got.Subject != tc.want {
				t.Errorf("got %q, want %q", got.Subject, tc.want)
			}
		})
	}
}
