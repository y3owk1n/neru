package config

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core"
)

// ReloadWithAppContext reloads configuration with app-specific context and side effects.
// This handles the app-specific logic for configuration reloading including:
// - UI alerts for validation errors
// - Accessibility role updates
// - Global config updates.
func (s *Service) ReloadWithAppContext(
	ctx context.Context,
	path string,
	logger *zap.Logger,
) (*LoadResult, error) {
	loadResult := s.LoadWithValidation(path)

	if loadResult.ValidationError != nil {
		logger.Warn("Config validation failed during reload",
			zap.Error(loadResult.ValidationError),
			zap.String("config_path", loadResult.ConfigPath))

		if s.alertProvider != nil {
			// ShowAlert(ctx, title, message):
			//   title   = human-readable error summary
			//   message = config file path so the user knows which file to fix
			_ = s.alertProvider.ShowAlert(
				ctx,
				loadResult.ValidationError.Error(),
				loadResult.ConfigPath,
			)
		}

		return loadResult, core.WrapConfigFailed(loadResult.ValidationError, "validate config")
	}

	// Update the service with the new config
	s.mu.Lock()
	s.config = loadResult.Config
	s.path = loadResult.ConfigPath
	watchers := make([]chan<- *Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	// Update global config for backward compatibility (must happen before watcher notifications)
	SetGlobal(loadResult.Config)

	// Notify watchers (outside the lock to avoid deadlock)
	for _, watcher := range watchers {
		select {
		case watcher <- loadResult.Config:
		case <-ctx.Done():
			return loadResult, core.WrapContextCanceled(ctx, "notify config watchers")
		default:
			// Skip if watcher is not ready
		}
	}

	logger.Info("Configuration reloaded successfully")

	return loadResult, nil
}
