package ipcctrl

import (
	"context"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// probeOptions is what a hints probe needs to answer.
//
// It is deliberately not an activation. A probe draws nothing and enters
// nothing, so the flags that describe an activation — the pending action, the
// modifiers held during it, toggling, repeating, what runs on exit — mean
// nothing here. Only the four that shape which elements get collected carry
// over, which is why the probe reads its own request rather than a mode
// command.
type probeOptions struct {
	FilterRoles        []string
	FilterTextContains []string
	Strategy           string
	CaptureScope       string
	SplitWord          bool
}

// extractProbeOptions reads a probe request's flags.
//
// Anything outside the probe's own vocabulary is refused rather than ignored,
// so a caller that sends an activation flag learns it had no effect instead of
// assuming it did. The four flags are named and spelled from the mode-command
// grammar even so: a probe is written with the same words as the mode it
// reports on.
func (h *ModesHandler) extractProbeOptions(cmd ipc.Command) (probeOptions, *ipc.Response) {
	var opts probeOptions

	args := newProbeArgs(cmd)
	for ; args.more(); args.next() {
		resp := readProbeFlag(args, &opts)
		if resp != nil {
			return opts, resp
		}
	}

	return opts, nil
}

// readProbeFlag reads the flag the reader is positioned on into opts.
func readProbeFlag(args *probeArgs, opts *probeOptions) *ipc.Response {
	switch {
	case args.is(modecmd.FlagRole):
		return readProbeList(args, modecmd.FlagRole, &opts.FilterRoles)

	case args.is(modecmd.FlagText):
		return readProbeList(args, modecmd.FlagText, &opts.FilterTextContains)

	case args.is(modecmd.FlagStrategy):
		value, resp := args.take(valueMessage(modecmd.FlagStrategy))
		if resp != nil {
			return resp
		}

		strategy, err := modecmd.ParseStrategy(value)
		if err != nil {
			return refuse(valueMessage(modecmd.FlagStrategy))
		}

		opts.Strategy = strategy

		return nil

	case args.is(modecmd.FlagCaptureScope):
		value, resp := args.take(valueMessage(modecmd.FlagCaptureScope))
		if resp != nil {
			return resp
		}

		scope, err := modecmd.ParseCaptureScope(value)
		if err != nil {
			return refuse(valueMessage(modecmd.FlagCaptureScope))
		}

		opts.CaptureScope = scope

		return nil

	case args.is(modecmd.FlagSplitWord):
		opts.SplitWord = true

		return nil

	default:
		return refuse("unexpected argument for a hints probe: " + args.arg())
	}
}

// readProbeList appends a comma-separated filter. The flag is repeatable, so
// entries accumulate across occurrences.
func readProbeList(args *probeArgs, flag modecmd.Flag, field *[]string) *ipc.Response {
	value, resp := args.take(valueMessage(flag))
	if resp != nil {
		return resp
	}

	// An empty value filters nothing, and a caller who wrote one meant
	// something by it.
	if value == "" {
		return refuse(valueMessage(flag))
	}

	*field = append(*field, parseCSV(value)...)

	return nil
}

// valueMessage returns the one message a flag gives when its value is missing
// or unusable, so a probe and a mode command answer the same mistake the same
// way.
func valueMessage(flag modecmd.Flag) string {
	descriptor, known := modecmd.Lookup(flag)
	if !known {
		return flag.Long() + " requires a value"
	}

	return descriptor.ValueMessage()
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
		opts.CaptureScope,
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
