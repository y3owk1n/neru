package bridge

import "go.uber.org/zap"

type loggingBridge struct {
	logger *zap.Logger
}

func (b *loggingBridge) Debug(msg string, fields ...zap.Field) {
	if b.logger != nil {
		b.logger.Debug(msg, fields...)
	}
}

func (b *loggingBridge) Info(msg string, fields ...zap.Field) {
	if b.logger != nil {
		b.logger.Info(msg, fields...)
	}
}

func (b *loggingBridge) Warn(msg string, fields ...zap.Field) {
	if b.logger != nil {
		b.logger.Warn(msg, fields...)
	}
}

func (b *loggingBridge) Error(msg string, fields ...zap.Field) {
	if b.logger != nil {
		b.logger.Error(msg, fields...)
	}
}

var (
	bridgeLogger *zap.Logger
	log          *loggingBridge
)

// InitializeLogger sets the global logger instance for the bridge package.
func InitializeLogger(logger *zap.Logger) {
	bridgeLogger = logger
	log = &loggingBridge{logger: logger}
}

// Logger returns the global logger instance for the bridge package.
func Logger() *zap.Logger {
	return bridgeLogger
}

func getLogger() *loggingBridge {
	if log == nil {
		return &loggingBridge{}
	}

	return log
}
