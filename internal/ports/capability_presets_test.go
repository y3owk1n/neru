package ports_test

import (
	"reflect"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
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
		name                string
		capabilities        ports.PlatformCapabilities
		accessibilityStatus ports.FeatureStatus
		notificationsStatus ports.FeatureStatus
	}{
		// Linux discovers clickable elements via an AT-SPI (D-Bus) tree walk;
		// Windows discovers them via UI Automation. Linux notifications go to
		// the freedesktop notification daemon over the same session bus, so the
		// preset is supported and the adapter downgrades it live when no daemon
		// is running; Windows anchors balloon tips to the tray icon it owns.
		{
			name:                "linux",
			capabilities:        ports.LinuxCapabilities(),
			accessibilityStatus: ports.FeatureStatusSupported,
			notificationsStatus: ports.FeatureStatusSupported,
		},
		{
			name:                "windows",
			capabilities:        ports.WindowsCapabilities(),
			accessibilityStatus: ports.FeatureStatusSupported,
			notificationsStatus: ports.FeatureStatusSupported,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.capabilities.Platform != testCase.name {
				t.Fatalf("Platform = %q, want %s", testCase.capabilities.Platform, testCase.name)
			}

			if testCase.capabilities.Accessibility.Status != testCase.accessibilityStatus {
				t.Fatalf(
					"Accessibility status = %q, want %q",
					testCase.capabilities.Accessibility.Status,
					testCase.accessibilityStatus,
				)
			}

			if testCase.capabilities.Notifications.Status != testCase.notificationsStatus {
				t.Fatalf(
					"Notifications status = %q, want %q",
					testCase.capabilities.Notifications.Status,
					testCase.notificationsStatus,
				)
			}
		})
	}
}

func TestCapabilityPresets_PopulateAllCapabilityStatuses(t *testing.T) {
	tests := []struct {
		name         string
		capabilities ports.PlatformCapabilities
	}{
		{name: "darwin", capabilities: ports.DarwinCapabilities()},
		{name: "linux", capabilities: ports.LinuxCapabilities()},
		{name: "windows", capabilities: ports.WindowsCapabilities()},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			capabilitiesValue := reflect.ValueOf(testCase.capabilities)
			capabilitiesType := capabilitiesValue.Type()
			fieldCount := capabilitiesValue.NumField()

			for index := range fieldCount {
				fieldType := capabilitiesType.Field(index)
				if fieldType.Type != reflect.TypeFor[ports.FeatureCapability]() {
					continue
				}

				capability, ok := capabilitiesValue.Field(index).Interface().(ports.FeatureCapability)
				if !ok {
					t.Fatalf(
						"%s is not a FeatureCapability in %s preset",
						fieldType.Name,
						testCase.name,
					)
				}

				if capability.Status == "" {
					t.Fatalf("%s status is empty in %s preset", fieldType.Name, testCase.name)
				}
			}
		})
	}
}
