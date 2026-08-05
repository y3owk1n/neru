package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// alwaysOffered are the flags every mode command has, regardless of what the
// mode supports.
func alwaysOffered() []modecmd.Flag {
	return []modecmd.Flag{
		modecmd.FlagAction,
		modecmd.FlagModifier,
		modecmd.FlagOnExit,
		modecmd.FlagRepeat,
		modecmd.FlagToggle,
		modecmd.FlagCursorSelectionMode,
	}
}

// optionalOffered maps each remaining flag to the ModeConfig field that turns it
// on, so a mode declaring support must actually get the flag.
func optionalOffered() map[modecmd.Flag]func(*ModeConfig) {
	return map[modecmd.Flag]func(*ModeConfig){
		modecmd.FlagSearch:            func(c *ModeConfig) { c.SupportSearch = true },
		modecmd.FlagHideOnEmptySearch: func(c *ModeConfig) { c.SupportHideOnEmptySearch = true },
		modecmd.FlagRole:              func(c *ModeConfig) { c.SupportFiltering = true },
		modecmd.FlagText:              func(c *ModeConfig) { c.SupportFiltering = true },
		modecmd.FlagStrategy:          func(c *ModeConfig) { c.SupportStrategy = true },
		modecmd.FlagLabelDirection:    func(c *ModeConfig) { c.SupportLabelDirection = true },
		modecmd.FlagSplitWord:         func(c *ModeConfig) { c.SupportSplitWord = true },
		modecmd.FlagZoomToDepth:       func(c *ModeConfig) { c.SupportZoomToDepth = true },
	}
}

// TestBuildModeCommand_EveryFlagIsAccountedFor stops the two lists above from drifting out of
// step with the vocabulary as flags are added.
//
// The grammar pins that every flag in the shared vocabulary is one the daemon
// acts on. These cases pin the other end: that the command a user types offers
// those same flags under those same names, and that what it puts on the wire is
// spelled the way the vocabulary says.
//
// Between the two, a flag renamed in one place and not the other fails here or
// there rather than going quietly dead in the gap.
func TestBuildModeCommand_EveryFlagIsAccountedFor(t *testing.T) {
	optional := optionalOffered()

	for _, descriptor := range modecmd.All() {
		_, isOptional := optional[descriptor.Name()]
		if isOptional || slices.Contains(alwaysOffered(), descriptor.Name()) {
			continue
		}

		t.Errorf("--%s is in neither list; say whether every mode offers it", descriptor.Name())
	}
}

func TestBuildModeCommand_OffersTheFlagsItDeclares(t *testing.T) {
	for _, name := range alwaysOffered() {
		t.Run(string(name), func(t *testing.T) {
			cmd := BuildModeCommand(ModeConfig{Name: modeHints})

			if cmd.Flags().Lookup(name.String()) == nil {
				t.Errorf("--%s is in the vocabulary but the command does not offer it", name)
			}
		})
	}

	for name, enable := range optionalOffered() {
		t.Run(string(name), func(t *testing.T) {
			config := ModeConfig{Name: modeHints}
			enable(&config)

			flag := BuildModeCommand(config).Flags().Lookup(name.String())
			if flag == nil {
				t.Fatalf("--%s is declared supported but the command does not offer it", name)
			}

			if flag.Shorthand != shortOf(name) {
				t.Errorf("--%s shorthand = %q, want %q from the vocabulary",
					name, flag.Shorthand, shortOf(name))
			}
		})
	}
}

// TestBuildModeCommand_WithholdsFlagsItDoesNotSupport is the other half: a mode that
// does not declare a flag must not offer it, or a user would be able to type
// something the mode cannot act on.
func TestBuildModeCommand_WithholdsFlagsItDoesNotSupport(t *testing.T) {
	bare := BuildModeCommand(ModeConfig{Name: modeGrid})

	for name := range optionalOffered() {
		if bare.Flags().Lookup(name.String()) != nil {
			t.Errorf("--%s is offered by a mode that does not declare support for it", name)
		}
	}
}

// TestBuildModeCommand_OffersDebugOnItsOwn covers the one flag the lists above
// do not: --debug asks for a probe rather than an activation, so it is not in
// the mode-command vocabulary. Its command-line spelling still has to survive
// that, since it is a diagnostic habit people already have.
func TestBuildModeCommand_OffersDebugOnItsOwn(t *testing.T) {
	cmd := BuildModeCommand(ModeConfig{Name: modeHints, SupportDebug: true})

	flag := cmd.Flags().Lookup(flagDebug.String())
	if flag == nil {
		t.Fatal("--debug is declared supported but the command does not offer it")
	}

	if flag.Shorthand != flagDebugShort {
		t.Errorf("--debug shorthand = %q, want %q", flag.Shorthand, flagDebugShort)
	}

	if BuildModeCommand(ModeConfig{Name: modeGrid}).Flags().Lookup(flagDebug.String()) != nil {
		t.Error("--debug is offered by a mode that does not declare support for it")
	}
}

// TestIPCArgs_UsesTheSharedSpelling pins that what goes on the wire is written
// from the vocabulary. A flag the CLI spells itself would be a flag the daemon
// silently skips.
func TestIPCArgs_UsesTheSharedSpelling(t *testing.T) {
	args := modeFlags{
		action:              actLeftClick,
		modifier:            modCmd,
		onExitSteps:         []string{stepScroll},
		role:                roleAXButton,
		text:                "OK",
		strategy:            strategyVision,
		labelDirection:      labelReverse,
		cursorSelectionMode: cursorHold,
		repeat:              true,
		toggle:              true,
		search:              true,
		debug:               true,
		splitWord:           true,
		hideOnEmptySearch:   true,
		zoomToDepth:         3,
	}.ipcArgs(hintsMode())

	known := make([]string, 0, len(modecmd.All()))
	for _, descriptor := range modecmd.All() {
		known = append(known, descriptor.Name().Long())
	}

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			// The mode name and the positional action travel bare.
			continue
		}

		name, _, _ := strings.Cut(arg, "=")
		if !slices.Contains(known, name) {
			t.Errorf("ipcArgs emitted %q, which is not in the vocabulary the daemon reads", arg)
		}
	}
}
