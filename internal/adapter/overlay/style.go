package overlay

import (
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// Style is every overlay's resolved appearance: the configuration combined
// with the current light/dark theme. It carries no configuration and no theme
// of its own — by the time a caller holds one, both have already been applied.
type Style struct {
	Hints            hints.StyleMode
	HintSearchInput  hints.SearchInputStyle
	HintSearchLayout SearchInputLayout
	Grid             grid.Style
	RecursiveGrid    recursivegrid.Style
	MonitorSelect    MonitorSelectStyle
	VirtualPointer   VirtualPointerStyle
}

// SearchInputLayout is where the hint search input sits and how big it is,
// resolved from configuration. It travels with the Style rather than with the
// draw because it changes when the configuration does and not otherwise; the
// screen it is placed against arrives with each draw.
type SearchInputLayout struct {
	// Position names the corner or edge the input is anchored to: one of the
	// values `hints.search_input_ui.position` accepts
	// (config.SearchInputPositions), carried through as written rather than
	// respelled here.
	Position string
	// Width and Height are the input's size in pixels.
	Width  int
	Height int
	// XOffset and YOffset are the configured insets from the anchor.
	XOffset int
	YOffset int
}

// VirtualPointerStyle is the resolved appearance of the virtual pointer drawn
// inside the grid and recursive-grid frames.
type VirtualPointerStyle struct {
	FontSize   int
	FillColor  string
	Char       string
	FontFamily string
}

// StyleSource is what a caller that needs a resolved Style depends on. It is
// deliberately read-only: resolving is the overlay's job, and a caller that
// could resolve is a caller that would.
type StyleSource interface {
	// Style returns the current resolved Style.
	Style() Style
}

// StyleOwner is the whole of what the adapter depends on for appearance: read
// the resolved Style, and re-resolve it when the configuration or the theme
// moves. It is what lets the adapter answer the port's ApplyConfig and
// RefreshStyles without the app holding a resolver of its own.
type StyleOwner interface {
	StyleSource

	// Apply re-resolves every Style from cfg and hands cfg to the render
	// components.
	Apply(cfg *config.Config)

	// Refresh re-resolves against the configuration already held.
	Refresh()
}

// ResolvedStyle reads source, or returns the zero Style when there is none.
// Every consumer needs that guard and none of them should spell it out again.
//
// It only catches an untyped nil. A nil *StyleResolver stored in the interface
// is still a live interface value, which is why Style itself guards its
// receiver rather than trusting this.
func ResolvedStyle(source StyleSource) Style {
	if source == nil {
		return Style{}
	}

	return source.Style()
}

// StyleResolver is the one place configuration and theme become a Style, and
// the one place a configuration or theme change is applied to the render
// components.
//
// Resolution happens on a config reload or a theme change, never per draw: a
// draw reads the cached value, so the draw path allocates no more than it did
// when every call site built its own style.
type StyleResolver struct {
	manager ManagerInterface
	theme   config.ThemeProvider
	logger  *zap.Logger

	// applyMu serializes whole applications. Resolving publishes under mu and
	// pushing runs outside it, so without this a slower Apply could hand the
	// render components an older configuration than the Style everything else
	// is now reading. Lock order is applyMu -> mu, never the reverse, and no
	// other lock is taken while either is held.
	applyMu sync.Mutex

	mu     sync.RWMutex
	config *config.Config
	style  Style
}

// NewStyleResolver returns a resolver holding the Style resolved from cfg and
// the theme provider's current state. The manager may hold no render
// components yet; they are picked up on the first Apply after registration.
func NewStyleResolver(
	manager ManagerInterface,
	cfg *config.Config,
	theme config.ThemeProvider,
	logger *zap.Logger,
) *StyleResolver {
	if logger == nil {
		logger = zap.NewNop()
	}

	resolver := &StyleResolver{
		manager: manager,
		theme:   theme,
		logger:  logger.Named("overlay_style"),
	}
	resolver.resolve(cfg)

	return resolver
}

// Style returns the Style resolved by the most recent Apply or Refresh.
//
// A nil receiver resolves to the zero Style rather than panicking: the app
// hands this out as a StyleSource, and a typed nil in an interface passes
// every `!= nil` guard a caller could write.
func (r *StyleResolver) Style() Style {
	if r == nil {
		return Style{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.style
}

// Apply re-resolves every overlay's Style from cfg and the live theme, then
// hands cfg to the render components so the native caches they keep are
// rebuilt against the new values on the next draw.
//
// This is the single notification a config reload or a theme change needs: no
// caller fans out to individual overlays.
func (r *StyleResolver) Apply(cfg *config.Config) {
	r.applyMu.Lock()
	defer r.applyMu.Unlock()

	r.applyLocked(cfg)
}

// Refresh re-applies the configuration the resolver already holds. A theme
// change goes through here: the configuration has not moved, only the colors
// it resolves to.
//
// The stored config is read with applyMu already held. Reading it first and
// then applying would be a read-modify-write across the lock: a reload landing
// in between would be undone by this theme change republishing the config it
// had snapshotted.
func (r *StyleResolver) Refresh() {
	r.applyMu.Lock()
	defer r.applyMu.Unlock()

	r.mu.RLock()
	cfg := r.config
	r.mu.RUnlock()

	r.applyLocked(cfg)
}

// applyLocked resolves and hands out the result. Caller must hold applyMu.
func (r *StyleResolver) applyLocked(cfg *config.Config) {
	style := r.resolve(cfg)
	r.push(cfg, style)
}

// resolve builds the Style and stores it. The theme provider is consulted
// outside the lock — it reaches the platform — and only the result is
// published under it.
func (r *StyleResolver) resolve(cfg *config.Config) Style {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	style := Style{
		Hints:            hints.BuildStyle(cfg.Hints, r.theme),
		HintSearchInput:  hints.BuildSearchInputStyle(cfg.Hints, r.theme),
		HintSearchLayout: buildSearchInputLayout(cfg.Hints.SearchInputUI),
		Grid:             grid.BuildStyle(cfg.Grid, r.theme),
		RecursiveGrid:    recursivegrid.BuildStyle(cfg.RecursiveGrid, r.theme),
		MonitorSelect:    buildMonitorSelectStyle(cfg, r.theme),
		VirtualPointer:   buildVirtualPointerStyle(cfg.VirtualPointer, r.theme),
	}

	r.mu.Lock()
	r.config = cfg
	r.style = style
	r.mu.Unlock()

	r.logger.Debug("Overlay style resolved")

	return style
}

// push hands the configuration to the render components. Which components
// those are is the manager's business — it built them — so this passes the
// configuration and the resolved values they need and asks it to apply them.
// Adding a component is not another wiring site here.
func (r *StyleResolver) push(cfg *config.Config, style Style) {
	if r.manager == nil || cfg == nil {
		return
	}

	r.manager.ConfigureComponents(cfg, PointerAppearance{
		FillColor:  style.VirtualPointer.FillColor,
		FontFamily: style.VirtualPointer.FontFamily,
		Char:       style.VirtualPointer.Char,
		FontSize:   style.VirtualPointer.FontSize,
	})
}

// buildVirtualPointerStyle resolves the cursor stand-in drawn inside the grid
// and recursive-grid frames, falling back to the documented defaults for the
// two fields a user can leave empty.
func buildVirtualPointerStyle(
	cfg config.VirtualPointerConfig,
	theme config.ThemeProvider,
) VirtualPointerStyle {
	size := cfg.UI.FontSize
	if size < 1 {
		size = config.DefaultVirtualPointerFontSize
	}

	char := cfg.UI.Char
	if char == "" {
		char = config.DefaultVirtualPointerChar
	}

	return VirtualPointerStyle{
		FontSize: size,
		FillColor: cfg.UI.TextColor.ForTheme(
			theme,
			config.VirtualPointerTextColorLight,
			config.VirtualPointerTextColorDark,
		),
		Char: char,
		// Through the shared resolver, like every other overlay's family: a
		// generic alias reaches the platform as a family it can actually find
		// rather than as a name nothing is installed under (#1305).
		FontFamily: ports.ResolveFont(cfg.UI.FontFamily),
	}
}

// buildSearchInputLayout resolves the hint search input's geometry, filling in
// the documented defaults for the two values a user can leave unset. The
// height is derived here rather than at draw time because it depends only on
// the font size and padding a configuration reload already notifies.
func buildSearchInputLayout(cfg config.SearchInputUI) SearchInputLayout {
	width := cfg.Width
	if width <= 0 {
		width = config.DefaultSearchInputWidth
	}

	paddingY := cfg.PaddingY
	if paddingY < 0 {
		paddingY = max(
			config.DefaultSearchInputMinPaddingY,
			cfg.FontSize/config.DefaultSearchInputCenterDivisor,
		)
	}

	height := cfg.FontSize +
		paddingY*config.DefaultSearchInputPaddingMultiplier +
		config.DefaultSearchInputHeightPadding

	return SearchInputLayout{
		Position: cfg.Position,
		Width:    width,
		Height:   height,
		XOffset:  cfg.XOffset,
		YOffset:  cfg.YOffset,
	}
}

// buildMonitorSelectStyle resolves the monitor picker's panels. It reads
// General as well as MonitorSelect: whether the overlay hides from a screen
// share is part of how it is drawn.
func buildMonitorSelectStyle(cfg *config.Config, theme config.ThemeProvider) MonitorSelectStyle {
	uiCfg := cfg.MonitorSelect.UI

	// An unset subtitle family means "draw the subtitle in the label's family",
	// so it falls back before resolution and not after: the resolver answers an
	// empty name with the platform's sans-serif face, which would make the
	// fallback unreachable.
	subtitleFamily := uiCfg.SubtitleFontFamily
	if subtitleFamily == "" {
		subtitleFamily = uiCfg.FontFamily
	}

	return MonitorSelectStyle{
		FontSize:         uiCfg.FontSize,
		SubtitleFontSize: uiCfg.SubtitleFontSize,
		// Both families go through the shared resolver, like every other
		// overlay's, so a generic alias reaches the platform as a family it can
		// actually find (#1305). The label is drawn bold and the subtitle is
		// not, but weight is the drawing layer's business: resolution answers
		// on the family name alone.
		FontFamily:         ports.ResolveFont(uiCfg.FontFamily),
		SubtitleFontFamily: ports.ResolveFont(subtitleFamily),
		BorderRadius:       uiCfg.BorderRadius,
		PaddingX:           uiCfg.PaddingX,
		PaddingY:           uiCfg.PaddingY,
		BorderWidth:        uiCfg.BorderWidth,
		BackgroundColor: uiCfg.BackgroundColor.ForTheme(theme,
			config.MonitorSelectBackgroundColorLight,
			config.MonitorSelectBackgroundColorDark),
		TextColor: uiCfg.TextColor.ForTheme(theme,
			config.MonitorSelectTextColorLight,
			config.MonitorSelectTextColorDark),
		MatchedTextColor: uiCfg.MatchedTextColor.ForTheme(theme,
			config.MonitorSelectMatchedTextColorLight,
			config.MonitorSelectMatchedTextColorDark),
		BorderColor: uiCfg.BorderColor.ForTheme(theme,
			config.MonitorSelectBorderColorLight,
			config.MonitorSelectBorderColorDark),
		BackdropColor: uiCfg.BackdropColor.ForTheme(theme,
			config.MonitorSelectBackdropColorLight,
			config.MonitorSelectBackdropColorDark),
		SubtitleTextColor: uiCfg.SubtitleTextColor.ForTheme(theme,
			config.MonitorSelectSubtitleTextColorLight,
			config.MonitorSelectSubtitleTextColorDark),
		HideInScreenShare: cfg.General.HideOverlayInScreenShare,
	}
}

// Ensure StyleResolver satisfies the read-only contract its consumers take.
var _ StyleSource = (*StyleResolver)(nil)
