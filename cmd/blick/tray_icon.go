// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"strconv"
)

// trayIconPNG is the iOS app icon's sun on a transparent background, 64px
// square, so it sits on the desktop's own top bar rather than in a navy tile.
//
//go:embed tray_icon.png
var trayIconPNG []byte

// badgeGlyphs is a 3x5 pixel font covering what badgeText can produce. The
// tray draws at a few pixels per glyph cell, where a pixel font stays sharper
// than a scaled outline font.
var badgeGlyphs = map[rune][5]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"},
	'1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"},
	'3': {"###", "..#", ".##", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"},
	'5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"},
	'7': {"###", "..#", "..#", "..#", "..#"},
	'8': {"###", "#.#", "###", "#.#", "###"},
	'9': {"###", "#.#", "###", "..#", "###"},
	'+': {"...", ".#.", "###", ".#.", "..."},
}

const (
	badgeRadius = 18 // badge disc, in icon pixels
	badgeGap    = 2  // transparent ring cut between the disc and the sun
	badgeScale  = 4  // icon pixels per glyph cell
)

var badgeRed = color.RGBA{0xFF, 0x3B, 0x30, 0xFF}

// badgeText is the count as the badge shows it: one digit, or "9+" beyond.
func badgeText(n int) string {
	if n > 9 {
		return "9+"
	}
	return strconv.Itoa(n)
}

// renderTrayIcon returns the tray icon as PNG: the sun alone when nothing is
// unread, with a red count badge in the lower right when something is, and
// greyed out with no badge when stale, meaning the count could not be fetched.
func renderTrayIcon(unread int, stale bool) []byte {
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
	if !stale && unread > 0 {
		drawBadge(img, badgeText(unread))
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		panic("encoding tray icon: " + err.Error())
	}
	return out.Bytes()
}

// drawBadge cuts a ring out of the lower-right corner, fills a red disc inside
// it, and centers text on the disc in white glyph cells.
func drawBadge(img *image.NRGBA, text string) {
	b := img.Bounds()
	cx, cy := b.Max.X-badgeRadius-badgeGap, b.Max.Y-badgeRadius-badgeGap
	outer := (badgeRadius + badgeGap) * (badgeRadius + badgeGap)
	for y := cy - badgeRadius - badgeGap; y < b.Max.Y; y++ {
		for x := cx - badgeRadius - badgeGap; x < b.Max.X; x++ {
			// Distance from pixel centers, doubled to stay in integers.
			dx, dy := 2*(x-cx)+1, 2*(y-cy)+1
			d := dx*dx + dy*dy
			switch {
			case d <= 4*badgeRadius*badgeRadius:
				img.Set(x, y, badgeRed)
			case d <= 4*outer:
				img.Set(x, y, color.Transparent)
			}
		}
	}

	cols := 4*len(text) - 1
	left := cx - cols*badgeScale/2
	top := cy - 5*badgeScale/2
	for i, r := range text {
		for row, line := range badgeGlyphs[r] {
			for col, cell := range line {
				if cell != '#' {
					continue
				}
				x0 := left + (4*i+col)*badgeScale
				y0 := top + row*badgeScale
				for y := y0; y < y0+badgeScale; y++ {
					for x := x0; x < x0+badgeScale; x++ {
						img.Set(x, y, color.White)
					}
				}
			}
		}
	}
}
