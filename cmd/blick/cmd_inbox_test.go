package main

import (
	"testing"
	"time"
)

func TestParseInboxArgs(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    int
		wantErr bool
	}{
		{"no args is today only", nil, 0, false},
		{"explicit zero is today only", []string{"0"}, 0, false},
		{"one day back", []string{"1"}, 1, false},
		{"three days back", []string{"3"}, 3, false},
		{"cap boundary", []string{"30"}, 30, false},
		{"past cap errors", []string{"31"}, 0, true},
		{"negative errors", []string{"-2"}, 0, true},
		{"non-number errors", []string{"abc"}, 0, true},
		{"too many args errors", []string{"2", "3"}, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseInboxArgs(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got days=%d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("days: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWindowStart(t *testing.T) {
	now := time.Now()

	s0 := windowStart(0)
	if s0.Hour() != 0 || s0.Minute() != 0 || s0.Second() != 0 || s0.Nanosecond() != 0 {
		t.Errorf("windowStart(0) not at local midnight: %v", s0)
	}
	if s0.Location() != time.Local {
		t.Errorf("windowStart(0) location = %v, want Local", s0.Location())
	}
	if s0.After(now) {
		t.Errorf("windowStart(0) = %v is in the future (now %v)", s0, now)
	}
	if now.Sub(s0) >= 24*time.Hour {
		t.Errorf("windowStart(0) = %v is more than a day before now %v", s0, now)
	}

	// N days back is exactly N local days before today's midnight.
	if s2 := windowStart(2); !s2.Equal(s0.AddDate(0, 0, -2)) {
		t.Errorf("windowStart(2) = %v, want %v", s2, s0.AddDate(0, 0, -2))
	}
}
