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

func TestUnwrapSafeLink(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "amp-encoded separators",
			in:   `https://nam12.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com%2Freport&amp;data=05%7Cabc&amp;reserved=0`,
			want: "https://example.com/report",
		},
		{
			name: "literal ampersand separators",
			in:   `https://nam12.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com&data=05%7Cabc`,
			want: "https://example.com",
		},
		{
			name: "tenant subdomain host",
			in:   `https://contoso.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com%2Fx&amp;data=z`,
			want: "https://example.com/x",
		},
		{
			name: "uppercase host still matches",
			in:   `https://NAM06.SafeLinks.Protection.Outlook.com/?url=https%3A%2F%2Fexample.com&amp;data=z`,
			want: "https://example.com",
		},
		{
			name: "double-encoded nested query recovered",
			in:   `https://nam06.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com%2Fa%3Fx%3D1%26y%3D2&amp;data=z`,
			want: "https://example.com/a?x=1&y=2",
		},
		{
			name: "non-safelinks url with url param untouched",
			in:   `https://example.com/redirect?url=https%3A%2F%2Fevil.com`,
			want: `https://example.com/redirect?url=https%3A%2F%2Fevil.com`,
		},
		{
			name: "safelinks host without url param untouched",
			in:   `https://nam12.safelinks.protection.outlook.com/`,
			want: `https://nam12.safelinks.protection.outlook.com/`,
		},
		{
			name: "plain url untouched",
			in:   "https://example.com/foo",
			want: "https://example.com/foo",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := unwrapSafeLink(c.in); got != c.want {
				t.Errorf("unwrapSafeLink(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestStripHTMLUnwrapsSafeLinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "named safelink keeps original destination",
			in:   `See <a href="https://nam12.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com%2Freport&amp;data=05%7Cabc&amp;reserved=0">the report</a>.`,
			want: "See the report (https://example.com/report).",
		},
		{
			name: "safelink whose text is the original url collapses",
			in:   `<a href="https://nam12.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com&amp;data=z">https://example.com</a>`,
			want: "https://example.com",
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
