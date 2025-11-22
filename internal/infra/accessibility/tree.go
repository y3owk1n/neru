package accessibility

/*
#cgo CFLAGS: -x objective-c
#include "../bridge/accessibility.h"
#include <stdlib.h>

*/
import "C"

import (
	"errors"
	"image"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
)

var (
	treeLogger = logger.Get()

	// Pre-allocated common errors.
	errRootElementNil = errors.New("root element is nil")
)

// TreeNode represents a node in the accessibility element hierarchy.
type TreeNode struct {
	Element  *Element
	Info     *ElementInfo
	Children []*TreeNode
	Parent   *TreeNode
}

// TreeOptions configures accessibility tree traversal behavior and filtering.
type TreeOptions struct {
	FilterFunc         func(*ElementInfo) bool
	IncludeOutOfBounds bool
	Cache              *InfoCache
	ParallelThreshold  int
	MaxParallelDepth   int
}

// DefaultTreeOptions returns the default configuration for accessibility tree traversal.
func DefaultTreeOptions() TreeOptions {
	return TreeOptions{
		FilterFunc:         nil,
		IncludeOutOfBounds: false,
		Cache:              NewInfoCache(5 * time.Second),
		ParallelThreshold:  8, // Only parallelize if there are more than 8 elements
		MaxParallelDepth:   4, // Don't parallelize deeper than 4 levels
	}
}

// BuildTree constructs an accessibility tree starting from the specified root element.
func BuildTree(root *Element, opts TreeOptions) (*TreeNode, error) {
	if root == nil {
		logger.Debug("BuildTree called with nil root element")
		return nil, errRootElementNil
	}

	// Try to get from cache first
	info := opts.Cache.Get(root)
	if info == nil {
		var err error
		info, err = root.GetInfo()
		if err != nil {
			logger.Warn("Failed to get root element info", zap.Error(err))
			return nil, err
		}
		opts.Cache.Set(root, info)
	}

	if info == nil {
		return nil, errors.New("root element info is nil")
	}

	logger.Debug("Building tree from root element",
		zap.String("role", info.Role),
		zap.String("title", info.Title),
		zap.Int("pid", info.PID))

	// Calculate window bounds for spatial filtering
	windowBounds := rectFromInfo(info)

	node := &TreeNode{
		Element: root,
		Info:    info,
	}

	buildTreeRecursive(node, 1, opts, windowBounds)

	logger.Debug("Tree building completed",
		zap.String("root_role", info.Role),
		zap.String("root_title", info.Title))

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
	if nonInteractiveRoles[parent.Info.Role] {
		logger.Debug("Skipping non-interactive role",
			zap.String("role", parent.Info.Role),
			zap.Int("depth", depth))
		return
	}

	// Don't traverse deeper into interactive leaf elements
	if interactiveLeafRoles[parent.Info.Role] {
		logger.Debug("Stopping at interactive leaf role",
			zap.String("role", parent.Info.Role),
			zap.Int("depth", depth))
		return
	}

	children, err := parent.Element.GetChildren()
	if err != nil || len(children) == 0 {
		if err != nil {
			logger.Debug("No children found due to error",
				zap.String("role", parent.Info.Role),
				zap.Error(err),
				zap.Int("depth", depth))
		} else {
			logger.Debug("No children found",
				zap.String("role", parent.Info.Role),
				zap.Int("depth", depth))
		}
		return
	}

	// Decide whether to parallelize
	shouldParallelize := depth <= opts.MaxParallelDepth &&
		len(children) >= opts.ParallelThreshold

	logger.Debug("Processing children",
		zap.String("parent_role", parent.Info.Role),
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
		info := opts.Cache.Get(child)
		if info == nil {
			var err error
			info, err = child.GetInfo()
			if err != nil {
				logger.Debug("Failed to get child element info", zap.Error(err))
				continue
			}
			opts.Cache.Set(child, info)
		}

		if !shouldIncludeElement(info, opts, windowBounds) {
			logger.Debug("Skipping child element (filtered out)",
				zap.String("role", info.Role),
				zap.String("title", info.Title))
			continue
		}

		validChildren = append(validChildren, childData{element: child, info: info})
	}

	// Pre-allocate with exact capacity
	parent.Children = make([]*TreeNode, 0, len(validChildren))

	// Second pass: create nodes and recurse
	for _, data := range validChildren {
		childNode := &TreeNode{
			Element:  data.element,
			Info:     data.info,
			Parent:   parent,
			Children: []*TreeNode{},
		}

		parent.Children = append(parent.Children, childNode)
		buildTreeRecursive(childNode, depth+1, opts, windowBounds)
	}

	logger.Debug("Sequential child processing completed",
		zap.String("parent_role", parent.Info.Role),
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
		zap.String("parent_role", parent.Info.Role),
		zap.Int("child_count", len(children)),
		zap.Int("depth", depth))

	// Pre-allocate result slice with exact capacity
	type childResult struct {
		node  *TreeNode
		index int
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
			info := opts.Cache.Get(elem)
			if info == nil {
				var err error
				info, err = elem.GetInfo()
				if err != nil {
					logger.Debug(
						"Failed to get child element info in parallel processing",
						zap.Error(err),
					)
					return
				}
				opts.Cache.Set(elem, info)
			}

			if !shouldIncludeElement(info, opts, windowBounds) {
				logger.Debug("Skipping child element in parallel processing (filtered out)",
					zap.String("role", info.Role),
					zap.String("title", info.Title))
				return
			}

			childNode := &TreeNode{
				Element:  elem,
				Info:     info,
				Parent:   parent,
				Children: []*TreeNode{},
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
		collected[result.index] = result.node
		validCount++
	}

	// Pre-allocate final children slice with exact valid count
	parent.Children = make([]*TreeNode, 0, validCount)
	for _, node := range collected {
		if node != nil {
			parent.Children = append(parent.Children, node)
		}
	}

	logger.Debug("Parallel child processing completed",
		zap.String("parent_role", parent.Info.Role),
		zap.Int("processed_children", validCount),
		zap.Int("total_children", len(children)))
}

// shouldIncludeElement combines all filtering logic into one function.

// shouldIncludeElement combines all filtering logic into one function.
func shouldIncludeElement(info *ElementInfo, opts TreeOptions, windowBounds image.Rectangle) bool {
	if !opts.IncludeOutOfBounds {
		elementRect := rectFromInfo(info)

		// Filter out zero-sized interactive elements (they're broken/invalid)
		if elementRect.Dx() == 0 || elementRect.Dy() == 0 {
			if interactiveLeafRoles[info.Role] {
				return false
			}
		}

		// For non-zero sized elements, check if they overlap with window bounds
		if elementRect.Dx() > 0 && elementRect.Dy() > 0 {
			if !elementRect.Overlaps(windowBounds) {
				logger.Debug("Element filtered out (no overlap)",
					zap.String("role", info.Role),
					zap.String("title", info.Title),
					zap.Any("element_rect", elementRect),
					zap.Any("window_bounds", windowBounds))
				return false
			}
		}
	}

	if opts.FilterFunc != nil && !opts.FilterFunc(info) {
		return false
	}

	return true
}

// FindClickableElements finds all clickable elements in the tree.
func (n *TreeNode) FindClickableElements() []*TreeNode {
	var result []*TreeNode
	n.walkTree(func(node *TreeNode) bool {
		if node.Element.IsClickable(node.Info) {
			result = append(result, node)
		}
		return true
	})
	return result
}

// FindScrollableElements finds all scrollable elements in the tree

// walkTree walks the tree and calls the visitor function for each node.
func (n *TreeNode) walkTree(visitor func(*TreeNode) bool) {
	if !visitor(n) {
		return
	}

	for _, child := range n.Children {
		child.walkTree(visitor)
	}
}
