//go:build linux

package linux

import (
	"image"
	"os"
	"time"

	"go.uber.org/zap"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	linuxplatform "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/mousestate"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// Element is a UI element for Linux (e.g., AT-SPI).
type Element struct {
	bundleIdentifier string
	title            string
	pid              int
}

// linuxHeldButtons records which mouse buttons Neru is currently holding down.
var linuxHeldButtons mousestate.Tracker

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

// SetClickableRoles configures which accessibility roles are treated as clickable (Linux stub).
func SetClickableRoles(_ []string, _ *zap.Logger) {}

// ClickableRoles returns the configured clickable roles (Linux stub).
func ClickableRoles() []string { return nil }

// ElementInfo contains metadata and positioning information for a UI element.
type ElementInfo struct {
	position        image.Point
	size            image.Point
	title           string
	description     string
	value           string
	searchText      string
	role            string
	subrole         string
	roleDescription string
	isEnabled       bool
	isFocused       bool
	pid             int
}

// Position returns the element position.
func (ei *ElementInfo) Position() image.Point { return ei.position }

// Size returns the element size.
func (ei *ElementInfo) Size() image.Point { return ei.size }

// Title returns the element title.
func (ei *ElementInfo) Title() string { return ei.title }

// Description returns the element description.
func (ei *ElementInfo) Description() string { return ei.description }

// Value returns the element value.
func (ei *ElementInfo) Value() string { return ei.value }

// SearchText returns extra searchable text collected from descendant elements.
func (ei *ElementInfo) SearchText() string { return ei.searchText }

// Role returns the element role.
func (ei *ElementInfo) Role() string { return ei.role }

// Subrole returns the element subrole.
func (ei *ElementInfo) Subrole() string { return ei.subrole }

// RoleDescription returns the element role description.
func (ei *ElementInfo) RoleDescription() string { return ei.roleDescription }

// IsEnabled returns whether the element is enabled.
func (ei *ElementInfo) IsEnabled() bool { return ei.isEnabled }

// IsFocused returns whether the element is focused.
func (ei *ElementInfo) IsFocused() bool { return ei.isFocused }

// PID returns the element's process ID.
func (ei *ElementInfo) PID() int { return ei.pid }

// CheckAccessibilityPermissions verifies permissions for Linux (Linux stub).
func CheckAccessibilityPermissions() bool { return true }

// SystemWideElement returns the system-wide element (Linux stub).
func SystemWideElement() *Element { return nil }

// FocusedApplication returns the focused application.
func FocusedApplication() *Element {
	if currentLinuxBackend() == linuxBackendWayland {
		bundleID, pid := wlrootsFocusedApplicationIdentity()
		if bundleID != "" || pid != 0 {
			return &Element{
				bundleIdentifier: bundleID,
				pid:              pid,
			}
		}

		if os.Getenv("DISPLAY") != "" {
			bundleID, pid = linuxFocusedApplicationIdentity()
			if bundleID != "" || pid != 0 {
				return &Element{
					bundleIdentifier: bundleID,
					pid:              pid,
				}
			}
		}

		return nil
	}

	bundleID, pid := linuxFocusedApplicationIdentity()
	if bundleID == "" && pid == 0 {
		return nil
	}

	return &Element{
		bundleIdentifier: bundleID,
		pid:              pid,
	}
}

// ApplicationByPID returns an application by PID.
func ApplicationByPID(pid int) *Element {
	if currentLinuxBackend() == linuxBackendWayland {
		bundleID := wlrootsApplicationBundleIdentifier(pid)
		if bundleID != "" {
			return &Element{
				bundleIdentifier: bundleID,
				pid:              pid,
			}
		}

		if os.Getenv("DISPLAY") != "" {
			bundleID = linuxApplicationBundleIdentifier(pid)
			if bundleID != "" {
				return &Element{
					bundleIdentifier: bundleID,
					pid:              pid,
				}
			}
		}

		return nil
	}

	bundleID := linuxApplicationBundleIdentifier(pid)
	if bundleID == "" {
		return nil
	}

	return &Element{
		bundleIdentifier: bundleID,
		pid:              pid,
	}
}

// ApplicationByBundleID returns an application by bundle ID.
func ApplicationByBundleID(bundleID string) *Element {
	if bundleID == "" {
		return nil
	}

	return &Element{bundleIdentifier: bundleID}
}

// ElementAtPosition returns the element at a position (Linux stub).
func ElementAtPosition(_, _ int) *Element { return nil }

// Info retrieves metadata and positioning information for the element (Linux stub).
func (e *Element) Info() (*ElementInfo, error) {
	return &ElementInfo{
		title: e.title,
		pid:   e.pid,
	}, nil
}

// Children returns the element's children (Linux stub).
func (e *Element) Children(role string) ([]*Element, error) { return nil, nil }

// SetFocus sets focus on the element (Linux stub).
func (e *Element) SetFocus() error { return nil }

// Attribute returns the value of the named attribute (Linux stub).
func (e *Element) Attribute(_ string) (string, error) { return "", nil }

// Release releases the element (Linux stub).
func (e *Element) Release() {}

// ReleaseAll releases all elements (Linux stub).
func ReleaseAll(_ []*Element) {}

// Hash returns a hash of the element (Linux stub).
func (e *Element) Hash() (uint64, error) { return 0, nil }

// Equal returns true if the elements are equal (Linux stub).
func (e *Element) Equal(_ *Element) bool { return false }

// Clone returns a clone of the element (Linux stub).
func (e *Element) Clone() (*Element, error) { return &Element{}, nil }

// AllWindows returns all windows (Linux stub).
func AllWindows() ([]*Element, error) { return []*Element{}, nil }

// FrontmostAndPopoverWindows returns frontmost/popover windows (Linux stub).
func FrontmostAndPopoverWindows() ([]*Element, error) { return []*Element{}, nil }

// FrontmostWindow returns the frontmost window.
func FrontmostWindow() *Element {
	if currentLinuxBackend() == linuxBackendWayland {
		bundleID, pid := wlrootsFocusedApplicationIdentity()
		if bundleID != "" || pid != 0 {
			return &Element{
				bundleIdentifier: bundleID,
				pid:              pid,
			}
		}

		if os.Getenv("DISPLAY") != "" {
			bundleID, pid = linuxFocusedApplicationIdentity()
			if bundleID != "" || pid != 0 {
				return &Element{
					bundleIdentifier: bundleID,
					pid:              pid,
				}
			}
		}

		return nil
	}

	bundleID, pid := linuxFocusedApplicationIdentity()
	if bundleID == "" && pid == 0 {
		return nil
	}

	return &Element{
		bundleIdentifier: bundleID,
		pid:              pid,
	}
}

// MenuBar returns the menu bar element (Linux stub).
func (e *Element) MenuBar() *Element { return nil }

// ApplicationName returns the application name (Linux stub).
func (e *Element) ApplicationName() string { return e.bundleIdentifier }

// BundleIdentifier returns the bundle identifier (Linux stub).
func (e *Element) BundleIdentifier() string { return e.bundleIdentifier }

// ScrollBounds returns the scroll bounds (Linux stub).
func (e *Element) ScrollBounds() image.Rectangle { return image.Rectangle{} }

// IsMouseButtonDown returns whether the given mouse button is held down.
func IsMouseButtonDown(button action.MouseButton) bool {
	return linuxHeldButtons.IsDown(button)
}

// EnsureMouseUp releases every mouse button Neru is currently holding down.
// EnsureMouseUp releases any mouse button left held.
func EnsureMouseUp() {
	for _, button := range linuxHeldButtons.HeldButtons() {
		_ = MouseUp(button)
	}
}

// MoveMouseToPoint moves the mouse.
func MoveMouseToPoint(point image.Point, _ bool) {
	if currentLinuxBackend() == linuxBackendX11 {
		_ = x11MoveMouseToPoint(point)
	} else if currentLinuxBackend() == linuxBackendWayland {
		_ = wlrootsMoveMouseToPoint(point)
	}
}

// LeftClickAtPoint performs a left click.
func LeftClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11LeftClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsLeftClickAtPoint(point, restoreCursor, modifiers)
	}

	return nil
}

// RightClickAtPoint performs a right click.
func RightClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11RightClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsRightClickAtPoint(point, restoreCursor, modifiers)
	}

	return nil
}

// MiddleClickAtPoint performs a middle click.
func MiddleClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11MiddleClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsMiddleClickAtPoint(point, restoreCursor, modifiers)
	}

	return nil
}

// MouseDownAtPoint presses and holds the given mouse button at the point.
func MouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	if currentLinuxBackend() == linuxBackendX11 {
		err := x11MouseDownAtPoint(point, button, modifiers)
		if err == nil {
			linuxHeldButtons.SetDown(button, point, modifiers)
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsMouseDownAtPoint(point, button, modifiers)
		if err == nil {
			linuxHeldButtons.SetDown(button, point, modifiers)
		}

		return err
	}

	return nil
}

// MouseUpAtPoint releases the given mouse button at the point.
//
// A press holds its modifiers until the matching release, so the release has to
// undo the set recorded at press time rather than whatever this call was given —
// releasing a modified press with a bare "up" action would otherwise leave the
// modifier logically held. The wlroots path applies the same rule internally.
func MouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	if currentLinuxBackend() == linuxBackendX11 {
		if heldModifiers, wasHeld := linuxHeldButtons.DownModifiers(button); wasHeld {
			modifiers = heldModifiers
		}

		err := x11MouseUpAtPoint(point, button, modifiers)
		if err == nil {
			linuxHeldButtons.Clear(button)
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsMouseUpAtPoint(point, button, modifiers)
		if err == nil {
			linuxHeldButtons.Clear(button)
		}

		return err
	}

	return nil
}

// MouseUp releases the given mouse button at the cursor, together with the
// modifiers its press is holding. This is the idle-cleanup path, so a modified
// press that is never explicitly released still leaves nothing held.
func MouseUp(button action.MouseButton) error {
	if currentLinuxBackend() == linuxBackendX11 {
		heldModifiers, _ := linuxHeldButtons.DownModifiers(button)

		err := x11MouseUp(button, heldModifiers)
		if err == nil {
			linuxHeldButtons.Clear(button)
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsMouseUp(button)
		if err == nil {
			linuxHeldButtons.Clear(button)
		}

		return err
	}

	return nil
}

// scrollPixelsPerNotch is how many pixels of a caller's delta make one wheel
// notch. The scroll service speaks in pixels (scroll_step = 50,
// scroll_step_half = 500, scroll_step_full = 1000000) and every Linux injection
// path has to land on the same conversion, or the same binding would travel a
// different distance depending on which backend answered.
const scrollPixelsPerNotch = 30

// ScrollAtCursor scrolls at the cursor, presenting modifiers as held.
//
// Linux has no event-flags concept, so a modifier is a real key press around
// the scroll. On Wayland that forces a choice of injection layer: the modifier
// can only be pressed on the wlroots virtual keyboard (or libei on KDE), while
// the fast path for the scroll itself is the uinput evdev device. Hyprland
// reliably handles the uinput path, provided the virtual keyboard modifier has
// had time to reach the compositor first.
//
// With smooth_scroll.enabled the scroll is handed to the animator instead and
// arrives as a sequence of eased chunks. Modified Wayland scrolls use the
// immediate uinput path instead, because Hyprland does not reliably merge the
// virtual keyboard modifier with virtual-pointer scroll events.
func ScrollAtCursor(deltaX, deltaY int, modifiers action.Modifiers) error {
	cfg := currentLinuxConfig()
	if cfg != nil && cfg.SmoothScroll.Enabled {
		if currentLinuxBackend() == linuxBackendWayland && modifiers != 0 {
			scrollAnim.stop()

			return scrollAtCursorNow(deltaX, deltaY, modifiers)
		}

		err := scrollBackendAvailable()
		if err != nil {
			return err
		}

		if deltaX == 0 && deltaY == 0 {
			return nil
		}

		scrollAnim.animate(
			deltaX,
			deltaY,
			modifiers,
			cfg.SmoothScroll.Steps,
			cfg.SmoothScroll.MaxDuration,
			cfg.SmoothScroll.DurationPerPixel,
		)

		return nil
	}

	// A scroll arriving with the animation switched off must not be chased by
	// chunks scheduled before the reload.
	scrollAnim.stop()

	return scrollAtCursorNow(deltaX, deltaY, modifiers)
}

// scrollAtCursorNow injects the whole scroll in one go, which is what every
// caller got before smooth scroll existed and what a caller still gets with it
// switched off.
func scrollAtCursorNow(deltaX, deltaY int, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11ScrollAtCursor(deltaX, deltaY, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := pressWaylandScrollModifiers(modifiers)
		if err != nil {
			return err
		}
		defer func() { _ = releaseWaylandScrollModifiers(modifiers) }()

		// Keyboard and uinput events arrive through separate input sources.
		// Let Hyprland apply the modifier before the scroll device reports it.
		if modifiers != 0 {
			const modifierSettleDelay = 5 * time.Millisecond
			time.Sleep(modifierSettleDelay)
		}

		// Scale factor: each uinput scroll event approximates ~1 line.
		const scrollScale = scrollPixelsPerNotch

		// maxBatchEvents caps the number of uinput events sent per
		// write/flush to avoid overflowing the kernel evdev buffer (~8192
		// bytes) or the Wayland socket buffer.  Each batch is kept small so
		// the compositor and client can process events incrementally.
		const maxBatchEvents = 50

		sendScaledScroll := func(axis int, delta int) int {
			if delta == 0 {
				return 0
			}

			totalNotches := abs(delta) / scrollScale
			if totalNotches == 0 {
				totalNotches = 1
			}

			remainingNotches := totalNotches
			batch := make([]int, 0, maxBatchEvents)

			value := 1
			if delta < 0 {
				value = -1
			}

			for remainingNotches > 0 {
				batch = append(batch, value)
				remainingNotches--

				if len(batch) >= maxBatchEvents || remainingNotches == 0 {
					err := eventtaplinux.ScrollDeviceScrollBatch(axis, batch)
					if err != nil {
						// uinput unavailable — add back unsent notches so the
						// remaining delta is retried via wlroots virtual pointer
						// fallback without double-counting already-sent notches.
						remainingNotches += len(batch)

						break
					}

					batch = batch[:0]
				}
			}

			notchesSent := totalNotches - remainingNotches

			pixelsSent := notchesSent * scrollScale
			if delta > 0 {
				return max(delta-pixelsSent, 0)
			}

			return min(delta+pixelsSent, 0)
		}

		remainY := sendScaledScroll(0, deltaY)
		remainX := sendScaledScroll(1, deltaX)

		if remainY == 0 && remainX == 0 {
			return nil
		}

		return wlrootsScrollAtCursor(remainX, remainY, 0)
	}

	// No backend to inject through. An unmodified scroll has always been a
	// silent no-op here; a modified one must not be, because the user would
	// read the missing zoom as a broken binding rather than as a missing
	// backend.
	if modifiers != 0 {
		return derrors.Newf(
			derrors.CodeNotSupported,
			"modified scroll (%s) is not supported without an X11 or Wayland backend",
			modifiers,
		)
	}

	return nil
}

func pressWaylandScrollModifiers(modifiers action.Modifiers) error {
	for _, item := range []struct {
		mod  action.Modifiers
		name string
	}{
		{action.ModShift, "shift"},
		{action.ModCtrl, "ctrl"},
		{action.ModAlt, "alt"},
		{action.ModCmd, "cmd"},
	} {
		if !modifiers.Has(item.mod) {
			continue
		}

		err := linuxplatform.WaylandModifierEvent(item.name, true)
		if err != nil {
			_ = releaseWaylandScrollModifiers(modifiers)
			return err
		}
	}

	return nil
}

func releaseWaylandScrollModifiers(modifiers action.Modifiers) error {
	var firstErr error
	for _, item := range []struct {
		mod  action.Modifiers
		name string
	}{
		{action.ModCmd, "cmd"},
		{action.ModAlt, "alt"},
		{action.ModCtrl, "ctrl"},
		{action.ModShift, "shift"},
	} {
		if !modifiers.Has(item.mod) {
			continue
		}

		err := linuxplatform.WaylandModifierEvent(item.name, false)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// CurrentCursorPosition returns the cursor position.
func CurrentCursorPosition() image.Point {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11CurrentCursorPosition()
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsCurrentCursorPosition()
	}

	return image.Point{}
}

// IsClickable checks if the element is clickable (Linux stub).
func (e *Element) IsClickable(
	_ *ElementInfo,
	_ map[string]struct{},
	_ config.Provider,
	_ bool,
) bool {
	return false
}

// IsMissionControlActive returns whether Mission Control is active (Linux stub).
func IsMissionControlActive() bool { return false }

type linuxBackend string

const (
	linuxBackendUnknown linuxBackend = "unknown"
	linuxBackendX11     linuxBackend = "x11"
	linuxBackendWayland linuxBackend = "wayland"
)

// DetectBundleType returns "" on Linux (stub).
func DetectBundleType(_ string) string { return "" }

// currentLinuxBackend delegates to the canonical platform.DetectLinuxBackend
// so that compositor-family detection (GNOME, KDE, wlroots, etc.) is
// consistent across all layers.
func currentLinuxBackend() linuxBackend {
	switch platform.DetectLinuxBackend() {
	case platform.BackendX11:
		return linuxBackendX11
	case platform.BackendWaylandWlroots, platform.BackendWaylandKDE:
		return linuxBackendWayland
	case platform.BackendUnknown, platform.BackendWaylandGNOME,
		platform.BackendWaylandOther:
		return linuxBackendUnknown
	}

	return linuxBackendUnknown
}
