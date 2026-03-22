//go:build darwin

package darwin

/*
#include "appwatcher.h"
*/
import "C"

// AppWatcherInterface interface defines callbacks for application lifecycle events.
type AppWatcherInterface interface {
	HandleLaunch(appName, bundleID string)
	HandleTerminate(appName, bundleID string)
	HandleActivate(appName, bundleID string)
	HandleDeactivate(appName, bundleID string)
	HandleScreenParametersChanged()
}
