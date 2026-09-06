package loader

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// listedInertWords is how many inert words a single warning names before it
// stops listing and counts. The warning exists to tell a person that something
// they wrote does nothing; a wall of forty paths in a `neru config validate`
// run does that worse than a sentence and a pointer to the command that lists
// them all.
const listedInertWords = 10

// warnInertWords records one warning for everything a configuration writes that
// does nothing on this platform, and hands the findings back for the result to
// carry.
//
// One warning rather than one per word, because it is one thing to learn: an
// option that is inert here is not a mistake in the file, it is a promise this
// platform does not keep, and the file is very often the same one the person
// uses on macOS. That is also why it is a warning at all and never a refusal —
// refusing would cost the user every other line in the file (ADR 0002), and
// would trade a silent lie for a portability break
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
//
// The platform is passed in rather than read here, so the sentence a Linux user
// is shown can be read on the machine this is developed on.
func warnInertWords(
	warnings *config.Warnings,
	written config.Written,
	platform parity.Platform,
) parity.Declaration {
	inert := config.InertWords(written, platform)
	if len(inert) == 0 {
		return nil
	}

	warnings.Addf(
		"%d %s in this configuration %s nothing on %s and %s ignored: %s. "+
			"They load and the daemon runs; `neru doctor` lists them with why",
		len(inert),
		plural(len(inert), "setting", "settings"),
		plural(len(inert), "does", "do"),
		platform,
		plural(len(inert), "is", "are"),
		listInert(inert),
	)

	return inert
}

// warnBackendInert records one warning for everything a configuration writes
// that the display backend this process runs under cannot honor, and hands the
// findings back for the result to carry beside the platform-inert ones.
//
// One warning for the set, in the voice of warnInertWords, with the reason in
// the sentence: the platform warning can defer its reasons to `neru doctor`
// because there is one per word, while every word here shares the one note,
// and a person on X11 reading "passthrough does nothing here" wants to know why
// before they go and read anything else.
func warnBackendInert(warnings *config.Warnings, inert parity.Declaration) parity.Declaration {
	if len(inert) == 0 {
		return nil
	}

	warnings.Addf(
		"%d %s in this configuration %s nothing here and %s ignored: %s, because %s. "+
			"They load and the daemon runs",
		len(inert),
		plural(len(inert), "setting", "settings"),
		plural(len(inert), "does", "do"),
		plural(len(inert), "is", "are"),
		listInert(inert),
		inert[0].Note,
	)

	return inert
}

// listInert names the findings, stopping at listedInertWords and counting the
// rest.
func listInert(inert parity.Declaration) string {
	names := inert.Names()
	if len(names) <= listedInertWords {
		return strings.Join(names, ", ")
	}

	return fmt.Sprintf(
		"%s and %d more",
		strings.Join(names[:listedInertWords], ", "),
		len(names)-listedInertWords,
	)
}

// plural picks the wording for a count, so the one-finding case does not read
// as though the check itself is broken.
func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}

// writtenConfiguration reads what the user's files actually say: the TOML paths
// they carry and the steps their bindings, macros and hooks are written with.
//
// It reads the decoded files rather than the loaded configuration because only
// the files answer "did somebody write this?". The loaded configuration is
// every layer merged, where a default nobody chose is indistinguishable from a
// line somebody typed — and a warning about a default is a warning nobody can
// act on.
func writtenConfiguration(raw map[string]any, overridePath string) config.Written {
	written := config.Written{Options: make(map[string]string)}

	collectWritten(raw, "", &written)

	if overridePath != "" {
		var overrideRaw map[string]any

		// A parse failure here is not this reading's to report: the override
		// file is decoded again by the layer that applies it, which refuses the
		// load with the message.
		_, decodeErr := toml.DecodeFile(overridePath, &overrideRaw)
		if decodeErr == nil {
			collectWritten(overrideRaw, "", &written)
		}
	}

	return written
}

// collectWritten walks a decoded TOML tree, recording the path of every key and
// the text of every string in it.
//
// Every string, rather than the ones under a table this knows the name of: the
// action strings live in the global hotkey table, in five per-mode tables, in
// the per-app overrides of both, in the macro bodies and in the two Mission
// Control hooks, and a list of those places here would be a second copy of the
// binding walk to drift from the first. A string that is not a step names no
// action and enters no mode, so it contributes nothing.
func collectWritten(node map[string]any, prefix string, into *config.Written) {
	for key, value := range node {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		switch typed := value.(type) {
		case map[string]any:
			into.Options[path] = ""

			collectWritten(typed, path, into)
		case []map[string]any:
			into.Options[path] = ""

			for _, entry := range typed {
				collectWritten(entry, path, into)
			}
		case []any:
			into.Options[path] = ""

			for _, entry := range typed {
				switch element := entry.(type) {
				case map[string]any:
					collectWritten(element, path, into)
				case string:
					into.Steps = append(into.Steps, element)
				}
			}
		case string:
			into.Options[path] = typed
			into.Steps = append(into.Steps, typed)
		default:
			into.Options[path] = ""
		}
	}
}
