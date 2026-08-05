package modecmd_test

import (
	"reflect"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// A mode command makes the trip twice: what a user writes is parsed into an
// activation, and an activation is written back out as the arguments that
// travel to the daemon. The two halves have to agree, or a flag survives the
// first trip and is lost on the second — which is exactly how flags went
// missing before this module existed.
//
// These cases pin that agreement flag by flag, so a new flag whose renderer
// disagrees with its parser fails here rather than in a user's binding.
func TestRoundTrip_ParsingARenderingReturnsTheSameActivation(t *testing.T) {
	t.Parallel()

	for name, testCase := range flagCases() {
		t.Run(string(name), func(t *testing.T) {
			t.Parallel()

			assertRoundTrip(t, testCase.mode, testCase.args)
		})
	}
}

// TestRoundTrip_CarriesAnActivationBuiltByHand pins the same agreement from the
// other side: an activation assembled field by field, as a caller holding typed
// flags assembles one, renders to a command that parses back into it.
//
// This is the half a parsed activation cannot prove. Parsing only ever produces
// what parsing can produce, so a field the renderer drops would go unnoticed
// until the CLI set it directly.
func TestRoundTrip_CarriesAnActivationBuiltByHand(t *testing.T) {
	t.Parallel()

	for name, testCase := range flagCases() {
		t.Run(string(name), func(t *testing.T) {
			t.Parallel()

			built := modecmd.Activation{Mode: testCase.mode}
			testCase.build(&built)

			parsed, err := modecmd.Parse(testCase.mode, modecmd.Render(built))
			if err != nil {
				t.Fatalf("Parse(Render(%+v)) error = %v", built, err)
			}

			if !reflect.DeepEqual(built, parsed) {
				t.Errorf("round trip changed the activation:\n want %+v\n  got %+v", built, parsed)
			}
		})
	}
}

// TestRoundTrip_CarriesAWholeCommand pins that the flags survive together, not
// only one at a time: a renderer that overwrites another flag's argument would
// pass every case above.
func TestRoundTrip_CarriesAWholeCommand(t *testing.T) {
	t.Parallel()

	assertRoundTrip(t, domain.ModeHints, []string{
		"--action=left_click,left_click",
		"--modifier=cmd,shift",
		"--on-exit=action left_click",
		"--on-exit=idle",
		flagRepeat,
		argToggle,
		argSearch,
		flagHideOnEmpty,
		"--role=AXButton,AXLink",
		"--text=OK,Cancel",
		"--strategy=vision",
		"--label-direction=reverse",
		"--split-word",
		"--cursor-selection-mode=hold",
	})
}

// TestRoundTrip_KeepsOnExitAbsentApartFromEmpty pins the sharpest edge in the
// whole grammar.
//
// An absent --on-exit leaves the steps a previous activation stored alone,
// because a repeat re-activation goes round again without repeating them. A
// given-but-empty --on-exit clears them. Rendering the two the same way would
// resurrect stale steps on the next activation.
func TestRoundTrip_KeepsOnExitAbsentApartFromEmpty(t *testing.T) {
	t.Parallel()

	absent := assertRoundTrip(t, domain.ModeHints, []string{"--action=left_click"})
	if absent.OnExit != nil {
		t.Errorf("OnExit = %#v, want nil so the stored steps are preserved", absent.OnExit)
	}

	empty := assertRoundTrip(t, domain.ModeHints, []string{"--action=left_click", "--on-exit="})
	if empty.OnExit == nil {
		t.Fatal("OnExit = nil, want a non-nil empty slice so the stored steps are cleared")
	}

	if len(empty.OnExit) != 0 {
		t.Errorf("OnExit = %#v, want no steps", empty.OnExit)
	}
}

// TestRender_OmitsTheModeName pins that a rendering carries flags only. The
// name is the request's own action, and repeating it inside the arguments is
// the redundancy the wire is being freed of.
func TestRender_OmitsTheModeName(t *testing.T) {
	t.Parallel()

	activation, err := modecmd.Parse(domain.ModeHints, []string{modeNameHints, argSearch})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	rendered := modecmd.Render(activation)
	for _, arg := range rendered {
		if arg == modeNameHints {
			t.Errorf("Render() = %v, want the mode name left out", rendered)
		}
	}
}

// assertRoundTrip parses args, renders the result, parses that, and reports the
// activation once both parses agree.
func assertRoundTrip(t *testing.T, mode domain.Mode, args []string) modecmd.Activation {
	t.Helper()

	first, err := modecmd.Parse(mode, args)
	if err != nil {
		t.Fatalf("Parse(%v) error = %v", args, err)
	}

	rendered := modecmd.Render(first)

	second, err := modecmd.Parse(mode, rendered)
	if err != nil {
		t.Fatalf("Parse(Render(%v)) = %v, error = %v", args, rendered, err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("round trip changed the activation:\n from %v\n via  %v\n  got %+v\n want %+v",
			args, rendered, second, first)
	}

	return second
}
