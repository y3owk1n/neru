package axnotify

import (
	"sort"
	"testing"
)

func TestNamesIsSortedAndDefaultSet(t *testing.T) {
	names := Names()

	if len(names) != len(defaultNames) {
		t.Fatalf("Names returned %d entries, defaultNames has %d", len(names), len(defaultNames))
	}

	if !sort.StringsAreSorted(names) {
		t.Errorf("Names not sorted: %v", names)
	}

	for _, name := range names {
		if !IsName(name) {
			t.Errorf("Names returned %q which IsName rejects", name)
		}

		if name == ValueChanged {
			t.Errorf("%q is opt-in and must not appear in the default set", ValueChanged)
		}
	}
}

func TestAllNamesIsSortedAndSupersetOfDefaults(t *testing.T) {
	all := AllNames()

	if !sort.StringsAreSorted(all) {
		t.Errorf("AllNames not sorted: %v", all)
	}

	if len(all) != len(defaultNames)+len(optionalNames) {
		t.Fatalf("AllNames returned %d entries, want %d defaults + %d optional",
			len(all), len(defaultNames), len(optionalNames))
	}

	for _, name := range Names() {
		if !contains(all, name) {
			t.Errorf("AllNames is missing default name %q", name)
		}
	}

	if !contains(all, ValueChanged) {
		t.Errorf("AllNames should include the opt-in name %q", ValueChanged)
	}
}

func TestIsName(t *testing.T) {
	if !IsName(LayoutChanged) {
		t.Errorf("%q should be a known notification name", LayoutChanged)
	}

	if !IsName(FocusedUIElementChanged) {
		t.Errorf("%q should be a known notification name", FocusedUIElementChanged)
	}

	if !IsName(ValueChanged) {
		t.Errorf("%q is a valid opt-in notification name", ValueChanged)
	}

	if IsName("not_a_real_notification") {
		t.Error("an unknown name must not be reported as known")
	}

	if IsName("") {
		t.Error("the empty string is not a notification name")
	}
}

func TestNamesReturnsACopy(t *testing.T) {
	first := Names()
	first[0] = "mutated"

	if IsName("mutated") {
		t.Fatal("mutating the returned slice must not affect the registry")
	}

	if second := Names(); second[0] == "mutated" {
		t.Fatal("Names must return a fresh copy on each call")
	}
}

func contains(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}

	return false
}
