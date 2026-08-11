package vision

import (
	"github.com/y3owk1n/neru/internal/domain/element"
)

// classifierRoles is the set of native role names the heuristic classifier
// emits, for one platform's accessibility vocabulary.
//
// It is native rather than semantic because of where the answer is consumed:
// the hint pipeline filters vision elements with ports.ElementFilter.Roles,
// which holds the configured clickable_roles already resolved to the running
// platform's vocabulary (internal/domain/element/vocabulary.go). A classifier
// that answered "AXButton" on Linux would be compared against a list holding
// "push button" and match nothing — a vision strategy that reads the screen
// correctly and then hints none of it.
//
// Every name here is one the corresponding semantic role expands to, which is
// what TestClassifierRolesFor_AnswersInTheVocabularyItWasAskedFor checks
// against the vocabulary rather than against a copy of this table.
type classifierRoles struct {
	Button     string
	Link       string
	StaticText string
	CheckBox   string
	Image      string
	// Generic is what a region scoring below the generic-clickable threshold
	// becomes: something was detected, nothing about it says what.
	Generic string
	// Unknown is the answer for a degenerate region, which is a bug in the
	// caller rather than a finding about the screen.
	Unknown string
}

// The per-vocabulary tables. Windows has no vision backend today
// (adapter_other.go refuses), and its row exists anyway for the same reason
// config's platform column carries one: a table that only covers the platforms
// currently implemented cannot be told from a table somebody forgot to extend.
var (
	axClassifierRoles = classifierRoles{
		Button:     "AXButton",
		Link:       "AXLink",
		StaticText: "AXStaticText",
		CheckBox:   "AXCheckBox",
		Image:      "AXImage",
		Generic:    "AXGenericElement",
		Unknown:    "AXUnknown",
	}

	// AT-SPI role names are lowercase and space separated, as
	// Accessible.GetRoleName returns them. "static" is the name AT-SPI gives
	// text that is not editable and not a label for something else, which is
	// all OCR can honestly claim about a run of recognized characters.
	atspiClassifierRoles = classifierRoles{
		Button:     "push button",
		Link:       "link",
		StaticText: "static",
		CheckBox:   "check box",
		Image:      "image",
		Generic:    "unknown",
		Unknown:    "unknown",
	}

	// UI Automation programmatic control-type names.
	uiaClassifierRoles = classifierRoles{
		Button:     "Button",
		Link:       "Hyperlink",
		StaticText: "Text",
		CheckBox:   "CheckBox",
		Image:      "Image",
		Generic:    "Custom",
		Unknown:    "Custom",
	}
)

// classifierRolesByVocabulary is the lookup the classifier is built from.
var classifierRolesByVocabulary = map[element.NativeVocabulary]classifierRoles{
	element.VocabularyAX:    axClassifierRoles,
	element.VocabularyATSPI: atspiClassifierRoles,
	element.VocabularyUIA:   uiaClassifierRoles,
}

// classifierRolesFor returns the role names to emit for one vocabulary. The
// second result is false for a vocabulary with no table, which no supported
// platform has.
func classifierRolesFor(vocab element.NativeVocabulary) (classifierRoles, bool) {
	roles, ok := classifierRolesByVocabulary[vocab]

	return roles, ok
}

// currentClassifierRoles returns the role names for the running platform.
//
// The fallback is the macOS table, and it is unreachable rather than a default:
// every GOOS with an accessibility backend has a vocabulary, and a platform
// with none has no vision adapter either — adapter_other.go refuses before a
// classifier is ever built.
func currentClassifierRoles() classifierRoles {
	vocab, hasVocabulary := element.CurrentVocabulary()
	if !hasVocabulary {
		return axClassifierRoles
	}

	roles, hasRoles := classifierRolesFor(vocab)
	if !hasRoles {
		return axClassifierRoles
	}

	return roles
}
