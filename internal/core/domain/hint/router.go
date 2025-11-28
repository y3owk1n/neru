package hint

import "go.uber.org/zap"

// Router handles hint-related key routing and returns routing results.
type Router struct {
	manager *Manager
	logger  *zap.Logger
}

// RouteResult contains the result of routing a key press in hint mode.
type RouteResult struct {
	exit      bool       // Whether to exit hint mode
	exactHint *Interface // The exact matched hint (domain hint)
}

// Exit returns whether to exit hint mode.
func (rr *RouteResult) Exit() bool {
	return rr.exit
}

// ExactHint returns the exact matched hint.
func (rr *RouteResult) ExactHint() *Interface {
	return rr.exactHint
}

// NewRouter creates a new hint router with the specified manager and logger.
func NewRouter(manager *Manager, logger *zap.Logger) *Router {
	return &Router{
		manager: manager,
		logger:  logger,
	}
}

// RouteKey processes a key press and returns the routing result.
func (r *Router) RouteKey(key string) RouteResult {
	// Handle escape key
	if key == "\x1b" || key == "escape" {
		return RouteResult{exit: true}
	}

	// Process input through manager
	hint, exactMatch := r.manager.HandleInput(key)
	if exactMatch {
		if r.logger != nil {
			r.logger.Debug("Hints router: Exact hint match found",
				zap.String("label", hint.Label()))
		}

		return RouteResult{
			exit:      false,
			exactHint: hint,
		}
	}

	// No exact match, continue in hint mode
	return RouteResult{
		exit:      false,
		exactHint: nil,
	}
}
