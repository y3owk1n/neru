//go:build linux

package atspi

import (
	"context"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// The AT-SPI role vocabulary: native role names, aliases, the name->ID table,
// and the clickable-role defaults. It does not walk the tree.
//
// atspiRoleNames lists AT-SPI role names in AtspiRole declaration order, so an
// index is its AtspiRole id, which the Collection.GetMatches bitfield needs.
// Deriving ids from position makes a gap or a duplicate impossible.
//
// A name is the role's GEnum nick with hyphens as spaces, exactly what
// Accessible.GetRoleName returns. The enum is append-only. The list runs
// through ATSPI_ROLE_SWITCH, and later roles fall back to the per-node walk.
// Inserting or removing a line shifts every later id, so
// TestAtspiRoleNameToIDAnchors pins ids spread across the enum.
var atspiRoleNames = []string{
	"invalid",
	"accelerator label",
	"alert",
	"animation",
	"arrow",
	"calendar",
	"canvas",
	"check box",
	"check menu item",
	"color chooser",
	"column header",
	"combo box",
	"date editor",
	"desktop icon",
	"desktop frame",
	"dial",
	"dialog",
	"directory pane",
	"drawing area",
	"file chooser",
	"filler",
	"focus traversable",
	"font chooser",
	atspiRoleFrame,
	"glass pane",
	"html container",
	"icon",
	"image",
	"internal frame",
	"label",
	"layered pane",
	"list",
	"list item",
	"menu",
	"menu bar",
	"menu item",
	"option pane",
	"page tab",
	"page tab list",
	"panel",
	"password text",
	"popup menu",
	"progress bar",
	atspiRolePushButton,
	"radio button",
	"radio menu item",
	"root pane",
	"row header",
	"scroll bar",
	"scroll pane",
	"separator",
	"slider",
	"spin button",
	"split pane",
	"status bar",
	"table",
	"table cell",
	"table column header",
	"table row header",
	"tearoff menu item",
	"terminal",
	"text",
	"toggle button",
	"tool bar",
	"tool tip",
	"tree",
	"tree table",
	"unknown",
	"viewport",
	"window",
	"extended",
	"header",
	"footer",
	"paragraph",
	"ruler",
	"application",
	"autocomplete",
	"editbar",
	"embedded",
	"entry",
	"chart",
	"caption",
	"document frame",
	"heading",
	"page",
	"section",
	"redundant object",
	"form",
	"link",
	"input method window",
	"table row",
	"tree item",
	"document spreadsheet",
	"document presentation",
	"document text",
	"document web",
	"document email",
	"comment",
	"list box",
	"grouping",
	"image map",
	"notification",
	"info bar",
	"level bar",
	"title bar",
	"block quote",
	"audio",
	"video",
	"definition",
	"article",
	"landmark",
	"log",
	"marquee",
	"math",
	"rating",
	"timer",
	"static",
	"math fraction",
	"math root",
	"subscript",
	"superscript",
	"description list",
	"description term",
	"description value",
	"footnote",
	"content deletion",
	"content insertion",
	"mark",
	"suggestion",
	atspiRolePushButtonMenu,
	"switch",
}

// atspiRoleNameAliases map an accepted spelling onto the canonical role name
// it shares an id with. Id 43 is ATSPI_ROLE_BUTTON on current at-spi2-core,
// formerly ATSPI_ROLE_PUSH_BUTTON and reported as "push button", so both are
// accepted and one config works on either. "menu button" is accepted because
// users reach for it before "push button menu".
var atspiRoleNameAliases = map[string]string{
	"button":      atspiRolePushButton,
	"menu button": atspiRolePushButtonMenu,
}

// atspiRoleNameToID indexes atspiRoleNames by name, with the aliases folded in.
var atspiRoleNameToID = func() map[string]int32 {
	ids := make(map[string]int32, len(atspiRoleNames)+len(atspiRoleNameAliases))

	for index, name := range atspiRoleNames {
		ids[name] = int32(index)
	}

	for alias, canonical := range atspiRoleNameAliases {
		if id, ok := ids[canonical]; ok {
			ids[alias] = id
		}
	}

	return ids
}()

// defaultClickableRoles is the shipped default role list in AT-SPI names, used
// when the caller passes no filter, so both select the same elements.
var defaultClickableRoles = func() map[string]struct{} {
	resolution := element.ResolveRoles(element.DefaultClickableRoles, "linux")

	set := make(map[string]struct{}, len(resolution.Native))
	for _, native := range resolution.Native {
		set[strings.ToLower(native)] = struct{}{}
	}

	return set
}()

// The two roles effectiveRole tells apart by state.
const (
	atspiRoleEntry = "entry"
	atspiRoleText  = "text"
)

// effectiveRole returns the role the client matches and reports an element
// as. GTK 3 reports entries and text views as text, other toolkits use text
// for read-only text, and only the EDITABLE state tells them apart. Editable
// text reads as an entry (text_field, text_area), read-only stays text
// (static_text).
func effectiveRole(roleName string, states []uint32) string {
	if roleName == atspiRoleText && atspiStateBit(states, atspiStateEditable) {
		return atspiRoleEntry
	}

	return roleName
}

// candidateRole reports whether an element of roleName can be in roles once
// effectiveRole has read its states.
func candidateRole(roleName string, roles map[string]struct{}) bool {
	if _, ok := roles[roleName]; ok {
		return true
	}

	_, wantsEntry := roles[atspiRoleEntry]

	return wantsEntry && roleName == atspiRoleText
}

// deriveTargetRoleIDs returns the AtspiRole ids for the requested role names,
// so Collection keeps exactly what the walk would. Entries also fetch text,
// which effectiveRole may read as an entry. Unknown names are skipped, and the
// caller falls back to the walk when nothing resolves.
func deriveTargetRoleIDs(roleSet map[string]struct{}) []int32 {
	var ids []int32

	seen := make(map[int32]struct{})

	names := make([]string, 0, len(roleSet)+1)
	for roleName := range roleSet {
		names = append(names, roleName)
	}

	if _, ok := roleSet[atspiRoleEntry]; ok {
		names = append(names, atspiRoleText)
	}

	for _, roleName := range names {
		roleID, ok := atspiRoleNameToID[roleName]
		if !ok {
			continue
		}

		if _, dup := seen[roleID]; dup {
			continue
		}

		seen[roleID] = struct{}{}

		ids = append(ids, roleID)
	}

	return ids
}

// roleBitfield packs AtspiRole ids into the fixed 5-word int32 bitfield libatspi
// expects: bit (id%32) of word (id/32).
func roleBitfield(ids []int32) []int32 {
	words := make([]int32, atspiRoleWords)

	for _, roleID := range ids {
		if roleID < 0 {
			continue
		}

		word := roleID / atspiBitsPerWord
		if int(word) < len(words) {
			words[word] |= 1 << uint(roleID%atspiBitsPerWord)
		}
	}

	return words
}

// isWindowRole reports whether an AT-SPI role name denotes a top-level window
// (the only frames hint selection considers).
func isWindowRole(role string) bool {
	return role == atspiRoleFrame || role == "window" || role == "dialog"
}

// roleName returns the AT-SPI localized-independent role name (e.g. "push button").
func (c *Client) roleName(ctx context.Context, conn *dbus.Conn, ref accRef) string {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	var name string

	err := conn.Object(ref.Name, ref.Path).
		CallWithContext(callCtx, atspiAccessibleIfc+".GetRoleName", 0).Store(&name)
	if err != nil {
		c.noteCallErr(ctx, err)

		return ""
	}

	return name
}

// rolesSet converts the caller's native AT-SPI role list into a lookup set,
// falling back to the default clickable roles when empty. It lowercases, as
// names read from GetRoleName are, so config casing does not matter.
func rolesSet(roles []string) map[string]struct{} {
	if len(roles) == 0 {
		return defaultClickableRoles
	}

	set := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		if trimmed := strings.TrimSpace(r); trimmed != "" {
			set[strings.ToLower(trimmed)] = struct{}{}
		}
	}

	if len(set) == 0 {
		return defaultClickableRoles
	}

	return set
}
