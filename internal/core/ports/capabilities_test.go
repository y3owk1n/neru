package ports_test

import (
	"reflect"
	"testing"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// TestPlatformCapabilities_EntriesCoverEveryField is the guardrail behind the
// capability registry: Entries is the single list every renderer iterates, so a
// FeatureCapability field that is not registered would be invisible to
// `neru doctor` and the IPC info response without anything failing to compile.
func TestPlatformCapabilities_EntriesCoverEveryField(t *testing.T) {
	registered := make(map[string]ports.CapabilityKey)

	for _, entry := range (ports.PlatformCapabilities{}).Entries() {
		if entry.Key == "" {
			t.Errorf("entry for field %q has an empty Key", entry.Field)
		}

		if previous, seen := registered[entry.Field]; seen {
			t.Errorf(
				"field %s is registered twice (keys %q and %q)",
				entry.Field,
				previous,
				entry.Key,
			)
		}

		registered[entry.Field] = entry.Key
	}

	featureType := reflect.TypeFor[ports.FeatureCapability]()

	for field := range reflect.TypeFor[ports.PlatformCapabilities]().Fields() {
		if field.Type != featureType {
			continue
		}

		if _, ok := registered[field.Name]; !ok {
			t.Errorf(
				"PlatformCapabilities.%s is not in Entries(); add it there so "+
					"`neru doctor` and the IPC info response report it",
				field.Name,
			)
		}

		delete(registered, field.Name)
	}

	for field, key := range registered {
		t.Errorf("Entries() reports field %q (key %q) that PlatformCapabilities does not have",
			field, key)
	}
}

// TestPlatformCapabilities_EntriesKeysAreUnique pins the wire contract: the
// keys become map keys in the IPC response, so a duplicate would silently drop
// one capability from the output.
func TestPlatformCapabilities_EntriesKeysAreUnique(t *testing.T) {
	seen := make(map[ports.CapabilityKey]string)

	for _, entry := range (ports.PlatformCapabilities{}).Entries() {
		if previous, duplicate := seen[entry.Key]; duplicate {
			t.Errorf(
				"capability key %q is used by both %s and %s",
				entry.Key,
				previous,
				entry.Field,
			)
		}

		seen[entry.Key] = entry.Field
	}
}

// TestPlatformCapabilities_EntriesReadTheMatchingField catches a copy-paste
// slip where an entry is wired to the wrong struct field — the registry would
// still look exhaustive but would report one capability's status under
// another's key.
func TestPlatformCapabilities_EntriesReadTheMatchingField(t *testing.T) {
	featureType := reflect.TypeFor[ports.FeatureCapability]()

	for field := range reflect.TypeFor[ports.PlatformCapabilities]().Fields() {
		if field.Type != featureType {
			continue
		}

		// Give exactly one field a distinctive value, then assert only the
		// entry claiming that field carries it.
		probe := ports.PlatformCapabilities{}
		marker := ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "probe:" + field.Name,
		}

		reflect.ValueOf(&probe).Elem().FieldByIndex(field.Index).Set(reflect.ValueOf(marker))

		for _, entry := range probe.Entries() {
			wantDetail := ""
			if entry.Field == field.Name {
				wantDetail = marker.Detail
			}

			if entry.Detail != wantDetail {
				t.Errorf(
					"entry %q (field %s) has Detail %q, want %q when %s is set",
					entry.Key,
					entry.Field,
					entry.Detail,
					wantDetail,
					field.Name,
				)
			}
		}
	}
}
