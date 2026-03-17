package configs

import _ "embed"

// DefaultConfig contains the default configuration file contents.
//
//go:embed default-config.toml
var DefaultConfig []byte
