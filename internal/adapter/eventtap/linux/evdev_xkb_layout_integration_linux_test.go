//go:build integration && linux && cgo

package linux

import "testing"

// Scan codes the tests press that the proxy has no constant for, from
// linux/input-event-codes.h.
const (
	testEvdevKeyCapsLock = 58
	testEvdevKeyNumLock  = 69
)

// Every case runs with three XKB options. ctrl:swapcaps makes the left Ctrl key
// Caps Lock and the Caps Lock key Control. grp:rctrl_toggle makes a tap of the
// right Ctrl key switch to the next layout, as a user switching language
// would. lv3:ralt_switch makes the right Alt key AltGr.
const testXkbOptions = "ctrl:swapcaps,grp:rctrl_toggle,lv3:ralt_switch"

// Most cases use a keymap with a Latin layout first and a Cyrillic one second,
// both at their default variant.
const (
	testXkbLatinThenCyrillic = "us,ru"
	testXkbDefaultVariants   = ","

	// testXkbNoLatinLayout has no ASCII-capable layout at all.
	testXkbNoLatinLayout = "ru,gr"

	// A keymap of two Latin layouts, us and us(dvorak), and the XKB name of
	// the second, as general.kb_layout_to_use writes it.
	testXkbTwoLatinLayouts  = "us,us"
	testXkbDvorakSecond     = ",dvorak"
	testXkbDvorakLayoutName = "English (Dvorak)"
)

// TestWaylandEvdevCapture_KeyName_ResolvesInTheReferenceLayout pins which
// physical key a binding matches while another language is active. It is the
// key that types the binding in the first ASCII-capable layout of the keymap,
// with the modifiers the user holds, whatever layout the keyboard is in.
func TestWaylandEvdevCapture_KeyName_ResolvesInTheReferenceLayout(t *testing.T) {
	t.Parallel()

	probe, err := newTestXkbCapture("us", "", "")
	if err != nil {
		t.Skipf("no XKB data to compile a us layout from: %v", err)
	}

	probe.destroyTestXkb()

	for _, test := range []struct {
		name    string
		layout  string
		variant string
		tap     []uint16 // pressed and released, in order
		hold    []uint16 // pressed and left down, after the taps
		code    uint16
		command string
		live    string
	}{
		{
			name:   "a Latin first layout names keys while it is active",
			layout: testXkbLatinThenCyrillic, variant: testXkbDefaultVariants,
			code: evdevKeyQ, command: "q", live: "q",
		},
		{
			name:   "switching to a non-Latin layout leaves the binding on its key",
			layout: testXkbLatinThenCyrillic, variant: testXkbDefaultVariants,
			tap:  []uint16{evdevKeyRightCtrl},
			code: evdevKeyQ, command: "q", live: "й",
		},
		{
			name:   "Shift chooses the level inside the reference layout",
			layout: testXkbLatinThenCyrillic, variant: testXkbDefaultVariants,
			tap: []uint16{evdevKeyRightCtrl}, hold: []uint16{evdevKeyLeftShift},
			code: evdevKeyQ, command: "Q", live: "Й",
		},
		{
			name:   "Caps Lock chooses the level inside the reference layout",
			layout: testXkbLatinThenCyrillic, variant: testXkbDefaultVariants,
			tap:  []uint16{evdevKeyRightCtrl, evdevKeyLeftCtrl},
			code: evdevKeyQ, command: "Q", live: "Й",
		},
		{
			name:   "a non-Latin first layout yields to the first Latin one",
			layout: "ru,us", variant: testXkbDefaultVariants,
			code: evdevKeyQ, command: "q", live: "й",
		},
		{
			name:   "a keymap with no Latin layout names keys in the active one",
			layout: testXkbNoLatinLayout, variant: testXkbDefaultVariants,
			code: evdevKeyQ, command: "й", live: "й",
		},
		{
			name:   "a keymap with no Latin layout follows a switch to its second",
			layout: testXkbNoLatinLayout, variant: testXkbDefaultVariants,
			tap:  []uint16{evdevKeyRightCtrl},
			code: evdevKeyQ, command: ";", live: ";",
		},
		{
			name:   "Dvorak is a Latin layout although its letter row starts with punctuation",
			layout: testXkbLatinThenCyrillic, variant: "dvorak,",
			tap:  []uint16{evdevKeyRightCtrl},
			code: evdevKeyQ, command: "'", live: "й",
		},
		{
			name:   "AltGr reaches the third level of the reference layout",
			layout: "de,ru", variant: testXkbDefaultVariants,
			tap: []uint16{evdevKeyRightCtrl}, hold: []uint16{evdevKeyRightAlt},
			code: evdevKeyQ, command: "@",
		},
		{
			name:   "the keypad follows NumLock",
			layout: testXkbLatinThenCyrillic, variant: testXkbDefaultVariants,
			tap:  []uint16{evdevKeyRightCtrl, testEvdevKeyNumLock},
			code: evdevKeyKP7, command: "7",
		},
		{
			name:   "the keypad without NumLock is the navigation key",
			layout: testXkbLatinThenCyrillic, variant: testXkbDefaultVariants,
			tap:  []uint16{evdevKeyRightCtrl},
			code: evdevKeyKP7, command: evdevKeyNameHome,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			capture, err := newTestXkbCapture(test.layout, test.variant, testXkbOptions)
			if err != nil {
				t.Fatalf("layouts %q variants %q: %v", test.layout, test.variant, err)
			}
			defer capture.destroyTestXkb()

			for _, code := range test.tap {
				capture.feedKey(code, true)
				capture.feedKey(code, false)
			}

			for _, code := range test.hold {
				capture.feedKey(code, true)
			}

			if got := capture.keyName(test.code); got != test.command {
				t.Errorf("keyName(%d) = %q, want %q", test.code, got, test.command)
			}

			if test.live == "" {
				return
			}

			if got := capture.liveKeyName(test.code); got != test.live {
				t.Errorf("liveKeyName(%d) = %q, want %q", test.code, got, test.live)
			}
		})
	}
}

// TestWaylandEvdevCapture_ModifierName_FollowsTheLiveLayout pins that Neru names
// a physical modifier by the remap its layout carries, before and after a
// language switch. Under ctrl:swapcaps the Caps Lock key is Control.
func TestWaylandEvdevCapture_ModifierName_FollowsTheLiveLayout(t *testing.T) {
	t.Parallel()

	capture, err := newTestXkbCapture(
		testXkbLatinThenCyrillic,
		testXkbDefaultVariants,
		testXkbOptions,
	)
	if err != nil {
		t.Skipf("no XKB data to compile us,ru from: %v", err)
	}
	defer capture.destroyTestXkb()

	for _, switched := range []bool{false, true} {
		if switched {
			capture.feedKey(evdevKeyRightCtrl, true)
			capture.feedKey(evdevKeyRightCtrl, false)
		}

		if got := capture.modifierName(testEvdevKeyCapsLock); got != evdevModifierCtrl {
			t.Errorf(
				"switched=%v: modifierName(CapsLock) = %q, want %q",
				switched,
				got,
				evdevModifierCtrl,
			)
		}

		if got := capture.modifierName(evdevKeyQ); got != "" {
			t.Errorf("switched=%v: modifierName(Q) = %q, want no modifier", switched, got)
		}
	}
}

// TestEventTap_SetKeyboardLayout_ForcesTheWaylandReferenceLayout pins that
// general.kb_layout_to_use names the layout keys resolve in by its XKB name,
// ignoring case, and that a name the keymap lacks leaves the automatic choice.
// It sets the process-wide request, so it does not run in parallel.
func TestEventTap_SetKeyboardLayout_ForcesTheWaylandReferenceLayout(t *testing.T) {
	var eventTap EventTap

	t.Cleanup(func() { eventTap.SetKeyboardLayout("") })

	for _, test := range []struct {
		name      string
		requested string
		want      string
		reference string
		unmatched bool
	}{
		{
			name:      "the automatic choice takes the first Latin layout",
			requested: "",
			want:      "q",
			reference: "English (US)",
		},
		{
			name:      "a forced layout names keys whichever layout is active",
			requested: testXkbDvorakLayoutName,
			want:      "'",
			reference: testXkbDvorakLayoutName,
		},
		{
			name:      "the forced name ignores case",
			requested: "english (dvorak)",
			want:      "'",
			reference: testXkbDvorakLayoutName,
		},
		{
			name:      "a name the keymap lacks leaves the automatic choice",
			requested: "Klingon",
			want:      "q",
			reference: "English (US)",
			unmatched: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			capture, err := newTestXkbCapture(
				testXkbTwoLatinLayouts,
				testXkbDvorakSecond,
				testXkbOptions,
			)
			if err != nil {
				t.Skipf("no XKB data to compile us and Dvorak from: %v", err)
			}
			defer capture.destroyTestXkb()

			eventTap.SetKeyboardLayout(test.requested)

			if got := capture.keyName(evdevKeyQ); got != test.want {
				t.Errorf("forcing %q: keyName(Q) = %q, want %q", test.requested, got, test.want)
			}

			layouts := capture.keyboardLayouts()
			if layouts.Unmatched != test.unmatched {
				t.Errorf(
					"forcing %q: unmatched = %v, want %v",
					test.requested,
					layouts.Unmatched,
					test.unmatched,
				)
			}

			if layouts.Reference != test.reference {
				t.Errorf("forcing %q: reference = %q, want %q (layouts %q)",
					test.requested, layouts.Reference, test.reference, layouts.Names)
			}
		})
	}
}
