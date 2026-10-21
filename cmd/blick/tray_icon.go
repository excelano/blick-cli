// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import (
	"image"
	"image/color"
	"math"
)

// trayIconSizes are the square sizes the tray icon is drawn at. The panel
// shows the one matching its slot as drawn, where any other size would be
// resampled and come out grainy.
var trayIconSizes = []int{16, 22, 24, 32, 48, 64}

// trayLook is what the tray icon says at a glance.
type trayLook int

const (
	trayUnread trayLook = iota // blue: something to look at
	trayIdle                   // grey: nothing unread
	trayStale                  // faded grey: the count could not be fetched
)

// The orb from the iOS app icon, in units of its radius with y pointing down:
// lit from the upper left, with a sharp white highlight there and a rim that
// darkens toward the edge.
var (
	orbLight     = [2]float64{-0.35, -0.35}
	orbHighlight = [2]float64{-0.37, -0.40}
	orbStops     = []orbStop{
		{0.15, rgb{117, 211, 247}},
		{0.50, rgb{79, 198, 243}},
		{0.90, rgb{30, 181, 237}},
		{1.45, rgb{10, 157, 213}},
	}
	orbRim   = rgb{15, 160, 218}
	orbShine = rgb{248, 252, 254}
)

const (
	orbRadius    = 0.46 // of the icon's side
	orbShineSize = 0.14

	// orbSamples is the supersampling grid per pixel, per axis, which gives
	// the edge and highlight their antialiasing.
	orbSamples = 4
)

type rgb [3]float64

type orbStop struct {
	at  float64 // distance from orbLight
	col rgb
}

func mix(a, b rgb, t float64) rgb {
	return rgb{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// orbShade is the orb's color at (u, v), a point inside it.
func orbShade(u, v float64) rgb {
	if math.Hypot(u-orbHighlight[0], v-orbHighlight[1]) <= orbShineSize {
		return orbShine
	}
	d := math.Hypot(u-orbLight[0], v-orbLight[1])
	c := orbStops[len(orbStops)-1].col
	for i, s := range orbStops {
		if d <= s.at {
			if i == 0 {
				c = s.col
			} else {
				prev := orbStops[i-1]
				c = mix(prev.col, s.col, (d-prev.at)/(s.at-prev.at))
			}
			break
		}
	}
	if rho := math.Hypot(u, v); rho > 0.75 {
		c = mix(c, orbRim, (rho-0.75)/0.25)
	}
	return c
}

// drawOrb draws the tray icon at size pixels square in the given look.
func drawOrb(size int, look trayLook) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	radius := orbRadius * float64(size)
	center := float64(size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var sum rgb
			hits := 0
			for sy := 0; sy < orbSamples; sy++ {
				for sx := 0; sx < orbSamples; sx++ {
					u := (float64(x) + (float64(sx)+0.5)/orbSamples - center) / radius
					v := (float64(y) + (float64(sy)+0.5)/orbSamples - center) / radius
					if u*u+v*v > 1 {
						continue
					}
					c := orbShade(u, v)
					sum = rgb{sum[0] + c[0], sum[1] + c[1], sum[2] + c[2]}
					hits++
				}
			}
			if hits == 0 {
				continue
			}
			n := float64(hits)
			c := rgb{sum[0] / n, sum[1] / n, sum[2] / n}
			if look != trayUnread {
				gray := 0.299*c[0] + 0.587*c[1] + 0.114*c[2]
				c = rgb{gray, gray, gray}
			}
			alpha := 255 * n / (orbSamples * orbSamples)
			if look == trayStale {
				alpha /= 2
			}
			img.SetNRGBA(x, y, color.NRGBA{
				uint8(math.Round(c[0])), uint8(math.Round(c[1])), uint8(math.Round(c[2])), uint8(math.Round(alpha)),
			})
		}
	}
	return img
}
