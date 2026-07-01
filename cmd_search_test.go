package main

import (
	"reflect"
	"testing"
)

func TestParseSearchArgs(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    searchQuery
		wantErr bool
	}{
		{"bare words are text", []string{"pizza", "party"}, searchQuery{text: []string{"pizza", "party"}}, false},
		{"from long flag", []string{"--from", "alice"}, searchQuery{from: "alice"}, false},
		{"from short flag", []string{"-f", "alice"}, searchQuery{from: "alice"}, false},
		{"subject flag", []string{"--subject", "report"}, searchQuery{subject: "report"}, false},
		{"text flag accumulates with positionals", []string{"--text", "a", "b"}, searchQuery{text: []string{"a", "b"}}, false},
		{"combined", []string{"--from", "bob", "--subject", "Q3", "invoice"}, searchQuery{from: "bob", subject: "Q3", text: []string{"invoice"}}, false},
		{"empty input", nil, searchQuery{}, false},
		{"trailing --from errors", []string{"--from"}, searchQuery{}, true},
		{"trailing -s errors", []string{"-s"}, searchQuery{}, true},
		{"trailing --text errors", []string{"--text"}, searchQuery{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSearchArgs(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSearchKQL(t *testing.T) {
	cases := []struct {
		name string
		sq   searchQuery
		want string
	}{
		{"from only", searchQuery{from: "alice"}, "from:alice"},
		{"subject only", searchQuery{subject: "report"}, "subject:report"},
		{"single text", searchQuery{text: []string{"pizza"}}, "pizza"},
		{"multiple text ANDs", searchQuery{text: []string{"pizza", "party"}}, "pizza party"},
		{"from and text", searchQuery{from: "bob", text: []string{"invoice"}}, "from:bob invoice"},
		{"phrase quoted subject", searchQuery{subject: "quarterly report"}, `subject:"quarterly report"`},
		{"phrase quoted text", searchQuery{text: []string{"pizza party"}}, `"pizza party"`},
		{"all three", searchQuery{from: "bob", subject: "Q3", text: []string{"invoice"}}, "from:bob subject:Q3 invoice"},
		{"empty", searchQuery{}, ""},
		{"blank text dropped", searchQuery{text: []string{"  "}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sq.kql(); got != c.want {
				t.Errorf("kql() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSearchIsEmpty(t *testing.T) {
	if !(searchQuery{}).isEmpty() {
		t.Error("zero searchQuery should be empty")
	}
	if !(searchQuery{text: []string{"", "  "}}).isEmpty() {
		t.Error("whitespace-only text should be empty")
	}
	if (searchQuery{from: "a"}).isEmpty() {
		t.Error("from:a should not be empty")
	}
}
