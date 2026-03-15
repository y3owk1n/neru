package stickyindicator

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// Service manages the sticky modifiers indicator overlay.
type Service struct {
	system  ports.SystemPort
	overlay ports.OverlayPort
	logger  *zap.Logger
}

// NewService creates a new sticky indicator service.
func NewService(
	system ports.SystemPort,
	overlay ports.OverlayPort,
	logger *zap.Logger,
) *Service {
	return &Service{
		system:  system,
		overlay: overlay,
		logger:  logger,
	}
}

// GetCursorPosition returns the current cursor position.
func (s *Service) GetCursorPosition(ctx context.Context) (int, int, error) {
	if s.system == nil {
		return 0, 0, derrors.New(derrors.CodeActionFailed, "system port not available")
	}

	point, err := s.system.CursorPosition(ctx)
	if err != nil {
		return 0, 0, core.WrapAccessibilityFailed(err, "get cursor position")
	}

	return point.X, point.Y, nil
}

// UpdateIndicatorPosition draws the sticky modifiers indicator at the given position.
// symbols is the string of modifier symbols to display (e.g. "⌘⇧").
func (s *Service) UpdateIndicatorPosition(x, y int, symbols string) {
	s.overlay.DrawStickyModifiersIndicator(x, y, symbols)
}

// ModifierSymbolsString converts a Modifiers bitmask to a display string.
func ModifierSymbolsString(mods action.Modifiers) string {
	if mods == 0 {
		return ""
	}

	var symbols string
	if mods.Has(action.ModCmd) {
		symbols += "⌘"
	}

	if mods.Has(action.ModShift) {
		symbols += "⇧"
	}

	if mods.Has(action.ModAlt) {
		symbols += "⌥"
	}

	if mods.Has(action.ModCtrl) {
		symbols += "⌃"
	}

	return symbols
}
