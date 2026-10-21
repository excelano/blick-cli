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

// trayIconPNG is the orb from the iOS app icon on a transparent background,
// 64px square. The rays are left out, since they thin to nothing at the
// panel's 16px.
//
//go:embed tray_icon.png
var trayIconPNG []byte

// trayLook is what the tray icon says at a glance.
type trayLook int

const (
	trayUnread trayLook = iota // blue: something to look at
	trayIdle                   // grey: nothing unread
	trayStale                  // faded grey: the count could not be fetched
)

// renderTrayIcon returns the tray icon as PNG in the given look.
func renderTrayIcon(look trayLook) []byte {
	src, err := png.Decode(bytes.NewReader(trayIconPNG))
	if err != nil {
		panic("embedded tray icon: " + err.Error())
	}
	b := src.Bounds()
	img := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			if look != trayUnread {
				gray := uint8((299*uint32(c.R) + 587*uint32(c.G) + 114*uint32(c.B)) / 1000)
				c = color.NRGBA{gray, gray, gray, c.A}
			}
			if look == trayStale {
				c.A /= 2
			}
			img.SetNRGBA(x, y, c)
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		panic("encoding tray icon: " + err.Error())
	}
	return out.Bytes()
}
