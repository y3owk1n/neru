// Package architecture holds the guardrail tests that keep the codebase's
// shape honest. Prose rules alone don't hold; each rule here fails `just test`
// when broken. Layering, the darwin One Rule, platform file slots, port mocks,
// cgo includes, doc links, comment paths, and test quality each have a file
// named for them, as do the two halves of the config option chain: the pairing
// between the schema and the example TOML, and the links no projection can
// show — every validator reaching the ladder, every field reaching a default.
// Exemption lists are self-checking: a companion test fails when an entry
// stops being real, so a list can only shrink.
package architecture
