package config

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/derrors"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// ValidateGrid validates the grid configuration, warning about the key sets
// that load and leave part of the grid unreachable; see [warnGridKeySets].
//
// written is what the user wrote, which is what tells a label they typed from
// one the derivation settled; the zero value means there is none, and the
// warnings fall back to a comparison. See [WrittenConfig].
func (c *Config) ValidateGrid(warnings *Warnings, written WrittenConfig) error {
	if !c.Grid.Enabled {
		return nil
	}

	if strings.TrimSpace(c.Grid.Characters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "grid.characters cannot be empty")
	}

	for _, r := range c.Grid.Characters {
		if r > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.characters can only contain ASCII characters",
			)
		}
	}

	if c.Grid.SublayerKeys != "" {
		for _, r := range c.Grid.SublayerKeys {
			if r > unicode.MaxASCII {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"grid.sublayer_keys can only contain ASCII characters",
				)
			}
		}
	}

	if c.Grid.MaxLabelLength < domainGrid.MinLabelLength ||
		c.Grid.MaxLabelLength > domainGrid.DefaultMaxLabelLength {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"grid.max_label_length must be between %d and %d",
			domainGrid.MinLabelLength,
			domainGrid.DefaultMaxLabelLength,
		)
	}

	err := validateColors([]colorField{
		{c.Grid.UI.BackgroundColor, "grid.ui.background_color"},
		{c.Grid.UI.TextColor, "grid.ui.text_color"},
		{c.Grid.UI.MatchedTextColor, "grid.ui.matched_text_color"},
		{c.Grid.UI.MatchedBackgroundColor, "grid.ui.matched_background_color"},
		{c.Grid.UI.MatchedBorderColor, "grid.ui.matched_border_color"},
		{c.Grid.UI.BorderColor, "grid.ui.border_color"},
	})
	if err != nil {
		return err
	}

	if c.Grid.UI.FontSize < 1 || c.Grid.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"grid.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	if c.Grid.UI.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.ui.border_width must be non-negative")
	}

	err = validateAppConfigsWithCallback(
		"grid",
		c.Grid.AppConfigs,
		rejectModeSpecificFields("grid"),
	)
	if err != nil {
		return err
	}

	c.warnGridKeySets(warnings, written)

	return nil
}

// gridKeySet is one set of characters a grid is labeled from, and what a set
// too short to label with costs that particular field — which differs, so the
// warning cannot word it once: a short grid.characters is replaced wholesale by
// a-z, while short labels are used as they are and cap the grid to what they
// can name.
type gridKeySet struct {
	field    string
	keys     string
	tooShort string
}

// derivedGridKeySet is a key set the derivation settles when the user leaves it
// empty: the set as the grid is now labeled from it, plus how to read the same
// field out of the configuration the user wrote, which is what says whether the
// set is theirs to fix. grid.characters is not one of these — it is the field
// the others are inferred from, and is always the user's own.
type derivedGridKeySet struct {
	gridKeySet

	// readWritten reads this field out of a *written* configuration, and is a
	// reader rather than a value because the configuration to read it from is
	// the caller's, and may not exist at all.
	readWritten func(*Config) string
}

// warnGridKeySets reports the character sets a grid is labeled from that will
// not label the grid the user asked for: too short to name every row or column,
// a character used twice, or one that cannot be typed.
//
// It warns rather than refuses because none of the three stops a grid being
// built — the grid is capped, relabelled, labeled from a shorter set than was
// written, or has one cell that answers to a key nobody can press — and refusing
// would replace the user's whole configuration with the defaults over a label set
// they can fix in one line (ADR 0002). The refusals
// above are unchanged: an empty or non-ASCII grid.characters still costs the
// load, and this pass is what the fields it does not cover now get instead of
// nothing.
//
// It is the tier the neighboring hints.hint_characters does not use — that one
// refuses the same three shapes. The difference is that these labels are
// derived: leave row_labels empty and the value validated is the one the
// derivation wrote, and refusing a configuration over a line the user never
// wrote is the shape ADR 0002 exists to avoid.
//
// grid.sublayer_keys is deliberately not here. The runtime trims, uppercases and
// caps it at the number of subgrid cells (domain/grid.SubgridKeys), so which of
// its characters are used at all depends on a subgrid size the mode layer owns
// and this package cannot see — a warning from here would report faults in
// characters that are never drawn.
//
// It reads the resolved labels rather than the written ones, because those are
// what the grid is now drawn with (ResolveGridLabels): the written value is
// legitimately empty and means "infer from characters".
//
// Which field a fault belongs to is the other half, and it is what written
// answers: a label the user typed is reported under its own name, and one the
// derivation settled is left to grid.characters, the one name that is in the
// user's file. Reported against every field it reached, one mistake would read
// as three, two of them naming a line the user cannot go and fix.
func (c *Config) warnGridKeySets(warnings *Warnings, written WrittenConfig) {
	warnGridKeySet(warnings, gridKeySet{
		field:    "grid.characters",
		keys:     c.Grid.Characters,
		tooShort: "so the grid is labeled a-z instead",
	})

	// The fallback for a caller with no written configuration to consult, and
	// only for that: a resolved label equal to what an empty one settles to was
	// probably inferred. Probably, because a label somebody typed can equal the
	// inference exactly — which is the case the written configuration is here
	// to settle (#1281).
	inferred := c.inferredGridKeys()

	for _, labels := range []derivedGridKeySet{
		{
			gridKeySet:  gridKeySet{field: "grid.row_labels", keys: c.Grid.RowLabels},
			readWritten: func(cfg *Config) string { return cfg.Grid.RowLabels },
		},
		{
			gridKeySet:  gridKeySet{field: "grid.col_labels", keys: c.Grid.ColLabels},
			readWritten: func(cfg *Config) string { return cfg.Grid.ColLabels },
		},
	} {
		if !written.wroteDerived(labels.readWritten, labels.keys, inferred) {
			continue
		}

		labels.tooShort = "so the grid is capped to the cells they can name"
		warnGridKeySet(warnings, labels.gridKeySet)
	}
}

// warnGridKeySet reports what one character set will not do. An empty set is
// silent: it is the written value of a label the derivation has not settled
// yet, and it means "infer", not "label with nothing".
func warnGridKeySet(warnings *Warnings, set gridKeySet) {
	if set.keys == "" {
		return
	}

	chars := []rune(set.keys)

	// Counted after the repeats come out, because that is the count the grid
	// applies its floor to (domain/grid.newGridAlphabet): "aa" is two characters
	// written and one character to label with, and reading the written length
	// here would let it reach the a-z fallback without a word said.
	usable := len(domainGrid.DistinctKeys(set.keys))

	if usable < MinCharactersLength {
		warnings.Addf(
			"%s has %d usable character and needs at least %d, %s",
			set.field, usable, MinCharactersLength, set.tooShort,
		)
	}

	warnDuplicateGridKeys(warnings, set.field, chars)
	warnUntypeableGridKeys(warnings, set.field, chars)
}

// warnDuplicateGridKeys reports each character a key set repeats, once, on the
// second occurrence, so that a character written three times is one thing wrong
// rather than two.
//
// Case is folded because matching is: "aA" is one label written twice. The grid
// drops the second one (domain/grid.DistinctKeys) rather than labeling a cell
// nothing can reach, so what the repeat costs is the size of the set the grid is
// labeled from — which is why this is worth saying even though nothing is broken
// by it. The character is reported folded too: the grid draws its labels
// upper-cased, so that is the form the user is looking for on screen.
func warnDuplicateGridKeys(warnings *Warnings, field string, chars []rune) {
	seen := make(map[rune]struct{}, len(chars))
	reported := make(map[rune]struct{})

	for _, char := range chars {
		upper := unicode.ToUpper(char)

		_, duplicate := seen[upper]
		if !duplicate {
			seen[upper] = struct{}{}

			continue
		}

		// A character written three times is one thing wrong, not two.
		_, done := reported[upper]
		if done {
			continue
		}

		reported[upper] = struct{}{}

		warnings.Addf(
			"%s uses %q more than once; the repeat is dropped, "+
				"so the grid is labeled from fewer characters than the option lists",
			field, upper,
		)
	}
}

// warnUntypeableGridKeys reports the characters in a key set that a user cannot
// press: whitespace, control characters, and anything outside ASCII, which the
// overlay may have no glyph for and a keyboard may have no key for.
func warnUntypeableGridKeys(warnings *Warnings, field string, chars []rune) {
	reported := make(map[rune]struct{})

	for _, char := range chars {
		if char <= unicode.MaxASCII && unicode.IsPrint(char) && !unicode.IsSpace(char) {
			continue
		}

		if _, done := reported[char]; done {
			continue
		}

		reported[char] = struct{}{}

		warnings.Addf(
			"%s contains %q, which cannot be typed as a grid label, "+
				"so the cells it names cannot be reached",
			field, char,
		)
	}
}

// ValidateMonitorSelect validates the monitor_select configuration.
func (c *Config) ValidateMonitorSelect() error {
	if !c.MonitorSelect.Enabled {
		return nil
	}

	if c.MonitorSelect.Characters == "" {
		return derrors.New(derrors.CodeInvalidConfig, "monitor_select.characters cannot be empty")
	}

	if utf8.RuneCountInString(c.MonitorSelect.Characters) < MinCharactersLength {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"monitor_select.characters must contain at least 2 characters",
		)
	}

	err := validateColors([]colorField{
		{c.MonitorSelect.UI.BackgroundColor, "monitor_select.ui.background_color"},
		{c.MonitorSelect.UI.TextColor, "monitor_select.ui.text_color"},
		{c.MonitorSelect.UI.MatchedTextColor, "monitor_select.ui.matched_text_color"},
		{c.MonitorSelect.UI.BorderColor, "monitor_select.ui.border_color"},
		{c.MonitorSelect.UI.BackdropColor, "monitor_select.ui.backdrop_color"},
		{c.MonitorSelect.UI.SubtitleTextColor, "monitor_select.ui.subtitle_text_color"},
	})
	if err != nil {
		return err
	}

	if c.MonitorSelect.UI.FontSize < 1 || c.MonitorSelect.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"monitor_select.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	if c.MonitorSelect.UI.SubtitleFontSize < 1 ||
		c.MonitorSelect.UI.SubtitleFontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"monitor_select.ui.subtitle_font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	return nil
}

// ValidateRecursiveGrid validates recursive grid configuration.
func (c *Config) ValidateRecursiveGrid() error {
	if !c.RecursiveGrid.Enabled {
		return nil
	}

	if c.RecursiveGrid.GridCols < DefaultRecursiveGridMinGridCols {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.grid_cols must be >= %d",
			DefaultRecursiveGridMinGridCols,
		)
	}

	if c.RecursiveGrid.GridRows < DefaultRecursiveGridMinGridRows {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.grid_rows must be >= %d",
			DefaultRecursiveGridMinGridRows,
		)
	}

	if c.RecursiveGrid.GridCols*c.RecursiveGrid.GridRows < DefaultRecursiveGridMinTotalCells {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid grid must have at least 2 cells (grid_cols * grid_rows >= 2); a 1x1 grid cannot subdivide",
		)
	}

	if c.RecursiveGrid.MaxDepth < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "recursive_grid.max_depth must be >= 1")
	}

	if c.RecursiveGrid.Animation.DurationMS < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.animation.duration_ms must be non-negative",
		)
	}

	expectedKeys := c.RecursiveGrid.GridCols * c.RecursiveGrid.GridRows
	if utf8.RuneCountInString(c.RecursiveGrid.Keys) != expectedKeys {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.keys must have %d characters",
			expectedKeys,
		)
	}

	for _, layer := range c.RecursiveGrid.Layers {
		if layer.Depth < 0 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers.depth must be >= 0",
			)
		}

		if layer.GridCols < DefaultRecursiveGridMinGridCols ||
			layer.GridRows < DefaultRecursiveGridMinGridRows {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers grid dimensions must be >= 1",
			)
		}

		if layer.GridCols*layer.GridRows < DefaultRecursiveGridMinTotalCells {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers depth %d must have at least 2 cells (grid_cols * grid_rows >= 2); a 1x1 grid cannot subdivide",
				layer.Depth,
			)
		}

		if utf8.RuneCountInString(layer.Keys) != layer.GridCols*layer.GridRows {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers depth %d keys length mismatch",
				layer.Depth,
			)
		}
	}

	err := validateColors([]colorField{
		{c.RecursiveGrid.UI.LineColor, "recursive_grid.ui.line_color"},
		{c.RecursiveGrid.UI.HighlightColor, "recursive_grid.ui.highlight_color"},
		{c.RecursiveGrid.UI.TextColor, "recursive_grid.ui.text_color"},
		{c.RecursiveGrid.UI.LabelBackgroundColor, "recursive_grid.ui.label_background_color"},
		{c.RecursiveGrid.UI.SubKeyPreviewTextColor, "recursive_grid.ui.sub_key_preview_text_color"},
	})
	if err != nil {
		return err
	}

	if c.RecursiveGrid.UI.LineWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.line_width must be non-negative",
		)
	}

	if c.RecursiveGrid.UI.FontSize < 1 || c.RecursiveGrid.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	if c.RecursiveGrid.UI.SubKeyPreviewFontSize < 1 ||
		c.RecursiveGrid.UI.SubKeyPreviewFontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.sub_key_preview_font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	if utf8.RuneCountInString(c.RecursiveGrid.UI.LabelChar) > 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.label_char must be empty or a single character",
		)
	}

	if utf8.RuneCountInString(c.RecursiveGrid.UI.SubKeyPreviewLabelChar) > 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.sub_key_preview_label_char must be empty or a single character",
		)
	}

	err = validateAppConfigsWithCallback(
		"recursive_grid",
		c.RecursiveGrid.AppConfigs,
		rejectModeSpecificFields("recursive_grid"),
	)
	if err != nil {
		return err
	}

	return nil
}
