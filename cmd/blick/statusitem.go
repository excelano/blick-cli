// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

//go:build linux

package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// A tray icon on Linux is a StatusNotifierItem, with its menu served over the
// dbusmenu protocol: the panel reads both from this process over the session
// bus. Only the parts a plain menu of labels needs are implemented.
const (
	itemPath    = dbus.ObjectPath("/StatusNotifierItem")
	itemIface   = "org.kde.StatusNotifierItem"
	menuPath    = dbus.ObjectPath("/MenuBar")
	menuIface   = "com.canonical.dbusmenu"
	watcherName = "org.kde.StatusNotifierWatcher"
)

// pixmap is one size of an icon: ARGB32 pixels, in network byte order.
type pixmap struct {
	W, H int32
	Pix  []byte
}

type toolTip struct {
	Icon   string
	Images []pixmap
	Title  string
	Text   string
}

// menuEntry is one row of the menu. Its id on the bus is its index plus one;
// id 0 is the root.
type menuEntry struct {
	label     string
	enabled   bool
	hidden    bool
	separator bool
}

type menuLayout struct {
	ID       int32
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

type menuProps struct {
	ID    int32
	Props map[string]dbus.Variant
}

type menuEvent struct {
	ID        int32
	EventID   string
	Data      dbus.Variant
	Timestamp uint32
}

// statusItem is the tray icon and its menu. clicks carries the index of each
// clicked entry.
type statusItem struct {
	conn   *dbus.Conn
	props  *prop.Properties
	clicks chan int

	mu       sync.Mutex
	entries  []menuEntry
	revision uint32
}

// newStatusItem exports the item and its menu on conn and registers it with
// the panel, again whenever the panel's watcher restarts.
func newStatusItem(conn *dbus.Conn, id, title string) (*statusItem, error) {
	s := &statusItem{conn: conn, clicks: make(chan int, 4)}

	item := sniItem{s}
	menu := sniMenu{s}
	props, err := prop.Export(conn, itemPath, prop.Map{
		itemIface: {
			"Category":      {Value: "Communications"},
			"Id":            {Value: id},
			"Title":         {Value: title},
			"Status":        {Value: "Active"},
			"IconName":      {Value: ""},
			"IconPixmap":    {Value: []pixmap{}},
			"IconThemePath": {Value: ""},
			"ToolTip":       {Value: toolTip{Images: []pixmap{}}},
			"ItemIsMenu":    {Value: true},
			"Menu":          {Value: menuPath},
		},
	})
	if err != nil {
		return nil, err
	}
	s.props = props
	menuProps, err := prop.Export(conn, menuPath, prop.Map{
		menuIface: {
			"Version":       {Value: uint32(3)},
			"TextDirection": {Value: "ltr"},
			"Status":        {Value: "normal"},
			"IconThemePath": {Value: []string{}},
		},
	})
	if err != nil {
		return nil, err
	}

	exports := []struct {
		obj   any
		path  dbus.ObjectPath
		iface string
		props *prop.Properties
	}{
		{item, itemPath, itemIface, props},
		{menu, menuPath, menuIface, menuProps},
	}
	for _, e := range exports {
		if err := conn.Export(e.obj, e.path, e.iface); err != nil {
			return nil, err
		}
		node := &introspect.Node{
			Name: string(e.path),
			Interfaces: []introspect.Interface{
				introspect.IntrospectData,
				prop.IntrospectData,
				{Name: e.iface, Methods: introspect.Methods(e.obj), Properties: e.props.Introspection(e.iface)},
			},
		}
		if err := conn.Export(introspect.NewIntrospectable(node), e.path, "org.freedesktop.DBus.Introspectable"); err != nil {
			return nil, err
		}
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, watcherName),
	); err != nil {
		return nil, err
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	s.register()
	go func() {
		for sig := range signals {
			if sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(sig.Body) == 3 && sig.Body[2] != "" {
				s.register()
			}
		}
	}()
	return s, nil
}

// register announces the item to the panel's watcher. With no watcher yet,
// as when the tray starts before the panel, the item waits for one.
func (s *statusItem) register() {
	call := s.conn.Object(watcherName, "/StatusNotifierWatcher").Call(watcherName+".RegisterStatusNotifierItem", 0, string(itemPath))
	if call.Err != nil {
		fmt.Fprintf(os.Stderr, "blick tray: waiting for a tray host: %v\n", call.Err)
	}
}

func (s *statusItem) setIcon(look trayLook) {
	s.props.SetMust(itemIface, "IconPixmap", trayPixmaps(look))
	s.emit(itemPath, itemIface+".NewIcon")
}

func (s *statusItem) setToolTip(text string) {
	s.props.SetMust(itemIface, "ToolTip", toolTip{Images: []pixmap{}, Title: text})
	s.emit(itemPath, itemIface+".NewToolTip")
}

func (s *statusItem) setMenu(entries []menuEntry) {
	s.mu.Lock()
	s.entries = entries
	s.revision++
	revision := s.revision
	s.mu.Unlock()
	s.emit(menuPath, menuIface+".LayoutUpdated", revision, int32(0))
}

func (s *statusItem) emit(path dbus.ObjectPath, name string, values ...any) {
	if err := s.conn.Emit(path, name, values...); err != nil {
		fmt.Fprintf(os.Stderr, "blick tray: %v\n", err)
	}
}

// trayPixmaps draws the icon at every size in trayIconSizes.
func trayPixmaps(look trayLook) []pixmap {
	var out []pixmap
	for _, size := range trayIconSizes {
		img := drawOrb(size, look)
		pix := make([]byte, 0, 4*size*size)
		for i := 0; i < len(img.Pix); i += 4 {
			pix = append(pix, img.Pix[i+3], img.Pix[i], img.Pix[i+1], img.Pix[i+2])
		}
		out = append(out, pixmap{int32(size), int32(size), pix})
	}
	return out
}

// entryProps is the dbusmenu property set of the entry with the given id, or
// false when there is none.
func (s *statusItem) entryProps(id int32) (map[string]dbus.Variant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == 0 {
		return map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")}, true
	}
	if id < 0 || int(id) > len(s.entries) {
		return nil, false
	}
	e := s.entries[id-1]
	if e.separator {
		return map[string]dbus.Variant{
			"type":    dbus.MakeVariant("separator"),
			"visible": dbus.MakeVariant(!e.hidden),
		}, true
	}
	return map[string]dbus.Variant{
		"label":   dbus.MakeVariant(e.label),
		"enabled": dbus.MakeVariant(e.enabled),
		"visible": dbus.MakeVariant(!e.hidden),
	}, true
}

func (s *statusItem) click(id int32) {
	select {
	case s.clicks <- int(id) - 1:
	default:
	}
}

// sniItem carries the StatusNotifierItem methods. The menu answers every
// click, so activation does nothing.
type sniItem struct{ *statusItem }

func (sniItem) Activate(x, y int32) *dbus.Error                    { return nil }
func (sniItem) SecondaryActivate(x, y int32) *dbus.Error           { return nil }
func (sniItem) ContextMenu(x, y int32) *dbus.Error                 { return nil }
func (sniItem) Scroll(delta int32, orientation string) *dbus.Error { return nil }

// sniMenu carries the dbusmenu methods.
type sniMenu struct{ *statusItem }

func (m sniMenu) GetLayout(parent, depth int32, names []string) (uint32, menuLayout, *dbus.Error) {
	props, ok := m.entryProps(parent)
	if !ok {
		return 0, menuLayout{}, dbus.MakeFailedError(fmt.Errorf("no menu item %d", parent))
	}
	m.mu.Lock()
	revision, count := m.revision, len(m.entries)
	m.mu.Unlock()
	layout := menuLayout{ID: parent, Props: props, Children: []dbus.Variant{}}
	if parent == 0 && depth != 0 {
		for id := int32(1); id <= int32(count); id++ {
			child, _ := m.entryProps(id)
			layout.Children = append(layout.Children, dbus.MakeVariant(menuLayout{ID: id, Props: child, Children: []dbus.Variant{}}))
		}
	}
	return revision, layout, nil
}

func (m sniMenu) GetGroupProperties(ids []int32, names []string) ([]menuProps, *dbus.Error) {
	var out []menuProps
	for _, id := range ids {
		if props, ok := m.entryProps(id); ok {
			out = append(out, menuProps{id, props})
		}
	}
	return out, nil
}

func (m sniMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	props, ok := m.entryProps(id)
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("no menu item %d", id))
	}
	v, ok := props[name]
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("menu item %d has no %s", id, name))
	}
	return v, nil
}

func (m sniMenu) Event(id int32, eventID string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if eventID == "clicked" {
		m.click(id)
	}
	return nil
}

func (m sniMenu) EventGroup(events []menuEvent) ([]int32, *dbus.Error) {
	for _, e := range events {
		if e.EventID == "clicked" {
			m.click(e.ID)
		}
	}
	return []int32{}, nil
}

func (sniMenu) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }

func (sniMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return []int32{}, []int32{}, nil
}
