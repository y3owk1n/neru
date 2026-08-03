package ipcctrl

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain/modeflag"
)

// The CLI and the daemon are two parsers of one flag vocabulary, separated by a
// socket. The daemon skips arguments it does not recognize — it has to, since a
// newer CLI may send a flag an older daemon has never heard of — which means a
// flag the daemon fails to handle produces no error anywhere. It just stops
// working.
//
// These cases pin that every flag in the shared vocabulary is one this parser
// actually acts on, so a flag added to modeflag without being wired in here, or
// renamed on one side only, fails rather than going quietly dead.

// The wire literals these cases repeat. They are spelled out rather than built
// from modeflag, so that a case still pins the exact text a user would type.
const (
	argActionLeftClick = "--action=left_click"
	argModifierCmd     = "--modifier=cmd"
	argSearch          = "--search"
)

// flagProbe is a flag together with proof the parser acted on it.
type flagProbe struct {
	// args is the flag as it arrives, value included where it needs one.
	args []string

	// applied reports whether the parsed options show the flag took effect.
	applied func(ModeActivationOptions) bool
}

// probes covers the whole vocabulary. TestEveryModeFlagHasAProbe is what keeps
// it that way.
func probes() map[modeflag.Name]flagProbe {
	return map[modeflag.Name]flagProbe{
		modeflag.Action: {
			args:    []string{argActionLeftClick},
			applied: func(o ModeActivationOptions) bool { return o.Action != nil },
		},
		// A modifier is held during the action, so it needs one to hold it for.
		modeflag.Modifier: {
			args:    []string{argActionLeftClick, argModifierCmd},
			applied: func(o ModeActivationOptions) bool { return o.Modifier != nil },
		},
		modeflag.OnExit: {
			args:    []string{"--on-exit=scroll"},
			applied: func(o ModeActivationOptions) bool { return len(o.OnExit) == 1 },
		},
		// Repeat re-activates after the action, so it needs one to follow.
		modeflag.Repeat: {
			args:    []string{argActionLeftClick, "--repeat"},
			applied: func(o ModeActivationOptions) bool { return o.Repeat != nil && *o.Repeat },
		},
		modeflag.Toggle: {
			args:    []string{"--toggle"},
			applied: func(o ModeActivationOptions) bool { return o.Toggle != nil && *o.Toggle },
		},
		modeflag.Search: {
			args:    []string{argSearch},
			applied: func(o ModeActivationOptions) bool { return o.Search != nil && *o.Search },
		},
		modeflag.HideOnEmptySearch: {
			args: []string{"--hide-on-empty-search", argSearch},
			applied: func(o ModeActivationOptions) bool {
				return o.HideOnEmptySearch != nil && *o.HideOnEmptySearch
			},
		},
		modeflag.Role: {
			args:    []string{"--role=AXButton"},
			applied: func(o ModeActivationOptions) bool { return len(o.FilterRoles) == 1 },
		},
		modeflag.Text: {
			args:    []string{"--text=OK"},
			applied: func(o ModeActivationOptions) bool { return len(o.FilterTextContains) == 1 },
		},
		modeflag.Strategy: {
			args:    []string{"--strategy=vision"},
			applied: func(o ModeActivationOptions) bool { return o.Strategy != nil },
		},
		modeflag.LabelDirection: {
			args:    []string{"--label-direction=reverse"},
			applied: func(o ModeActivationOptions) bool { return o.LabelDirection != nil },
		},
		modeflag.SplitWord: {
			args:    []string{"--split-word"},
			applied: func(o ModeActivationOptions) bool { return o.SplitWord != nil && *o.SplitWord },
		},
		modeflag.ZoomToDepth: {
			args:    []string{"--zoom-to-depth=3"},
			applied: func(o ModeActivationOptions) bool { return o.ZoomToDepth != nil },
		},
		modeflag.CursorSelectionMode: {
			args: []string{"--cursor-selection-mode=hold"},
			applied: func(o ModeActivationOptions) bool {
				return o.CursorFollowSelection != nil
			},
		},
		// Debug is the one flag the daemon deliberately accepts and ignores:
		// probing the focused window is work the CLI does. Recognizing it still
		// matters, because an unrecognized bare word would be taken as the
		// positional action instead.
		modeflag.Debug: {
			args:    []string{"--debug"},
			applied: func(o ModeActivationOptions) bool { return o.Action == nil },
		},
	}
}

// TestEveryModeFlagHasAProbe is what stops the coverage below from silently
// shrinking as flags are added.
func TestEveryModeFlagHasAProbe(t *testing.T) {
	covered := probes()

	for _, spec := range modeflag.All() {
		_, exists := covered[spec.Name]
		if !exists {
			t.Errorf("modeflag.%s has no probe; add one so the daemon side stays pinned", spec.Name)
		}
	}

	if len(covered) != len(modeflag.All()) {
		t.Errorf("probes = %d, vocabulary = %d; a probe names a flag that no longer exists",
			len(covered), len(modeflag.All()))
	}
}

func TestTheDaemonActsOnEveryModeFlag(t *testing.T) {
	handler := &ModesHandler{}

	for name, probe := range probes() {
		t.Run(string(name), func(t *testing.T) {
			opts, resp := handler.extractModeOptions(ipc.Command{
				Action: onExitTestMode,
				Args:   append([]string{onExitTestMode}, probe.args...),
			})

			if resp != nil {
				t.Fatalf("%v was refused: %s", probe.args, resp.Message)
			}

			if !probe.applied(opts) {
				t.Errorf("%v parsed without taking effect; the daemon does not handle --%s",
					probe.args, name)
			}
		})
	}
}

// TestValueFlagsRefuseAMissingValue pins the other half of the shape the
// vocabulary declares: a flag marked as taking a value must say so when it
// arrives without one, rather than swallowing whatever follows.
func TestValueFlagsRefuseAMissingValue(t *testing.T) {
	handler := &ModesHandler{}

	for _, spec := range modeflag.All() {
		if !spec.TakesValue {
			continue
		}

		t.Run(string(spec.Name), func(t *testing.T) {
			_, resp := handler.extractModeOptions(ipc.Command{
				Action: onExitTestMode,
				Args:   []string{onExitTestMode, spec.Name.Flag()},
			})

			if resp == nil {
				t.Errorf("--%s with no value was accepted; it is declared as taking one",
					spec.Name)
			}
		})
	}
}

// TestShortFormsReachTheSameFlag pins that the short spellings work, since a
// hotkey binding is written by hand and may use either form.
func TestShortFormsReachTheSameFlag(t *testing.T) {
	handler := &ModesHandler{}
	covered := probes()

	for _, spec := range modeflag.All() {
		if spec.Short == "" {
			continue
		}

		t.Run(string(spec.Name), func(t *testing.T) {
			probe := covered[spec.Name]

			short := make([]string, len(probe.args))
			copy(short, probe.args)
			// Rewrite only this flag's own argument into its short form; any
			// others the probe needed stay as they are.
			for index, arg := range short {
				if spec.Match(arg) {
					short[index] = "-" + spec.Short + valueOf(arg)
				}
			}

			opts, resp := handler.extractModeOptions(ipc.Command{
				Action: onExitTestMode,
				Args:   append([]string{onExitTestMode}, short...),
			})

			if resp != nil {
				t.Fatalf("%v was refused: %s", short, resp.Message)
			}

			if !probe.applied(opts) {
				t.Errorf("%v did not take effect; -%s does not reach --%s",
					short, spec.Short, spec.Name)
			}
		})
	}
}

// valueOf returns the "=value" part of an argument, or an empty string.
func valueOf(arg string) string {
	for index := range arg {
		if arg[index] == '=' {
			return arg[index:]
		}
	}

	return ""
}
