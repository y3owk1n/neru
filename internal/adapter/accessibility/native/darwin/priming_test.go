//go:build darwin

package darwin

import "testing"

// The priming walk roots at the application element, so the menu bar is in
// reach of it. Reading a menu bar item's children is not an observation —
// AppKit builds and displays the menu behind it — so a status-bar app gets its
// menu popped open on the user's screen every time priming runs. These pin the
// guard that stops the walk before that happens.

// TestMenuRolesCoverTheStatusItemPath pins the two roles that stand between the
// application element and a status item's menu. Losing either one puts the walk
// back on the path that opens it.
func TestMenuRolesCoverTheStatusItemPath(t *testing.T) {
	for _, role := range []string{"AXMenuBar", "AXMenuBarItem", "AXMenu", "AXMenuItem"} {
		if _, ok := menuRoles[role]; !ok {
			t.Errorf("menuRoles is missing %q, so the priming walk would descend into it", role)
		}
	}
}

// TestMenuRolesCannotMaskAReadyRole is the invariant that makes the guard free:
// a role the walk refuses to enter must never be the role it is looking for,
// or skipping it would turn a ready tree into a timeout. Web content never
// lives inside a menu, so the two sets are disjoint by construction — this
// fails if a later edit breaks that.
func TestMenuRolesCannotMaskAReadyRole(t *testing.T) {
	for role := range menuRoles {
		if _, ready := readyRoles[role]; ready {
			t.Errorf(
				"%q is both a menu role and a ready role, so the guard hides a primed tree",
				role,
			)
		}
	}
}
