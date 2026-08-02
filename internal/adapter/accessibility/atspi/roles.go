//go:build linux

// internal/adapter/accessibility/atspi_roles_linux.go
// The AT-SPI role vocabulary: native role names, their aliases, the name->ID
// table, and the clickable-role defaults hints filters on.
// It does not walk the tree; it only answers what a role is called and whether
// it counts.

package atspi

import (
	"context"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// atspiRoleNames lists AT-SPI role names in AtspiRole declaration order, so a
// name's index in this slice is its AtspiRole id. Ids are needed to build the
// Collection.GetMatches role bitfield, and deriving them from position rather
// than writing them out makes a gap or a duplicate impossible.
//
// A name is the GEnum nick of the role with hyphens replaced by spaces, which
// is exactly what Accessible.GetRoleName returns. The enum is ABI-stable
// (append-only), so existing entries never move; the list is complete through
// ATSPI_ROLE_SWITCH, and roles added later fall back to the per-node walk
// instead of the Collection query.
//
// Inserting or removing a line shifts every id after it, so
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

// atspiRoleNameAliases map an accepted spelling onto the canonical role name it
// shares an id with.
//
// Id 43 is ATSPI_ROLE_BUTTON on current at-spi2-core and was
// ATSPI_ROLE_PUSH_BUTTON before it was renamed, and the name reported for it
// has historically been "push button". Rather than depend on which spelling a
// given release reports, both are accepted so one config works on either.
// "menu button" is accepted because users reach for it before the less obvious
// "push button menu".
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

// defaultClickableRoles is used when the caller passes no explicit role filter.
// It is the shipped default role list resolved into AT-SPI role names, so the
// fallback and a default configuration select exactly the same elements.
var defaultClickableRoles = func() map[string]struct{} {
	resolution := element.ResolveRoles(element.DefaultClickableRoles, "linux")

	set := make(map[string]struct{}, len(resolution.Native))
	for _, native := range resolution.Native {
		set[strings.ToLower(native)] = struct{}{}
	}

	return set
}()

// deriveTargetRoleIDs returns the AtspiRole ids for the requested role names —
// exactly the roles the per-node walk would keep — so the Collection query has
// identical semantics. Requested names with no known id are skipped; the caller
// falls back to the walk when nothing resolves.
func deriveTargetRoleIDs(roleSet map[string]struct{}) []int32 {
	var ids []int32

	seen := make(map[int32]struct{})

	for roleName := range roleSet {
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
// falling back to the default clickable role set when empty. AT-SPI role names
// are canonically lowercase, and both this set and the names read from
// Accessible.GetRoleName are lowercased so a config written with different
// casing still matches.
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
