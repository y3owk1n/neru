//go:build darwin

package appwatcher

import "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"

func platformRegisterWatcher(w *Watcher) {
	darwin.SetAppWatcher(darwin.AppWatcherInterface(w))
}

func platformStartWatcher() {
	darwin.StartAppWatcher()
}

func platformStopWatcher() {
	darwin.StopAppWatcher()
}
