// Package configschema renders the JSON Schema for config.toml from the
// config structs.
//
// Editors read the schema to complete and check a config file. Taplo and Even
// Better TOML find it through a `#:schema` comment, which `neru config init`
// writes as the first line of a new file. The schema comes from the `toml`
// tags of [config.Config], so the structs stay the one declaration of what a
// config file may contain. It is a new projection of them. The defaults, the
// example and the reference rows stay hand-written, as docs/adr/0006 decided.
//
// [Render] produces the file, `just genschema` writes it to
// schemas/config.schema.json, and the guardrail test in internal/architecture
// fails while the committed file is out of date.
package configschema
