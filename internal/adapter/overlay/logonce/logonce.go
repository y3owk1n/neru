// Package logonce limits a warning that an overlay draw would repeat on every
// frame or keystroke to one Warn line per failure streak. Repeats log at Debug.
package logonce

import (
	"sync/atomic"

	"go.uber.org/zap"
)

// Latch logs the first warning for a key at Warn and every repeat of that key
// at Debug. The zero value is ready to use and safe for concurrent use.
type Latch struct {
	last atomic.Pointer[string]
}

// Warn logs msg at Warn unless the last warning carried the same key, in which
// case it logs at Debug. A nil logger logs nothing.
func (l *Latch) Warn(logger *zap.Logger, key, msg string, fields ...zap.Field) {
	if logger == nil {
		return
	}

	prev := l.last.Swap(&key)
	if prev != nil && *prev == key {
		logger.Debug(msg, fields...)

		return
	}

	logger.Warn(msg, fields...)
}

// Reset forgets the last warning, so the next failure warns again. Call it
// once the failing operation succeeds.
func (l *Latch) Reset() {
	l.last.Store(nil)
}
