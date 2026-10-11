package sequence

import "context"

// hookKey types the context value that marks a sequence as a hook's.
type hookKey struct{}

// envKey types the context value holding what a sequence adds to the
// environment of its exec steps.
type envKey struct{}

// WithHook returns a context marking the sequence run under it as a hook's,
// with env added to the environment of every exec step in it, nested
// sequences included.
//
// A hook's exec steps run while Neru is stopped, because the hooks that fire
// then exist to report the pause, and the hook runner decides which hooks may
// run at all.
func WithHook(ctx context.Context, env []string) context.Context {
	return WithEnv(context.WithValue(ctx, hookKey{}, true), env)
}

// WithEnv returns a context adding env to the environment of every exec step
// in the sequence run under it, nested sequences included. Unlike WithHook it
// lets no step run while Neru is stopped.
func WithEnv(ctx context.Context, env []string) context.Context {
	return context.WithValue(ctx, envKey{}, env)
}

// isHook reports whether ctx is a hook's.
func isHook(ctx context.Context) bool {
	hook, _ := ctx.Value(hookKey{}).(bool)

	return hook
}

// execEnv returns what ctx adds to an exec step's environment.
func execEnv(ctx context.Context) []string {
	env, _ := ctx.Value(envKey{}).([]string)

	return env
}
