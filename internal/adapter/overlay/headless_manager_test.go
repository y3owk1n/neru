package overlay_test

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	overlaymanager "github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	rendergrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	renderhints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	renderrecursivegrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// headlessManager answers the overlay's backend contract without a display.
//
// It is test support and lives here for that reason: the app layer has had no
// use for a no-op backend since #1213, when the simulation harness started
// implementing ports.OverlayPort outright. What is still wide is the contract
// between this adapter and its own backends, which is deliberately out of
// scope for the port work — so the fakes below build on this rather than
// spelling out forty methods each. That means a fake here silently succeeds at
// anything it does not override; every one of them asserts on what it recorded
// rather than on what it was not asked.
type headlessManager struct{}

var (
	_ overlay.ManagerInterface = (*headlessManager)(nil)
	_ overlay.HeadlessReporter = (*headlessManager)(nil)
)

// WaylandKeyboardChannel returns nil channel.
func (n *headlessManager) WaylandKeyboardChannel() <-chan string { return nil }

// Show is a no-op implementation.
func (n *headlessManager) Show() {}

// Hide is a no-op implementation.
func (n *headlessManager) Hide() {}

// Clear is a no-op implementation.
func (n *headlessManager) Clear() {}

// ClearCache is a no-op implementation.
func (n *headlessManager) ClearCache() {}

// ResizeToActiveScreen is a no-op implementation.
func (n *headlessManager) ResizeToActiveScreen() {}

// SetActiveScreenOrigin is a no-op implementation.
func (n *headlessManager) SetActiveScreenOrigin(origin image.Point) {}

// SwitchTo is a no-op implementation.
func (n *headlessManager) SwitchTo(next overlay.Mode) {}

// Subscribe is a no-op implementation.
func (n *headlessManager) Subscribe(fn func(overlay.StateChange)) uint64 { return 0 }

// Unsubscribe is a no-op implementation.
func (n *headlessManager) Unsubscribe(id uint64) {}

// Destroy is a no-op implementation.
func (n *headlessManager) Destroy() {}

// overlay.Mode returns overlay.ModeIdle.
func (n *headlessManager) Mode() overlay.Mode { return overlay.ModeIdle }

// Headless reports that the no-op manager has no surface to render on.
func (n *headlessManager) Headless() bool { return true }

// BuildComponents builds nothing. This manager declares itself headless, and
// that is the whole reason: there is no surface to build against.
func (n *headlessManager) BuildComponents(
	cfg *config.Config,
	theme config.ThemeProvider,
) (overlaymanager.Components, error) {
	return overlaymanager.Components{}, nil
}

// ConfigureComponents is a no-op implementation.
func (n *headlessManager) ConfigureComponents(
	cfg *config.Config,
	pointer overlay.PointerAppearance,
) {
}

// DrawHintsWithStyle is a no-op implementation.
func (n *headlessManager) DrawHintsWithStyle(
	hs []*renderhints.Hint,
	style renderhints.StyleMode,
) error {
	return nil
}

// DrawHintSearchInput is a no-op implementation.
func (n *headlessManager) DrawHintSearchInput(
	query string,
	resultCount int,
	frame renderhints.SearchInputFrame,
	style renderhints.SearchInputStyle,
) error {
	return nil
}

// HideHintSearchInput is a no-op implementation.
func (n *headlessManager) HideHintSearchInput() {}

// DrawModeIndicator is a no-op implementation.
func (n *headlessManager) DrawModeIndicator(x, y int) {}

// DrawStickyModifiersIndicator is a no-op implementation.
func (n *headlessManager) DrawStickyModifiersIndicator(x, y int, symbols string) {}

// DrawVirtualPointer is a no-op implementation.
func (n *headlessManager) DrawVirtualPointer(_, _ int, _ int, _ string) {}

// ShowIndicator is a no-op implementation.
func (n *headlessManager) ShowIndicator(indicator ports.Indicator) {}

// HideIndicator is a no-op implementation.
func (n *headlessManager) HideIndicator(indicator ports.Indicator) {}

// ResizeIndicatorToActiveScreen is a no-op implementation.
func (n *headlessManager) ResizeIndicatorToActiveScreen(indicator ports.Indicator) {}

// DrawMouseActionIndicator is a no-op implementation.
func (n *headlessManager) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
}

// DrawGrid is a no-op implementation.
func (n *headlessManager) DrawGrid(
	g *domainGrid.Grid,
	input string,
	style rendergrid.Style,
) error {
	return nil
}

// DrawRecursiveGrid is a no-op implementation.
func (n *headlessManager) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	style renderrecursivegrid.Style,
	virtualPointer renderrecursivegrid.VirtualPointerState,
) error {
	return nil
}

// UpdateGridMatches is a no-op implementation.
func (n *headlessManager) UpdateGridMatches(prefix string) {}

// ShowSubgrid is a no-op implementation.
func (n *headlessManager) ShowSubgrid(cell *domainGrid.Cell, style rendergrid.Style) {}

// SetHideUnmatched is a no-op implementation.
func (n *headlessManager) SetHideUnmatched(hide bool) {}

// DrawGridPointer is a no-op implementation.
func (n *headlessManager) DrawGridPointer(
	_ overlay.Mode,
	_ image.Point,
	_ overlay.PointerAppearance,
) {
}

// HideGridPointer is a no-op implementation.
func (n *headlessManager) HideGridPointer(_ overlay.Mode) {}

// SetSharingType is a no-op implementation.
func (n *headlessManager) SetSharingType(hide bool) {}

// Flush is a no-op implementation.
func (n *headlessManager) Flush() {}

// OverlayCapabilities reports that headlessManager does not render overlays.
func (n *headlessManager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusHeadless,
		Detail: "headless no-op overlay manager",
	}
}
