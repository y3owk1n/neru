//go:build windows

package windows

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
	"github.com/y3owk1n/neru/internal/adapter/platform/fontgeneric"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	defaultWindowsSans  = "Segoe UI"
	defaultWindowsMono  = "Consolas"
	defaultWindowsSerif = "Cambria"

	// lfFaceSize is LF_FACESIZE: the LOGFONTW face-name field, terminator
	// included.
	lfFaceSize     = 32
	defaultCharset = 1
)

// windowsFamilies is what the generic aliases mean on Windows.
var windowsFamilies = fontgeneric.Families{
	Sans:  defaultWindowsSans,
	Serif: defaultWindowsSerif,
	Mono:  defaultWindowsMono,
}

// logFontW mirrors LOGFONTW; only lfCharSet and lfFaceName take part in an
// EnumFontFamiliesExW query.
type logFontW struct {
	height         int32
	width          int32
	escapement     int32
	orientation    int32
	weight         int32
	italic         byte
	underline      byte
	strikeOut      byte
	charSet        byte
	outPrecision   byte
	clipPrecision  byte
	quality        byte
	pitchAndFamily byte
	faceName       [lfFaceSize]uint16
}

var (
	procEnumFontFamiliesExW = gdi32.NewProc("EnumFontFamiliesExW")

	// fontEnumProcPtr is allocated once for the process (see
	// displayWndProcPtr).
	fontEnumProcPtr = sync.OnceValue(func() uintptr {
		return syscall.NewCallback(fontEnumProc)
	})

	// fontEnumMu serializes enumerations so fontEnumFound belongs to one query
	// at a time. The flag lives here rather than behind the LPARAM because a
	// Go pointer smuggled through a uintptr may not survive the callback
	// growing the stack.
	fontEnumMu    sync.Mutex
	fontEnumFound bool
)

// fontEnumProc is the FONTENUMPROCW EnumFontFamiliesExW calls once per
// matching face. Being called at all is the answer, so it records that and
// returns 0 to stop the enumeration.
func fontEnumProc(_, _ uintptr, _ uint32, _ uintptr) uintptr {
	fontEnumFound = true
	return 0
}

// NewFontResolver returns a GDI-backed ports.FontResolver. Each family is
// resolved on first use and cached for the lifetime of the process.
func NewFontResolver() ports.FontResolver {
	return &winFontResolver{cache: fontcache.New(resolveWindowsFamily)}
}

// winFontResolver implements ports.FontResolver. It maps generic aliases to
// the Windows baseline families and asks GDI whether a user-supplied family
// is installed, falling back to the sans baseline when it is not.
type winFontResolver struct {
	cache *fontcache.Resolver
}

// Resolve implements ports.FontResolver.
func (r *winFontResolver) Resolve(family string) string {
	return r.cache.Resolve(family)
}

// resolveWindowsFamily maps the input to a concrete family and asks GDI
// whether that family is installed: if it is, the answer is the name as
// written, which is what ports.FontResolver promises. If it is not, the answer
// is the sans baseline, the rule the fontconfig-backed Linux resolver follows.
// When GDI cannot be consulted nothing can be verified, so the name passes
// through as written and the text layer substitutes when it draws.
func resolveWindowsFamily(family string) string {
	mapped := windowsFamilies.Resolve(family)
	if familyInstalled(mapped) == familyMissing {
		return defaultWindowsSans
	}

	return mapped
}

// familyPresence is what GDI can say about a family. An unusable device
// context is not the same answer as a family that is missing.
type familyPresence int

const (
	familyUnknown familyPresence = iota
	familyMissing
	familyPresent
)

// familyInstalled asks GDI whether any installed face carries the given
// family name. EnumFontFamiliesExW with a face name set enumerates only that
// family, comparing names the way GDI does, ignoring case. A name GDI cannot
// hold, longer than LOGFONTW allows, cannot be selected by GDI either and
// counts as missing.
func familyInstalled(family string) familyPresence {
	name, err := windows.UTF16FromString(family)
	if err != nil || len(name) > lfFaceSize {
		return familyMissing
	}

	fontEnumMu.Lock()
	defer fontEnumMu.Unlock()

	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return familyUnknown
	}

	defer func() { discardCall(procDeleteDC.Call(hdc)) }()

	logFont := logFontW{charSet: defaultCharset}
	copy(logFont.faceName[:], name)

	fontEnumFound = false

	discardCall(procEnumFontFamiliesExW.Call(
		hdc,
		uintptr(unsafe.Pointer(&logFont)),
		fontEnumProcPtr(),
		0,
		0,
	))

	if fontEnumFound {
		return familyPresent
	}

	return familyMissing
}
