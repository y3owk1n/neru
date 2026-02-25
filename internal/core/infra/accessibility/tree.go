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

	node := getTreeNode(root, info, nil, config.DefaultChildrenCapacity)

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

	// Reuse the pooled backing array when it has enough room.
	if cap(parent.children) >= len(validChildren) {
		parent.children = parent.children[:0]
	} else {
		parent.children = make([]*TreeNode, 0, len(validChildren))
	}

	// Second pass: create nodes and recurse
	for _, data := range validChildren {
		childNode := getTreeNode(data.element, data.info, parent, 0)

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

			childNode := getTreeNode(elem, info, parent, 0)

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

	// Reuse the pooled backing array when it has enough room.
	if cap(parent.children) >= validCount {
		parent.children = parent.children[:0]
	} else {
		parent.children = make([]*TreeNode, 0, validCount)
	}

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
