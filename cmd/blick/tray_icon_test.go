// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestBadgeText(t *testing.T) {
	for n, want := range map[int]string{1: "1", 9: "9", 10: "9+", 57: "9+"} {
		if got := badgeText(n); got != want {
			t.Errorf("badgeText(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestBadgeGlyphsCoverBadgeText(t *testing.T) {
	for n := 1; n <= 10; n++ {
		for _, r := range badgeText(n) {
			if _, ok := badgeGlyphs[r]; !ok {
				t.Errorf("no glyph for %q", r)
			}
		}
	}
}

func decodeTrayIcon(t *testing.T, unread int, stale bool) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(renderTrayIcon(unread, stale)))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestRenderTrayIcon(t *testing.T) {
	plain := decodeTrayIcon(t, 0, false)
	if plain.Bounds().Dx() != 64 || plain.Bounds().Dy() != 64 {
		t.Fatalf("icon is %v, want 64x64", plain.Bounds())
	}

	// The badge's disc sits at the lower right; its edge, clear of the
	// glyphs, is red only when something is unread and the count is fresh.
	edge := image.Pt(64-badgeRadius-badgeGap, 64-badgeGap-2)
	isRed := func(img image.Image) bool {
		return color.NRGBAModel.Convert(img.At(edge.X, edge.Y)) == color.NRGBA(badgeRed)
	}
	if isRed(plain) {
		t.Error("badge drawn with nothing unread")
	}
	if !isRed(decodeTrayIcon(t, 3, false)) {
		t.Error("no badge with 3 unread")
	}
	if isRed(decodeTrayIcon(t, 3, true)) {
		t.Error("badge drawn on a stale icon")
	}
}
