// Package config defines Neru's TOML configuration: the schema structs, the
// shared and per-platform defaults, the validators, and the derivations that
// turn what a user wrote into what the daemon runs on.
//
// The loader subpackage finds, decodes, watches and persists the file.
package config
