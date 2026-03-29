package ports_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/ports"
)

func TestDarwinCapabilities_ReportSupportedFeatures(t *testing.T) {
	capabilities := ports.DarwinCapabilities()

	if capabilities.Platform != "darwin" {
		t.Fatalf("Platform = %q, want darwin", capabilities.Platform)
	}

	if capabilities.Overlay.Status != ports.FeatureStatusSupported {
		t.Fatalf("Overlay status = %q, want supported", capabilities.Overlay.Status)
	}

	if capabilities.KeyboardEventTap.Status != ports.FeatureStatusSupported {
		t.Fatalf(
			"KeyboardEventTap status = %q, want supported",
			capabilities.KeyboardEventTap.Status,
		)
	}
}

func TestNonDarwinCapabilities_ReportStubbedFeatures(t *testing.T) {
	tests := []struct {
		name         string
		capabilities ports.PlatformCapabilities
	}{
		{name: "linux", capabilities: ports.LinuxCapabilities()},
		{name: "windows", capabilities: ports.WindowsCapabilities()},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.capabilities.Platform != testCase.name {
				t.Fatalf("Platform = %q, want %s", testCase.capabilities.Platform, testCase.name)
			}

			if testCase.capabilities.Overlay.Status != ports.FeatureStatusStub {
				t.Fatalf("Overlay status = %q, want stub", testCase.capabilities.Overlay.Status)
			}

			if testCase.capabilities.Process.Status != ports.FeatureStatusStub {
				t.Fatalf("Process status = %q, want stub", testCase.capabilities.Process.Status)
			}
		})
	}
}
