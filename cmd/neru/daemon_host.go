package main

import "github.com/y3owk1n/neru/internal/app"

// daemonHost owns the top-level process lifecycle for a specific platform.
// Darwin hosts the app inside Cocoa's main-thread event loop, while other
// platforms can block directly on app.Run().
type daemonHost interface {
	Run(application *app.App) error
}
