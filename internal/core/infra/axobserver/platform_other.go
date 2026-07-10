//go:build !darwin

package axobserver

// unsupportedPlatform is the observer backend on platforms that have no AX
// observer implementation yet. The Manager is platform-independent, so it always
// has a Platform to talk to; here every Arm fails, so nothing is ever armed and
// no change is ever reported. Hints still work, they just do not auto-refresh.
//
// To add push-based auto-refresh for a platform, replace this with a build-tagged
// backend (for example platform_linux.go) that implements Platform against the
// OS accessibility API, mirroring platform_darwin.go.
type unsupportedPlatform struct{}

func newPlatform() Platform {
	return unsupportedPlatform{}
}

func (unsupportedPlatform) Arm(_ int, _ Mask) bool                { return false }
func (unsupportedPlatform) Disarm(_ int)                          {}
func (unsupportedPlatform) DisarmAll()                            {}
func (unsupportedPlatform) SetSink(_ func(pid int, notif string)) {}
func (unsupportedPlatform) Close()                                {}
