//go:build linux

// internal/core/infra/systray/systray_linux_dbus.go
// Implements the org.kde.StatusNotifierItem + com.canonical.dbusmenu D-Bus
// interfaces so the Linux daemon exports a real tray icon and menu on the
// session bus (consumed by KDE/GNOME/Cinnamon SNI hosts). Hand-rolled over
// godbus to avoid pulling in a GTK build dependency for Linux CI.
// Does not implement the darwin/Windows tray; those have their own backends.

package systray

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	_ "image/png" // PNG decoder for tray-icon pixmap conversion
	"sync"

	"github.com/godbus/dbus/v5"
)

// Sentinel errors for the D-Bus property interfaces. Static errors satisfy
// err113; godbus wraps them into D-Bus error replies on the wire.
var (
	errUnknownInterface = errors.New("unknown interface")
	errUnknownProperty  = errors.New("unknown property")
	errReadOnly         = errors.New("properties are read-only")
	errCannotOwnSNIName = errors.New("could not own StatusNotifierItem bus name")
)

const (
	// dbusmenuVersion is the com.canonical.dbusmenu protocol version we serve.
	dbusmenuVersion int32 = 2
	// bytesPerPixel is the ARGB32 stride used when packing the tray icon pixmap.
	bytesPerPixel = 4
)

// pixmap is one entry of the SNI IconPixmap signature a(iiay): width, height,
// and a byte array holding one 32-bit pixel per cell in ARGB (network byte
// order) as required by the org.kde.StatusNotifierItem spec.
type pixmap struct {
	W    int
	H    int
	Data []byte
}

// toolTip matches the SNI ToolTip signature (sa(iiay)ss): an icon-name, an array
// of icon pixmaps, a title, and a description. We ship pixmaps only (no named
// icon), so IconName is empty and the host falls back to the pixmap array.
type toolTip struct {
	IconName string
	Pixmaps  []pixmap
	Title    string
	Desc     string
}

// sniServer implements org.kde.StatusNotifierItem on the tray object path. The
// host reads item state through the org.freedesktop.DBus.Properties Get/GetAll
// methods; Activate/SecondaryActivate are no-ops because the menu is served via
// the separate menu object (the host shows it on click).
type sniServer struct {
	mu sync.Mutex

	id       string
	category string
	status   string
	title    string
	iconName string
	iconPix  []pixmap
	tip      toolTip
	menuPath dbus.ObjectPath
}

func (s *sniServer) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	if iface != "org.kde.StatusNotifierItem" {
		return dbus.Variant{}, dbus.NewError(
			"org.freedesktop.DBus.Error.UnknownInterface",
			[]any{errUnknownInterface.Error()},
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch prop {
	case "Id":
		return dbus.MakeVariant(s.id), nil
	case "Category":
		return dbus.MakeVariant(s.category), nil
	case "Status":
		return dbus.MakeVariant(s.status), nil
	case "Title":
		return dbus.MakeVariant(s.title), nil
	case "IconName":
		return dbus.MakeVariant(s.iconName), nil
	case "IconPixmap":
		return dbus.MakeVariant(s.iconPix), nil
	case "ToolTip":
		return dbus.MakeVariant(s.tip), nil
	case "Menu":
		return dbus.MakeVariant(s.menuPath), nil
	case "ItemIsMenu":
		return dbus.MakeVariant(true), nil
	default:
		return dbus.Variant{}, dbus.NewError(
			"org.freedesktop.DBus.Properties.Error.UnknownProperty",
			[]any{errUnknownProperty.Error()},
		)
	}
}

func (s *sniServer) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != "org.kde.StatusNotifierItem" {
		return nil, dbus.NewError(
			"org.freedesktop.DBus.Error.UnknownInterface",
			[]any{errUnknownInterface.Error()},
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]dbus.Variant{
		"Id":         dbus.MakeVariant(s.id),
		"Category":   dbus.MakeVariant(s.category),
		"Status":     dbus.MakeVariant(s.status),
		"Title":      dbus.MakeVariant(s.title),
		"IconName":   dbus.MakeVariant(s.iconName),
		"IconPixmap": dbus.MakeVariant(s.iconPix),
		"ToolTip":    dbus.MakeVariant(s.tip),
		"Menu":       dbus.MakeVariant(s.menuPath),
		"ItemIsMenu": dbus.MakeVariant(true),
	}, nil
}

func (s *sniServer) Set(iface, prop string, val dbus.Variant) *dbus.Error {
	return dbus.NewError(
		"org.freedesktop.DBus.Properties.Error.ReadOnly",
		[]any{errReadOnly.Error()},
	)
}

func (s *sniServer) Activate(x, y int32) *dbus.Error                    { return nil }
func (s *sniServer) SecondaryActivate(x, y int32) *dbus.Error           { return nil }
func (s *sniServer) Scroll(delta int32, orientation string) *dbus.Error { return nil }

// menuNode is the com.canonical.dbusmenu layout tuple (ia{sv}av): an item id,
// its property dict, and its children as an array of variants each wrapping the
// same tuple. godbus derives the signature from the struct field order.
type menuNode struct {
	ID       int
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

// menuServer implements com.canonical.dbusmenu on the menu object path. The
// tray host calls GetLayout to fetch the whole tree, Event to deliver clicks,
// and AboutToShow before opening a submenu. We serve the tree from the shared
// menuItems map maintained by the package API in systray_linux.go.
type menuServer struct {
	mu       sync.Mutex
	revision uint32
}

func (m *menuServer) GetLayout(
	parentID int32,
	recursionDepth int32,
	propertyNames []string,
) (uint32, menuNode, *dbus.Error) {
	m.mu.Lock()
	rev := m.revision
	m.mu.Unlock()

	menuItemsLock.RLock()
	defer menuItemsLock.RUnlock()

	node := buildMenuNodeLocked(int(parentID), int(recursionDepth), propertyNames)

	return rev, node, nil
}

func (m *menuServer) GetGroupProperties(ids []int32, propertyNames []string) ([]struct {
	ID    int32
	Props map[string]dbus.Variant
}, *dbus.Error,
) {
	menuItemsLock.RLock()
	defer menuItemsLock.RUnlock()

	out := make([]struct {
		ID    int32
		Props map[string]dbus.Variant
	}, 0, len(ids))

	for _, itemID := range ids {
		item, ok := menuItems[int(itemID)]
		if !ok {
			continue
		}

		out = append(out, struct {
			ID    int32
			Props map[string]dbus.Variant
		}{
			ID:    itemID,
			Props: item.props(propertyNames),
		})
	}

	return out, nil
}

func (m *menuServer) GetProperty(itemID int32, name string) (dbus.Variant, *dbus.Error) {
	menuItemsLock.RLock()
	defer menuItemsLock.RUnlock()

	item, ok := menuItems[int(itemID)]
	if !ok {
		return dbus.Variant{}, dbus.NewError(
			"com.canonical.dbusmenu.NoSuchItem",
			[]any{"no such item"},
		)
	}

	props := item.props([]string{name})

	if v, ok := props[name]; ok {
		return v, nil
	}

	return dbus.Variant{}, dbus.NewError(
		"com.canonical.dbusmenu.NoSuchProperty",
		[]any{"no such property"},
	)
}

func (m *menuServer) Event(
	itemID int32,
	eventID string,
	data dbus.Variant,
	timestamp uint32,
) *dbus.Error {
	if eventID != "clicked" {
		return nil
	}

	item, ok := menuItemByID(int(itemID))
	if !ok {
		return nil
	}

	sendClicked(item)

	return nil
}

func (m *menuServer) EventGroup(events []struct {
	ID      int32
	EventID string
	Data    dbus.Variant
	Time    uint32
},
) ([]int32, *dbus.Error) {
	changed := make([]int32, 0, len(events))

	for _, evt := range events {
		if evt.EventID != "clicked" {
			continue
		}

		item, ok := menuItemByID(int(evt.ID))
		if !ok {
			continue
		}

		if sendClicked(item) {
			changed = append(changed, evt.ID)
		}
	}

	return changed, nil
}

func (m *menuServer) AboutToShow(itemID int32) (bool, *dbus.Error) {
	return false, nil
}

func (m *menuServer) AboutToShowGroup(ids []int32) ([]struct {
	ID          int32
	NeedsUpdate bool
}, *dbus.Error,
) {
	return nil, nil
}

// getMenuProperty returns the menu's top-level Properties (Version, Status,
// TextDirection) consumed by the host via org.freedesktop.DBus.Properties.
func getMenuProperty(iface, prop string) (dbus.Variant, error) {
	if iface != "com.canonical.dbusmenu" {
		return dbus.Variant{}, errUnknownInterface
	}

	switch prop {
	case "Version":
		return dbus.MakeVariant(dbusmenuVersion), nil
	case "Status":
		return dbus.MakeVariant("normal"), nil
	case "TextDirection":
		return dbus.MakeVariant("ltr"), nil
	default:
		return dbus.Variant{}, errUnknownProperty
	}
}

// menuItemByID returns a menu item by id under a brief read lock. It exists so
// the D-Bus method handlers do not cuddle lock/unlock with unrelated
// statements (wsl_v5) and keep the hot path allocation-free.
func menuItemByID(id int) (*MenuItem, bool) {
	menuItemsLock.RLock()
	defer menuItemsLock.RUnlock()

	item, ok := menuItems[id]

	return item, ok
}

// sendClicked delivers a non-blocking click signal to the item's channel and
// reports whether a signal was dropped (channel full) or sent. The host treats
// a click as no-op when no listener is attached yet.
func sendClicked(item *MenuItem) bool {
	if item.ClickedCh == nil {
		return false
	}

	select {
	case item.ClickedCh <- struct{}{}:
		return true
	default:
		return false
	}
}

// buildMenuNodeLocked constructs a menuNode for the given parent id, recursing
// up to recursionDepth (-1 means unlimited). propertyNames filters which
// properties are returned; an empty list means all. The caller must hold
// menuItemsLock (read) so the recursion does not re-lock.
func buildMenuNodeLocked(parentID, recursionDepth int, propertyNames []string) menuNode {
	node := menuNode{
		ID:    parentID,
		Props: map[string]dbus.Variant{},
	}

	if parentID != 0 {
		if item, ok := menuItems[parentID]; ok {
			node.Props = item.props(propertyNames)
		}
	}

	var childIDs []int

	if parentID == 0 {
		childIDs = rootChildren
	} else if parent, ok := menuItems[parentID]; ok {
		childIDs = parent.children
	}

	if recursionDepth == 0 {
		return node
	}

	nextDepth := recursionDepth
	if nextDepth > 0 {
		nextDepth--
	}

	for _, cid := range childIDs {
		child := buildMenuNodeLocked(cid, nextDepth, propertyNames)
		node.Children = append(node.Children, dbus.MakeVariant(child))
	}

	return node
}

// pngToPixmaps decodes a PNG byte slice into an SNI IconPixmap (a(iiay)) with
// ARGB32 network-byte-order pixels. Returns nil if the PNG is invalid so the
// caller can fall back to an iconless tray instead of failing to register.
func pngToPixmaps(png []byte) []pixmap {
	img, err := decodePNG(png)
	if err != nil {
		return nil
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// ARGB32, network byte order: alpha, red, green, blue per pixel.
	data := make([]byte, 0, width*height*bytesPerPixel)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			px, _ := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			data = append(data, px.A, px.R, px.G, px.B)
		}
	}

	return []pixmap{{W: width, H: height, Data: data}}
}

// decodePNG wraps image.Decode so the PNG driver (registered by the blank
// image/png import) handles the format without leaking the dependency into the
// caller's import graph.
func decodePNG(b []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(b))

	return img, err
}
