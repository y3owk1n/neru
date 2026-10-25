// Package darwin drives the darwin system tray.
//
// The whole directory is darwin-only, so the directory carries the platform and
// the filenames do not repeat it. The API is package-level functions rather
// than methods because the backend wraps a single process-wide native tray.
// The parent package's Adapter turns that into an injectable port.
package darwin
