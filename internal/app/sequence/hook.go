package sequence

import "context"

// hookKey types the context value that marks a sequence as a hook's.
type hookKey struct{}

// WithHook returns a context marking the sequence run under it as a hook's,
// with env added to the environment of every exec step in it, nested
// sequences included.
//
// A hook's exec steps run while Neru is stopped, because the hooks that fire
// then exist to report the pause, and the hook runner decides which hooks may
// run at all.
func WithHook(ctx context.Context, env []string) context.Context {
	return context.WithValue(ctx, hookKey{}, env)
}

// hookEnv returns the environment a hook added, and whether ctx is a hook's.
func hookEnv(ctx context.Context) ([]string, bool) {
	env, ok := ctx.Value(hookKey{}).([]string)

	return env, ok
}
