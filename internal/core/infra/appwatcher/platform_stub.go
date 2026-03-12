//go:build !darwin

package appwatcher

func platformRegisterWatcher(_ *Watcher) {}
func platformStartWatcher()              {}
func platformStopWatcher()               {}
