//go:build darwin

package accessibility

/*
#cgo CFLAGS: -x objective-c
#include "../platform/darwin/accessibility.h"
#include <stdlib.h>

*/
import "C"

import (
	"image"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// Pre-allocated common errors.
var errRootElementNil = derrors.New(derrors.CodeAccessibilityFailed, "root element is nil")

// rectFromInfo converts an ElementInfo's position and size into an image.Rectangle.
func rectFromInfo(info *ElementInfo) image.Rectangle {
	pos := info.Position()
	size := info.Size()

	return image.Rect(
		pos.X,
		pos.Y,
		pos.X+size.X,
		pos.Y+size.Y,
	)
}

// screenBoundsOrRect returns the active screen bounds when the provided rect is
// empty; otherwise returns the rect unchanged. This ensures that tree builders
// for all sources (main window, supplementary) always have meaningful clip
// bounds even when the root element has no position/size (e.g. an AXApplication).
func screenBoundsOrRect(r image.Rectangle) image.Rectangle {
	if r.Empty() {
		return platformActiveScreenBounds()
	}

	return r
}

// treeNodePool is a pool of TreeNode structs to reduce GC pressure during
// tree building. Hundreds of nodes are allocated per activation and released
// shortly after processClickableNodes extracts the kept elements.
var treeNodePool = sync.Pool{
	New: func() any {
		return &TreeNode{}
	},
}

// getTreeNode retrieves a TreeNode from the pool and initializes its fields.
// The children slice is reused from a previous lifecycle when available,
// otherwise a fresh slice with the requested capacity is allocated.
func getTreeNode(elem *Element, info *ElementInfo, parent *TreeNode, childrenCap int) *TreeNode {
	_node, ok := treeNodePool.Get().(*TreeNode)
	if !ok {
		_node = &TreeNode{}
	}
	_node.element = elem
	_node.info = info
	_node.parent = parent
	// Reuse the pooled backing array when it is large enough for the
	// requested capacity; otherwise allocate a fresh slice.
	// When childrenCap == 0 the children slice is still explicitly reset
	// to [:0] (or nil→[:0]) so the node is always in a clean state —
	// callers are not required to overwrite children before use.
	if cap(_node.children) >= childrenCap {
		_node.children = _node.children[:0]
	} else {
		_node.children = make([]*TreeNode, 0, childrenCap)
	}

	return _node
}

// maxPooledChildrenCap is the maximum children slice capacity retained when a
// TreeNode is returned to the pool. Nodes that once held more children than
// this threshold have their backing array discarded to avoid holding
// disproportionately large allocations in the pool indefinitely.
const maxPooledChildrenCap = 64

// putTreeNode clears all references in the node and returns it to the pool.
func putTreeNode(node *TreeNode) {
	// Nil out pointer fields to avoid retaining references to released
	// elements / info structs while the node sits in the pool.
	node.element = nil
	node.info = nil
	node.parent = nil
	// Clear child pointers but keep the backing array for reuse — unless it
	// exceeds the cap threshold, in which case discard it entirely so the
	// pool does not retain oversized allocations from pathological trees.
	if cap(node.children) > maxPooledChildrenCap {
		node.children = nil
	} else {
		for i := range node.children {
			node.children[i] = nil
		}
		node.children = node.children[:0]
	}
	treeNodePool.Put(node)
}

// TreeNode represents a node in the accessibility element hierarchy.
//
// After Release is called on the tree, non-kept nodes are recycled into
// treeNodePool. Kept nodes (those in the keep set) have their children and
// parent fields cleared to prevent dangling references to recycled nodes.
// Callers holding kept nodes (e.g. via InfraNode) must only access Element()
// and Info() after Release.
type TreeNode struct {
	element  *Element
	info     *ElementInfo
	children []*TreeNode
	parent   *TreeNode
}

// Element returns the node's element.
func (n *TreeNode) Element() *Element {
	return n.element
}

// Info returns the node's element info.
func (n *TreeNode) Info() *ElementInfo {
	return n.info
}

// Children returns the node's children.
func (n *TreeNode) Children() []*TreeNode {
	return n.children
}

// Parent returns the node's parent.
func (n *TreeNode) Parent() *TreeNode {
	return n.parent
}

// AddChild adds a child node.
func (n *TreeNode) AddChild(child *TreeNode) {
	n.children = append(n.children, child)
	child.parent = n
}

// TreeOptions configures accessibility tree traversal behavior and filtering.
type TreeOptions struct {
	filterFunc           func(*ElementInfo) bool
	maxDepth             int
	logger               *zap.Logger
	stats                *treeStats
	bundleID             string          // Bundle ID for auto-detecting Chromium/Electron strict filtering
	configProvider       config.Provider // For checking user-configured Chromium/Electron bundles
	isChromiumOrElectron bool            // Pre-computed flag for fast check
	onNode               func(*TreeNode) // Optional callback invoked for each valid node during tree building; called after the TreeNode is created, before recursing into its children
}

// FilterFunc returns the filter function.
func (o *TreeOptions) FilterFunc() func(*ElementInfo) bool {
	return o.filterFunc
}

// MaxDepth returns the max depth.
func (o *TreeOptions) MaxDepth() int {
	return o.maxDepth
}

// Logger returns the logger.
func (o *TreeOptions) Logger() *zap.Logger {
	return o.logger
}

// Stats returns the tree stats.
func (o *TreeOptions) Stats() *treeStats {
	return o.stats
}

// SetBundleID sets the bundle ID for auto-detecting Chromium/Electron strict filtering.
func (o *TreeOptions) SetBundleID(bundleID string) {
	o.bundleID = bundleID
}

// SetConfigProvider sets the config provider for checking user-configured Chromium/Electron bundles.
func (o *TreeOptions) SetConfigProvider(cp config.Provider) {
	o.configProvider = cp
}

// SetFilterFunc sets the filter function.
func (o *TreeOptions) SetFilterFunc(fn func(*ElementInfo) bool) {
	o.filterFunc = fn
}

// SetMaxDepth sets the max depth for tree traversal.
func (o *TreeOptions) SetMaxDepth(depth int) {
	o.maxDepth = depth
}

// DefaultTreeOptions returns default tree traversal options.
func DefaultTreeOptions(logger *zap.Logger) TreeOptions {
	return TreeOptions{
		filterFunc: nil,
		maxDepth:   config.DefaultMaxDepth,
		logger:     logger,
	}
}

// treeStats collects aggregate counters during tree traversal.
// All fields use atomic operations for goroutine safety.
type treeStats struct {
	nodesVisited          atomic.Int64
	skippedNonInteractive atomic.Int64
	stoppedAtLeaf         atomic.Int64
	maxDepthHits          atomic.Int64
	childErrors           atomic.Int64
	filteredOut           atomic.Int64
	noChildren            atomic.Int64
	childrenErrors        atomic.Int64
	sequentialBatches     atomic.Int64
	maxDepthSeen          atomic.Int64
	outOfBoundsSkipped    atomic.Int64
}

// recordDepth atomically updates the max depth seen.
func (s *treeStats) recordDepth(depth int) {
	for {
		old := s.maxDepthSeen.Load()
		if int64(depth) <= old {
			return
		}
		if s.maxDepthSeen.CompareAndSwap(old, int64(depth)) {
			return
		}
	}
}

// BuildTree constructs an accessibility tree starting from the specified root element.
func BuildTree(root *Element, opts TreeOptions) (*TreeNode, error) {
	if root == nil {
		if ce := opts.Logger().
			Check(zap.DebugLevel, "BuildTree called with nil root element"); ce != nil {
			ce.Write()
		}

		return nil, errRootElementNil
	}

	info, infoErr := root.Info()
	if infoErr != nil {
		opts.Logger().Warn("Failed to get root element info", zap.Error(infoErr))

		return nil, infoErr
	}

	if info == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "root element info is nil")
	}

	stats := &treeStats{}

	// Calculate window bounds for spatial filtering.
	// Fall back to active screen bounds when the root element has no
	// position/size (e.g. an AXApplication for supplementary sources).
	// For AXMenuBar, always use screen bounds since dropdown menus
	// render below the bar itself and would be clipped otherwise.
	windowBounds := screenBoundsOrRect(rectFromInfo(info))
	if info.Role() == string(element.RoleMenuBar) {
		windowBounds = platformActiveScreenBounds()
	}

	// Auto-detect Chromium/Electron from root element's bundle ID for strict filtering.
	if opts.bundleID == "" {
		opts.bundleID = root.BundleIdentifier()
	}
	opts.isChromiumOrElectron = isChromiumOrElectron(opts.bundleID, opts.configProvider)

	node := getTreeNode(root, info, nil, config.DefaultChildrenCapacity)

	opts.stats = stats
	buildTreeRecursive(node, 1, opts, windowBounds, windowBounds)
	accumulateSearchText(node)

	if ce := opts.Logger().Check(zap.DebugLevel, "Tree build completed"); ce != nil {
		ce.Write(
			zap.String("root_role", info.Role()),
			zap.String("root_title", info.Title()),
			zap.Int("pid", info.PID()),
			zap.Int64("nodes_visited", stats.nodesVisited.Load()),
			zap.Int64("skipped_non_interactive", stats.skippedNonInteractive.Load()),
			zap.Int64("stopped_at_leaf", stats.stoppedAtLeaf.Load()),
			zap.Int64("max_depth_hits", stats.maxDepthHits.Load()),
			zap.Int64("child_errors", stats.childErrors.Load()),
			zap.Int64("filtered_out", stats.filteredOut.Load()),
			zap.Int64("no_children", stats.noChildren.Load()),
			zap.Int64("children_errors", stats.childrenErrors.Load()),
			zap.Int64("sequential_batches", stats.sequentialBatches.Load()),
			zap.Int64("out_of_bounds_skipped", stats.outOfBoundsSkipped.Load()),
			zap.Int64("max_depth_seen", stats.maxDepthSeen.Load()),
		)
	}

	return node, nil
}

// Roles that typically don't contain interactive elements.
var nonInteractiveRoles = map[element.Role]bool{
	element.RoleStaticText: true,
	element.RoleImage:      true,
}

// Roles that are themselves interactive (leaf nodes).
var interactiveLeafRoles = map[element.Role]bool{
	element.RoleButton:             true,
	element.RoleMenuButton:         true,
	element.RoleComboBox:           true,
	element.RoleCheckBox:           true,
	element.RoleLink:               true,
	element.RolePopUpButton:        true,
	element.RoleSlider:             true,
	element.RoleTabButton:          true,
	element.RoleSwitch:             true,
	element.RoleDisclosureTriangle: true,
	element.RoleTextField:          true,
	element.RoleGenericElement:     true,
	element.RoleTextArea:           true,
	element.RoleRadioButton:        true,
}

// Roles that can contain important interactive children even when their
// parent is an interactive leaf (e.g., a button that opens a popover).
// This set is checked to ensure we don't stop traversal at buttons/menus
// that trigger popovers, sheets, or menus.
var importantContainerRoles = map[element.Role]bool{
	element.RolePopover: true,
	element.RoleSheet:   true,
	element.RoleMenu:    true,
	element.RoleSGTMenu: true,
	element.RoleList:    true,
	element.RoleLink:    true,
	element.RoleButton:  true,
}

// Roles that commonly spawn important container children (popovers, sheets, menus).
// Only for these parent roles do we fetch children to check for important containers
// when the parent is itself an interactive leaf. This avoids wasting Info() calls
// on children of leaf roles that never contain important containers.
var leafRolesWithImportantChildren = map[element.Role]bool{
	element.RoleButton:             true,
	element.RoleMenuButton:         true,
	element.RolePopUpButton:        true,
	element.RoleLink:               true,
	element.RoleComboBox:           true, // dropdown AXMenu
	element.RoleDisclosureTriangle: true, // reveals children when expanded
	element.RoleGenericElement:     true, // catch-all — could contain anything
	element.RoleTextArea:           true, // in notes.app, link field in note needs to be clickable...
	element.RoleRadioButton:        true, // in safari, url bar is radio button, but has nested button in it...
}

func buildTreeRecursive(
	parent *TreeNode,
	depth int,
	opts TreeOptions,
	clipBounds image.Rectangle,
	windowBounds image.Rectangle,
) {
	if opts.stats != nil {
		opts.stats.nodesVisited.Add(1)
		opts.stats.recordDepth(depth)
	}

	// Safety limit for recursion depth
	if opts.maxDepth > 0 && depth > opts.maxDepth {
		if opts.stats != nil {
			opts.stats.maxDepthHits.Add(1)
		}

		return
	}

	// Early exit for roles that can't have interactive children
	if nonInteractiveRoles[element.Role(parent.info.Role())] {
		if opts.stats != nil {
			opts.stats.skippedNonInteractive.Add(1)
		}

		return
	}

	// Don't traverse children of AX-hidden parents. This handles
	// CSS visibility:hidden / opacity:0 inheritance in web content
	// where the parent is marked hidden but children are not
	// individually flagged. Also skip AXVisible=false elements.
	if parent.info != nil && (parent.info.IsHidden() || !parent.info.IsVisible()) {
		return
	}

	// Early exit if element is out of window bounds
	// and only targeted on Chromium/Electron apps.
	// They tend to over fetch children and is extremely noisy.
	//
	// This check is intentionally placed before the Children() call to avoid
	// expensive AXAPI round-trips for off-screen subtrees — the majority of
	// noise on long Chromium pages comes from elements outside the viewport.
	//
	// Example, try this site: https://nix-darwin.github.io/nix-darwin/manual/
	elementRect := rectFromInfo(parent.info)
	if !elementRect.Overlaps(windowBounds) && opts.isChromiumOrElectron {
		if opts.stats != nil {
			opts.stats.outOfBoundsSkipped.Add(1)
		}

		if ce := opts.logger.Check(
			zap.DebugLevel,
			"Out-of-bounds element, skipping subtree",
		); ce != nil {
			ce.Write(
				zap.Int("depth", depth),
				zap.String("parent_role", parent.info.Role()),
				zap.String("parent_title", parent.info.Title()),
			)
		}

		return
	}

	// Don't traverse deeper into interactive leaf elements,
	// unless they have important container children (e.g., popovers, sheets, menus).
	// This handles cases like a toolbar button that opens a popover.
	var children []*Element
	if interactiveLeafRoles[element.Role(parent.info.Role())] {
		if !leafRolesWithImportantChildren[element.Role(parent.info.Role())] {
			if opts.stats != nil {
				opts.stats.stoppedAtLeaf.Add(1)
			}

			return
		}

		var childrenErr error
		children, childrenErr = parent.element.Children(parent.info.Role())
		hasImportantContainer := false
		if childrenErr == nil && len(children) > 0 {
			for _, child := range children {
				childInfo, infoErr := child.Info()
				if infoErr != nil {
					continue
				}
				if childInfo != nil && importantContainerRoles[element.Role(childInfo.Role())] {
					hasImportantContainer = true

					break
				}
			}
		}
		if !hasImportantContainer {
			for _, child := range children {
				child.Release()
			}
			if opts.stats != nil {
				switch {
				case childrenErr != nil:
					opts.stats.childrenErrors.Add(1)
				case len(children) == 0:
					opts.stats.noChildren.Add(1)
				default:
					opts.stats.stoppedAtLeaf.Add(1)
				}
			}

			return
		}
		// Reuse children slice for traversal below, skip the second Children() call.
	} else {
		var err error
		children, err = parent.element.Children(parent.info.Role())
		if err != nil || len(children) == 0 {
			if opts.stats != nil {
				if err != nil {
					opts.stats.childrenErrors.Add(1)
				} else {
					opts.stats.noChildren.Add(1)
				}
			}

			return
		}
	}

	// Process children sequentially to avoid goroutine/thread explosion
	// from nested parallel tree building. Concurrency is controlled at
	// the ClickableElements level (maxConcurrentWindows = 4).
	buildChildrenSequential(parent, children, depth, opts, clipBounds, windowBounds)
}

func buildChildrenSequential(
	parent *TreeNode,
	children []*Element,
	depth int,
	opts TreeOptions,
	clipBounds image.Rectangle,
	windowBounds image.Rectangle,
) {
	// First pass: count valid children and collect their info
	type childData struct {
		element *Element
		info    *ElementInfo
	}
	validChildren := make([]childData, 0, len(children))

	for _, child := range children {
		info, err := child.Info()
		if err != nil {
			if opts.stats != nil {
				opts.stats.childErrors.Add(1)
			}
			child.Release()

			continue
		}

		if !shouldIncludeElement(info, opts, clipBounds) {
			if opts.stats != nil {
				opts.stats.filteredOut.Add(1)
			}
			child.Release()

			continue
		}

		validChildren = append(validChildren, childData{element: child, info: info})
	}

	// Reuse the pooled backing array when it has enough room.
	if cap(parent.children) >= len(validChildren) {
		parent.children = parent.children[:0]
	} else {
		parent.children = make([]*TreeNode, 0, len(validChildren))
	}

	// Second pass: create nodes and recurse
	for _, data := range validChildren {
		// Skip hit-test for elements within the original window bounds
		// (not in a scroll area). These elements are guaranteed visible
		// by the AX tree — only scroll areas can have off-screen children.
		// Must be set before getTreeNode to avoid relying on pointer aliasing.
		data.info.skipHitTest = clipBounds == windowBounds

		childNode := getTreeNode(data.element, data.info, parent, 0)

		parent.children = append(parent.children, childNode)

		// Notify the streaming callback, if set, so that clickable elements
		// found during tree building can be consumed before the full tree
		// is built (true fluid streaming).
		if opts.onNode != nil {
			opts.onNode(childNode)
		}

		newClipBounds := clipBounds
		if element.Role(data.info.Role()) == element.RoleScrollArea {
			childRect := rectFromInfo(data.info)
			if childRect.Dx() > 0 && childRect.Dy() > 0 {
				newClipBounds = childRect.Intersect(clipBounds)
				if ce := opts.Logger().
					Check(zap.DebugLevel, "Scroll area detected, tightening clip bounds"); ce != nil {
					ce.Write(
						zap.String("role", data.info.Role()),
						zap.Int("clip_x", newClipBounds.Min.X),
						zap.Int("clip_y", newClipBounds.Min.Y),
						zap.Int("clip_w", newClipBounds.Dx()),
						zap.Int("clip_h", newClipBounds.Dy()),
					)
				}
			}
		}

		buildTreeRecursive(childNode, depth+1, opts, newClipBounds, windowBounds)
	}

	if opts.stats != nil {
		opts.stats.sequentialBatches.Add(1)
	}
}

// minElementSize is the minimum size threshold for elements.
// Elements smaller than this are filtered out as noise (especially in Chromium DOM trees).
// This matches similar filtering in tools like Glyphlow.
const minElementSize = 15

// shouldIncludeElement combines all filtering logic into one function.
//
// The clip-bounds overlap check is always applied so that ALL tree sources —
// including supplementary ones (dock, menubar, PIP, etc.) — benefit from
// spatial filtering against their respective clip bounds. Supplementary sources
// historically passed includeOutOfBounds=true to bypass this check, which let
// in off-screen elements from those sources.
func shouldIncludeElement(
	info *ElementInfo,
	opts TreeOptions,
	clipBounds image.Rectangle,
) bool {
	elementRect := rectFromInfo(info)

	// Clip bounds check: always filter elements completely outside the
	// visible area. This handles both window-level clipping (main window
	// tree) and scroll-container clipping (AXScrollArea viewport), as well
	// as screen-level clipping for supplementary sources where clip bounds
	// fall back to the active screen bounds when the root rect is empty.
	if elementRect.Dx() > 0 && elementRect.Dy() > 0 {
		if !elementRect.Overlaps(clipBounds) {
			if ce := opts.Logger().
				Check(zap.DebugLevel, "Element filtered by clip bounds"); ce != nil {
				ce.Write(
					zap.String("role", info.Role()),
					zap.Int("elem_x", elementRect.Min.X),
					zap.Int("elem_y", elementRect.Min.Y),
					zap.Int("elem_w", elementRect.Dx()),
					zap.Int("elem_h", elementRect.Dy()),
					zap.Int("clip_x", clipBounds.Min.X),
					zap.Int("clip_y", clipBounds.Min.Y),
					zap.Int("clip_w", clipBounds.Dx()),
					zap.Int("clip_h", clipBounds.Dy()),
				)
			}

			return false
		}
	}

	// Filter out zero-sized interactive elements (they're broken/invalid)
	if elementRect.Dx() == 0 || elementRect.Dy() == 0 {
		if interactiveLeafRoles[element.Role(info.Role())] {
			return false
		}
	}

	// Strict filtering: auto-enabled for Chromium/Electron apps with noisy DOM trees
	// For native apps like Safari, we trust the semantic tree to not have noise
	if opts.isChromiumOrElectron && elementRect.Dx() > 0 &&
		elementRect.Dy() > 0 {
		// Filter out tiny elements that are likely noise in Chromium DOM trees.
		// This is especially important for web content where the DOM can have
		// thousands of tiny placeholder/structure elements.
		// Filter if either dimension is too small (not just both)
		if elementRect.Dx() < minElementSize || elementRect.Dy() < minElementSize {
			// Only filter if it's not a known important role
			if !interactiveLeafRoles[element.Role(info.Role())] {
				return false
			}
		}

		// Filter elements whose center is outside clip bounds (overflowing/scroll-clipped elements)
		// But keep interactive roles (buttons, links, etc.) even if at edges
		halfWidth := elementRect.Dx() / 2  //nolint:mnd
		halfHeight := elementRect.Dy() / 2 //nolint:mnd
		centerX := elementRect.Min.X + halfWidth
		centerY := elementRect.Min.Y + halfHeight
		if centerX < clipBounds.Min.X || centerX > clipBounds.Max.X ||
			centerY < clipBounds.Min.Y || centerY > clipBounds.Max.Y {
			if !interactiveLeafRoles[element.Role(info.Role())] {
				return false
			}
		}
	}

	if opts.filterFunc != nil && !opts.filterFunc(info) {
		return false
	}

	return true
}

// FindClickableElements finds all clickable elements in the tree.
// Search text is already accumulated during tree building via accumulateSearchText.
func (n *TreeNode) FindClickableElements(
	allowedRoles map[string]struct{},
	configProvider config.Provider,
	ignoreClickableCheck bool,
) []*TreeNode {
	var result []*TreeNode
	n.walkTree(func(node *TreeNode) bool {
		if !node.element.IsClickable(
			node.info,
			allowedRoles,
			configProvider,
			ignoreClickableCheck,
		) {
			return true
		}

		rect := rectFromInfo(node.info)
		if rect.Dx() == 0 || rect.Dy() == 0 {
			return true
		}

		result = append(result, node)

		return true
	})

	return result
}

func appendSearchText(builder *strings.Builder, seen map[string]struct{}, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if _, ok := seen[text]; ok {
		return
	}

	if builder.Len() > 0 {
		builder.WriteByte(' ')
	}
	builder.WriteString(text)
	seen[text] = struct{}{}
}

// accumulateSearchText performs a single post-order walk over the tree to
// collect text from each node's subtree. This replaces the previous approach
// where collectSearchText was called separately for each clickable element,
// which resulted in O(n*m) descendant walks.
func accumulateSearchText(root *TreeNode) {
	root.walkTreePostOrder(func(node *TreeNode) {
		if node.info == nil {
			return
		}

		hasAny := false
		if strings.TrimSpace(node.info.Title()) != "" ||
			strings.TrimSpace(node.info.Description()) != "" ||
			strings.TrimSpace(node.info.Value()) != "" {
			hasAny = true
		} else {
			for _, child := range node.children {
				if child.info != nil && child.info.searchText != "" {
					hasAny = true

					break
				}
			}
		}
		if !hasAny {
			node.info.searchText = ""

			return
		}

		var builder strings.Builder
		seen := make(map[string]struct{})

		// Collect text from this node itself
		appendSearchText(&builder, seen, node.info.Title())
		appendSearchText(&builder, seen, node.info.Description())
		appendSearchText(&builder, seen, node.info.Value())

		// Merge text from children (already computed since this is post-order)
		for _, child := range node.children {
			if child.info != nil && child.info.searchText != "" {
				appendSearchText(&builder, seen, child.info.searchText)
			}
		}

		node.info.searchText = builder.String()
	})
}

// Release releases the AXUIElementRef for every node in the subtree except
// those whose elements appear in the provided keep set. Nodes in the keep set
// are left untouched so callers can continue using them. The root element is
// always skipped because it is owned by the caller (e.g., the frontmost window).
//
// All non-root, non-kept TreeNode structs are returned to treeNodePool for
// reuse. Kept nodes only have their tree pointers cleared. The root node is
// never pooled — it is owned by the caller and must remain valid after Release
// so that callers can still safely (if accidentally) read Element()/Info().
// A post-order walk is used so that children are processed before their parent,
// which is required because putTreeNode clears the children slice.
func (n *TreeNode) Release(keep map[*Element]struct{}) {
	n.walkTreePostOrder(func(node *TreeNode) {
		if node == n {
			// Root element is owned by the caller — do not release it.
			// Never pool the root node: the caller holds a reference to it
			// (the `tree` variable) and may still read Element()/Info().
			// Only clear tree pointers to avoid dangling references.
			node.children = nil
			node.parent = nil

			return
		}
		if node.element == nil {
			putTreeNode(node)

			return
		}
		if _, kept := keep[node.element]; kept {
			// Clear tree pointers so the kept node does not retain
			// references to recycled pool nodes. InfraNode only uses
			// Element() and Info(), so children/parent are not needed.
			node.children = nil
			node.parent = nil

			return
		}
		node.element.Release()

		putTreeNode(node)
	})
}

// walkTree walks the tree in pre-order and calls the visitor function for each node.
func (n *TreeNode) walkTree(visit func(*TreeNode) bool) {
	if !visit(n) {
		return
	}

	for _, child := range n.children {
		child.walkTree(visit)
	}
}

// walkTreePostOrder walks the tree in post-order (children before parent).
// This is safe for operations that modify or recycle the visited node because
// all descendants have already been visited by the time the parent callback runs.
func (n *TreeNode) walkTreePostOrder(visit func(*TreeNode)) {
	for _, child := range n.children {
		child.walkTreePostOrder(visit)
	}
	visit(n)
}

// isChromiumOrElectron returns true if the bundle ID matches known Chromium/Electron apps
// or user-configured additional bundles.
func isChromiumOrElectron(bundleID string, configProvider config.Provider) bool {
	if isLikelyChromiumOrElectron(bundleID) {
		return true
	}

	return isUserConfiguredChromiumElectron(bundleID, configProvider)
}

var (
	knownChromiumMap map[string]struct{}
	knownElectronMap map[string]struct{}
	initKnownOnce    sync.Once
)

func initKnownMaps() {
	knownChromiumMap = make(map[string]struct{}, len(config.KnownChromiumBundles))
	for _, b := range config.KnownChromiumBundles {
		knownChromiumMap[strings.ToLower(strings.TrimSpace(b))] = struct{}{}
	}
	knownElectronMap = make(map[string]struct{}, len(config.KnownElectronBundles))
	for _, b := range config.KnownElectronBundles {
		knownElectronMap[strings.ToLower(strings.TrimSpace(b))] = struct{}{}
	}
}

// isLikelyChromiumOrElectron returns true if the bundle ID matches known Chromium/Electron apps.
// This is duplicated from electron package to avoid import cycle.
func isLikelyChromiumOrElectron(bundleID string) bool {
	if bundleID == "" {
		return false
	}
	initKnownOnce.Do(initKnownMaps)

	bundleID = strings.ToLower(strings.TrimSpace(bundleID))
	if _, ok := knownChromiumMap[bundleID]; ok {
		return true
	}
	if _, ok := knownElectronMap[bundleID]; ok {
		return true
	}

	return false
}

// isUserConfiguredChromiumElectron checks if the bundle ID matches user-configured
// additional Chromium/Electron bundles from config. Supports exact matches and
// wildcard patterns (ending with *).
func isUserConfiguredChromiumElectron(bundleID string, configProvider config.Provider) bool {
	if bundleID == "" || configProvider == nil {
		return false
	}

	cfg := configProvider.Get()
	if cfg == nil {
		return false
	}

	chromiumBundles := cfg.Hints.AdditionalAXSupport.AdditionalChromiumBundles
	if config.MatchesAdditionalBundle(bundleID, chromiumBundles) {
		return true
	}

	electronBundles := cfg.Hints.AdditionalAXSupport.AdditionalElectronBundles

	return config.MatchesAdditionalBundle(bundleID, electronBundles)
}
