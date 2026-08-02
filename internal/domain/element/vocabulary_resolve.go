package element

import (
	"fmt"
	"runtime"
	"strings"
)

// vocabularySeparator separates a vocabulary prefix from a native role name in
// a configuration entry, e.g. "atspi:page tab list". No accessibility role name
// on any supported platform contains a colon, so splitting on the first one is
// unambiguous.
const vocabularySeparator = ":"

// RoleDiagnosticKind classifies a problem found while resolving configured
// roles into native role names.
type RoleDiagnosticKind int

const (
	// RoleDiagnosticUnknownName marks a bare entry that is not a semantic role.
	// It is fatal: the semantic vocabulary is closed, so this is a typo or a
	// native role name written without its vocabulary prefix.
	RoleDiagnosticUnknownName RoleDiagnosticKind = iota
	// RoleDiagnosticUnknownVocabulary marks an entry whose prefix is not one of
	// the known native vocabularies. It is fatal.
	RoleDiagnosticUnknownVocabulary
	// RoleDiagnosticForeignVocabulary marks a well-formed native entry that
	// belongs to a different platform. It is informational: cross-platform
	// configurations are expected to carry entries for every machine.
	RoleDiagnosticForeignVocabulary
	// RoleDiagnosticNoNativeEquivalent marks a valid semantic role that has no
	// counterpart on the running platform. It is informational.
	RoleDiagnosticNoNativeEquivalent
)

// RoleDiagnostic describes one configuration entry that could not be resolved
// into a usable native role, together with a human-readable explanation.
type RoleDiagnostic struct {
	// Entry is the configuration entry verbatim.
	Entry string
	// Kind classifies the problem.
	Kind RoleDiagnosticKind
	// Suggestion is a replacement entry, when one can be derived.
	Suggestion string
	// GOOS is the platform the entry was resolved against.
	GOOS string
}

// IsFatal reports whether the diagnostic should reject the configuration
// rather than merely be reported. Only malformed or unrecognized entries are
// fatal; entries that simply do not apply to this platform are not.
func (d RoleDiagnostic) IsFatal() bool {
	return d.Kind == RoleDiagnosticUnknownName || d.Kind == RoleDiagnosticUnknownVocabulary
}

// Message returns a human-readable explanation of the diagnostic.
func (d RoleDiagnostic) Message() string {
	switch d.Kind {
	case RoleDiagnosticUnknownName:
		if d.Suggestion != "" {
			return fmt.Sprintf("unknown role %q: use %q", d.Entry, d.Suggestion)
		}

		return fmt.Sprintf("unknown role %q", d.Entry)
	case RoleDiagnosticUnknownVocabulary:
		return fmt.Sprintf(
			"unknown role vocabulary in %q: expected a semantic role or one of %q, %q, %q",
			d.Entry, VocabularyAX, VocabularyATSPI, VocabularyUIA,
		)
	case RoleDiagnosticForeignVocabulary:
		return fmt.Sprintf("%q does not apply on %s and is ignored", d.Entry, d.GOOS)
	case RoleDiagnosticNoNativeEquivalent:
		return fmt.Sprintf("%q has no equivalent on %s and is ignored", d.Entry, d.GOOS)
	default:
		return fmt.Sprintf("invalid role %q", d.Entry)
	}
}

// ResolvedRoleEntry records how a single configuration entry resolved. It backs
// both role resolution and the `neru roles` explanation output.
type ResolvedRoleEntry struct {
	// Entry is the configuration entry verbatim.
	Entry string
	// Semantic is set when the entry named a semantic role.
	Semantic SemanticRole
	// Vocabulary is set when the entry carried a native vocabulary prefix.
	Vocabulary NativeVocabulary
	// Native lists the native role names the entry contributes on this
	// platform. It is empty for entries that do not apply here.
	Native []string
	// Diagnostic is set when the entry could not be fully resolved.
	Diagnostic *RoleDiagnostic
}

// RoleResolution is the outcome of resolving a configured role list.
type RoleResolution struct {
	// GOOS is the platform the entries were resolved against.
	GOOS string
	// Native is the deduplicated union of native role names, in entry order.
	Native []string
	// Entries records the per-entry outcome, in configuration order.
	Entries []ResolvedRoleEntry
	// Diagnostics lists every problem found, in configuration order.
	Diagnostics []RoleDiagnostic
}

// HasFatal reports whether any diagnostic should reject the configuration.
func (r RoleResolution) HasFatal() bool {
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.IsFatal() {
			return true
		}
	}

	return false
}

// FatalMessages returns the messages of every fatal diagnostic.
func (r RoleResolution) FatalMessages() []string {
	var messages []string

	for _, diagnostic := range r.Diagnostics {
		if diagnostic.IsFatal() {
			messages = append(messages, diagnostic.Message())
		}
	}

	return messages
}

// IgnoredMessages returns the messages of every non-fatal diagnostic, i.e. the
// entries that are valid but do not apply on this platform.
func (r RoleResolution) IgnoredMessages() []string {
	var messages []string

	for _, diagnostic := range r.Diagnostics {
		if !diagnostic.IsFatal() {
			messages = append(messages, diagnostic.Message())
		}
	}

	return messages
}

// ResolveRoles expands configured role entries into the native role names used
// by the accessibility backend of the given platform.
//
// An entry is either a semantic role from the closed vocabulary ("button"), or
// a native role name addressed through its vocabulary prefix ("atspi:page tab
// list"). Prefixed entries for other platforms resolve to nothing and are
// reported, not rejected, so one configuration can serve several machines.
// Empty and whitespace-only entries are skipped silently.
func ResolveRoles(entries []string, goos string) RoleResolution {
	current, hasVocabulary := VocabularyForGOOS(goos)

	resolution := RoleResolution{GOOS: goos}
	seen := make(map[string]struct{}, len(entries))

	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		resolved := resolveRoleEntry(entry, current, hasVocabulary, goos)

		for _, native := range resolved.Native {
			if _, duplicate := seen[native]; duplicate {
				continue
			}

			seen[native] = struct{}{}

			resolution.Native = append(resolution.Native, native)
		}

		if resolved.Diagnostic != nil {
			resolution.Diagnostics = append(resolution.Diagnostics, *resolved.Diagnostic)
		}

		resolution.Entries = append(resolution.Entries, resolved)
	}

	return resolution
}

// ResolveRolesForCurrentPlatform resolves entries against the running platform.
func ResolveRolesForCurrentPlatform(entries []string) RoleResolution {
	return ResolveRoles(entries, runtime.GOOS)
}

// resolveRoleEntry resolves a single non-empty configuration entry.
func resolveRoleEntry(
	entry string,
	current NativeVocabulary,
	hasVocabulary bool,
	goos string,
) ResolvedRoleEntry {
	if vocab, native, prefixed := splitVocabularyEntry(entry); prefixed {
		return resolveNativeEntry(entry, vocab, native, current, hasVocabulary, goos)
	}

	return resolveSemanticEntry(entry, current, hasVocabulary, goos)
}

// splitVocabularyEntry splits a prefixed entry into its vocabulary and native
// role name. The third result reports whether the entry carried a separator at
// all; an unrecognized prefix still returns true so the caller can report it
// rather than treating the whole string as a semantic name.
func splitVocabularyEntry(entry string) (NativeVocabulary, string, bool) {
	prefix, native, found := strings.Cut(entry, vocabularySeparator)
	if !found {
		return "", "", false
	}

	return NativeVocabulary(strings.TrimSpace(prefix)), strings.TrimSpace(native), true
}

// resolveNativeEntry resolves an entry that carried a vocabulary prefix.
func resolveNativeEntry(
	entry string,
	vocab NativeVocabulary,
	native string,
	current NativeVocabulary,
	hasVocabulary bool,
	goos string,
) ResolvedRoleEntry {
	resolved := ResolvedRoleEntry{Entry: entry, Vocabulary: vocab}

	if !knownVocabulary(vocab) || native == "" {
		resolved.Diagnostic = &RoleDiagnostic{
			Entry: entry,
			Kind:  RoleDiagnosticUnknownVocabulary,
			GOOS:  goos,
		}

		return resolved
	}

	if !hasVocabulary || vocab != current {
		resolved.Diagnostic = &RoleDiagnostic{
			Entry: entry,
			Kind:  RoleDiagnosticForeignVocabulary,
			GOOS:  goos,
		}

		return resolved
	}

	resolved.Native = []string{native}

	return resolved
}

// resolveSemanticEntry resolves a bare entry, which must name a semantic role.
func resolveSemanticEntry(
	entry string,
	current NativeVocabulary,
	hasVocabulary bool,
	goos string,
) ResolvedRoleEntry {
	semantic := SemanticRole(entry)

	mapping, ok := LookupSemantic(semantic)
	if !ok {
		return ResolvedRoleEntry{
			Entry: entry,
			Diagnostic: &RoleDiagnostic{
				Entry:      entry,
				Kind:       RoleDiagnosticUnknownName,
				Suggestion: suggestionForBareEntry(entry),
				GOOS:       goos,
			},
		}
	}

	resolved := ResolvedRoleEntry{Entry: entry, Semantic: semantic}

	if hasVocabulary {
		resolved.Native = mapping.Native(current)
	}

	if len(resolved.Native) == 0 {
		resolved.Diagnostic = &RoleDiagnostic{
			Entry: entry,
			Kind:  RoleDiagnosticNoNativeEquivalent,
			GOOS:  goos,
		}
	}

	return resolved
}

// knownVocabulary reports whether vocab is one of the native vocabularies.
func knownVocabulary(vocab NativeVocabulary) bool {
	switch vocab {
	case VocabularyAX, VocabularyATSPI, VocabularyUIA:
		return true
	default:
		return false
	}
}

// suggestionForBareEntry derives a replacement for a bare entry that is not a
// semantic role. A native role name written without its prefix — the shape
// every pre-vocabulary configuration has — resolves to the semantic role that
// covers it, which is the migration users need.
func suggestionForBareEntry(entry string) string {
	for _, vocab := range []NativeVocabulary{VocabularyAX, VocabularyATSPI, VocabularyUIA} {
		semantic, ok := SemanticForNative(vocab, entry)
		if !ok {
			continue
		}

		return string(semantic)
	}

	return ""
}
