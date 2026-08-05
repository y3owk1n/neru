package modes

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// The flag values these cases repeat.
const (
	stepScroll      = "scroll"
	dirReverse      = "reverse"
	strategyVision  = "vision"
	strategyAXTree  = "axtree"
	actionLeftClick = "left_click"
)

// populatedContext is a context that already carries every flag, so a case can
// tell "kept" apart from "reset" for each of them.
func populatedContext() *hints.Context {
	action := actionLeftClick
	modifier := keyPartCmd

	ctx := &hints.Context{}
	ctx.SetPendingAction(&action)
	ctx.SetPendingModifier(&modifier)
	ctx.SetOnExit([]string{stepScroll})
	ctx.SetCursorFollowSelection(true)
	ctx.SetFilterRoles([]string{"AXButton"})
	ctx.SetFilterTextContains([]string{"OK"})
	ctx.SetStartWithSearch(true)
	ctx.SetHideOnEmptySearch(true)
	ctx.SetStrategyOverride(strategyVision)
	ctx.SetLabelDirectionOverride(dirReverse)
	ctx.SetSplitWord(true)

	return ctx
}

// A refresh and a fresh activation disagree about what an absent flag means,
// and getting that backwards is invisible until a user loses a flag they set.
// On a refresh an absent flag means "keep what is there", because a refresh
// can be triggered by something that carries no flags at all. On a fresh
// activation it means "back to the default", because the user just issued a
// command without that flag.
func TestApplyHintFlags_RefreshKeepsUnsetFlags(t *testing.T) {
	ctx := populatedContext()

	applyHintFlags(ctx, modecmd.Activation{}, true)

	if ctx.PendingAction() == nil || *ctx.PendingAction() != actionLeftClick {
		t.Errorf(
			"PendingAction = %v, want the action the mode was activated with",
			ctx.PendingAction(),
		)
	}

	if ctx.PendingModifier() == nil || *ctx.PendingModifier() != keyPartCmd {
		t.Errorf("PendingModifier = %v, want it kept", ctx.PendingModifier())
	}

	if !ctx.CursorFollowSelection() {
		t.Error("CursorFollowSelection was reset; want it kept across a refresh")
	}

	if ctx.StrategyOverride() != strategyVision {
		t.Errorf("StrategyOverride = %q, want it kept", ctx.StrategyOverride())
	}

	if ctx.LabelDirectionOverride() != dirReverse {
		t.Errorf("LabelDirectionOverride = %q, want it kept", ctx.LabelDirectionOverride())
	}

	if !ctx.SplitWord() {
		t.Error("SplitWord was reset; want it kept across a refresh")
	}

	if !ctx.StartWithSearch() {
		t.Error("StartWithSearch was reset; want it kept across a refresh")
	}

	if !ctx.HideOnEmptySearch() {
		t.Error("HideOnEmptySearch was reset; want it kept across a refresh")
	}

	if len(ctx.FilterRoles()) != 1 {
		t.Errorf("FilterRoles = %v, want them kept", ctx.FilterRoles())
	}

	if len(ctx.FilterTextContains()) != 1 {
		t.Errorf("FilterTextContains = %v, want them kept", ctx.FilterTextContains())
	}

	if len(ctx.OnExit()) != 1 {
		t.Errorf("OnExit = %v, want it kept", ctx.OnExit())
	}
}

func TestApplyHintFlags_RefreshWritesTheFlagsItWasGiven(t *testing.T) {
	ctx := populatedContext()

	action := "right_click"
	splitWord := false
	strategy := strategyAXTree

	applyHintFlags(ctx, modecmd.Activation{
		Action:    &action,
		SplitWord: &splitWord,
		Strategy:  &strategy,
	}, true)

	if ctx.PendingAction() == nil || *ctx.PendingAction() != "right_click" {
		t.Errorf("PendingAction = %v, want the flag that was given", ctx.PendingAction())
	}

	if ctx.SplitWord() {
		t.Error("SplitWord = true, want the false it was given rather than the old true")
	}

	if ctx.StrategyOverride() != strategyAXTree {
		t.Errorf("StrategyOverride = %q, want the flag that was given", ctx.StrategyOverride())
	}

	// Everything else stays as it was.
	if ctx.LabelDirectionOverride() != dirReverse {
		t.Errorf("LabelDirectionOverride = %q, want it kept", ctx.LabelDirectionOverride())
	}
}

// TestApplyHintFlags_RefreshTellsAbsentOnExitFromEmptyOne pins the one place
// where nil and empty are different values rather than two spellings of
// nothing. A repeat re-activation carries no --on-exit and must keep the steps
// the user activated the mode with; a command that gave --on-exit no steps is
// asking for none to run.
func TestApplyHintFlags_RefreshTellsAbsentOnExitFromEmptyOne(t *testing.T) {
	absent := populatedContext()

	applyHintFlags(absent, modecmd.Activation{}, true)

	if len(absent.OnExit()) != 1 {
		t.Errorf("OnExit = %v, want an absent --on-exit to keep the stored steps", absent.OnExit())
	}

	given := populatedContext()

	applyHintFlags(given, modecmd.Activation{OnExit: []string{}}, true)

	if len(given.OnExit()) != 0 {
		t.Errorf(
			"OnExit = %v, want a given-but-empty --on-exit to clear the stored steps",
			given.OnExit(),
		)
	}
}

// TestApplyHintFlags_FreshResetsUnsetFlags is the other half of the rule: a
// command issued without a flag must not inherit that flag from the last time
// the mode ran.
func TestApplyHintFlags_FreshResetsUnsetFlags(t *testing.T) {
	ctx := populatedContext()
	ctx.SetRepeat(true)

	applyHintFlags(ctx, modecmd.Activation{}, false)

	if ctx.PendingAction() != nil {
		t.Errorf("PendingAction = %v, want it cleared", ctx.PendingAction())
	}

	if ctx.PendingModifier() != nil {
		t.Errorf("PendingModifier = %v, want it cleared", ctx.PendingModifier())
	}

	if ctx.StrategyOverride() != "" {
		t.Errorf("StrategyOverride = %q, want it cleared", ctx.StrategyOverride())
	}

	if ctx.LabelDirectionOverride() != "" {
		t.Errorf("LabelDirectionOverride = %q, want it cleared", ctx.LabelDirectionOverride())
	}

	if ctx.SplitWord() {
		t.Error("SplitWord = true, want it cleared")
	}

	if ctx.StartWithSearch() {
		t.Error("StartWithSearch = true, want it cleared")
	}

	if ctx.HideOnEmptySearch() {
		t.Error("HideOnEmptySearch = true, want it cleared")
	}

	if ctx.Repeat() {
		t.Error("Repeat = true, want a fresh activation to start without it")
	}

	if ctx.FilterRoles() != nil {
		t.Errorf("FilterRoles = %v, want them cleared", ctx.FilterRoles())
	}

	if ctx.FilterTextContains() != nil {
		t.Errorf("FilterTextContains = %v, want them cleared", ctx.FilterTextContains())
	}

	if ctx.OnExit() != nil {
		t.Errorf("OnExit = %v, want it cleared", ctx.OnExit())
	}
}

func TestApplyHintFlags_FreshWritesTheFlagsItWasGiven(t *testing.T) {
	ctx := &hints.Context{}

	action := "double_click"
	search := true
	labelDirection := dirReverse

	applyHintFlags(ctx, modecmd.Activation{
		Action:         &action,
		Search:         &search,
		LabelDirection: &labelDirection,
		FilterRoles:    []string{"AXLink"},
	}, false)

	if ctx.PendingAction() == nil || *ctx.PendingAction() != "double_click" {
		t.Errorf("PendingAction = %v, want the flag that was given", ctx.PendingAction())
	}

	if !ctx.StartWithSearch() {
		t.Error("StartWithSearch = false, want the flag that was given")
	}

	if ctx.LabelDirectionOverride() != dirReverse {
		t.Errorf(
			"LabelDirectionOverride = %q, want the flag that was given",
			ctx.LabelDirectionOverride(),
		)
	}

	if len(ctx.FilterRoles()) != 1 || ctx.FilterRoles()[0] != "AXLink" {
		t.Errorf("FilterRoles = %v, want the flag that was given", ctx.FilterRoles())
	}
}
