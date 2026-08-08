package fontcache

import "sync"

// Resolver answers what family a written font family name resolves to,
// remembering each answer so the platform's font system is asked once per
// name. It is safe for concurrent use.
type Resolver struct {
	mu      sync.RWMutex
	entries map[string]string
	resolve func(family string) string
}

// New returns a Resolver that remembers what resolve answers.
func New(resolve func(family string) string) *Resolver {
	return &Resolver{
		entries: make(map[string]string),
		resolve: resolve,
	}
}

// Resolve returns the family that family resolves to, asking the wrapped
// resolution on the first request for that name and remembering its answer for
// later ones.
//
// A name is remembered exactly as it was written, so a caller is always
// answered from its own spelling: resolving "Arial" and then "ARIAL" resolves
// both, rather than answering the second from the first one's entry.
func (r *Resolver) Resolve(family string) string {
	r.mu.RLock()
	cached, ok := r.entries[family]
	r.mu.RUnlock()

	if ok {
		return cached
	}

	resolved := r.resolve(family)

	r.mu.Lock()
	r.entries[family] = resolved
	r.mu.Unlock()

	return resolved
}
