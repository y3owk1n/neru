package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// SelectWith is hints' select_with entries in force. Each is the steps a
// selection runs when the key completing a label is typed with a given set of
// modifiers, keyed by that set in comparison form ("shift", "cmd+shift").
//
// The zero SelectWith holds no entries, so every selection runs the session's
// --action.
type SelectWith struct {
	byModifiers map[string]Binding
}

// Lookup returns the entry for a modifier set in comparison form, the part of
// a normalized chord before its last "+".
func (s SelectWith) Lookup(modifiers string) (Binding, bool) {
	binding, ok := s.byModifiers[modifiers]

	return binding, ok
}

// Len reports how many modifier sets have an entry.
func (s SelectWith) Len() int {
	return len(s.byModifiers)
}

// ModifierSets returns every modifier set with an entry, in comparison form
// and sorted.
func (s SelectWith) ModifierSets() []string {
	return slices.Sorted(maps.Keys(s.byModifiers))
}

// ResolveSelectWith resolves the select_with entries in force with an
// application focused. That is the [hints.select_with] table, with that app's
// [[hints.app_configs]] select_with entries replacing the ones for the same
// modifier set. An entry of "__disabled__" removes the global one for that app.
func (c *Config) ResolveSelectWith(focusedApp string) SelectWith {
	primary := c.General.PrimaryModifier
	byModifiers := make(map[string]Binding, len(c.Hints.SelectWith))

	for _, key := range slices.Sorted(maps.Keys(c.Hints.SelectWith)) {
		modifiers, ok := primary.selectWithModifiers(key)
		if !ok {
			continue
		}

		byModifiers[modifiers] = Binding{Key: key, Steps: c.Hints.SelectWith[key]}
	}

	if focusedApp != "" {
		if appConfig := c.Hints.AppConfigForBundleID(focusedApp); appConfig != nil {
			for _, key := range slices.Sorted(maps.Keys(appConfig.SelectWith)) {
				modifiers, ok := primary.selectWithModifiers(key)
				if !ok {
					continue
				}

				steps := appConfig.SelectWith[key]
				if len(steps) == 1 && steps[0] == DisabledSentinel {
					delete(byModifiers, modifiers)

					continue
				}

				byModifiers[modifiers] = Binding{Key: key, Steps: steps}
			}
		}
	}

	if len(byModifiers) == 0 {
		return SelectWith{}
	}

	return SelectWith{byModifiers: byModifiers}
}

// HasAppSelectWithOverrides reports whether any [[hints.app_configs]] entry
// writes select_with, which makes the focused app part of what a selection
// runs.
func (c *HintsConfig) HasAppSelectWithOverrides() bool {
	for idx := range c.AppConfigs {
		if len(c.AppConfigs[idx].SelectWith) > 0 {
			return true
		}
	}

	return false
}

// selectWithModifiers reads a select_with key into its comparison form, the
// modifiers it names resolved, deduplicated and sorted as a normalized chord
// spells them. It reports false for a key that names anything but modifiers.
func (p PrimaryModifierConfig) selectWithModifiers(key string) (string, bool) {
	// Normalizing a chord built on the key resolves aliases and Primary and
	// sorts the modifiers, in exactly the form a keystroke is matched in.
	normalized := p.NormalizeKey(key + "+a")
	parts := strings.Split(normalized, "+")
	modifiers := parts[:len(parts)-1]
	seen := make(map[string]struct{}, len(modifiers))

	for _, part := range modifiers {
		switch part {
		case keyvocab.ModifierCmd,
			keyvocab.ModifierCtrl,
			keyvocab.ModifierAlt,
			keyvocab.ModifierShift:
		default:
			return "", false
		}

		if _, repeated := seen[part]; repeated {
			return "", false
		}

		seen[part] = struct{}{}
	}

	return strings.Join(modifiers, "+"), true
}

// ValidateSelectWith refuses a select_with entry that could never run: a key
// that is not a set of modifiers, two keys naming the same set, an empty or
// malformed step, or a modifier set whose chord with a hint character is
// already bound, since that binding answers the key before any label sees it.
func (c *Config) ValidateSelectWith() error {
	err := c.validateSelectWithTable("hints.select_with", c.Hints.SelectWith, false)
	if err != nil {
		return err
	}

	for idx := range c.Hints.AppConfigs {
		err = c.validateSelectWithTable(
			fmt.Sprintf("hints.app_configs[%d].select_with", idx),
			c.Hints.AppConfigs[idx].SelectWith,
			true,
		)
		if err != nil {
			return err
		}
	}

	return c.refuseShadowedSelectWith()
}

// validateSelectWithTable checks one select_with table for shape. Only a
// per-app table may write the disabled sentinel, which removes a global entry
// and means nothing in the global table itself.
func (c *Config) validateSelectWithTable(
	field string,
	table map[string]StringOrStringArray,
	allowDisabled bool,
) error {
	seen := make(map[string]string, len(table))

	for _, key := range slices.Sorted(maps.Keys(table)) {
		entry := fmt.Sprintf("%s.%s", field, key)

		modifiers, ok := c.General.PrimaryModifier.selectWithModifiers(key)
		if !ok {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s: key must be one or more of cmd, ctrl, alt, shift and primary, joined with +, each named once",
				entry,
			)
		}

		if other, taken := seen[modifiers]; taken {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s: names the same modifiers as %s.%s",
				entry, field, other,
			)
		}

		seen[modifiers] = key

		steps := table[key]
		if len(steps) == 0 {
			return derrors.Newf(derrors.CodeInvalidConfig, "%s cannot be empty", entry)
		}

		if len(steps) == 1 && steps[0] == DisabledSentinel {
			if allowDisabled {
				continue
			}

			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s: %q only removes a global entry inside [[hints.app_configs]]",
				entry, DisabledSentinel,
			)
		}

		for idx, step := range steps {
			trimmed := strings.TrimSpace(step)
			if trimmed == "" {
				return derrors.Newf(derrors.CodeInvalidConfig, "%s[%d] cannot be empty", entry, idx)
			}

			err := validateHotkeyActionString(trimmed)
			if err != nil {
				return derrors.Newf(derrors.CodeInvalidConfig, "%s[%d]: %v", entry, idx, err)
			}
		}
	}

	return nil
}

// refuseShadowedSelectWith refuses a modifier set whose chord with a hint
// character is bound, in the hints table, the global one, or either of them
// as an app's overrides leave it. The binding would answer that chord first,
// so the entry could never select a label ending in that character.
//
// Each focus that can change the answer is checked on its own: no app, every
// [[hints.app_configs]] entry and every [[app_configs]] entry.
func (c *Config) refuseShadowedSelectWith() error {
	if len(c.Hints.SelectWith) == 0 && !c.Hints.HasAppSelectWithOverrides() {
		return nil
	}

	apps := []string{""}
	for idx := range c.Hints.AppConfigs {
		apps = append(apps, c.Hints.AppConfigs[idx].BundleID)
	}

	for idx := range c.AppConfigs {
		apps = append(apps, c.AppConfigs[idx].BundleID)
	}

	for _, app := range apps {
		err := c.refuseShadowedSelectWithFor(app)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) refuseShadowedSelectWithFor(app string) error {
	selectWith := c.ResolveSelectWith(app)
	if selectWith.Len() == 0 {
		return nil
	}

	tables := []struct {
		keymap Keymap
		field  string
	}{
		{keymap: c.ResolveKeymap(ModeNameHints, app), field: "hints.hotkeys"},
		{keymap: c.ResolveGlobalKeymap(app), field: "hotkeys"},
	}

	hintCharacters := strings.ToLower(c.Hints.HintCharacters)

	for _, modifiers := range selectWith.ModifierSets() {
		entry, _ := selectWith.Lookup(modifiers)

		for _, char := range hintCharacters {
			chord := c.General.PrimaryModifier.NormalizeKey(modifiers + "+" + string(char))

			for _, table := range tables {
				binding, bound := table.keymap.Lookup(chord)
				if !bound {
					continue
				}

				where := table.field
				if app != "" {
					where = fmt.Sprintf("%s with %s focused", where, app)
				}

				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"select_with %q: %q is bound in %s, so a label ending in %q could never be selected with %s. Rebind %q or remove %q from hints.hint_characters",
					entry.Key,
					binding.Key,
					where,
					string(char),
					entry.Key,
					binding.Key,
					string(char),
				)
			}
		}
	}

	return nil
}
