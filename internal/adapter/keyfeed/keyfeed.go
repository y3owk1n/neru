package keyfeed

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// Adapter implements ports.KeyFeedPort.
//
// Normalization is shared and lives here; only the final injection differs per
// platform, behind the unexported postKey dispatch (keyfeed_darwin.go,
// keyfeed_linux.go, keyfeed_windows.go, keyfeed_other.go).
type Adapter struct {
	logger *zap.Logger
}

// NewAdapter creates a KeyFeed adapter. A nil logger is replaced with a no-op.
func NewAdapter(logger *zap.Logger) *Adapter {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Adapter{logger: logger.Named("keyfeed")}
}

// Feed posts a single key or chord to the focused application.
func (a *Adapter) Feed(ctx context.Context, key string) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	normalized, err := NormalizeKeyForFeed(key)
	if err != nil {
		return err
	}

	a.logger.Debug("Feeding key to focused application")

	return postKey(normalized)
}

// Ensure Adapter implements ports.KeyFeedPort.
var _ ports.KeyFeedPort = (*Adapter)(nil)

// Feed posts a key or key chord directly to the OS.
//
// Key strings follow the canonical form used by config.CanonicalHotkeyForPlatform:
//   - single character: "a", "B", "1"
//   - named key: "Return", "F1", "Space"
//   - modifier+key: "Ctrl+c", "Shift+F1", "Ctrl+Shift+Space"
//
// Prefer the Adapter; this function remains for callers that have no port
// handy, and returns CodeNotSupported on platforms without an injection path.
func Feed(key string) error {
	normalized, err := NormalizeKeyForFeed(key)
	if err != nil {
		return err
	}

	return postKey(normalized)
}

// NormalizeKeyForFeed normalizes a key string for feeding to the OS.
//
// A single uppercase letter (A-Z) with no explicit modifier gets Shift injected
// so it produces the uppercase character rather than the lowercase one.
func NormalizeKeyForFeed(key string) (string, error) {
	trimmed := strings.TrimSpace(key)

	isSingleUppercase := len(trimmed) == 1 && trimmed[0] >= 'A' && trimmed[0] <= 'Z'

	normalized := config.CanonicalHotkeyForPlatform(trimmed)
	if normalized == "" {
		return "", derrors.New(derrors.CodeInvalidInput, "key is required")
	}

	if isSingleUppercase && !strings.Contains(normalized, "+") {
		normalized = "Shift+" + normalized
	}

	return normalized, nil
}
