package grid

import (
	"image"

	"go.uber.org/zap"
)

// Router handles key routing for grid mode operations.
type Router struct {
	manager *Manager
	logger  *zap.Logger
}

// KeyResult captures the results of key routing decisions in grid mode.
type KeyResult struct {
	Exit        bool        // Escape pressed -> exit mode
	TargetPoint image.Point // Complete coordinate entered
	Complete    bool        // Coordinate selection complete
}

// NewRouter initializes a new grid router with the specified manager and logger.
func NewRouter(m *Manager, logger *zap.Logger) *Router {
	return &Router{
		manager: m,
		logger:  logger,
	}
}

// RouteKey processes a keypress and determines the appropriate action in grid mode.
func (r *Router) RouteKey(key string) KeyResult {
	var routeKeyResult KeyResult

	r.logger.Debug("Grid router processing key",
		zap.String("key", key),
	)

	// Exit grid mode with Escape
	if key == "\x1b" || key == "escape" {
		r.logger.Debug("Grid router: Exit key pressed")

		routeKeyResult.Exit = true

		return routeKeyResult
	}

	// Delegate coordinate input to the grid manager
	if point, complete := r.manager.HandleInput(key); complete {
		r.logger.Debug("Grid router: Coordinate selection complete",
			zap.Int("x", point.X),
			zap.Int("y", point.Y))
		routeKeyResult.TargetPoint = point
		routeKeyResult.Complete = true
	}

	return routeKeyResult
}
