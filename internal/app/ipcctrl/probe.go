package ipcctrl

import (
	"context"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain/modeflag"
)

// probeOptions is what a hints probe needs to answer.
//
// It is deliberately not a mode command's option set. A probe draws nothing
// and activates nothing, so the flags that describe an activation — the
// pending action, the modifiers held during it, toggling, repeating, what runs
// on exit — mean nothing here. Only the four that shape which elements get
// collected carry over.
type probeOptions struct {
	FilterRoles        []string
	FilterTextContains []string
	Strategy           string
	SplitWord          bool
}

// extractProbeOptions reads a probe request's flags.
//
// Anything outside the probe's own vocabulary is refused rather than ignored,
// so a caller that sends an activation flag learns it had no effect instead of
// assuming it did.
func (h *ModesHandler) extractProbeOptions(cmd ipc.Command) (probeOptions, *ipc.Response) {
	var opts probeOptions

	args := newModeArgs(cmd)
	for ; args.more(); args.next() {
		resp := readProbeFlag(args, &opts)
		if resp != nil {
			return opts, resp
		}
	}

	return opts, nil
}

// readProbeFlag reads the flag the reader is positioned on into opts.
func readProbeFlag(args *modeArgs, opts *probeOptions) *ipc.Response {
	switch {
	case args.is(modeflag.Role):
		return readListFlag(args, &opts.FilterRoles,
			"--role requires a value (use comma-separated: --role=AXButton,AXLink)")

	case args.is(modeflag.Text):
		return readListFlag(args, &opts.FilterTextContains,
			"--text requires a value (use comma-separated: --text=foo,bar)")

	case args.is(modeflag.Strategy):
		value, resp := args.take("--strategy requires a value: axtree or vision")
		if resp != nil {
			return resp
		}

		parsed, badValue := parseStrategyValue(value)
		if badValue != nil {
			return badValue
		}

		opts.Strategy = *parsed

		return nil

	case args.is(modeflag.SplitWord):
		opts.SplitWord = true

		return nil

	default:
		return refuse("unexpected argument for a hints probe: " + args.arg())
	}
}

// handleHintsProbe answers what hints mode would target for the focused
// window, without drawing an overlay or entering a mode.
func (h *ModesHandler) handleHintsProbe(ctx context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	opts, errResp := h.extractProbeOptions(cmd)
	if errResp != nil {
		return *errResp
	}

	summary, probeErr := h.modes.DebugProbeHints(
		ctx,
		opts.FilterRoles,
		opts.FilterTextContains,
		opts.Strategy,
		opts.SplitWord,
	)
	if probeErr != nil {
		return ipc.Response{
			Success: false,
			Message: "hints probe failed: " + probeErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{Success: true, Message: summary, Code: ipc.CodeOK}
}
