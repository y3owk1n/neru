package modes

import (
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/domain"
)

// applyHintOptions writes an activation's options into the hint context. A
// fresh activation and a refresh read an absent option differently, which is
// why this is two paths.
func applyHintOptions(ctx *hints.Context, opts ModeActivationOptions, isRefresh bool) {
	if isRefresh {
		applyHintOptionOverrides(ctx, opts)

		return
	}

	applyHintOptionsFresh(ctx, opts)
}

// applyHintOptionOverrides writes only what a refresh was given. A refresh can
// arrive with no options at all (a space change re-entering hints through a
// configured "hints" action), so anything unset keeps the value the user
// activated the mode with.
func applyHintOptionOverrides(ctx *hints.Context, opts ModeActivationOptions) {
	if opts.Action != nil {
		ctx.SetPendingAction(opts.Action)
	}

	if opts.OnExit != nil {
		ctx.SetOnExit(opts.OnExit)
	}

	if opts.Modifier != nil {
		ctx.SetPendingModifier(opts.Modifier)
	}

	if opts.CursorFollowSelection != nil {
		ctx.SetCursorFollowSelection(*opts.CursorFollowSelection)
	}

	if opts.FilterRoles != nil {
		ctx.SetFilterRoles(opts.FilterRoles)
	}

	if opts.FilterTextContains != nil {
		ctx.SetFilterTextContains(opts.FilterTextContains)
	}

	if opts.Search != nil {
		ctx.SetStartWithSearch(*opts.Search)
	}

	if opts.HideOnEmptySearch != nil {
		ctx.SetHideOnEmptySearch(*opts.HideOnEmptySearch)
	}

	if opts.Strategy != nil {
		ctx.SetStrategyOverride(*opts.Strategy)
	}

	if opts.LabelDirection != nil {
		ctx.SetLabelDirectionOverride(*opts.LabelDirection)
	}

	if opts.SplitWord != nil {
		ctx.SetSplitWord(*opts.SplitWord)
	}
}

// applyHintOptionsFresh writes every field, so an option left off the command
// line falls back to its default rather than inheriting the last activation's.
func applyHintOptionsFresh(ctx *hints.Context, opts ModeActivationOptions) {
	ctx.SetPendingAction(opts.Action)
	ctx.SetOnExit(opts.OnExit)
	ctx.SetPendingModifier(opts.Modifier)
	ctx.SetRepeat(false)
	ctx.SetCursorFollowSelection(resolveCursorFollowSelection(
		domain.ModeHints,
		opts.CursorFollowSelection,
	))
	ctx.SetFilterRoles(opts.FilterRoles)
	ctx.SetFilterTextContains(opts.FilterTextContains)
	ctx.SetStartWithSearch(opts.Search != nil && *opts.Search)
	ctx.SetHideOnEmptySearch(opts.HideOnEmptySearch != nil && *opts.HideOnEmptySearch)
	ctx.SetStrategyOverride(derefOr(opts.Strategy, ""))
	ctx.SetLabelDirectionOverride(derefOr(opts.LabelDirection, ""))
	ctx.SetSplitWord(derefOr(opts.SplitWord, false))
}

// derefOr reads an optional flag, falling back to what unset means.
func derefOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}

	return *value
}

// hintOverrides are the settings that decide how a hint scan runs.
type hintOverrides struct {
	strategy       string
	labelDirection string
	splitWord      bool
}

// resolveHintOverrides reads the overrides in force. They come from the
// context, which applyHintOptions has just written, so a refresh sees what the
// mode was activated with rather than the empty options it carries. The raw
// options are the fallback when there is no context.
func (h *handlerState) resolveHintOverrides(opts ModeActivationOptions) hintOverrides {
	if h.hints != nil && h.hints.Context != nil {
		return hintOverrides{
			strategy:       h.hints.Context.StrategyOverride(),
			labelDirection: h.hints.Context.LabelDirectionOverride(),
			splitWord:      h.hints.Context.SplitWord(),
		}
	}

	return hintOverrides{
		strategy:       derefOr(opts.Strategy, ""),
		labelDirection: derefOr(opts.LabelDirection, ""),
		splitWord:      derefOr(opts.SplitWord, false),
	}
}

// abandonHintActivation gives up on an activation. A refresh must also exit the
// mode, or its overlay keeps showing labels that no longer point at anything.
func (h *handlerState) abandonHintActivation(isRefresh bool) {
	if isRefresh {
		h.exitMode()
	}
}
