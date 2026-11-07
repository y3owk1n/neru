package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	// textWarmTimeout bounds one warm-up, which the first time an
	// executable runs it compiles the models for several seconds.
	textWarmTimeout = time.Minute
	// textWarmRetry is how long a warm-up a key's read canceled waits
	// before it tries again, so it runs while no one is typing.
	textWarmRetry = 2 * time.Second
	// textWarmAttempts is how many times a warm-up is tried before it is
	// left to vim mode's own accurate reads.
	textWarmAttempts = 10
)

// warmTextRecognizer compiles the accurate recognizer's models in the
// background once vim mode is enabled, so its first accurate read does not
// wait seconds for them. Vim mode's fast reads cancel a warm-up, and a
// canceled one loses what it compiled, so it tries again a little later. It
// runs once per process.
func (a *App) warmTextRecognizer(cfg *config.Config) {
	warmer, ok := a.vision.(ports.TextRecognizerWarmer)
	if !ok || !cfg.Vim.Enabled || !a.textWarmStarted.CompareAndSwap(false, true) {
		return
	}

	ctx, logger := a.ctx, a.logger

	go func() {
		start := time.Now()

		for attempt := range textWarmAttempts {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(textWarmRetry):
				}
			}

			attemptCtx, cancel := context.WithTimeout(ctx, textWarmTimeout)
			err := warmer.WarmTextRecognizer(attemptCtx)

			cancel()

			if err == nil {
				logger.Debug("Text recognizer ready",
					zap.Duration("elapsed", time.Since(start)), zap.Int("attempts", attempt+1))

				return
			}
		}

		logger.Debug("Text recognizer warm-up gave up", zap.Int("attempts", textWarmAttempts))
	}()
}
