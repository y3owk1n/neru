package architecture_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/flagref"
)

// A mode flag is declared once, in the grammar's descriptor table. That
// declaration is what registers the flag on the command line, decides which
// modes accept it, and writes its row in the published reference.
//
// "Declared once" is only worth anything if the other two follow, and they
// followed by diligence until these tests: a flag added to the table without
// being registered was a flag the daemon accepted and nobody could type, and
// one missing from the reference was documentation that quietly disagreed with
// the binary. Both are now build failures.

// cliOwnedFlag is a flag a mode command may offer without the grammar
// declaring it: which modes may offer it, and why it is not in the table.
//
// The modes are named rather than left open because a flag outside the grammar
// is exactly what per-mode divergence looks like. Allowing "debug" everywhere
// would let it be hand-added to grid — a flag offered by one command and
// understood by nothing — which is what this whole contract exists to catch.
type cliOwnedFlag struct {
	name string

	// modes may offer this flag. Empty means every mode may.
	modes  []domain.Mode
	reason string
}

// offeredBy reports whether this mode is one of the modes allowed to offer the
// flag.
func (f cliOwnedFlag) offeredBy(mode domain.Mode) bool {
	return len(f.modes) == 0 || slices.Contains(f.modes, mode)
}

// flagsOwnedByTheCLI are the flags a mode command registers that the grammar
// deliberately does not hold.
//
// The list is spelled out rather than pattern-matched so that adding to it is a
// decision somebody writes down. A flag that reaches a mode belongs in the
// descriptor table; a flag that does something else has to say what it does and
// where it is allowed.
var flagsOwnedByTheCLI = []cliOwnedFlag{
	{
		name:   "debug",
		modes:  []domain.Mode{domain.ModeHints},
		reason: "asks for a read-only probe rather than an activation, so it travels as its own IPC action",
	},
	{
		name:   "help",
		reason: "cobra's own, offered by every command",
	},
}

// ownedByTheCLI returns what this test knows about a flag the grammar does not
// declare.
func ownedByTheCLI(name string) (cliOwnedFlag, bool) {
	for _, owned := range flagsOwnedByTheCLI {
		if owned.name == name {
			return owned, true
		}
	}

	return cliOwnedFlag{}, false
}

// TestModeCommands_RegisterExactlyWhatTheGrammarDeclares pins the command line
// to the vocabulary in both directions: every flag a mode accepts is offered by
// that mode's command, spelled and explained as declared, and every flag a
// command offers is one the grammar declares or one this test knows the reason
// for.
//
// The second direction is the one that keeps the contract honest. Without it a
// contributor can re-add a hand-written flag literal to a single command, which
// is exactly the divergence the grammar was introduced to end.
func TestModeCommands_RegisterExactlyWhatTheGrammarDeclares(t *testing.T) {
	commands := modeCommands(t)

	for _, mode := range modecmd.Modes() {
		name := domain.ModeString(mode)

		command, registered := commands[name]
		if !registered {
			t.Errorf(
				"the grammar names mode %q but no command enters it; "+
					"build one with cli.BuildModeCommand",
				name,
			)

			continue
		}

		assertOffersWhatTheModeAccepts(t, mode, command)
		assertOffersNothingUndeclared(t, mode, command)
	}
}

// assertOffersWhatTheModeAccepts checks the descriptor table against one
// command: accepted flags are offered, unaccepted ones are not, and the
// spelling and wording are the vocabulary's rather than the command's.
func assertOffersWhatTheModeAccepts(t *testing.T, mode domain.Mode, command *cobra.Command) {
	t.Helper()

	name := domain.ModeString(mode)

	for _, descriptor := range modecmd.All() {
		flag := command.Flags().Lookup(descriptor.Name().String())

		if !descriptor.AcceptedBy(mode) {
			if flag != nil {
				t.Errorf("%s does not accept %s but its command offers it; "+
					"a flag offered and then dropped is indistinguishable from one that does not work",
					name, descriptor.Name().Long())
			}

			continue
		}

		if flag == nil {
			t.Errorf("%s accepts %s but its command does not offer it",
				name, descriptor.Name().Long())

			continue
		}

		if flag.Shorthand != descriptor.Short() {
			t.Errorf("%s on %s has shorthand %q, want %q from the vocabulary",
				descriptor.Name().Long(), name, flag.Shorthand, descriptor.Short())
		}

		if flag.Usage != descriptor.Usage() {
			t.Errorf("%s on %s is explained as %q, want the vocabulary's own wording",
				descriptor.Name().Long(), name, flag.Usage)
		}
	}
}

// assertOffersNothingUndeclared checks the other direction: a flag on a mode
// command that the grammar never declared, and one the CLI owns appearing on a
// mode it was never meant for.
func assertOffersNothingUndeclared(t *testing.T, mode domain.Mode, command *cobra.Command) {
	t.Helper()

	name := domain.ModeString(mode)

	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if _, declared := modecmd.Lookup(modecmd.Flag(flag.Name)); declared {
			return
		}

		owned, allowed := ownedByTheCLI(flag.Name)
		if !allowed {
			t.Errorf(
				"the %s command offers --%s, which the grammar does not declare; "+
					"add it to the descriptor table in internal/domain/modecmd so every "+
					"door reads it the same way",
				name,
				flag.Name,
			)

			return
		}

		if !owned.offeredBy(mode) {
			t.Errorf(
				"the %s command offers --%s, which is the CLI's own (%s) and belongs "+
					"only on %s",
				name,
				flag.Name,
				owned.reason,
				modeNames(owned.modes),
			)
		}
	})
}

// modeNames spells a list of modes the way a user writes them.
func modeNames(modes []domain.Mode) string {
	names := make([]string, 0, len(modes))
	for _, mode := range modes {
		names = append(names, domain.ModeString(mode))
	}

	return strings.Join(names, ", ")
}

// TestModeFlagReference_IsGeneratedFromTheDescriptorTable pins the published
// reference to the same table.
//
// It regenerates each page's flag region and compares: a descriptor added,
// removed or re-worded without regenerating fails here rather than shipping a
// page that describes a binary nobody built.
//
// Every page carrying the markers is checked, rather than the one that carries
// them today. The generator writes whatever page it is pointed at, and a second
// one that nothing checked would be the drift this exists to end, in a new
// place.
func TestModeFlagReference_IsGeneratedFromTheDescriptorTable(t *testing.T) {
	published := 0

	for _, doc := range markdownFiles(t, findRepoRoot(t)) {
		contents, err := os.ReadFile(doc.absPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", doc.relPath, err)
		}

		document := string(contents)
		if !strings.Contains(document, flagref.BeginMarker) {
			continue
		}

		published++

		assertReferenceIsCurrent(t, doc.relPath, document)
	}

	if published == 0 {
		t.Errorf(
			"no page carries the mode-flag reference; add the %q marker to the page "+
				"that documents the mode commands",
			flagref.BeginMarker,
		)
	}
}

// assertReferenceIsCurrent checks one page: every declared flag has a row, and
// the whole region is what the descriptor table renders today.
func assertReferenceIsCurrent(t *testing.T, name, document string) {
	t.Helper()

	region, err := flagref.Region(document)
	if err != nil {
		t.Errorf("%s: %v", name, err)

		return
	}

	for _, descriptor := range modecmd.All() {
		if !strings.Contains(region, "`"+descriptor.Name().Long()+"`") {
			t.Errorf(
				"%s is missing from the mode-flag reference in %s; run `just genflagref`",
				descriptor.Name().Long(),
				name,
			)
		}
	}

	regenerated, err := flagref.Rewrite(document)
	if err != nil {
		t.Errorf("%s: %v", name, err)

		return
	}

	if regenerated != document {
		t.Errorf(
			"the mode-flag reference in %s is out of date with the descriptor table; "+
				"run `just genflagref` (the region is generated — do not edit it by hand)",
			name,
		)
	}
}

// modeCommands returns the commands that enter a mode, keyed by the word they
// are invoked by.
//
// They are discovered from the root command rather than listed, so a mode
// command added later is held to the same contract without an edit here.
func modeCommands(t *testing.T) map[string]*cobra.Command {
	t.Helper()

	found := map[string]*cobra.Command{}

	for _, command := range cli.RootCmd.Commands() {
		if _, isMode := modecmd.LookupMode(command.Name()); isMode {
			found[command.Name()] = command
		}
	}

	if len(found) == 0 {
		t.Fatal("no mode commands are registered on the root command")
	}

	return found
}
