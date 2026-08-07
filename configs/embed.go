package configs

import _ "embed"

// DefaultConfig contains the default configuration file contents.
//
//go:embed default-config.toml
var DefaultConfig []byte

// ShippedExamples are the configs the project publishes under configs/.
//
// They are listed rather than globbed because the directory is also a working
// area: a local config kept there for testing is not a project artifact, and
// its problems are not this suite's business. Adding an example here is the
// deliberate step that puts it under test.
//
// Only default-config.toml is embedded, through DefaultConfig above; the rest
// are read from disk by the tests that check them.
var ShippedExamples = []string{
	"default-config.toml",
	"grid-only-config.toml",
	"hints-only-config.toml",
	"recursive-grid-only-config.toml",
}
