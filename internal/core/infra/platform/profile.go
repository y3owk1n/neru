package platform

import "os"

// DisplayServer identifies the active or planned display-system family.
type DisplayServer string

const (
	// DisplayServerCocoa is the native macOS window/display stack.
	DisplayServerCocoa DisplayServer = "cocoa"
	// DisplayServerWayland is the Linux Wayland compositor stack.
	DisplayServerWayland DisplayServer = "wayland"
	// DisplayServerX11 is the Linux X11 stack.
	DisplayServerX11 DisplayServer = "x11"
	// DisplayServerWin32 is the Windows desktop/windowing stack.
	DisplayServerWin32 DisplayServer = "win32"
	// DisplayServerUnknown means the display server could not be identified yet.
	DisplayServerUnknown DisplayServer = "unknown"
)

// BuildMode describes whether a backend is expected to need CGO.
type BuildMode string

const (
	// BuildModePureGo means the backend should build without CGO.
	BuildModePureGo BuildMode = "pure_go"
	// BuildModeCGORequired means the backend depends on CGO/native linkage.
	BuildModeCGORequired BuildMode = "cgo_required"
	// BuildModeBackendDependent means the answer depends on which backend is chosen.
	BuildModeBackendDependent BuildMode = "backend_dependent"
)

const defaultPrimaryModifier = "ctrl"

// BackendPlan describes the intended backend family for one subsystem and
// whether contributors should expect CGO to be required.
type BackendPlan struct {
	Name      string
	BuildMode BuildMode
	Notes     string
}

// Profile describes the platform conventions contributors should target when
// adding OS-specific implementations.
type Profile struct {
	OS              OS
	PrimaryModifier string
	DisplayServer   DisplayServer
	Accessibility   BackendPlan
	Hotkeys         BackendPlan
	KeyboardCapture BackendPlan
	Overlay         BackendPlan
	Notifications   BackendPlan
}

// ProfileFor returns the contributor-facing platform profile for a target OS.
func ProfileFor(target OS) Profile {
	switch target {
	case Darwin:
		return Profile{
			OS:              Darwin,
			PrimaryModifier: "cmd",
			DisplayServer:   DisplayServerCocoa,
			Accessibility: BackendPlan{
				Name:      "axuielement",
				BuildMode: BuildModeCGORequired,
				Notes:     "Objective-C bridge into macOS accessibility APIs",
			},
			Hotkeys: BackendPlan{
				Name:      "carbon-hotkeys",
				BuildMode: BuildModeCGORequired,
				Notes:     "Carbon registration lives behind the Objective-C bridge",
			},
			KeyboardCapture: BackendPlan{
				Name:      "quartz-event-tap",
				BuildMode: BuildModeCGORequired,
				Notes:     "Quartz event taps use CGO-backed Cocoa/CoreGraphics bindings",
			},
			Overlay: BackendPlan{
				Name:      "cocoa-overlay-window",
				BuildMode: BuildModeCGORequired,
				Notes:     "Native overlay windows are implemented through Cocoa",
			},
			Notifications: BackendPlan{
				Name:      "usernotifications/nsalert",
				BuildMode: BuildModeCGORequired,
				Notes:     "Current macOS notifications and alerts use the native bridge",
			},
		}
	case Linux:
		return Profile{
			OS:              Linux,
			PrimaryModifier: "ctrl",
			DisplayServer:   DetectLinuxDisplayServer(),
			Accessibility: BackendPlan{
				Name:      "at-spi",
				BuildMode: BuildModePureGo,
				Notes:     "Prefer D-Bus/pure-Go bindings before reaching for CGO",
			},
			Hotkeys: BackendPlan{
				Name:      "x11 or compositor-specific backend",
				BuildMode: BuildModeBackendDependent,
				Notes:     "X11 may stay pure Go; Wayland/compositor paths may need CGO or native helpers",
			},
			KeyboardCapture: BackendPlan{
				Name:      "x11 or compositor-specific backend",
				BuildMode: BuildModeBackendDependent,
				Notes:     "Backend choice determines whether pure Go is enough",
			},
			Overlay: BackendPlan{
				Name:      "x11 window or wayland layer-shell",
				BuildMode: BuildModeBackendDependent,
				Notes:     "Simple X11 overlays may stay pure Go; Wayland paths may require native linkage",
			},
			Notifications: BackendPlan{
				Name:      "freedesktop notifications",
				BuildMode: BuildModePureGo,
				Notes:     "D-Bus notifications should be achievable without CGO",
			},
		}
	case Windows:
		return Profile{
			OS:              Windows,
			PrimaryModifier: "ctrl",
			DisplayServer:   DisplayServerWin32,
			Accessibility: BackendPlan{
				Name:      "uia",
				BuildMode: BuildModePureGo,
				Notes:     "Prefer COM/Win32 bindings through x/sys or equivalent wrappers",
			},
			Hotkeys: BackendPlan{
				Name:      "RegisterHotKey",
				BuildMode: BuildModePureGo,
				Notes:     "Win32 hotkeys should not require CGO for a first implementation",
			},
			KeyboardCapture: BackendPlan{
				Name:      "low-level keyboard hook",
				BuildMode: BuildModePureGo,
				Notes:     "Hooks are reachable through Win32/syscall bindings",
			},
			Overlay: BackendPlan{
				Name:      "layered win32 window",
				BuildMode: BuildModePureGo,
				Notes:     "Keep the first overlay implementation CGO-free if practical",
			},
			Notifications: BackendPlan{
				Name:      "windows toast",
				BuildMode: BuildModePureGo,
				Notes:     "Toast APIs are expected to be reachable without CGO",
			},
		}
	case Unknown:
		return Profile{
			OS:              Unknown,
			PrimaryModifier: defaultPrimaryModifier,
			DisplayServer:   DisplayServerUnknown,
		}
	default:
		return Profile{
			OS:              Unknown,
			PrimaryModifier: defaultPrimaryModifier,
			DisplayServer:   DisplayServerUnknown,
		}
	}
}

// CurrentProfile returns the contributor-facing profile for the running OS.
func CurrentProfile() Profile {
	return ProfileFor(CurrentOS())
}

// DetectLinuxDisplayServer identifies the Linux display stack from environment
// variables. It is intentionally conservative because backend selection is an
// important contributor decision point.
func DetectLinuxDisplayServer() DisplayServer {
	return detectLinuxDisplayServer(
		os.Getenv("XDG_SESSION_TYPE"),
		os.Getenv("WAYLAND_DISPLAY"),
		os.Getenv("DISPLAY"),
	)
}

func detectLinuxDisplayServer(sessionType, waylandDisplay, xDisplay string) DisplayServer {
	switch {
	case stringsEqualFold(sessionType, "wayland"), waylandDisplay != "":
		return DisplayServerWayland
	case stringsEqualFold(sessionType, "x11"), xDisplay != "":
		return DisplayServerX11
	default:
		return DisplayServerUnknown
	}
}

func stringsEqualFold(left, right string) bool {
	if len(left) != len(right) {
		return false
	}

	for idx := range left {
		leftByte := left[idx]
		if 'A' <= leftByte && leftByte <= 'Z' {
			leftByte += 'a' - 'A'
		}

		rightByte := right[idx]
		if 'A' <= rightByte && rightByte <= 'Z' {
			rightByte += 'a' - 'A'
		}

		if leftByte != rightByte {
			return false
		}
	}

	return true
}
