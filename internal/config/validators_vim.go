package config

import (
	"maps"
	"slices"

	"github.com/y3owk1n/fude/vim"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/screentext"
)

// validateVimKeymaps refuses a mapping vim mode cannot make: keys no tap
// names, Escape, nothing to run, or a <Plug> command it does not have. It
// also refuses two spellings of one key sequence, such as "<Space>j" and
// "<space>j", since which of them would win is left to chance.
func validateVimKeymaps(keymaps VimKeymaps) error {
	for _, table := range []struct {
		field    string
		mappings map[string]string
	}{
		{"vim.keymaps.normal", keymaps.Normal},
		{"vim.keymaps.visual", keymaps.Visual},
	} {
		spelled := map[string]string{}

		for _, from := range slices.Sorted(maps.Keys(table.mappings)) {
			err := screentext.CheckMapping(from, table.mappings[from])
			if err != nil {
				return derrors.Newf(derrors.CodeInvalidConfig, "%s %q: %v", table.field, from, err)
			}

			notation := vim.Notation(from)
			if other, ok := spelled[notation]; ok {
				return derrors.Newf(derrors.CodeInvalidConfig,
					"%s %q: maps the same keys as %q", table.field, from, other)
			}

			spelled[notation] = from
		}
	}

	return nil
}
