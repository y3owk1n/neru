//go:build linux

package hints

import (
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// StyleMode represents the visual styling configuration for hint overlays.
type StyleMode struct {
	fontSize         int
	fontFamily       string
	borderRadius     int
	paddingX         int
	paddingY         int
	borderWidth      int
	backgroundColor  string
	textColor        string
	matchedTextColor string
	borderColor      string
}

// FontSize returns the font size.
func (s StyleMode) FontSize() int { return s.fontSize }

// FontFamily returns the font family.
func (s StyleMode) FontFamily() string { return s.fontFamily }

// BorderRadius returns the border radius.
func (s StyleMode) BorderRadius() int { return s.borderRadius }

// PaddingX returns the padding X.
func (s StyleMode) PaddingX() int { return s.paddingX }

// PaddingY returns the padding Y.
func (s StyleMode) PaddingY() int { return s.paddingY }

// BorderWidth returns the border width.
func (s StyleMode) BorderWidth() int { return s.borderWidth }

// BackgroundColor returns the background color.
func (s StyleMode) BackgroundColor() string { return s.backgroundColor }

// TextColor returns the text color.
func (s StyleMode) TextColor() string { return s.textColor }

// MatchedTextColor returns the matched text color.
func (s StyleMode) MatchedTextColor() string { return s.matchedTextColor }

// BorderColor returns the border color.
func (s StyleMode) BorderColor() string { return s.borderColor }

// Overlay manages the rendering of hint overlays using native platform APIs (Linux stub).
type Overlay struct {
	window unsafe.Pointer
	config config.HintsConfig
	logger *zap.Logger
}

// NewOverlay initializes a new hint overlay instance with its own window (Linux stub).
func NewOverlay(hintsCfg config.HintsConfig, logger *zap.Logger) (*Overlay, error) {
	return &Overlay{
		config: hintsCfg,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow initializes a new hint overlay instance with an existing window (Linux stub).
func NewOverlayWithWindow(
	hintsCfg config.HintsConfig,
	logger *zap.Logger,
	window unsafe.Pointer,
) (*Overlay, error) {
	return &Overlay{
		config: hintsCfg,
		logger: logger,
		window: window,
	}, nil
}

// DrawHints draws the hints using the specified style (Linux stub).
func (o *Overlay) DrawHints(hints []*Hint, style StyleMode) error {
	return nil
}

// Show shows the hint overlay (Linux stub).
func (o *Overlay) Show() {}

// Hide hides the hint overlay (Linux stub).
func (o *Overlay) Hide() {}

// Destroy destroys the hint overlay (Linux stub).
func (o *Overlay) Destroy() {}

// Clear clears the hint overlay (Linux stub).
func (o *Overlay) Clear() {}

// SetConfig updates the hints configuration (Linux stub).
func (o *Overlay) SetConfig(cfg config.HintsConfig) {
	o.config = cfg
}

// Config returns the hints configuration (Linux stub).
func (o *Overlay) Config() config.HintsConfig {
	return o.config
}

// BuildStyle builds the hints style from the configuration (Linux stub).
func BuildStyle(cfg config.HintsConfig, theme config.ThemeProvider) StyleMode {
	return StyleMode{}
}
