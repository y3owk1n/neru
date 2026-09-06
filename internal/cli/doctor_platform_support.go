package cli

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// platformSupportRow is the label the platform-support check reports under. It
// is a client-side check about the words in a configuration, which is a
// different question from the component health the daemon answers below it: a
// subsystem can be perfectly healthy while an option a person wrote means
// nothing here (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
const platformSupportRow = "platform_support"

// doctorConfigLoad reads the configuration the way the daemon would, for the
// client-side checks that judge it.
//
// One load for all of them: two would be two chances to disagree about which
// file is even being judged, and the load is what discovers that file.
func doctorConfigLoad() *config.LoadResult {
	svc := clientConfigLoader()

	path := configPath
	if path == "" {
		path = svc.FindConfigFile()
	}

	return svc.LoadWithValidation(path)
}

// printPlatformSupportCheck reports the options, actions and mode flags this
// configuration writes that do nothing on this platform.
//
// It never fails the doctor. An inert word is not a broken configuration — the
// same file is meant to work on every platform, which is why writing one is a
// warning and never a refusal — so this reports and returns.
func printPlatformSupportCheck(cmd *cobra.Command, loadResult *config.LoadResult) {
	platform, known := parity.Current()
	if !known {
		cmd.Printf("  %-27s %s\n", "  "+platformSupportRow,
			"no platform support is declared for "+runtime.GOOS)

		return
	}

	if loadResult.ValidationError != nil {
		cmd.Printf("  %-27s %s\n", "  "+platformSupportRow,
			"not checked: see neru config validate")

		return
	}

	if len(loadResult.Inert) == 0 {
		cmd.Printf("  ✅ %-24s %s\n", platformSupportRow,
			"every setting written here applies on "+string(platform))

		return
	}

	cmd.Printf("  ⚠️  %-24s %d %s nothing on %s (written and ignored; the daemon runs)\n",
		platformSupportRow,
		len(loadResult.Inert),
		pluralSettingsDo(len(loadResult.Inert)),
		platform,
	)

	for _, word := range loadResult.Inert {
		cmd.Printf("  %-27s %s — %s\n", "", word.Written(), word.Note)
	}
}

// pluralSettingsDo words the count so a single finding does not read as though
// the check itself is broken.
func pluralSettingsDo(count int) string {
	if count == 1 {
		return "setting does"
	}

	return "settings do"
}
