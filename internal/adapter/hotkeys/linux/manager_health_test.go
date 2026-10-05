//go:build linux

package linux

import (
	"testing"

	"go.uber.org/zap"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/ports"
)

// TestManager_HealthCheck pins the answer the sleep/resume handler acts on. A
// false makes the handler re-register hotkeys. The test sets the backend and
// listener directly, so every branch runs whatever session the host has.
func TestManager_HealthCheck(t *testing.T) {
	tests := []struct {
		name      string
		backend   platform.LinuxBackend
		callbacks int
		want      bool
	}{
		{
			name:      "x11 has no evdev listener to monitor",
			backend:   platform.BackendX11,
			callbacks: 1,
			want:      true,
		},
		{
			name:    "wayland with nothing registered",
			backend: platform.BackendWaylandWlroots,
			want:    true,
		},
		{
			// This is the #1092 case. Hotkeys are bound but the listener never
			// started, so nothing reads the keyboard until recovery restarts it.
			name:      "wayland with hotkeys registered and the listener not started",
			backend:   platform.BackendWaylandWlroots,
			callbacks: 1,
			want:      false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mgr := NewManager(zap.NewNop())
			mgr.backend = testCase.backend
			mgr.waylandHotkeys = eventtaplinux.NewGlobalHotkeyListener(nil)

			for i := range testCase.callbacks {
				mgr.callbacks[ports.HotkeyID(i+1)] = hotkeyCallbacks{}
			}

			if got := mgr.HealthCheck(); got != testCase.want {
				t.Errorf("HealthCheck() = %v, want %v", got, testCase.want)
			}
		})
	}
}
