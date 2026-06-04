package logger

// DefaultLogFilePath exposes the unexported defaultLogFilePath to external
// (logger_test) tests without widening the package's public API.
var DefaultLogFilePath = defaultLogFilePath
