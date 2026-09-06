package modes

import (
	"context"
	"fmt"
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// extensionAxis is one optional extension declared in extensions.go, paired
// with the type assertion that answers whether a mode carries it.
type extensionAxis struct {
	name    extensionName
	carried func(Mode) bool
}

// allExtensions is every optional extension there is. A new one is added here
// once; the matrix then asserts it for every mode.
var allExtensions = []extensionAxis{
	{
		name: extensionSelectionTracking,
		carried: func(mode Mode) bool {
			_, ok := mode.(selectionTracker)

			return ok
		},
	},
	{
		name: extensionCellNavigation,
		carried: func(mode Mode) bool {
			_, ok := mode.(cellNavigator)

			return ok
		},
	},
	{
		name: extensionCursorFollow,
		carried: func(mode Mode) bool {
			_, ok := mode.(cursorFollowSelector)

			return ok
		},
	},
	{
		name: extensionExitSteps,
		carried: func(mode Mode) bool {
			_, ok := mode.(exitStepReporter)

			return ok
		},
	},
	{
		name: extensionInputEditing,
		carried: func(mode Mode) bool {
			_, ok := mode.(inputEditor)

			return ok
		},
	},
	{
		name: extensionHotkeyOverrides,
		carried: func(mode Mode) bool {
			_, ok := mode.(hotkeyOverrideReporter)

			return ok
		},
	},
	{
		name: extensionThemeRefresh,
		carried: func(mode Mode) bool {
			_, ok := mode.(themeRefresher)

			return ok
		},
	},
	{
		name: extensionScreenRefresh,
		carried: func(mode Mode) bool {
			_, ok := mode.(screenRefresher)

			return ok
		},
	},
}

// modeExtensionMatrix states, per mode, every optional extension that mode
// carries. Omission is a statement too: a mode listed with no extensions has
// been considered on every axis and carries none.
//
// This table is what makes optional extensions safe; the modes area guide says
// why the guarantee lives here rather than in the linter.
var modeExtensionMatrix = map[domain.Mode][]extensionName{
	domain.ModeHints: {
		extensionCursorFollow,
		extensionExitSteps,
		extensionInputEditing,
		extensionHotkeyOverrides,
		extensionThemeRefresh,
		extensionScreenRefresh,
	},
	domain.ModeGrid: {
		extensionSelectionTracking,
		extensionCellNavigation,
		extensionCursorFollow,
		extensionExitSteps,
		extensionInputEditing,
		extensionHotkeyOverrides,
		extensionThemeRefresh,
		extensionScreenRefresh,
	},
	domain.ModeRecursiveGrid: {
		extensionSelectionTracking,
		extensionCellNavigation,
		extensionCursorFollow,
		extensionExitSteps,
		extensionInputEditing,
		extensionHotkeyOverrides,
		extensionThemeRefresh,
		extensionScreenRefresh,
	},
	domain.ModeScroll: {
		extensionHotkeyOverrides,
	},
	domain.ModeMonitorSelect: {
		extensionInputEditing,
		extensionThemeRefresh,
	},
	domain.ModeCustom: {
		extensionHotkeyOverrides,
	},
}

// TestModeExtensionMatrix pins what every registered mode does on every
// optional-extension axis.
//
// It builds a real handler so it reads the same mode map the daemon dispatches
// through: a mode added to newModes appears here without anyone remembering to
// list it, and fails until its row above is stated.
func TestModeExtensionMatrix(t *testing.T) {
	handler := NewHandler(HandlerDeps{})

	for _, problem := range checkModeExtensions(handler.modes, modeExtensionMatrix) {
		t.Error(problem)
	}
}

// TestCheckModeExtensions_RejectsAModeThatStatesNothing is the matrix test's
// own teeth. Registering a mode is the moment the matrix has to speak up, and
// the live map only ever holds modes whose rows are already stated, so the
// ways it must complain are driven here instead.
func TestCheckModeExtensions_RejectsAModeThatStatesNothing(t *testing.T) {
	// A mode value no constant names yet, standing in for the next mode
	// somebody adds.
	newMode := domain.Mode(90)

	tests := []struct {
		name       string
		registered map[domain.Mode]Mode
		matrix     map[domain.Mode][]extensionName
	}{
		{
			name:       "a registered mode with no row at all",
			registered: map[domain.Mode]Mode{newMode: &stubMode{modeType: newMode}},
			matrix:     map[domain.Mode][]extensionName{},
		},
		{
			name: "a mode carrying an extension its row does not state",
			registered: map[domain.Mode]Mode{
				newMode: &stubSelectionTrackingMode{stubMode{modeType: newMode}},
			},
			matrix: map[domain.Mode][]extensionName{newMode: {}},
		},
		{
			name:       "a mode whose row claims an extension it does not carry",
			registered: map[domain.Mode]Mode{newMode: &stubMode{modeType: newMode}},
			matrix:     map[domain.Mode][]extensionName{newMode: {extensionCellNavigation}},
		},
		{
			name:       "a row for a mode nothing registered",
			registered: map[domain.Mode]Mode{},
			matrix:     map[domain.Mode][]extensionName{newMode: {}},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			problems := checkModeExtensions(testCase.registered, testCase.matrix)
			if len(problems) == 0 {
				t.Fatal("expected the matrix to reject this, got no problems reported")
			}
		})
	}
}

// TestCheckModeExtensions_AcceptsAFullyStatedMode pins the other direction, so
// the cases above fail for the reason they name rather than because the check
// rejects everything.
func TestCheckModeExtensions_AcceptsAFullyStatedMode(t *testing.T) {
	newMode := domain.Mode(90)

	problems := checkModeExtensions(
		map[domain.Mode]Mode{
			newMode: &stubSelectionTrackingMode{stubMode{modeType: newMode}},
		},
		map[domain.Mode][]extensionName{newMode: {extensionSelectionTracking}},
	)
	if len(problems) != 0 {
		t.Fatalf("expected a fully stated mode to pass, got %v", problems)
	}
}

// TestActiveModeEffect_DeclinedEffectIsDiagnosable pins the half of the absence
// semantics a user can feel: pressing backspace in a mode that does not edit
// input does nothing, and "nothing happened" has to be answerable from a debug
// log rather than by reading the dispatch.
//
// It drives the handler's own entry points and reads the log the way a user
// running with --log-level debug would, so it names the mode and the axis as
// text rather than reaching for the extension it is about.
func TestActiveModeEffect_DeclinedEffectIsDiagnosable(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	appState := state.NewAppState()
	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.New(core),
	})

	appState.SetMode(domain.ModeScroll)

	handler.BackspaceCurrentMode()

	declined := logs.FilterLevelExact(zap.DebugLevel).All()
	if len(declined) != 1 {
		t.Fatalf("backspace in scroll mode logged %d debug entries, want 1", len(declined))
	}

	wantMode := domain.ModeString(domain.ModeScroll)

	fields := declined[0].ContextMap()
	if got := fields["mode"]; got != wantMode {
		t.Errorf("declined effect named mode %v, want %q", got, wantMode)
	}

	if got := fields["extension"]; got != string(extensionInputEditing) {
		t.Errorf("declined effect named extension %v, want %q", got, extensionInputEditing)
	}
}

// TestActiveModeExtension_AbsentGetterStaysSilent pins the other half: a getter
// the active mode does not carry answers with its zero value and says nothing,
// because the caller asked a question rather than for an effect.
func TestActiveModeExtension_AbsentGetterStaysSilent(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	appState := state.NewAppState()
	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.New(core),
	})

	appState.SetMode(domain.ModeScroll)

	if _, ok := handler.CurrentSelectionPoint(); ok {
		t.Fatal("scroll mode reported a selection point")
	}

	if _, ok := handler.CursorFollowSelection(); ok {
		t.Fatal("scroll mode reported a cursor-follow preference")
	}

	if logs.Len() != 0 {
		t.Fatalf("absent getters logged %v, want silence", logs.All())
	}
}

// checkModeExtensions reports every disagreement between the modes registered
// with the handler and the matrix: a mode with no row, a row naming an
// extension that does not exist, a row for a mode nothing registered, and
// either direction of a row disagreeing with what the mode's type carries.
func checkModeExtensions(
	registered map[domain.Mode]Mode,
	matrix map[domain.Mode][]extensionName,
) []string {
	var problems []string

	for mode, impl := range registered {
		modeName := domain.ModeString(mode)

		stated, hasRow := matrix[mode]
		if !hasRow {
			problems = append(problems, fmt.Sprintf(
				"mode %q (domain.Mode(%d)) is registered but has no row in "+
					"modeExtensionMatrix: state which optional extensions it carries",
				modeName, int(mode),
			))

			continue
		}

		for _, axis := range allExtensions {
			want := slices.Contains(stated, axis.name)

			if got := axis.carried(impl); got != want {
				problems = append(problems, fmt.Sprintf(
					"mode %q carries %q = %t, matrix says %t",
					modeName, axis.name, got, want,
				))
			}
		}

		for _, name := range stated {
			known := slices.ContainsFunc(allExtensions, func(axis extensionAxis) bool {
				return axis.name == name
			})
			if !known {
				problems = append(problems, fmt.Sprintf(
					"mode %q names unknown extension %q", modeName, name,
				))
			}
		}
	}

	for mode := range matrix {
		if _, isRegistered := registered[mode]; !isRegistered {
			problems = append(problems, fmt.Sprintf(
				"modeExtensionMatrix has a row for %q, which is not a registered mode",
				domain.ModeString(mode),
			))
		}
	}

	return problems
}

// stubMode is a mode carrying no optional extension at all.
type stubMode struct {
	modeType domain.Mode
}

func (m *stubMode) Activate(modecmd.Activation)                            {}
func (m *stubMode) HandleKey(string)                                       {}
func (m *stubMode) Exit()                                                  {}
func (m *stubMode) ModeType() domain.Mode                                  { return m.modeType }
func (m *stubMode) RefreshForMonitorMove(context.Context, image.Rectangle) {}

// stubSelectionTrackingMode is a mode carrying exactly one of them.
type stubSelectionTrackingMode struct {
	stubMode
}

func (m *stubSelectionTrackingMode) SelectionPoint() (image.Point, bool) {
	return image.Point{}, false
}

func (m *stubSelectionTrackingMode) ClearSelectionPoint() bool { return false }

func (m *stubSelectionTrackingMode) SelectionAnchor() (image.Point, bool) {
	return image.Point{}, false
}
