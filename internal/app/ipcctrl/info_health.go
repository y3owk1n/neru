// Health and capability reporting for neru doctor: the health handler
// plus the capability/profile serialization helpers it uses.

package ipcctrl

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/ports"
)

func (h *InfoHandler) handleHealth(ctx context.Context, _ ipc.Command) ipc.Response {
	hasErrors := false
	// --- component checks ---------------------------------------------------
	components := make(map[string]string)
	capabilities := capabilitiesMap(h.systemCapabilities())
	// Event tap — only enabled during active modes (hints/grid/scroll),
	// so "disabled" in idle mode is expected and healthy.
	if h.eventTap != nil {
		if h.eventTap.IsEnabled() {
			components["event_tap"] = "ok (active)"
		} else {
			components["event_tap"] = "ok (idle)"
		}
	} else {
		components["event_tap"] = healthNotInitialized
		hasErrors = true
	}
	// IPC server (implicitly healthy since we're responding, but verify)
	if h.ipcServer != nil {
		if h.ipcServer.IsRunning() {
			components["ipc_server"] = "ok"
		} else {
			components["ipc_server"] = "not running"
			hasErrors = true
		}
	} else {
		components["ipc_server"] = healthNotInitialized
		hasErrors = true
	}

	cfg := h.configSnapshot()
	if cfg != nil {
		validateErr := cfg.Validate()
		if validateErr != nil {
			components["config"] = validateErr.Error()
			hasErrors = true
		} else {
			components["config"] = "ok"
		}
	} else {
		components["config"] = "not loaded"
		hasErrors = true
	}

	for key, value := range capabilities {
		// Skip informational sibling fields (e.g. dark_mode_detection_detail);
		// see detailSuffix for the contract.
		if strings.HasSuffix(key, detailSuffix) {
			continue
		}

		status, _ := value.(string)

		components["capability."+key] = status
		if key != "platform" && !capabilityStatusSupported(status) {
			hasErrors = true
		}
	}
	// Service health checks (accessibility + overlay per service)
	serviceChecks := map[string]map[string]error{}
	if h.hintService != nil {
		serviceChecks["hints"] = h.hintService.Health(ctx)
	} else {
		components["hints"] = healthNotInitialized
		hasErrors = true
	}

	if h.gridService != nil {
		serviceChecks["grid"] = h.gridService.Health(ctx)
	} else {
		components["grid"] = healthNotInitialized
		hasErrors = true
	}

	if h.actionService != nil {
		serviceChecks["action"] = h.actionService.Health(ctx)
	} else {
		components["action"] = healthNotInitialized
		hasErrors = true
	}

	if h.scrollService != nil {
		serviceChecks["scroll"] = h.scrollService.Health(ctx)
	} else {
		components["scroll"] = healthNotInitialized
		hasErrors = true
	}
	// Flatten service sub-checks into components map
	for service, checks := range serviceChecks {
		for check, err := range checks {
			key := service + "." + check

			if err != nil {
				components[key] = err.Error()
				hasErrors = true

				h.logger.Warn("Health check failed",
					zap.String("service", service),
					zap.String("check", check),
					zap.Error(err))
			} else {
				components[key] = "ok"
			}
		}
	}

	// --- metadata -----------------------------------------------------------
	configPath := h.ResolveConfigPath()

	mode := ""
	if h.appState != nil {
		mode = domain.ModeString(h.appState.CurrentMode())
	}

	data := map[string]any{
		"version":      ipc.BuildVersion(),
		"config":       configPath,
		"mode":         mode,
		"capabilities": capabilities,
		"profile":      profileMap(platform.CurrentProfile()),
		"components":   components,
	}

	response := ipc.Response{
		Success: !hasErrors,
		Data:    data,
		Code:    ipc.CodeOK,
	}

	if hasErrors {
		response.Code = ipc.CodeActionFailed
	}

	return response
}

func (h *InfoHandler) systemCapabilities() ports.PlatformCapabilities {
	if h.systemPort != nil {
		return h.systemPort.Capabilities()
	}

	return ports.PlatformCapabilities{}
}

func capabilitiesMap(capabilities ports.PlatformCapabilities) map[string]any {
	if capabilities.Platform == "" {
		return map[string]any{}
	}

	out := map[string]any{"platform": capabilities.Platform}

	for _, entry := range capabilities.Entries() {
		out[string(entry.Key)] = capabilityString(entry.FeatureCapability)
	}

	// Surface dark-mode Detail as a sibling field so `neru doctor` can render
	// the current live state (e.g. "current state: dark (source=xdg-portal)")
	// without having to expand FeatureCapability into its own structured map.
	// Only Linux probes a live state into Detail; Darwin/Windows leave a static
	// description there, so gate on platform to keep it out of their output.
	if detail := capabilities.DarkModeDetection.Detail; detail != "" &&
		strings.HasPrefix(capabilities.Platform, "linux") {
		out[string(ports.CapabilityDarkModeDetection)+detailSuffix] = detail
	}

	// Notifications get the same treatment, gated on the status rather than on
	// the platform: "stub" alone does not tell a user whether a daemon to
	// install, a build to change, or nothing at all would fix it, while the
	// supported detail only restates the mechanism and is left out. Linux
	// probes a live reason into this; the other platforms carry a static one,
	// and either is worth the line when the capability is not there.
	if detail := capabilities.Notifications.Detail; detail != "" &&
		!capabilities.Notifications.Supported() {
		out[string(ports.CapabilityNotifications)+detailSuffix] = detail
	}

	return out
}

func profileMap(profile platform.Profile) map[string]any {
	return map[string]any{
		"os":                          string(profile.OS),
		"primary_modifier":            profile.PrimaryModifier,
		"display_server":              string(profile.DisplayServer),
		"accessibility_backend":       profile.Accessibility.Name,
		"accessibility_build_mode":    string(profile.Accessibility.BuildMode),
		"accessibility_notes":         profile.Accessibility.Notes,
		"hotkeys_backend":             profile.Hotkeys.Name,
		"hotkeys_build_mode":          string(profile.Hotkeys.BuildMode),
		"hotkeys_notes":               profile.Hotkeys.Notes,
		"keyboard_capture_backend":    profile.KeyboardCapture.Name,
		"keyboard_capture_build_mode": string(profile.KeyboardCapture.BuildMode),
		"keyboard_capture_notes":      profile.KeyboardCapture.Notes,
		"overlay_backend":             profile.Overlay.Name,
		"overlay_build_mode":          string(profile.Overlay.BuildMode),
		"overlay_notes":               profile.Overlay.Notes,
		"notifications_backend":       profile.Notifications.Name,
		"notifications_build_mode":    string(profile.Notifications.BuildMode),
		"notifications_notes":         profile.Notifications.Notes,
	}
}

func capabilityString(capability ports.FeatureCapability) string {
	if capability.Status == "" {
		return string(ports.FeatureStatusStub)
	}

	return string(capability.Status)
}

func capabilityStatusSupported(status string) bool {
	capability := ports.FeatureCapability{Status: ports.FeatureStatus(status)}

	return capability.Supported()
}
