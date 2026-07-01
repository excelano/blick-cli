package main

import "testing"

func TestStripHTMLPreservesLinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "named link keeps url in parens",
			in:   `See <a href="https://example.com/report">the report</a> today.`,
			want: "See the report (https://example.com/report) today.",
		},
		{
			name: "link text equal to url is not duplicated",
			in:   `<a href="https://example.com">https://example.com</a>`,
			want: "https://example.com",
		},
		{
			name: "empty anchor text falls back to url",
			in:   `<a href="https://example.com"></a>`,
			want: "https://example.com",
		},
		{
			name: "mailto strips scheme",
			in:   `<a href="mailto:alice@example.com">Alice</a>`,
			want: "Alice (alice@example.com)",
		},
		{
			name: "query entities decoded in url",
			in:   `<a href="https://x.com/s?a=1&amp;b=2">go</a>`,
			want: "go (https://x.com/s?a=1&b=2)",
		},
		{
			name: "single-quoted href",
			in:   `<a href='https://example.com/x'>click here</a>`,
			want: "click here (https://example.com/x)",
		},
		{
			name: "nested tags inside anchor text",
			in:   `<a href="https://example.com"><span>click <b>here</b></span></a>`,
			want: "click here (https://example.com)",
		},
		{
			name: "named anchor without destination drops to text",
			in:   `<a name="top">Top</a>`,
			want: "Top",
		},
		{
			name: "javascript href drops to text",
			in:   `<a href="javascript:void(0)">Menu</a>`,
			want: "Menu",
		},
		{
			name: "fragment href drops to text",
			in:   `<a href="#section">Section</a>`,
			want: "Section",
		},
		{
			name: "two links in one line",
			in:   `<a href="https://a.com">A</a> and <a href="https://b.com">B</a>`,
			want: "A (https://a.com) and B (https://b.com)",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripHTML(c.in); got != c.want {
				t.Errorf("stripHTML(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestStripHTMLBasics(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"paragraph breaks", "<p>one</p><p>two</p>", "one\ntwo"},
		{"br to newline", "line1<br>line2", "line1\nline2"},
		{"entities decoded", "a &amp; b &lt;c&gt;", "a & b <c>"},
		{"tags removed", "<b>bold</b> plain", "bold plain"},
		{"collapse blank runs", "a\n\n\n\nb", "a\n\nb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripHTML(c.in); got != c.want {
				t.Errorf("stripHTML(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
