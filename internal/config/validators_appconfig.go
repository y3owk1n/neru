package config

import (
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
)

// AppConfigFieldValidator is a callback for validating mode-specific fields in AppConfig.
// It's called for each app config after common validation passes.
type AppConfigFieldValidator func(idx int, appConfig *AppConfig) error

// validateAppConfigsWithCallback validates per-app configuration with optional field-level validation.
func validateAppConfigsWithCallback(
	modeName string,
	appConfigs []AppConfig,
	fieldValidator AppConfigFieldValidator,
) error {
	seen := make(map[string]struct{}, len(appConfigs))
	for idx, appConfig := range appConfigs {
		if strings.TrimSpace(appConfig.BundleID) == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.app_configs[%d].bundle_id cannot be empty",
				modeName, idx,
			)
		}

		lowerID := strings.ToLower(strings.TrimSpace(appConfig.BundleID))
		if _, ok := seen[lowerID]; ok {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"duplicate %s.app_configs bundle_id: %s",
				modeName, appConfig.BundleID,
			)
		}

		seen[lowerID] = struct{}{}

		err := validateHotkeyTable(
			fmt.Sprintf("%s.app_configs[%d].hotkeys", modeName, idx),
			appConfig.Hotkeys,
		)
		if err != nil {
			return err
		}

		// Call mode-specific field validator if provided
		if fieldValidator != nil {
			err = fieldValidator(idx, &appConfig)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// rejectScrollFields creates a field validator that rejects scroll-specific fields.
// Used for non-scroll modes (hints, grid, recursive_grid) to catch accidental configuration.
func rejectScrollFields(modeName string) AppConfigFieldValidator {
	return func(idx int, appConfig *AppConfig) error {
		if appConfig.ScrollStep != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.app_configs[%d].scroll_step is only valid for scroll mode",
				modeName, idx,
			)
		}

		if appConfig.ScrollStepHalf != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.app_configs[%d].scroll_step_half is only valid for scroll mode",
				modeName, idx,
			)
		}

		if appConfig.ScrollStepFull != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.app_configs[%d].scroll_step_full is only valid for scroll mode",
				modeName, idx,
			)
		}

		return nil
	}
}

// rejectHintsFields creates a field validator that rejects hints-specific fields.
// Used for non-hints modes (grid, recursive_grid) to catch accidental configuration.
func rejectHintsFields(modeName string) AppConfigFieldValidator {
	return func(idx int, appConfig *AppConfig) error {
		if len(appConfig.AdditionalClickable) > 0 {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.app_configs[%d].additional_clickable_roles is only valid for hints mode",
				modeName, idx,
			)
		}

		if appConfig.IgnoreClickableCheck != nil && *appConfig.IgnoreClickableCheck {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.app_configs[%d].ignore_clickable_check is only valid for hints mode",
				modeName, idx,
			)
		}

		if appConfig.VisibleCheckEnabled != nil && *appConfig.VisibleCheckEnabled {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.app_configs[%d].visible_check_enabled is only valid for hints mode",
				modeName, idx,
			)
		}

		return nil
	}
}

// rejectModeSpecificFields creates a combined validator that rejects both scroll and hints fields.
// Used for grid and recursive_grid modes.
func rejectModeSpecificFields(modeName string) AppConfigFieldValidator {
	return func(idx int, appConfig *AppConfig) error {
		err := rejectScrollFields(modeName)(idx, appConfig)
		if err != nil {
			return err
		}

		return rejectHintsFields(modeName)(idx, appConfig)
	}
}

// validateScrollAppConfigs validates per-app scroll configuration.
func validateScrollAppConfigs(modeName string, appConfigs []AppConfig) error {
	scrollFieldValidator := func(idx int, appConfig *AppConfig) error {
		// First, reject hints fields
		err := rejectHintsFields(modeName)(idx, appConfig)
		if err != nil {
			return err
		}

		// Then validate scroll fields
		if appConfig.ScrollStep != nil {
			err := validateMinValue(
				*appConfig.ScrollStep,
				1,
				fmt.Sprintf("%s.app_configs[%d].scroll_step", modeName, idx),
			)
			if err != nil {
				return err
			}
		}

		if appConfig.ScrollStepHalf != nil {
			err := validateMinValue(
				*appConfig.ScrollStepHalf,
				1,
				fmt.Sprintf("%s.app_configs[%d].scroll_step_half", modeName, idx),
			)
			if err != nil {
				return err
			}
		}

		if appConfig.ScrollStepFull != nil {
			err := validateMinValue(
				*appConfig.ScrollStepFull,
				1,
				fmt.Sprintf("%s.app_configs[%d].scroll_step_full", modeName, idx),
			)
			if err != nil {
				return err
			}
		}

		return nil
	}

	return validateAppConfigsWithCallback(modeName, appConfigs, scrollFieldValidator)
}

// validateHotkeysAppConfigs validates per-app global hotkey configuration.
func validateHotkeysAppConfigs(modeName string, appConfigs []AppConfig) error {
	return validateAppConfigsWithCallback(modeName, appConfigs, nil)
}

// ValidateAppConfigs validates per-app hint configuration.
func (c *Config) ValidateAppConfigs() error {
	return validateAppConfigsWithCallback(
		"hints",
		c.Hints.AppConfigs,
		func(idx int, appConfig *AppConfig) error {
			err := rejectScrollFields("hints")(idx, appConfig)
			if err != nil {
				return err
			}

			switch appConfig.Strategy {
			case domain.StrategyAXTree, domain.StrategyVision, domain.StrategyWLKBPTR, "":
			default:
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"hints.app_configs[%d].strategy must be %q, %q, or %q",
					idx, domain.StrategyAXTree, domain.StrategyVision, domain.StrategyWLKBPTR,
				)
			}

			switch appConfig.LabelDirection {
			case domain.LabelDirectionReverse, domain.LabelDirectionNormal, "":
			default:
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"hints.app_configs[%d].label_direction must be %q or %q",
					idx, domain.LabelDirectionReverse, domain.LabelDirectionNormal,
				)
			}

			return nil
		},
	)
}
