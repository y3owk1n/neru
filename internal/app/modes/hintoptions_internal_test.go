package modes

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
)

// The option values these cases repeat.
const (
	stepScroll     = "scroll"
	dirReverse     = "reverse"
	strategyVision = "vision"
	strategyAXTree = "axtree"
)

// A refresh and a fresh activation disagree about what an absent option means,
// and getting that backwards is invisible until a user loses a flag they set.
// On a refresh an absent option means "keep what is there", because a refresh
// can be triggered by something that carries no options at all. On a fresh
// activation it means "back to the default", because the user just issued a
// command without that flag.

// populatedContext is a context that already carries every option, so a case can
// tell "kept" apart from "reset" for each of them.
func populatedContext() *hints.Context {
	action := "left_click"
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

func TestApplyHintOptionsRefreshKeepsUnsetOptions(t *testing.T) {
	ctx := populatedContext()

	applyHintOptions(ctx, ModeActivationOptions{}, true)

	if ctx.PendingAction() == nil || *ctx.PendingAction() != "left_click" {
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

func TestApplyHintOptionsRefreshWritesTheOptionsItWasGiven(t *testing.T) {
	ctx := populatedContext()

	action := "right_click"
	splitWord := false
	strategy := strategyAXTree

	applyHintOptions(ctx, ModeActivationOptions{
		Action:    &action,
		SplitWord: &splitWord,
		Strategy:  &strategy,
	}, true)

	if ctx.PendingAction() == nil || *ctx.PendingAction() != "right_click" {
		t.Errorf("PendingAction = %v, want the option that was given", ctx.PendingAction())
	}

	if ctx.SplitWord() {
		t.Error("SplitWord = true, want the false it was given rather than the old true")
	}

	if ctx.StrategyOverride() != strategyAXTree {
		t.Errorf("StrategyOverride = %q, want the option that was given", ctx.StrategyOverride())
	}

	// Everything else stays as it was.
	if ctx.LabelDirectionOverride() != dirReverse {
		t.Errorf("LabelDirectionOverride = %q, want it kept", ctx.LabelDirectionOverride())
	}
}

// TestApplyHintOptionsFreshResetsUnsetOptions is the other half of the rule: a
// command issued without a flag must not inherit that flag from the last time
// the mode ran.
func TestApplyHintOptionsFreshResetsUnsetOptions(t *testing.T) {
	ctx := populatedContext()
	ctx.SetRepeat(true)

	applyHintOptions(ctx, ModeActivationOptions{}, false)

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

func TestApplyHintOptionsFreshWritesTheOptionsItWasGiven(t *testing.T) {
	ctx := &hints.Context{}

	action := "double_click"
	search := true
	labelDirection := dirReverse

	applyHintOptions(ctx, ModeActivationOptions{
		Action:         &action,
		Search:         &search,
		LabelDirection: &labelDirection,
		FilterRoles:    []string{"AXLink"},
	}, false)

	if ctx.PendingAction() == nil || *ctx.PendingAction() != "double_click" {
		t.Errorf("PendingAction = %v, want the option that was given", ctx.PendingAction())
	}

	if !ctx.StartWithSearch() {
		t.Error("StartWithSearch = false, want the option that was given")
	}

	if ctx.LabelDirectionOverride() != dirReverse {
		t.Errorf(
			"LabelDirectionOverride = %q, want the option that was given",
			ctx.LabelDirectionOverride(),
		)
	}

	if len(ctx.FilterRoles()) != 1 || ctx.FilterRoles()[0] != "AXLink" {
		t.Errorf("FilterRoles = %v, want the option that was given", ctx.FilterRoles())
	}
}
