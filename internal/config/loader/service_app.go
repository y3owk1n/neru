package loader

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// alertInvalidReload tells the user which file failed to load, without making
// the reload wait for them to read it.
//
// The alert is a modal dialog on macOS. Shown inline, it held the reload until
// someone dismissed it — so `neru config reload` reported a receive timeout,
// which reads as a hung daemon rather than as a bad config file. Every reload
// has someone waiting on it: the CLI, or the systray menu item. Only the
// systray has no other way to learn the reload failed, so the alert stays, off
// the reply's path.
func (s *Service) alertInvalidReload(
	ctx context.Context,
	loadResult *config.LoadResult,
	logger *zap.Logger,
) {
	if s.alertProvider == nil {
		return
	}

	// Showing the alert inline used to serialize these: a second reload could
	// not start until the first dialog was dismissed. Off the reply's path
	// that no longer holds, so repeated failures would stack a dialog and park
	// a goroutine each. One outstanding alert is enough — it names the file to
	// fix, and fixing it is what the next reload reports on.
	if !s.alertShowing.CompareAndSwap(false, true) {
		logger.Debug("Config validation alert already showing; not queueing another")

		return
	}

	// ShowAlert(ctx, title, message):
	//   title   = human-readable error summary
	//   message = config file path so the user knows which file to fix
	title := loadResult.ValidationError.Error()
	path := loadResult.ConfigPath

	// The dialog outlives the request that spawned it, so it must not be torn
	// down when that request's context is canceled.
	alertCtx := context.WithoutCancel(ctx)

	go func() {
		defer func() {
			s.alertShowing.Store(false)

			if recovered := recover(); recovered != nil {
				logger.Error("panic while showing the config alert",
					zap.Any("recover", recovered))
			}
		}()

		alertErr := s.alertProvider.ShowAlert(alertCtx, title, path)
		if alertErr != nil {
			logger.Warn("Failed to show config validation alert", zap.Error(alertErr))
		}
	}()
}

// ReloadWithAppContext reloads configuration with app-specific context and side effects.
// This handles the app-specific logic for configuration reloading including:
// - UI alerts for validation errors
// - Accessibility role updates
// - Global config updates.
func (s *Service) ReloadWithAppContext(
	ctx context.Context,
	path string,
	logger *zap.Logger,
) (*config.LoadResult, error) {
	loadResult := s.LoadWithValidation(path)

	if loadResult.ValidationError != nil {
		logger.Warn("Config validation failed during reload",
			zap.Error(loadResult.ValidationError),
			zap.String("config_path", loadResult.ConfigPath))

		s.alertInvalidReload(ctx, loadResult, logger)

		return loadResult, derrors.WrapConfigFailed(loadResult.ValidationError, "validate config")
	}

	// Update the service with the new config
	s.mu.Lock()
	s.config = loadResult.Config
	s.path = loadResult.ConfigPath
	watchers := make([]chan<- *config.Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	// Notify watchers (outside the lock to avoid deadlock)
	for _, watcher := range watchers {
		select {
		case watcher <- loadResult.Config:
		case <-ctx.Done():
			return loadResult, derrors.WrapContextCanceled(ctx, "notify config watchers")
		default:
			// Skip if watcher is not ready
		}
	}

	logger.Info("Configuration reloaded successfully")

	return loadResult, nil
}
