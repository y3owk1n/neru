package clipboard

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// writeTimeout bounds one clipboard write, so a copier that hangs fails the
// action instead of holding it open.
const writeTimeout = 2 * time.Second

// Adapter implements ports.ClipboardPort.
type Adapter struct {
	logger *zap.Logger
}

// NewAdapter creates a clipboard adapter. A nil logger is replaced with a
// no-op.
func NewAdapter(logger *zap.Logger) *Adapter {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Adapter{logger: logger.Named("clipboard")}
}

// WriteText replaces the clipboard's contents with text.
func (a *Adapter) WriteText(ctx context.Context, text string) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	if ctx.Err() != nil {
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	}

	text = strings.ReplaceAll(text, "\x00", "")

	a.logger.Debug("Writing to the clipboard", zap.Int("length", len(text)))

	return writeText(ctx, text)
}

// Ensure Adapter implements ports.ClipboardPort.
var _ ports.ClipboardPort = (*Adapter)(nil)
