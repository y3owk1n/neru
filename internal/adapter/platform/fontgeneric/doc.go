// Package fontgeneric reads a written font family name and says which family
// to ask the platform's font system for: the platform's sans, serif or
// monospaced face when the name is a generic alias, and the name itself when
// it is a family somebody named.
//
// Every platform font resolver needs that reading and none of them needs a
// different one — a person who writes "sans_serif" in their configuration
// means the same thing on macOS, Linux and Windows. Each backend supplies
// only the part that is genuinely its own: which concrete family its sans,
// serif and mono are. ADR 0007 has the reasoning.
package fontgeneric
