//go:build !darwin && !linux

package appwatcher

func platformRegisterWatcher(_ *Watcher) {}
func platformStartWatcher()              {}
func platformStopWatcher()               {}
func platformSetMCDetection(_ bool)      {}
