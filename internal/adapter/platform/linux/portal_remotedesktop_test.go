//go:build linux

package linux

import (
	"testing"
)

// TestPortalSelectDevicesOptions_OmitsTheRestoreTokenWhenThereIsNothingToRestore
// keeps an empty string out of the options map. A portal that validates the key
// it was given would refuse the call outright, which would turn "no grant
// stored yet" — the ordinary first run — into a hard failure.
func TestPortalSelectDevicesOptions_OmitsTheRestoreTokenWhenThereIsNothingToRestore(
	t *testing.T,
) {
	options := portalSelectDevicesOptions("")

	if _, ok := options[portalRestoreTokenKey]; ok {
		t.Errorf("options carry a restore_token key with nothing to restore: %v", options)
	}
}

// TestPortalSelectDevicesOptions_AsksForAPersistentKeyboardAndPointer pins the
// three values the whole feature rests on: both device types, and a persistence
// mode that outlives the process — persist_mode 1 would keep the grant only
// while this daemon runs, which is exactly the restart that fails today.
func TestPortalSelectDevicesOptions_AsksForAPersistentKeyboardAndPointer(t *testing.T) {
	options := portalSelectDevicesOptions(storedToken)

	types, isUint := options["types"].Value().(uint32)
	if !isUint {
		t.Fatalf("types option = %v, want a uint32", options["types"])
	}

	if types != portalDeviceKeyboard|portalDevicePointer {
		t.Errorf("types = %#b, want keyboard and pointer (%#b)",
			types, portalDeviceKeyboard|portalDevicePointer)
	}

	persist, isUint := options["persist_mode"].Value().(uint32)
	if !isUint {
		t.Fatalf("persist_mode option = %v, want a uint32", options["persist_mode"])
	}

	if persist != portalPersistUntilRevoked {
		t.Errorf("persist_mode = %d, want %d", persist, portalPersistUntilRevoked)
	}

	token, isString := options[portalRestoreTokenKey].Value().(string)
	if !isString || token != storedToken {
		t.Errorf("restore_token = %v, want the stored token", options[portalRestoreTokenKey])
	}
}
