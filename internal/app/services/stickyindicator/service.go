package stickyindicator

import (
	"runtime"

	"github.com/y3owk1n/neru/internal/app/services/indicator"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/ports"
)

// Service owns the sticky modifiers indicator for its whole life: whether it is
// on screen, how big its surface is, and the symbols it shows.
type Service struct {
	indicator.Base
}

// NewService creates a new sticky indicator service.
func NewService(
	system ports.SystemPort,
	overlay ports.OverlayPort,
) *Service {
	return &Service{
		Base: indicator.NewBase(ports.StickyModifiersIndicator, system, overlay),
	}
}

// UpdateIndicatorPosition draws the sticky modifiers indicator at the given position.
// symbols is the string of modifier symbols to display (e.g. "⌘⇧").
func (s *Service) UpdateIndicatorPosition(posX, posY int, symbols string) {
	overlay := s.Overlay()
	if overlay == nil {
		return
	}

	overlay.DrawStickyModifiersIndicator(posX, posY, symbols)
}

// cmdSymbol returns the platform-appropriate symbol for the Command / Super modifier.
// On macOS this is "⌘"; on Linux it is "❖" (the Super/Windows key symbol).
func cmdSymbol() string {
	if runtime.GOOS == "darwin" {
		return "⌘"
	}

	return "❖"
}

// ModifierSymbolsString converts a Modifiers bitmask to a display string.
func ModifierSymbolsString(mods action.Modifiers) string {
	if mods == 0 {
		return ""
	}

	var symbols string
	if mods.Has(action.ModCmd) {
		symbols += cmdSymbol()
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
