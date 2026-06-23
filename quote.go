package main

import (
	"regexp"
	"strings"
)

// splitQuotedHistory splits an email body at the first quoted-history
// boundary. Everything above is `visible`; everything from the boundary
// down is `quoted`. With no boundary the whole body is `visible` and
// `quoted` is empty.
//
// Straight port of klartext's TextSeam.swift — the same rules used by
// the iOS Blick and Zirbe apps. Detection is line-by-line, top down;
// the first marker wins. Mis-folds hide text behind `view N full`,
// never destroy it, so the heuristics lean toward folding.
//
// Markers, in detection order per line:
//   - `>` prefix at line start
//   - Outlook header block ("From:" plus ≥2 header labels within 5 lines)
//   - Forward / Original-Message separators
//   - Attribution line ending in "wrote:" (with multi-line back-walk)
func splitQuotedHistory(body string) (visible, quoted string, quotedLines int) {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")

	cut := boundaryIndex(lines)
	if cut < 0 {
		return strings.TrimSpace(normalized), "", 0
	}

	visible = strings.TrimSpace(strings.Join(lines[:cut], "\n"))
	quoted = strings.TrimSpace(strings.Join(lines[cut:], "\n"))
	if quoted == "" {
		return visible, "", 0
	}
	return visible, quoted, strings.Count(quoted, "\n") + 1
}

var (
	wroteRE         = regexp.MustCompile(`(?i)\bwrote:\s*$`)
	forwardMarkerRE = regexp.MustCompile(`(?i)^-{2,}\s*(Original Message|Forwarded message)\s*-*$`)
	fromLineRE      = regexp.MustCompile(`(?i)^from\s*:`)
	headerLabelRE   = regexp.MustCompile(`(?i)^(from|sent|to|cc|bcc|subject|date|reply-to)\s*:`)
)

func boundaryIndex(lines []string) int {
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, ">") {
			return i
		}
		if isForwardOrOriginalMarker(line) {
			return i
		}
		if isOutlookHeaderBlock(i, lines) {
			return i
		}
		if wroteRE.MatchString(line) {
			return attributionStart(i, lines)
		}
	}
	return -1
}

// attributionStart handles multi-line attributions ("On Mon, Jun 9,
// 2026,\nDavid <x> wrote:"). Given the line ending in "wrote:", walk
// back over the contiguous non-blank block above it; if that block
// begins with "On ", the boundary is its first line so the whole
// attribution folds together. Otherwise the "wrote:" line stands alone.
func attributionStart(index int, lines []string) int {
	start := index
	cursor := index - 1
	for cursor >= 0 {
		line := strings.TrimSpace(lines[cursor])
		if line == "" {
			break
		}
		start = cursor
		if strings.HasPrefix(line, "On ") {
			break
		}
		cursor--
	}
	if strings.HasPrefix(strings.TrimSpace(lines[start]), "On ") {
		return start
	}
	return index
}

func isForwardOrOriginalMarker(line string) bool {
	if forwardMarkerRE.MatchString(line) {
		return true
	}
	return strings.EqualFold(line, "Begin forwarded message:")
}

// isOutlookHeaderBlock recognizes a "From:" line followed within a
// five-line window by at least two header labels (counting the "From:"
// itself). The two-label requirement keeps an ordinary sentence opening
// "From: the desk of …" from being mistaken for a quote boundary.
func isOutlookHeaderBlock(index int, lines []string) bool {
	first := strings.TrimSpace(lines[index])
	if !fromLineRE.MatchString(first) {
		return false
	}
	labels := 0
	end := index + 5
	if end > len(lines) {
		end = len(lines)
	}
	for i := index; i < end; i++ {
		if headerLabelRE.MatchString(strings.TrimSpace(lines[i])) {
			labels++
		}
	}
	return labels >= 2
}
