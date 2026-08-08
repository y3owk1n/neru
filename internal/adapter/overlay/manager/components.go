package manager

import (
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/modeindicator"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/stickyindicator"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/virtualpointer"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Components are the render components a manager draws through. A nil entry is
// an overlay this manager will not draw — disabled in configuration, or a
// backend with no surface to build it on.
//
// The manager owns them; it hands the set back so the few app-layer call sites
// that still talk to a render component directly have something to hold. Those
// go away as the frame port lands (#1210, #1211), and this type with them.
type Components struct {
	Hints           *hints.Overlay
	Grid            *grid.Overlay
	RecursiveGrid   *recursivegrid.Overlay
	ModeIndicator   *modeindicator.Overlay
	StickyModifiers *stickyindicator.Overlay
	VirtualPointer  *virtualpointer.Overlay
}

// ComponentSpec is everything building the render components needs that the
// shared Base cannot know: the surface they attach to, and whether there is
// one at all. Backends fill it in from their own window and their own headless
// verdict, which is why construction is delegated here rather than inferred.
type ComponentSpec struct {
	// Config is the configuration the components are built from.
	Config *config.Config
	// Theme is the appearance the indicators resolve their colors against.
	Theme config.ThemeProvider
	// Logger may be nil.
	Logger *zap.Logger
	// Window is the native surface the mode overlays draw into. The
	// indicators own their own windows and ignore it.
	Window unsafe.Pointer
	// Headless states that this manager has no surface to render on, so
	// nothing should be built against it.
	Headless bool
}

// BuildComponents constructs the render components from spec and keeps them.
//
// A component whose constructor fails is logged and left nil rather than
// failing the build: the mode it belongs to loses its overlay, and the rest of
// the session still starts. The virtual pointer is the exception — it is the
// one component whose absence the app treated as a startup failure, and the
// numbered startup phases still unwind on that.
func (b *Base) BuildComponents(spec ComponentSpec) (Components, error) {
	// Nil-guarded for the same reason the backend delegates are: a backend
	// with no display server is handed out as a typed nil, and Base sits at
	// its offset zero, so this receiver is the nil that arrives.
	if b == nil {
		return Components{}, nil
	}

	logger := spec.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	cfg := spec.Config
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if spec.Headless {
		logger.Debug("Overlay is headless; building no render components")

		return Components{}, nil
	}

	built := Components{
		// The grid and recursive-grid overlays are built even when their mode
		// is disabled: enabling one through a config reload must not need a
		// restart to get an overlay.
		Grid:          grid.NewOverlayWithWindow(cfg.Grid, logger, spec.Window),
		RecursiveGrid: recursivegrid.NewOverlayWithWindow(cfg.RecursiveGrid, logger, spec.Window),
	}

	// Hints are the one mode whose overlay follows the enabled flag, because
	// the app has always skipped building it when hints are off.
	if cfg.Hints.Enabled {
		hintOverlay, err := hints.NewOverlayWithWindow(cfg.Hints, logger, spec.Window)
		if err != nil {
			logger.Warn("Failed to build the hints overlay; hints will not draw", zap.Error(err))
		} else {
			built.Hints = hintOverlay
		}
	}

	modeIndicator, err := modeindicator.NewOverlay(cfg.ModeIndicator, spec.Theme, logger)
	if err != nil {
		logger.Warn("Failed to build the mode indicator overlay", zap.Error(err))
	} else {
		built.ModeIndicator = modeIndicator
	}

	sticky, err := stickyindicator.NewOverlay(cfg.StickyModifiers.UI, spec.Theme, logger)
	if err != nil {
		logger.Warn("Failed to build the sticky modifiers indicator overlay", zap.Error(err))
	} else {
		built.StickyModifiers = sticky
	}

	pointer, err := virtualpointer.NewOverlay(cfg.VirtualPointer, spec.Theme, logger)
	if err != nil {
		// Keep what was built before handing back the failure: the indicators
		// own native windows, and Destroy only reaches the components the
		// manager was given, so a caller that tears the manager down after
		// this releases them. The caller still gets nothing back, because a
		// partial set is not one it can draw with.
		b.useComponents(built)

		return Components{}, derrors.Wrap(
			err,
			derrors.CodeOverlayFailed,
			"failed to create virtual pointer overlay",
		)
	}

	built.VirtualPointer = pointer

	b.useComponents(built)

	return built, nil
}

// useComponents stores the built set, keeping the registry writes in one
// place. A backend that adds behavior to a registration — darwin hands the
// virtual pointer the current screen-share visibility — applies it in its own
// BuildComponents: an embedded Base cannot call back into the type embedding
// it, so overriding a setter would not be seen from here.
func (b *Base) useComponents(built Components) {
	b.UseHintOverlay(built.Hints)
	b.UseGridOverlay(built.Grid)
	b.UseRecursiveGridOverlay(built.RecursiveGrid)
	b.UseModeIndicatorOverlay(built.ModeIndicator)
	b.UseStickyModifiersOverlay(built.StickyModifiers)
	b.UseVirtualPointerOverlay(built.VirtualPointer)
}

// ConfigureComponents hands a new configuration to the render components, so
// the native caches they keep are rebuilt against it on the next draw. pointer
// is the resolved appearance of the virtual pointer; it arrives already themed
// and with its font family, char and font size already settled, because the
// overlay resolves Style in one place and this is not it.
//
// A disabled overlay is left alone: nothing draws it, so reconfiguring it would
// only invalidate caches nobody reads. The grid overlays are built even when
// their mode is disabled, so they are the ones that actually sit in that state.
func (b *Base) ConfigureComponents(cfg *config.Config, pointer PointerAppearance) {
	// This one is reached straight through the contract rather than through a
	// backend delegate, so it carries the nil-receiver guard itself.
	if b == nil || cfg == nil {
		return
	}

	// The pointer's configuration reaches its components with the settled
	// values in place of the written ones: a component draws what it is handed,
	// and only this notification knows what an alias, an empty char or a zero
	// font size settled to. Substituting here rather than in each component
	// keeps the defaulting in one place — the style resolver — instead of three
	// copies on the Objective-C boundary (#1305, #1337).
	pointerCfg := cfg.VirtualPointer
	pointerCfg.UI.FontFamily = pointer.FontFamily
	pointerCfg.UI.Char = pointer.Char
	pointerCfg.UI.FontSize = pointer.FontSize

	if overlay := b.hintOverlay; overlay != nil && cfg.Hints.Enabled {
		overlay.SetConfig(cfg.Hints)
	}

	if overlay := b.gridOverlay; overlay != nil && cfg.Grid.Enabled {
		overlay.SetConfig(cfg.Grid)
		overlay.SetVirtualPointerConfig(pointerCfg.UI, pointer.FillColor)
	}

	if overlay := b.recursiveGridOverlay; overlay != nil && cfg.RecursiveGrid.Enabled {
		overlay.SetConfig(cfg.RecursiveGrid)
		overlay.SetVirtualPointerConfig(pointerCfg.UI, pointer.FillColor)
	}

	if overlay := b.modeIndicatorOverlay; overlay != nil {
		overlay.SetConfig(cfg.ModeIndicator)
	}

	if overlay := b.stickyModifiersOverlay; overlay != nil {
		overlay.SetConfig(cfg.StickyModifiers.UI)
	}

	if overlay := b.virtualPointerOverlay; overlay != nil {
		overlay.SetConfig(pointerCfg)
	}
}
