package sequence

import "context"

// MaxDepth bounds how deeply one action sequence may invoke another.
// Sequences nest because a step can itself be a "run" command, so a binding
// that refers back to itself would otherwise recurse until the daemon runs out
// of stack.
const MaxDepth = 5

// depthKey types the context value that carries the current nesting depth.
// A struct key keeps the value private to this package.
type depthKey struct{}

// Depth reports how many action sequences are already running above ctx.
func Depth(ctx context.Context) int {
	if ctx == nil {
		return 0
	}

	depth, ok := ctx.Value(depthKey{}).(int)
	if !ok {
		return 0
	}

	return depth
}

// WithDepth returns a context carrying the given nesting depth.
func WithDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, depthKey{}, depth)
}
