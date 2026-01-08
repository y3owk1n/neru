package scroll_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/components/scroll"
)

func TestNormalizeKey(t *testing.T) {
	keyMap := scroll.NewKeyMap(map[string][]string{})

	testCases := []struct {
		input    string
		expected string
	}{
		{"Up", scroll.ArrowUp},
		{"Down", scroll.ArrowDown},
		{"Left", scroll.ArrowLeft},
		{"Right", scroll.ArrowRight},
		{"\x1f", scroll.ArrowUp},
		{"\x1e", scroll.ArrowDown},
		{"\x1d", scroll.ArrowLeft},
		{"\x1c", scroll.ArrowRight},
		{"Ctrl+U", "ctrl+u"},
		{"Ctrl+D", "ctrl+d"},
		{"Ctrl+Z", "ctrl+z"},
		{"\x1a", "ctrl+z"},
		{"\x03", "ctrl+c"},
		{"\x07", "ctrl+g"},
		{"Alt+Z", "alt+z"},
		{"Cmd+Z", "cmd+z"},
		{"Option+Z", "option+z"},
		{"k", "k"},
		{"j", "j"},
		{"gg", "gg"},
		{"Home", "Home"},
		{"End", "End"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := keyMap.Normalize(tc.input)
			if result != tc.expected {
				t.Errorf("Normalize(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestKeyMapBasicBindings(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionScrollUp:    {"k", "Up"},
		scroll.ActionScrollDown:  {"j", "Down"},
		scroll.ActionGoTop:       {"gg"},
		scroll.ActionGoBottom:    {"G"},
		scroll.ActionPageUp:      {"ctrl+u", "PageUp", "cmd+up"},
		scroll.ActionPageDown:    {"ctrl+d", "PageDown", "cmd+down"},
		scroll.ActionScrollLeft:  {"h", "Left"},
		scroll.ActionScrollRight: {"l", "Right"},
	}

	keyMap := scroll.NewKeyMap(bindings)

	testCases := []struct {
		key     string
		wantOK  bool
		wantAct string
	}{
		{"k", true, scroll.ActionScrollUp},
		{"j", true, scroll.ActionScrollDown},
		{"Up", true, scroll.ActionScrollUp},
		{"Down", true, scroll.ActionScrollDown},
		{"G", true, scroll.ActionGoBottom},
		{"ctrl+u", true, scroll.ActionPageUp},
		{"ctrl+d", true, scroll.ActionPageDown},
		{"PageUp", true, scroll.ActionPageUp},
		{"PageDown", true, scroll.ActionPageDown},
		{"cmd+up", true, scroll.ActionPageUp},
		{"cmd+down", true, scroll.ActionPageDown},
		{"h", true, scroll.ActionScrollLeft},
		{"l", true, scroll.ActionScrollRight},
		{"x", false, ""},
		{"Ctrl+U", true, scroll.ActionPageUp},
		{"CTRL+U", true, scroll.ActionPageUp},
		{"CTRL+u", true, scroll.ActionPageUp},
	}

	for _, testCase := range testCases {
		t.Run(testCase.key, func(t *testing.T) {
			act, found := keyMap.Lookup(testCase.key)
			if found != testCase.wantOK {
				t.Errorf("Lookup(%q) found = %v, want %v", testCase.key, found, testCase.wantOK)
			}

			if found && act != testCase.wantAct {
				t.Errorf("Lookup(%q) action = %q, want %q", testCase.key, act, testCase.wantAct)
			}
		})
	}
}

func TestKeyMapSequences(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionGoTop: {"gg"},
	}

	keyMap := scroll.NewKeyMap(bindings)

	if !keyMap.IsSequenceStart("g") {
		t.Error("IsSequenceStart('g') = false, want true")
	}

	if keyMap.IsSequenceStart("x") {
		t.Error("IsSequenceStart('x') = true, want false")
	}

	act, found := keyMap.LookupSequence("gg")
	if !found {
		t.Error("LookupSequence('gg') found = false")
	}

	if act != scroll.ActionGoTop {
		t.Errorf("LookupSequence('gg') = %q, want %q", act, scroll.ActionGoTop)
	}

	if !keyMap.CanCompleteSequence("g", "g") {
		t.Error("CanCompleteSequence('g', 'g') = false, want true")
	}
}

func TestActionLookup(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionScrollUp:   {"k"},
		scroll.ActionScrollDown: {"j"},
	}

	keyMap := scroll.NewKeyMap(bindings)

	act, found := keyMap.Action(scroll.ActionScrollUp)
	if !found {
		t.Errorf("Action(%q) found = false", scroll.ActionScrollUp)
	}

	if act.Direction != 0 {
		t.Errorf("Action(%q) Direction = %d, want 0", scroll.ActionScrollUp, act.Direction)
	}
}

func TestActionNotFound(t *testing.T) {
	keyMap := scroll.NewKeyMap(map[string][]string{})

	_, found := keyMap.Action("nonexistent")
	if found {
		t.Error("Action('nonexistent') found = true, want false")
	}
}

func TestKeyMapMultipleKeysPerAction(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionScrollUp:    {"k", "Up", "ctrl+p"},
		scroll.ActionScrollDown:  {"j", "Down", "ctrl+n"},
		scroll.ActionScrollLeft:  {"h", "Left"},
		scroll.ActionScrollRight: {"l", "Right"},
	}

	keyMap := scroll.NewKeyMap(bindings)

	testCases := []struct {
		key     string
		wantAct string
	}{
		{"k", scroll.ActionScrollUp},
		{"Up", scroll.ActionScrollUp},
		{"ctrl+p", scroll.ActionScrollUp},
		{"j", scroll.ActionScrollDown},
		{"Down", scroll.ActionScrollDown},
		{"ctrl+n", scroll.ActionScrollDown},
	}

	for _, testCase := range testCases {
		t.Run(testCase.key, func(t *testing.T) {
			act, found := keyMap.Lookup(testCase.key)
			if !found {
				t.Errorf("Lookup(%q) not found", testCase.key)
			}

			if act != testCase.wantAct {
				t.Errorf("Lookup(%q) = %q, want %q", testCase.key, act, testCase.wantAct)
			}
		})
	}
}

func TestKeyMapMixedSequencesAndSingles(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionScrollUp:   {"k", "Up"},
		scroll.ActionScrollDown: {"j", "Down"},
		scroll.ActionGoTop:      {"gg"},
		scroll.ActionGoBottom:   {"G"},
	}

	keyMap := scroll.NewKeyMap(bindings)

	if !keyMap.IsSequenceStart("g") {
		t.Error("IsSequenceStart('g') = false, want true for 'gg' sequence")
	}

	act, found := keyMap.Lookup("k")
	if !found || act != scroll.ActionScrollUp {
		t.Errorf("Lookup('k') = %q, %v, want %q, true", act, found, scroll.ActionScrollUp)
	}

	act, found = keyMap.LookupSequence("gg")
	if !found || act != scroll.ActionGoTop {
		t.Errorf("LookupSequence('gg') = %q, %v, want %q, true", act, found, scroll.ActionGoTop)
	}
}

func TestKeyMapControlCharacterNormalization(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionPageUp:   {"ctrl+u", "\x15"},
		scroll.ActionPageDown: {"ctrl+d", "\x04"},
	}

	keyMap := scroll.NewKeyMap(bindings)

	act1, found1 := keyMap.Lookup("ctrl+u")
	if !found1 || act1 != scroll.ActionPageUp {
		t.Errorf("Lookup('ctrl+u') = %q, %v, want %q, true", act1, found1, scroll.ActionPageUp)
	}

	act2, found2 := keyMap.Lookup("\x15")
	if !found2 || act2 != scroll.ActionPageUp {
		t.Errorf("Lookup('\\x15') = %q, %v, want %q, true", act2, found2, scroll.ActionPageUp)
	}
}

func TestKeyMapModifierCaseInsensitivity(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionScrollUp: {"CTRL+U", "Alt+Z", "CMD+K"},
	}

	keyMap := scroll.NewKeyMap(bindings)

	testCases := []struct {
		key     string
		wantAct string
	}{
		{"CTRL+U", scroll.ActionScrollUp},
		{"ctrl+u", scroll.ActionScrollUp},
		{"Ctrl+U", scroll.ActionScrollUp},
		{"ALT+Z", scroll.ActionScrollUp},
		{"alt+z", scroll.ActionScrollUp},
		{"Alt+Z", scroll.ActionScrollUp},
		{"CMD+K", scroll.ActionScrollUp},
		{"cmd+k", scroll.ActionScrollUp},
		{"Cmd+K", scroll.ActionScrollUp},
	}

	for _, testCase := range testCases {
		t.Run(testCase.key, func(t *testing.T) {
			act, found := keyMap.Lookup(testCase.key)
			if !found {
				t.Errorf("Lookup(%q) not found", testCase.key)
			}

			if act != testCase.wantAct {
				t.Errorf("Lookup(%q) = %q, want %q", testCase.key, act, testCase.wantAct)
			}
		})
	}
}

func TestKeyMapInvalidKeysIgnored(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionScrollUp: {"k", ""},
	}

	keyMap := scroll.NewKeyMap(bindings)

	_, found := keyMap.Lookup("k")
	if !found {
		t.Error("Lookup('k') not found")
	}

	_, found = keyMap.Lookup("")
	if found {
		t.Error("Lookup('') found = true, want false for empty key")
	}
}

func TestKeyMapExports(t *testing.T) {
	bindings := map[string][]string{
		scroll.ActionScrollUp: {"k"},
	}

	keyMap := scroll.NewKeyMap(bindings)

	keyToAction := keyMap.KeyToAction()
	if keyToAction["k"] != scroll.ActionScrollUp {
		t.Error("KeyToAction() does not contain expected mapping")
	}

	sequences := keyMap.Sequences()
	if len(sequences) != 0 {
		t.Error("Sequences() should be empty for single-key bindings")
	}

	seqBindings := map[string][]string{
		scroll.ActionGoTop: {"gg"},
	}

	seqKeyMap := scroll.NewKeyMap(seqBindings)
	if len(seqKeyMap.KeyToAction()) != 0 {
		t.Error("KeyToAction() should be empty when all keys are sequences")
	}

	seqMap := seqKeyMap.Sequences()
	if seqMap["gg"] != scroll.ActionGoTop {
		t.Error("Sequences() does not contain expected sequence mapping")
	}
}
