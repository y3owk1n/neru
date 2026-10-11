// Package ipcschema renders the JSON Schema for the IPC wire format from the
// Go types the daemon encodes.
//
// docs/reference/ipc.md states the contract in words, and the schema states
// its shapes, so a client can generate types from them or check a reply in a
// test. The walk follows the JSON encoder's rules, which differ from the TOML
// decoder's that internal/configschema follows. A field without omitempty is
// always present, and a pointer without it may be null.
//
// A reply allows keys the schema does not list, because a protocol can gain
// them and the contract tells clients to ignore them. A request refuses them,
// as the daemon does.
//
// [Render] produces the file, `just genschema` writes it to
// schemas/ipc.schema.json, and the guardrail test in internal/architecture
// fails while the committed file is out of date.
package ipcschema
