// Package loader owns the runtime half of configuration: locating and
// decoding the TOML file, merging user hotkeys over the platform defaults,
// validating the result, hot-reload watching, persistence of overrides, and
// the reflection-based field access behind `neru config set`. The schema,
// defaults and validators live in the parent config package, which this
// package builds on; nothing in config imports loader.
package loader
