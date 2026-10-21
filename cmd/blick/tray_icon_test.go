// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import "testing"

func TestDrawOrb(t *testing.T) {
	for _, size := range trayIconSizes {
		img := drawOrb(size, trayUnread)
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Fatalf("size %d: drew %v", size, b)
		}
		if a := img.NRGBAAt(0, 0).A; a != 0 {
			t.Errorf("size %d: corner alpha %d, want transparent", size, a)
		}
		if c := img.NRGBAAt(size/2, size/2); c.A != 0xFF || c.B <= c.R {
			t.Errorf("size %d: center %v, want opaque blue", size, c)
		}
	}
}

func TestDrawOrbLooks(t *testing.T) {
	center := func(look trayLook) (r, g, b, a uint8) {
		c := drawOrb(32, look).NRGBAAt(20, 20)
		return c.R, c.G, c.B, c.A
	}
	if r, g, b, _ := center(trayIdle); r != g || g != b {
		t.Errorf("idle orb is not grey: %d,%d,%d", r, g, b)
	}
	_, _, _, idle := center(trayIdle)
	_, _, _, stale := center(trayStale)
	if idle != 0xFF || stale >= idle {
		t.Errorf("stale orb (alpha %d) is not fainter than idle (alpha %d)", stale, idle)
	}
}
