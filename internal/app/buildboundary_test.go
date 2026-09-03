package app

import (
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

// TestAnnounceBuildBoundary pins ADR 0012's criterion applied to a build
// variant: a CGO_ENABLED=0 Linux daemon starts and then fails feature by
// feature, so it must say once, up front, what kind of build it is — and no
// other build may say it, or the warning is noise the blessed stack teaches
// people to ignore.
func TestAnnounceBuildBoundary(t *testing.T) {
	tests := []struct {
		name       string
		targetOS   platform.OS
		cgoEnabled bool
		wantWarn   bool
	}{
		{
			name:       "linux build without cgo announces its boundary",
			targetOS:   platform.Linux,
			cgoEnabled: false,
			wantWarn:   true,
		},
		{
			name:       "linux build with cgo says nothing",
			targetOS:   platform.Linux,
			cgoEnabled: true,
		},
		{
			name:       "windows build without cgo says nothing",
			targetOS:   platform.Windows,
			cgoEnabled: false,
		},
		{
			name:       "darwin build says nothing",
			targetOS:   platform.Darwin,
			cgoEnabled: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)

			announceBuildBoundary(zap.New(core), testCase.targetOS, testCase.cgoEnabled)

			entries := logs.All()

			if !testCase.wantWarn {
				if len(entries) != 0 {
					t.Fatalf("announced %d entries on a supported build: %v", len(entries), entries)
				}

				return
			}

			if len(entries) != 1 {
				t.Fatalf("want exactly one announcement, got %d: %v", len(entries), entries)
			}

			entry := entries[0]

			if entry.Level != zapcore.WarnLevel {
				t.Errorf("announcement level = %v, want warn", entry.Level)
			}

			if !strings.Contains(entry.Message, "CGO_ENABLED=0") {
				t.Errorf("announcement does not name the build: %q", entry.Message)
			}

			unavailable := fmt.Sprint(entry.ContextMap()["unavailable"])
			if unavailable == "" || unavailable == "[]" {
				t.Fatalf("announcement names nothing that will not work: %v", entry.ContextMap())
			}

			// A list that omits the capabilities a person reaches for first
			// leaves them discovering the boundary one keystroke at a time,
			// which is the whole failure this announcement exists to end.
			for _, want := range []string{"global hotkeys", "overlay"} {
				if !strings.Contains(unavailable, want) {
					t.Errorf("announcement does not name %q as unavailable: %v", want, unavailable)
				}
			}

			if entry.ContextMap()["remedy"] == "" {
				t.Error("announcement offers no remedy")
			}
		})
	}
}
