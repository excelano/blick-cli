// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

//go:build linux

package main

import "testing"

func TestTrayPixmaps(t *testing.T) {
	pixmaps := trayPixmaps(trayUnread)
	if len(pixmaps) != len(trayIconSizes) {
		t.Fatalf("%d pixmaps, want one per size", len(pixmaps))
	}
	for i, p := range pixmaps {
		size := trayIconSizes[i]
		if int(p.W) != size || int(p.H) != size || len(p.Pix) != 4*size*size {
			t.Fatalf("pixmap %d is %dx%d with %d bytes, want %d square", i, p.W, p.H, len(p.Pix), size)
		}
		c := drawOrb(size, trayUnread).NRGBAAt(size/2, size/2)
		at := 4 * (size/2*size + size/2)
		if got := [4]byte(p.Pix[at : at+4]); got != [4]byte{c.A, c.R, c.G, c.B} {
			t.Errorf("size %d: center bytes %v, want ARGB %v", size, got, [4]byte{c.A, c.R, c.G, c.B})
		}
	}
}

func TestMenuLayout(t *testing.T) {
	s := &statusItem{clicks: make(chan int, 4), entries: []menuEntry{
		{label: "Nothing unread"},
		{separator: true},
		{label: "Quit", enabled: true},
	}}
	m := sniMenu{s}

	_, root, err := m.GetLayout(0, -1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 3 {
		t.Fatalf("root has %d children, want 3", len(root.Children))
	}
	quit := root.Children[2].Value().(menuLayout)
	if quit.ID != 3 || quit.Props["label"].Value() != "Quit" || quit.Props["enabled"].Value() != true {
		t.Errorf("third child = %+v, want enabled Quit with id 3", quit)
	}
	if sep := root.Children[1].Value().(menuLayout); sep.Props["type"].Value() != "separator" {
		t.Errorf("second child = %+v, want a separator", sep)
	}
	if _, _, err := m.GetLayout(9, -1, nil); err == nil {
		t.Error("layout of a missing item succeeded")
	}

	m.Event(3, "clicked", root.Props["children-display"], 0)
	m.Event(1, "hovered", root.Props["children-display"], 0)
	if got := <-s.clicks; got != 2 {
		t.Errorf("click on id 3 reported entry %d, want 2", got)
	}
	select {
	case got := <-s.clicks:
		t.Errorf("a hover reported a click on entry %d", got)
	default:
	}
}
