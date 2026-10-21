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

func decodeTrayIcon(t *testing.T, look trayLook) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(renderTrayIcon(look)))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestRenderTrayIcon(t *testing.T) {
	center := func(look trayLook) color.NRGBA {
		img := decodeTrayIcon(t, look)
		if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
			t.Fatalf("icon is %v, want 32x32", img.Bounds())
		}
		return color.NRGBAModel.Convert(img.At(20, 20)).(color.NRGBA)
	}

	unread, idle, stale := center(trayUnread), center(trayIdle), center(trayStale)
	if unread.B <= unread.R {
		t.Errorf("unread orb is not blue: %v", unread)
	}
	if idle.R != idle.G || idle.G != idle.B {
		t.Errorf("idle orb is not grey: %v", idle)
	}
	if idle.A != 0xFF || stale.A >= idle.A {
		t.Errorf("stale orb (alpha %d) is not fainter than idle (alpha %d)", stale.A, idle.A)
	}
}
