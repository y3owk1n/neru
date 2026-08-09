package element

import "runtime"

// SemanticRole is a platform-neutral role name accepted in user configuration.
// Semantic roles are the closed vocabulary neru documents: each one expands to
// zero or more native role names for the platform neru is running on. Roles
// that have no semantic equivalent remain reachable through a vocabulary
// prefix (see NativeVocabulary).
type SemanticRole string

// The semantic role vocabulary. This set is closed: an unrecognized bare entry
// in clickable_roles is a configuration error, which is what makes typos
// detectable. Anything missing here is still addressable as "ax:...",
// "atspi:..." or "uia:...".
const (
	SemanticButton        SemanticRole = "button"
	SemanticMenuButton    SemanticRole = "menu_button"
	SemanticPopupButton   SemanticRole = "popup_button"
	SemanticComboBox      SemanticRole = "combo_box"
	SemanticLink          SemanticRole = "link"
	SemanticCheckbox      SemanticRole = "checkbox"
	SemanticRadio         SemanticRole = "radio"
	SemanticSwitch        SemanticRole = "switch"
	SemanticDisclosure    SemanticRole = "disclosure"
	SemanticTextField     SemanticRole = "text_field"
	SemanticTextArea      SemanticRole = "text_area"
	SemanticSearchField   SemanticRole = "search_field"
	SemanticSlider        SemanticRole = "slider"
	SemanticStepper       SemanticRole = "stepper"
	SemanticTab           SemanticRole = "tab"
	SemanticMenuItem      SemanticRole = "menu_item"
	SemanticMenubarItem   SemanticRole = "menubar_item"
	SemanticDockItem      SemanticRole = "dock_item"
	SemanticCell          SemanticRole = "cell"
	SemanticRow           SemanticRole = "row"
	SemanticListItem      SemanticRole = "list_item"
	SemanticImage         SemanticRole = "image"
	SemanticStaticText    SemanticRole = "static_text"
	SemanticHeading       SemanticRole = "heading"
	SemanticColorWell     SemanticRole = "color_well"
	SemanticToolbarButton SemanticRole = "toolbar_button"
)

// NativeVocabulary identifies one platform's accessibility role vocabulary. It
// doubles as the prefix users write in configuration to address a native role
// name directly, e.g. "atspi:page tab list".
type NativeVocabulary string

// The native vocabularies neru understands. Each maps to exactly one platform.
const (
	// VocabularyAX is the macOS Accessibility (AX*) role vocabulary.
	VocabularyAX NativeVocabulary = "ax"
	// VocabularyATSPI is the Linux AT-SPI role-name vocabulary, as returned by
	// Accessible.GetRoleName (lowercase, space separated).
	VocabularyATSPI NativeVocabulary = "atspi"
	// VocabularyUIA is the Windows UI Automation control-type vocabulary, using
	// the programmatic names behind the UIA_*ControlTypeId constants.
	VocabularyUIA NativeVocabulary = "uia"
)

// Go platform identifiers for the platforms with an accessibility backend.
const (
	goosDarwin  = "darwin"
	goosLinux   = "linux"
	goosWindows = "windows"
)

// Native role names shared by several semantic roles. Linux and Windows do not
// distinguish a text area or a search field from a plain text field.
const (
	atspiRoleEntry = "entry"
	uiaControlEdit = "Edit"
)

// vocabularyByGOOS maps a Go platform identifier to the native role vocabulary
// that platform's accessibility backend speaks.
var vocabularyByGOOS = map[string]NativeVocabulary{
	goosDarwin:  VocabularyAX,
	goosLinux:   VocabularyATSPI,
	goosWindows: VocabularyUIA,
}

// VocabularyForGOOS returns the native role vocabulary used by the given Go
// platform identifier. The second result is false for platforms with no
// accessibility backend, where every configured role resolves to nothing.
func VocabularyForGOOS(goos string) (NativeVocabulary, bool) {
	vocab, ok := vocabularyByGOOS[goos]

	return vocab, ok
}

// CurrentVocabulary returns the native role vocabulary for the running platform.
func CurrentVocabulary() (NativeVocabulary, bool) {
	return VocabularyForGOOS(runtime.GOOS)
}

// RoleMapping records how one semantic role expands into native role names on
// each supported platform. An empty slice means the platform has no equivalent;
// such roles are reported by `neru roles` rather than silently ignored.
type RoleMapping struct {
	// Semantic is the platform-neutral name written in configuration.
	Semantic SemanticRole
	// AX lists the macOS role names this semantic role expands to.
	AX []string
	// AXIsSubrole marks that AppKit declares this mapping's AX names as
	// subroles (NSAccessibilitySubrole constants in
	// NSAccessibilityConstants.h), not roles: an element reports such a name
	// in its AXSubrole attribute while its AXRole stays something more
	// generic, so the macOS matcher compares configured names against the
	// subrole as well as the role. AXSubroleNames collects the marked names.
	AXIsSubrole bool
	// ATSPI lists the Linux AT-SPI role names this semantic role expands to.
	ATSPI []string
	// UIA lists the Windows UI Automation control-type programmatic names this
	// semantic role expands to.
	UIA []string
}

// Native returns the native role names for the given vocabulary.
func (m RoleMapping) Native(vocab NativeVocabulary) []string {
	switch vocab {
	case VocabularyAX:
		return m.AX
	case VocabularyATSPI:
		return m.ATSPI
	case VocabularyUIA:
		return m.UIA
	default:
		return nil
	}
}

// RoleVocabulary is the full semantic role table, in documentation order. It is
// the single source of truth for `neru roles`, config resolution, and the role
// table in docs/CONFIGURATION.md.
//
// Overlapping expansions are intentional: Linux and Windows do not distinguish
// a text area or a search field from a plain text field, so those semantic
// roles expand onto the same native name there. Resolution unions the results.
var RoleVocabulary = []RoleMapping{
	{
		Semantic: SemanticButton,
		AX:       []string{string(RoleButton)},
		ATSPI:    []string{"push button", "button", "toggle button"},
		UIA:      []string{"Button", "SplitButton"},
	},
	{
		Semantic: SemanticMenuButton,
		AX:       []string{string(RoleMenuButton)},
		// AT-SPI reports the GEnum nick of ATSPI_ROLE_PUSH_BUTTON_MENU, which
		// is "push button menu" — not "menu button".
		ATSPI: []string{"push button menu"},
	},
	{
		Semantic: SemanticPopupButton,
		AX:       []string{string(RolePopUpButton)},
		ATSPI:    []string{"combo box"},
		UIA:      []string{"ComboBox"},
	},
	{
		Semantic: SemanticComboBox,
		AX:       []string{string(RoleComboBox)},
		ATSPI:    []string{"combo box"},
		UIA:      []string{"ComboBox"},
	},
	{
		Semantic: SemanticLink,
		AX:       []string{string(RoleLink)},
		ATSPI:    []string{"link"},
		UIA:      []string{"Hyperlink"},
	},
	{
		Semantic: SemanticCheckbox,
		AX:       []string{string(RoleCheckBox)},
		ATSPI:    []string{"check box", "check menu item"},
		UIA:      []string{"CheckBox"},
	},
	{
		Semantic: SemanticRadio,
		AX:       []string{string(RoleRadioButton)},
		ATSPI:    []string{"radio button", "radio menu item"},
		UIA:      []string{"RadioButton"},
	},
	{
		Semantic: SemanticSwitch,
		// SwiftUI toggles report AXCheckBox / AXSwitch.
		AX:          []string{string(RoleSwitch)},
		AXIsSubrole: true,
		// ATSPI_ROLE_SWITCH is the dedicated role (GTK4 GtkSwitch and friends);
		// older toolkits still report a toggle button.
		ATSPI: []string{"switch", "toggle button"},
	},
	{
		Semantic: SemanticDisclosure,
		AX:       []string{string(RoleDisclosureTriangle)},
	},
	{
		Semantic: SemanticTextField,
		AX:       []string{string(RoleTextField)},
		ATSPI:    []string{atspiRoleEntry, "password text"},
		UIA:      []string{uiaControlEdit},
	},
	{
		Semantic: SemanticTextArea,
		AX:       []string{string(RoleTextArea)},
		ATSPI:    []string{atspiRoleEntry},
		UIA:      []string{uiaControlEdit},
	},
	{
		Semantic: SemanticSearchField,
		// AppKit search fields report AXTextField / AXSearchField.
		AX:          []string{string(RoleSearchField)},
		AXIsSubrole: true,
		ATSPI:       []string{atspiRoleEntry},
		UIA:         []string{uiaControlEdit},
	},
	{
		Semantic: SemanticSlider,
		AX:       []string{string(RoleSlider)},
		ATSPI:    []string{"slider"},
		UIA:      []string{"Slider"},
	},
	{
		Semantic: SemanticStepper,
		AX:       []string{string(RoleIncrementor)},
		ATSPI:    []string{"spin button"},
		UIA:      []string{"Spinner"},
	},
	{
		Semantic: SemanticTab,
		// AppKit tab buttons report AXRadioButton / AXTabButton.
		AX:          []string{string(RoleTabButton)},
		AXIsSubrole: true,
		ATSPI:       []string{"page tab"},
		UIA:         []string{"TabItem"},
	},
	{
		Semantic: SemanticMenuItem,
		AX:       []string{string(RoleMenuItem)},
		ATSPI:    []string{"menu item"},
		UIA:      []string{"MenuItem"},
	},
	{
		Semantic: SemanticMenubarItem,
		AX:       []string{string(RoleMenuBarItem)},
	},
	{
		Semantic: SemanticDockItem,
		AX:       []string{string(RoleDockItem)},
	},
	{
		Semantic: SemanticCell,
		AX:       []string{string(RoleCell)},
		ATSPI:    []string{"table cell"},
		UIA:      []string{"DataItem"},
	},
	{
		Semantic: SemanticRow,
		AX:       []string{string(RoleRow)},
		ATSPI:    []string{"table row"},
		UIA:      []string{"TreeItem"},
	},
	{
		Semantic: SemanticListItem,
		AX:       []string{string(RoleRow)},
		ATSPI:    []string{"list item"},
		UIA:      []string{"ListItem"},
	},
	{
		Semantic: SemanticImage,
		AX:       []string{string(RoleImage)},
		ATSPI:    []string{"image", "icon"},
		UIA:      []string{"Image"},
	},
	{
		Semantic: SemanticStaticText,
		AX:       []string{string(RoleStaticText)},
		ATSPI:    []string{"static", "label", "text"},
		UIA:      []string{"Text"},
	},
	{
		Semantic: SemanticHeading,
		AX:       []string{string(RoleHeading)},
		ATSPI:    []string{"heading"},
	},
	{
		Semantic: SemanticColorWell,
		AX:       []string{string(RoleColorWell)},
		ATSPI:    []string{"color chooser"},
	},
	{
		Semantic: SemanticToolbarButton,
		// Toolbar buttons report AXButton / AXToolbarButton.
		AX:          []string{string(RoleToolbarButton)},
		AXIsSubrole: true,
	},
}

// AXSubroleNames collects the AX names RoleVocabulary marks with AXIsSubrole:
// the names AppKit delivers in AXSubrole rather than AXRole. None of them is
// also declared as a role, which is what keeps the matcher's two comparisons
// from colliding.
var AXSubroleNames = func() map[string]bool {
	names := make(map[string]bool)
	for _, mapping := range RoleVocabulary {
		if !mapping.AXIsSubrole {
			continue
		}

		for _, name := range mapping.AX {
			names[name] = true
		}
	}

	return names
}()

// DefaultClickableRoles is the role list neru ships as the default value of
// hints.clickable_roles. It is also the fallback an accessibility backend uses
// when a caller supplies no explicit role filter, so that the default hint
// targets are the same whether or not a config is present.
var DefaultClickableRoles = []string{
	string(SemanticButton),
	string(SemanticMenuButton),
	string(SemanticPopupButton),
	string(SemanticComboBox),
	string(SemanticCheckbox),
	string(SemanticRadio),
	string(SemanticSwitch),
	string(SemanticDisclosure),
	string(SemanticLink),
	string(SemanticTextField),
	string(SemanticTextArea),
	string(SemanticSlider),
	string(SemanticTab),
	string(SemanticMenuItem),
	string(SemanticCell),
	string(SemanticRow),
	// macOS web content and other AX-only containers surface as
	// AXGenericElement, which has no cross-platform meaning.
	string(VocabularyAX) + vocabularySeparator + string(RoleGenericElement),
}

// semanticIndex indexes RoleVocabulary by semantic name.
var semanticIndex = func() map[SemanticRole]RoleMapping {
	index := make(map[SemanticRole]RoleMapping, len(RoleVocabulary))
	for _, mapping := range RoleVocabulary {
		index[mapping.Semantic] = mapping
	}

	return index
}()

// nativeIndex indexes RoleVocabulary the other way round: from a native role
// name in a given vocabulary back to the semantic role that covers it. It is
// used to turn a bare native name in a config into an actionable error message.
var nativeIndex = func() map[NativeVocabulary]map[string]SemanticRole {
	index := map[NativeVocabulary]map[string]SemanticRole{
		VocabularyAX:    {},
		VocabularyATSPI: {},
		VocabularyUIA:   {},
	}

	for _, mapping := range RoleVocabulary {
		for vocab, names := range map[NativeVocabulary][]string{
			VocabularyAX:    mapping.AX,
			VocabularyATSPI: mapping.ATSPI,
			VocabularyUIA:   mapping.UIA,
		} {
			for _, name := range names {
				if _, exists := index[vocab][name]; !exists {
					index[vocab][name] = mapping.Semantic
				}
			}
		}
	}

	return index
}()

// LookupSemantic returns the mapping for a semantic role name.
func LookupSemantic(role SemanticRole) (RoleMapping, bool) {
	mapping, ok := semanticIndex[role]

	return mapping, ok
}

// SemanticForNative returns the semantic role covering a native role name in
// the given vocabulary. Where several semantic roles expand onto the same
// native name, the first in RoleVocabulary order wins.
func SemanticForNative(vocab NativeVocabulary, native string) (SemanticRole, bool) {
	names, ok := nativeIndex[vocab]
	if !ok {
		return "", false
	}

	semantic, found := names[native]

	return semantic, found
}
