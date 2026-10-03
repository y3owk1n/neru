package hint

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
)

// Router handles hint-related key routing and returns routing results.
type Router struct {
	*domain.Router

	manager *Manager
}

// RouteResult contains the result of routing a key press in hint mode.
type RouteResult struct {
	exactHint *Interface // The exact matched hint (domain hint)
	unmatched bool       // No hint matched the input prefix
}

// ExactHint returns the exact matched hint.
func (rr *RouteResult) ExactHint() *Interface {
	return rr.exactHint
}

// Unmatched returns whether no hint matched the input prefix.
func (rr *RouteResult) Unmatched() bool {
	return rr.unmatched
}

// NewRouter creates a new hint router with the specified manager and logger.
func NewRouter(manager *Manager, logger *zap.Logger) *Router {
	return &Router{
		Router:  domain.NewRouter(logger),
		manager: manager,
	}
}

// RouteKey processes a key press and returns the routing result.
func (r *Router) RouteKey(key string) (RouteResult, error) {
	hint, exactMatch, unmatched, err := r.manager.HandleInput(key)
	if err != nil {
		return RouteResult{}, err
	}

	if exactMatch {
		return RouteResult{
			exactHint: hint,
		}, nil
	}

	if unmatched {
		return RouteResult{
			unmatched: true,
		}, nil
	}

	// No exact match, continue in hint mode
	return RouteResult{}, nil
}
