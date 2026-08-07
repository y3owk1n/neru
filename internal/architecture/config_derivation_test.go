package architecture_test

import (
	"slices"
	"testing"
)

// derivationChain is the method that runs every derivation that writes to the
// Config. A derivation this one does not call runs when the file is read and
// never again — `neru config set` re-derives by running exactly this over the
// configuration the user wrote, so a step left out of it is a value that
// silently stops tracking its source the moment anything is set at runtime.
//
// loader.ResolveDerived wraps it with the one derivation that is the loader's
// rather than the schema's. That one is out of reach here by design: config
// sits below loader and cannot name it.
const derivationChain = "ResolveDerived"

// derivationPrefix is the naming convention the chain's steps share, the same
// bargain validatorPrefix makes: no interface, no table, only the name.
const derivationPrefix = "Resolve"

// derivationChainEntryPoints are the Resolve* methods that are the chain rather
// than a step in it, mapped to why they cannot appear inside it.
//
// TestDerivationChainExemptionsStayHonest fails on an entry that stopped
// describing the code, so this list can only shrink.
var derivationChainEntryPoints = map[string]string{
	derivationChain: "the chain itself, which cannot be a step in itself",
}

// minDerivationSteps guards against a vacuous pass: an AST walk that matched
// nothing — because the chain was renamed, or its receiver changed — would
// report a perfectly wired chain made of no derivations. It stays at one rather
// than at the number of steps there are, so that a step actually missing from
// the chain is reported as itself instead of as a broken walk.
const minDerivationSteps = 1

// TestEveryConfigDerivationRunsInTheChain pins the derivation link of the
// option chain (internal/config/AGENTS.md). Derivation writes over the value
// the user wrote, which destroys the difference between "inferred" and
// "typed" — so it can only run on a configuration nothing has derived on yet,
// and the chain is the only place that knows how to do all of it. DefaultConfig
// calls it too, which is what keeps the defaults and a load agreeing about what
// derivation means.
//
// It counts only the derivations that *write*: a Resolve* method with no
// parameters and no results has nowhere to put an answer except the Config, so
// it is one of these. ResolveKeymap takes a mode and returns a Keymap; it
// computes on read, leaves nothing behind, and has nothing to be re-run.
func TestEveryConfigDerivationRunsInTheChain(t *testing.T) {
	declared := declaredConfigDerivations(t)
	called := configMethodsCalledBy(t, derivationChain, derivationPrefix)

	if len(called) < minDerivationSteps {
		t.Fatalf(
			"found only %d derivation calls in Config.%s, expected at least %d; "+
				"the AST walk is broken and this check would pass vacuously",
			len(called), derivationChain, minDerivationSteps,
		)
	}

	for _, name := range declared {
		if called[name] {
			continue
		}

		if _, isEntryPoint := derivationChainEntryPoints[name]; isEntryPoint {
			continue
		}

		t.Errorf(
			"Config.%s writes a derived value but Config.%s never calls it, so it "+
				"runs at load and never again; add it to the chain, or delete it",
			name, derivationChain,
		)
	}
}

// TestDerivationChainExemptionsStayHonest keeps the exemption from outliving
// what it describes, the way TestValidatorLadderExemptionsStayHonest does for
// the ladder: an entry that is no longer the chain is a derivation nobody runs,
// wearing an exemption that says it is fine.
func TestDerivationChainExemptionsStayHonest(t *testing.T) {
	declared := declaredConfigDerivations(t)
	called := configMethodsCalledBy(t, derivationChain, derivationPrefix)

	for name, reason := range derivationChainEntryPoints {
		switch {
		case !slices.Contains(declared, name):
			t.Errorf(
				"derivationChainEntryPoints names Config.%s, which is not a writing "+
					"%s* method on *Config; drop the entry (%s)",
				name, derivationPrefix, reason,
			)
		case called[name]:
			t.Errorf(
				"derivationChainEntryPoints names Config.%s, which Config.%s now "+
					"calls; drop the entry (%s)",
				name, derivationChain, reason,
			)
		}
	}
}

// declaredConfigDerivations names every Resolve* method on Config that settles
// a value into the Config rather than computing one for a caller.
func declaredConfigDerivations(t *testing.T) []string {
	t.Helper()

	declared := configMethodsWithPrefix(t, derivationPrefix)

	return slices.DeleteFunc(declared, func(name string) bool {
		return !writesToTheConfig(configMethodDecl(t, name))
	})
}
