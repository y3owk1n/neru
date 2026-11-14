package native

import "github.com/y3owk1n/neru/internal/adapter/accessibility/ax"

func currentSettings(source ax.SettingsSource) ax.Settings {
	if source == nil {
		return nil
	}

	return source()
}
