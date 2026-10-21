// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
)

// trayIconPNG is the iOS app icon's sun on a transparent background, 64px
// square, with the rays and orb thickened so they survive the panel's 16px.
//
//go:embed tray_icon.png
var trayIconPNG []byte

const (
	dotRadius = 10 // unread dot, in icon pixels
	dotGap    = 3  // transparent ring cut between the dot and the sun
)

var dotRed = color.RGBA{0xFF, 0x3B, 0x30, 0xFF}

// renderTrayIcon returns the tray icon as PNG: the sun alone when nothing is
// unread, with a red dot in the upper right when something is, and greyed out
// with no dot when stale, meaning the count could not be fetched. The count
// itself is in the menu; a digit is unreadable at panel size.
func renderTrayIcon(unread, stale bool) []byte {
	src, err := png.Decode(bytes.NewReader(trayIconPNG))
	if err != nil {
		panic("embedded tray icon: " + err.Error())
	}
	b := src.Bounds()
	img := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			if stale {
				gray := uint8((299*uint32(c.R) + 587*uint32(c.G) + 114*uint32(c.B)) / 1000)
				c = color.NRGBA{gray, gray, gray, c.A / 2}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	if unread && !stale {
		drawDot(img)
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		panic("encoding tray icon: " + err.Error())
	}
	return out.Bytes()
}

// drawDot cuts a ring out of the upper-right corner and fills a red dot
// inside it.
func drawDot(img *image.NRGBA) {
	b := img.Bounds()
	cx, cy := b.Max.X-dotRadius-dotGap, b.Min.Y+dotRadius+dotGap
	outer := dotRadius + dotGap
	for y := b.Min.Y; y <= cy+outer; y++ {
		for x := cx - outer; x < b.Max.X; x++ {
			// Distance from pixel centers, doubled to stay in integers.
			dx, dy := 2*(x-cx)+1, 2*(y-cy)+1
			d := dx*dx + dy*dy
			switch {
			case d <= 4*dotRadius*dotRadius:
				img.Set(x, y, dotRed)
			case d <= 4*outer*outer:
				img.Set(x, y, color.Transparent)
			}
		}
	}
}
