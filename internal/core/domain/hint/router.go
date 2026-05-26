package hint

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
)

// Router handles hint-related key routing and returns routing results.
type Router struct {
	*domain.Router

	manager *Manager
}

// RouteResult contains the result of routing a key press in hint mode.
type RouteResult struct {
	exactHint *Interface // The exact matched hint (domain hint)
}

// ExactHint returns the exact matched hint.
func (rr *RouteResult) ExactHint() *Interface {
	return rr.exactHint
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
	// Process input through manager
	hint, exactMatch, err := r.manager.HandleInput(key)
	if err != nil {
		return RouteResult{}, err
	}

	if exactMatch {
		if r.Logger != nil {
			r.Logger.Debug("Hints router: Exact hint match found",
				zap.String("label", hint.Label()))
		}

		return RouteResult{
			exactHint: hint,
		}, nil
	}

	// No exact match, continue in hint mode
	return RouteResult{}, nil
}
