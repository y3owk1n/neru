package accessibility

/*
#cgo CFLAGS: -x objective-c
#include "../bridge/accessibility.h"
#include <stdlib.h>

*/
import "C"

import (
	"image"
	"sync"
	"sync/atomic"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

// Pre-allocated common errors.
var errRootElementNil = derrors.New(derrors.CodeAccessibilityFailed, "root element is nil")

// TreeNode represents a node in the accessibility element hierarchy.
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
	filterFunc         func(*ElementInfo) bool
	includeOutOfBounds bool
	cache              *InfoCache
	parallelThreshold  int
	maxParallelDepth   int
	maxDepth           int
	logger             *zap.Logger
	stats              *treeStats
}

// FilterFunc returns the filter function.
func (o *TreeOptions) FilterFunc() func(*ElementInfo) bool {
	return o.filterFunc
}

// IncludeOutOfBounds returns whether to include out of bounds elements.
func (o *TreeOptions) IncludeOutOfBounds() bool {
	return o.includeOutOfBounds
}

// Cache returns the info cache.
func (o *TreeOptions) Cache() *InfoCache {
	return o.cache
}

// ParallelThreshold returns the parallel threshold.
func (o *TreeOptions) ParallelThreshold() int {
	return o.parallelThreshold
}

// MaxParallelDepth returns the max parallel depth.
func (o *TreeOptions) MaxParallelDepth() int {
	return o.maxParallelDepth
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

// SetFilterFunc sets the filter function.
func (o *TreeOptions) SetFilterFunc(fn func(*ElementInfo) bool) {
	o.filterFunc = fn
}

// SetIncludeOutOfBounds sets whether to include out of bounds elements.
func (o *TreeOptions) SetIncludeOutOfBounds(include bool) {
	o.includeOutOfBounds = include
}

// SetCache sets the info cache.
func (o *TreeOptions) SetCache(cache *InfoCache) {
	o.cache = cache
}

// SetMaxDepth sets the max depth for tree traversal.
func (o *TreeOptions) SetMaxDepth(depth int) {
	o.maxDepth = depth
}

// SetParallelThreshold sets the threshold for parallel child processing.
func (o *TreeOptions) SetParallelThreshold(threshold int) {
	o.parallelThreshold = threshold
}

// DefaultTreeOptions returns default tree traversal options.
// Note: cache is nil by default; callers must set it via SetCache before
// passing opts to BuildTree (which requires a non-nil cache).
func DefaultTreeOptions(logger *zap.Logger) TreeOptions {
	return TreeOptions{
		filterFunc:         nil,
		includeOutOfBounds: false,
		cache:              nil,
		parallelThreshold:  config.DefaultParallelThreshold,
		maxParallelDepth:   config.DefaultMaxParallelDepth,
		maxDepth:           config.DefaultMaxDepth,
		logger:             logger,
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
	parallelBatches       atomic.Int64
	sequentialBatches     atomic.Int64
	maxDepthSeen          atomic.Int64
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

	// Ensure a cache is always available to avoid nil dereferences in traversal.
	if opts.cache == nil {
		return nil, derrors.New(
			derrors.CodeAccessibilityFailed,
			"opts.cache must not be nil; use DefaultTreeOptions()",
		)
	}

	// Try to get from cache first
	info := opts.cache.Get(root)
	if info == nil {
		var infoErr error
		info, infoErr = root.Info()
		if infoErr != nil {
			opts.Logger().Warn("Failed to get root element info", zap.Error(infoErr))

			return nil, infoErr
		}
		opts.cache.Set(root, info)
	}

	if info == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "root element info is nil")
	}

	stats := &treeStats{}

	// Calculate window bounds for spatial filtering
	windowBounds := rectFromInfo(info)

	node := &TreeNode{
		element: root,
		info:    info,
		children: make(
			[]*TreeNode,
			0,
			config.DefaultChildrenCapacity,
		), // Pre-allocate for typical children count
	}

	opts.stats = stats
	buildTreeRecursive(node, 1, opts, windowBounds)

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
			zap.Int64("parallel_batches", stats.parallelBatches.Load()),
			zap.Int64("sequential_batches", stats.sequentialBatches.Load()),
			zap.Int64("max_depth_seen", stats.maxDepthSeen.Load()),
		)
	}

	return node, nil
}

// Roles that typically don't contain interactive elements.
var nonInteractiveRoles = map[string]bool{
	"AXStaticText": true,
	"AXImage":      true,
	"AXHeading":    true,
}

// Roles that are themselves interactive (leaf nodes).
var interactiveLeafRoles = map[string]bool{
	"AXButton":             true,
	"AXComboBox":           true,
	"AXCheckBox":           true,
	"AXRadioButton":        true,
	"AXLink":               true,
	"AXPopUpButton":        true,
	"AXTextField":          true,
	"AXSlider":             true,
	"AXTabButton":          true,
	"AXSwitch":             true,
	"AXDisclosureTriangle": true,
	"AXTextArea":           true,
	"AXMenuButton":         true,
	"AXMenuItem":           true,
}

func buildTreeRecursive(
	parent *TreeNode,
	depth int,
	opts TreeOptions,
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
	if nonInteractiveRoles[parent.info.Role()] {
		if opts.stats != nil {
			opts.stats.skippedNonInteractive.Add(1)
		}

		return
	}

	// Don't traverse deeper into interactive leaf elements
	if interactiveLeafRoles[parent.info.Role()] {
		if opts.stats != nil {
			opts.stats.stoppedAtLeaf.Add(1)
		}

		return
	}

	children, err := parent.element.Children(opts.cache)
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

	// Decide whether to parallelize
	shouldParallelize := depth <= opts.maxParallelDepth &&
		len(children) >= opts.parallelThreshold

	if shouldParallelize {
		buildChildrenParallel(parent, children, depth, opts, windowBounds)
	} else {
		buildChildrenSequential(parent, children, depth, opts, windowBounds)
	}
}

func buildChildrenSequential(
	parent *TreeNode,
	children []*Element,
	depth int,
	opts TreeOptions,
	windowBounds image.Rectangle,
) {
	// First pass: count valid children and collect their info
	type childData struct {
		element *Element
		info    *ElementInfo
	}
	validChildren := make([]childData, 0, len(children))

	for _, child := range children {
		// Try cache first
		info := opts.cache.Get(child)
		if info == nil {
			var err error
			info, err = child.Info()
			if err != nil {
				if opts.stats != nil {
					opts.stats.childErrors.Add(1)
				}
				child.Release()

				continue
			}
			opts.cache.Set(child, info)
		}

		if !shouldIncludeElement(info, opts, windowBounds) {
			if opts.stats != nil {
				opts.stats.filteredOut.Add(1)
			}
			child.Release()

			continue
		}

		validChildren = append(validChildren, childData{element: child, info: info})
	}

	// Pre-allocate with exact capacity
	parent.children = make([]*TreeNode, 0, len(validChildren))

	// Second pass: create nodes and recurse
	for _, data := range validChildren {
		childNode := &TreeNode{
			element:  data.element,
			info:     data.info,
			parent:   parent,
			children: []*TreeNode{},
		}

		parent.children = append(parent.children, childNode)
		buildTreeRecursive(childNode, depth+1, opts, windowBounds)
	}

	if opts.stats != nil {
		opts.stats.sequentialBatches.Add(1)
	}
}

func buildChildrenParallel(
	parent *TreeNode,
	children []*Element,
	depth int,
	opts TreeOptions,
	windowBounds image.Rectangle,
) {
	// Pre-allocate result slice with exact capacity
	type childResult struct {
		node  *TreeNode
		index int
		err   error
	}

	// Use buffered channel sized to number of children
	results := make(chan childResult, len(children))
	var waitGroup sync.WaitGroup

	// Process children in parallel
	for index, child := range children {
		waitGroup.Add(1)
		go func(idx int, elem *Element) {
			defer waitGroup.Done()

			// Try cache first (cache must be thread-safe!)
			info := opts.cache.Get(elem)
			if info == nil {
				var err error
				info, err = elem.Info()
				if err != nil {
					if opts.stats != nil {
						opts.stats.childErrors.Add(1)
					}

					elem.Release()

					results <- childResult{node: nil, index: idx, err: err}

					return
				}
				opts.cache.Set(elem, info)
			}

			if !shouldIncludeElement(info, opts, windowBounds) {
				if opts.stats != nil {
					opts.stats.filteredOut.Add(1)
				}

				elem.Release()

				results <- childResult{node: nil, index: idx}

				return
			}

			childNode := &TreeNode{
				element:  elem,
				info:     info,
				parent:   parent,
				children: []*TreeNode{},
			}

			// Recursively build (this may spawn more goroutines at deeper levels)
			buildTreeRecursive(childNode, depth+1, opts, windowBounds)

			results <- childResult{node: childNode, index: idx}
		}(index, child)
	}

	// Close results channel when all goroutines complete
	go func() {
		waitGroup.Wait()
		close(results)
	}()

	// Pre-allocate collection slice with exact capacity
	collected := make([]*TreeNode, len(children))
	validCount := 0

	for result := range results {
		if result.node != nil {
			collected[result.index] = result.node
			validCount++
		}
	}

	// Pre-allocate final children slice with exact valid count
	parent.children = make([]*TreeNode, 0, validCount)
	for _, node := range collected {
		if node != nil {
			parent.children = append(parent.children, node)
		}
	}

	if opts.stats != nil {
		opts.stats.parallelBatches.Add(1)
	}
}

// shouldIncludeElement combines all filtering logic into one function.
func shouldIncludeElement(
	info *ElementInfo,
	opts TreeOptions,
	windowBounds image.Rectangle,
) bool {
	if !opts.includeOutOfBounds {
		elementRect := rectFromInfo(info)

		// Filter out zero-sized interactive elements (they're broken/invalid)
		if elementRect.Dx() == 0 || elementRect.Dy() == 0 {
			if interactiveLeafRoles[info.Role()] {
				return false
			}
		}

		// For non-zero sized elements, check if they overlap with window bounds
		if elementRect.Dx() > 0 && elementRect.Dy() > 0 {
			if !elementRect.Overlaps(windowBounds) {
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
func (n *TreeNode) FindClickableElements(
	allowedRoles map[string]struct{},
	cache *InfoCache,
) []*TreeNode {
	var result []*TreeNode
	n.walkTree(func(node *TreeNode) bool {
		if node.element.IsClickable(node.info, allowedRoles, cache) {
			result = append(result, node)
		}

		return true
	})

	return result
}

// Release releases the AXUIElementRef for every node in the subtree except
// those whose elements appear in the provided keep set. Nodes in the keep set
// are left untouched so callers can continue using them. The root element is
// always skipped because it is owned by the caller (e.g., the frontmost window).
func (n *TreeNode) Release(keep map[*Element]struct{}) {
	n.walkTree(func(node *TreeNode) bool {
		if node == n {
			// Skip root — owned by the caller.
			return true
		}
		if node.element == nil {
			return true
		}
		if _, kept := keep[node.element]; kept {
			return true
		}
		node.element.Release()

		return true
	})
}

// walkTree walks the tree and calls the visitor function for each node.
func (n *TreeNode) walkTree(visit func(*TreeNode) bool) {
	if !visit(n) {
		return
	}

	for _, child := range n.children {
		child.walkTree(visit)
	}
}
