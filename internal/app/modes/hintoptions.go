package modes

import (
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/domain"
)

// applyHintOptions writes an activation's options into the hint context.
//
// A fresh activation and a refresh treat an absent option differently, and the
// difference is the whole reason this is two paths rather than one.
func applyHintOptions(ctx *hints.Context, opts ModeActivationOptions, isRefresh bool) {
	if isRefresh {
		applyHintOptionOverrides(ctx, opts)

		return
	}

	applyHintOptionsFresh(ctx, opts)
}

// applyHintOptionOverrides writes only the options a refresh was given.
//
// A refresh can come from something that carries no options at all — a space
// change re-entering hints through a configured "hints" action with no flags —
// so anything left unset has to keep the value the user activated the mode with
// rather than fall back to a default.
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

// applyHintOptionsFresh writes every field, so that an option left off the
// command line falls back to its default instead of inheriting whatever the
// previous activation of the mode happened to set.
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

// derefOr reads an optional option, falling back to the value an unset option
// means.
func derefOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}

	return *value
}

// hintOverrides are the three settings that decide how a hint scan is run.
type hintOverrides struct {
	strategy       string
	labelDirection string
	splitWord      bool
}

// resolveHintOverrides reads the overrides in force for this activation.
//
// They come from the context, which applyHintOptions has just written, so that a
// refresh sees the values the mode was originally activated with rather than the
// empty options a refresh usually carries. Reading the options directly is the
// fallback for when there is no context to read.
func (h *Handler) resolveHintOverrides(opts ModeActivationOptions) hintOverrides {
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

// abandonHintActivation gives up on an activation that cannot finish.
//
// A refresh has to exit the mode as well: its overlay is still showing the
// labels from the previous scan, and leaving them up would present stale hints
// that no longer point at anything.
func (h *Handler) abandonHintActivation(isRefresh bool) {
	if isRefresh {
		h.exitModeLocked()
	}
}
