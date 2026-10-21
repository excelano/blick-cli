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

func decodeTrayIcon(t *testing.T, unread, stale bool) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(renderTrayIcon(unread, stale)))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestRenderTrayIcon(t *testing.T) {
	plain := decodeTrayIcon(t, false, false)
	if plain.Bounds().Dx() != 64 || plain.Bounds().Dy() != 64 {
		t.Fatalf("icon is %v, want 64x64", plain.Bounds())
	}

	center := image.Pt(64-dotRadius-dotGap, dotRadius+dotGap)
	isRed := func(img image.Image) bool {
		return color.NRGBAModel.Convert(img.At(center.X, center.Y)) == color.NRGBA(dotRed)
	}
	if isRed(plain) {
		t.Error("dot drawn with nothing unread")
	}
	if !isRed(decodeTrayIcon(t, true, false)) {
		t.Error("no dot with something unread")
	}
	if isRed(decodeTrayIcon(t, true, true)) {
		t.Error("dot drawn on a stale icon")
	}
}
