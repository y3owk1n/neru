package modes

import (
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// applyHintFlags writes the flags an activation carries into the hint context. A
// fresh activation and a refresh read an absent flag differently, which is
// why this is two paths.
func applyHintFlags(ctx *hints.Context, activation modecmd.Activation, isRefresh bool) {
	if isRefresh {
		applyHintFlagOverrides(ctx, activation)

		return
	}

	applyHintFlagsFresh(ctx, activation)
}

// applyHintFlagOverrides writes only what a refresh was given. A refresh can
// arrive with no flags at all (a space change re-entering hints through a
// configured "hints" action), so anything unset keeps the value the user
// activated the mode with.
func applyHintFlagOverrides(ctx *hints.Context, activation modecmd.Activation) {
	if activation.Action != nil {
		ctx.SetPendingAction(activation.Action)
	}

	if activation.OnExit != nil {
		ctx.SetOnExit(activation.OnExit)
	}

	if activation.Modifier != nil {
		ctx.SetPendingModifier(activation.Modifier)
	}

	if activation.CursorFollowSelection != nil {
		ctx.SetCursorFollowSelection(*activation.CursorFollowSelection)
	}

	if activation.FilterRoles != nil {
		ctx.SetFilterRoles(activation.FilterRoles)
	}

	if activation.FilterTextContains != nil {
		ctx.SetFilterTextContains(activation.FilterTextContains)
	}

	if activation.Search != nil {
		ctx.SetStartWithSearch(*activation.Search)
	}

	if activation.HideOnEmptySearch != nil {
		ctx.SetHideOnEmptySearch(*activation.HideOnEmptySearch)
	}

	if activation.Strategy != nil {
		ctx.SetStrategyOverride(*activation.Strategy)
	}

	if activation.CaptureScope != nil {
		ctx.SetCaptureScopeOverride(*activation.CaptureScope)
	}

	if activation.LabelDirection != nil {
		ctx.SetLabelDirectionOverride(*activation.LabelDirection)
	}

	if activation.SplitWord != nil {
		ctx.SetSplitWord(*activation.SplitWord)
	}
}

// applyHintFlagsFresh writes every field, so a flag left off the command
// line falls back to its default rather than inheriting the last activation's.
func applyHintFlagsFresh(ctx *hints.Context, activation modecmd.Activation) {
	ctx.SetPendingAction(activation.Action)
	ctx.SetOnExit(activation.OnExit)
	ctx.SetPendingModifier(activation.Modifier)
	ctx.SetRepeat(false)
	ctx.SetCursorFollowSelection(
		resolveCursorFollowSelection(activation.CursorFollowSelection),
	)
	ctx.SetFilterRoles(activation.FilterRoles)
	ctx.SetFilterTextContains(activation.FilterTextContains)
	ctx.SetStartWithSearch(activation.Search != nil && *activation.Search)
	ctx.SetHideOnEmptySearch(activation.HideOnEmptySearch != nil && *activation.HideOnEmptySearch)
	ctx.SetStrategyOverride(derefOr(activation.Strategy, ""))
	ctx.SetCaptureScopeOverride(derefOr(activation.CaptureScope, ""))
	ctx.SetLabelDirectionOverride(derefOr(activation.LabelDirection, ""))
	ctx.SetSplitWord(derefOr(activation.SplitWord, false))
}

// derefOr reads a flag that may not have been given, falling back to what
// leaving it out means.
func derefOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}

	return *value
}

// hintOverrides are the settings that decide how a hint scan runs.
type hintOverrides struct {
	strategy       string
	captureScope   string
	labelDirection string
	splitWord      bool
}

// resolveHintOverrides reads the overrides in force. They come from the
// context, which applyHintFlags has just written, so a refresh sees what the
// mode was activated with rather than the empty activation it carries. The
// activation's own flags are the fallback when there is no context.
func (h *handlerState) resolveHintOverrides(activation modecmd.Activation) hintOverrides {
	if h.hints != nil && h.hints.Context != nil {
		return hintOverrides{
			strategy:       h.hints.Context.StrategyOverride(),
			captureScope:   h.hints.Context.CaptureScopeOverride(),
			labelDirection: h.hints.Context.LabelDirectionOverride(),
			splitWord:      h.hints.Context.SplitWord(),
		}
	}

	return hintOverrides{
		strategy:       derefOr(activation.Strategy, ""),
		captureScope:   derefOr(activation.CaptureScope, ""),
		labelDirection: derefOr(activation.LabelDirection, ""),
		splitWord:      derefOr(activation.SplitWord, false),
	}
}

// abandonHintActivation gives up on an activation. A refresh must also exit the
// mode, or its overlay keeps showing labels that no longer point at anything.
func (h *handlerState) abandonHintActivation(isRefresh bool) {
	if isRefresh {
		h.exitMode()
	}
}
