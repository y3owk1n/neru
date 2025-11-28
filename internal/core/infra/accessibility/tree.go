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

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"go.uber.org/zap"
)

const (
	// DefaultParallelThreshold is the default threshold for parallel processing.
	DefaultParallelThreshold = 100

	// DefaultMaxParallelDepth is the default max depth for parallel recursion.
	DefaultMaxParallelDepth = 4

	// DefaultChildrenCapacity is the default capacity for children.
	DefaultChildrenCapacity = 8
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

// DefaultTreeOptions returns default tree traversal options.
func DefaultTreeOptions() TreeOptions {
	return TreeOptions{
		filterFunc:         nil,
		includeOutOfBounds: false,
		cache:              NewInfoCache(DefaultAccessibilityCacheTTL),
		parallelThreshold:  DefaultParallelThreshold,
		maxParallelDepth:   DefaultMaxParallelDepth,
	}
}

// BuildTree constructs an accessibility tree starting from the specified root element.
func BuildTree(root *Element, opts TreeOptions) (*TreeNode, error) {
	if root == nil {
		logger.Debug("BuildTree called with nil root element")

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
			logger.Warn("Failed to get root element info", zap.Error(infoErr))

			return nil, infoErr
		}
		opts.cache.Set(root, info)
	}

	if info == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "root element info is nil")
	}

	logger.Debug("Building tree from root element",
		zap.String("role", info.Role()),
		zap.String("title", info.Title()),
		zap.Int("pid", info.PID()))

	// Calculate window bounds for spatial filtering
	windowBounds := rectFromInfo(info)

	node := &TreeNode{
		element: root,
		info:    info,
		children: make(
			[]*TreeNode,
			0,
			DefaultChildrenCapacity,
		), // Pre-allocate for typical children count
	}

	buildTreeRecursive(node, 1, opts, windowBounds)

	logger.Debug("Tree building completed",
		zap.String("root_role", info.Role()),
		zap.String("root_title", info.Title()))

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
	// Early exit for roles that can't have interactive children
	if nonInteractiveRoles[parent.info.Role()] {
		logger.Debug("Skipping non-interactive role",
			zap.String("role", parent.info.Role()),
			zap.Int("depth", depth))

		return
	}

	// Don't traverse deeper into interactive leaf elements
	if interactiveLeafRoles[parent.info.Role()] {
		logger.Debug("Stopping at interactive leaf role",
			zap.String("role", parent.info.Role()),
			zap.Int("depth", depth))

		return
	}

	children, err := parent.element.Children()
	if err != nil || len(children) == 0 {
		if err != nil {
			logger.Debug("No children found due to error",
				zap.String("role", parent.info.Role()),
				zap.Error(err),
				zap.Int("depth", depth))
		} else {
			logger.Debug("No children found",
				zap.String("role", parent.info.Role()),
				zap.Int("depth", depth))
		}

		return
	}

	// Decide whether to parallelize
	shouldParallelize := depth <= opts.maxParallelDepth &&
		len(children) >= opts.parallelThreshold

	logger.Debug("Processing children",
		zap.String("parent_role", parent.info.Role()),
		zap.Int("child_count", len(children)),
		zap.Int("depth", depth),
		zap.Bool("parallel", shouldParallelize))

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
				logger.Debug("Failed to get child element info", zap.Error(err))

				continue
			}
			opts.cache.Set(child, info)
		}

		if !shouldIncludeElement(info, opts, windowBounds) {
			logger.Debug("Skipping child element (filtered out)",
				zap.String("role", info.Role()),
				zap.String("title", info.Title()))

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

	logger.Debug("Sequential child processing completed",
		zap.String("parent_role", parent.info.Role()),
		zap.Int("processed_children", len(validChildren)),
		zap.Int("total_children", len(children)))
}

func buildChildrenParallel(
	parent *TreeNode,
	children []*Element,
	depth int,
	opts TreeOptions,
	windowBounds image.Rectangle,
) {
	logger.Debug("Starting parallel child processing",
		zap.String("parent_role", parent.info.Role()),
		zap.Int("child_count", len(children)),
		zap.Int("depth", depth))

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
					logger.Debug(
						"Failed to get child element info in parallel processing",
						zap.Error(err),
					)
					results <- childResult{node: nil, index: idx, err: err}

					return
				}
				opts.cache.Set(elem, info)
			}

			if !shouldIncludeElement(info, opts, windowBounds) {
				logger.Debug("Skipping child element in parallel processing (filtered out)",
					zap.String("role", info.Role()),
					zap.String("title", info.Title()))
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

	logger.Debug("Parallel child processing completed",
		zap.String("parent_role", parent.info.Role()),
		zap.Int("processed_children", validCount),
		zap.Int("total_children", len(children)))
}

// shouldIncludeElement combines all filtering logic into one function.
func shouldIncludeElement(info *ElementInfo, opts TreeOptions, windowBounds image.Rectangle) bool {
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
				logger.Debug("Element filtered out (no overlap)",
					zap.String("role", info.Role()),
					zap.String("title", info.Title()),
					zap.Any("element_rect", elementRect),
					zap.Any("window_bounds", windowBounds))

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
func (n *TreeNode) FindClickableElements() []*TreeNode {
	var result []*TreeNode
	n.walkTree(func(node *TreeNode) bool {
		if node.element.IsClickable(node.info) {
			result = append(result, node)
		}

		return true
	})

	return result
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
