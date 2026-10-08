//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../../platform/darwin/accessibility.h"
*/
import "C"

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	// windowManagerBundleID is the process that draws Mission Control.
	windowManagerBundleID = "com.apple.WindowManager"
	// spacesBarIdentifier names the group holding the Spaces bar's desktops
	// and its add-desktop button.
	spacesBarIdentifier = "mc.spaces"
)

// WindowOnScreen reports whether the element names a window, as a Mission
// Control thumbnail does, and then whether that window is on screen.
func (e *Element) WindowOnScreen() (bool, bool) {
	if e.ref == nil {
		return false, false
	}

	var cHasWindow C.bool

	result := C.NeruIsElementWindowOnScreen(e.ref, &cHasWindow)

	return bool(cHasWindow), bool(result)
}

// MissionControlClickableElements returns what Mission Control draws that a
// hint can land on: the current desktop's windows, and the Spaces bar while it
// is expanded.
//
// WindowManager lists the windows of every desktop, so a thumbnail is kept
// only while its window is on screen, which during Mission Control is the
// current desktop's alone. A Spaces bar button reports its center as its
// position, so this moves its frame back by half its size. While the bar is
// collapsed those centers lie above the screen, so none of its buttons is
// kept until it expands.
func MissionControlClickableElements(
	ctx context.Context,
	logger *zap.Logger,
	configProvider config.Provider,
	maxDepth int,
) ([]*TreeNode, error) {
	app := ApplicationByBundleID(windowManagerBundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID",
			zap.String("bundle_id", windowManagerBundleID))

		return []*TreeNode{}, nil
	}
	defer app.Release()

	opts := DefaultTreeOptions(logger)
	opts.SetConfigProvider(configProvider)
	opts.SetMaxDepth(maxDepth)

	tree, err := BuildTree(ctx, app, opts)
	if err != nil {
		return nil, err
	}

	if tree == nil {
		return []*TreeNode{}, nil
	}

	allowedRoles := make(map[string]struct{})
	for _, role := range ClickableRoles() {
		allowedRoles[role] = struct{}{}
	}

	candidates := tree.FindClickableElements(allowedRoles, configProvider, false)

	screenTop := PlatformActiveScreenBounds().Min.Y
	barCollapsed := false

	for _, node := range candidates {
		if inSpacesBar(node) && node.Info().Position().Y < screenTop {
			barCollapsed = true

			break
		}
	}

	kept := make([]*TreeNode, 0, len(candidates))

	for _, node := range candidates {
		if inSpacesBar(node) {
			if !barCollapsed {
				info := node.Info()
				info.position = info.position.Sub(info.size.Div(2)) //nolint:mnd // half the button
				kept = append(kept, node)
			}

			continue
		}

		if hasWindow, onScreen := node.Element().WindowOnScreen(); hasWindow && !onScreen {
			continue
		}

		kept = append(kept, node)
	}

	ReleaseTreeExcept(tree, kept)

	return kept, nil
}

// inSpacesBar reports whether node sits in Mission Control's Spaces bar.
func inSpacesBar(node *TreeNode) bool {
	for ancestor := node.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		if ancestor.Info().Identifier() == spacesBarIdentifier {
			return true
		}
	}

	return false
}
