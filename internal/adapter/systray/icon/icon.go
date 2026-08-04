package icon

import _ "embed"

// Brand is the colored Neru tray tile as PNG bytes. It is the icon for hosts
// that render tray images literally (Windows notification area, Linux SNI):
// the monochrome template glyph below is white-on-transparent and would be
// invisible there.
//
//go:embed tray-icon.png
var Brand []byte

// Template is the monochrome menu-bar glyph as PNG bytes: 44×44 px
// (22 pt @2x), white on transparent. macOS renders it as a template image and
// adapts it to the current menu bar appearance (light/dark).
//
//go:embed tray-icon-template.png
var Template []byte

// TemplateDisabled is the Template variant shown while Neru is paused. Same
// size and format requirements as Template.
//
//go:embed tray-icon-template-disabled.png
var TemplateDisabled []byte
