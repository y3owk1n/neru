// Package mocks provides mock implementations of port interfaces for testing.
//
// These mocks use function fields to allow tests to customize behavior: set
// only the funcs a test cares about, and each mock supplies a sensible,
// stateful default when a field is nil. Every mock carries a compile-time
// interface-satisfaction assertion (var _ ports.X = (*MockX)(nil)), so a mock
// cannot silently drift from its port, and the architecture tests require a
// mock for every port with no exemptions.
//
// The mocks are hand-written by decision, not by omission. Generators
// (mockgen, moq, counterfeiter) were considered and rejected: the func-field
// pattern keeps call sites free of expectation DSLs, the curated defaults
// (in-memory state, realistic zero values) are behavior a generator cannot
// author, and no extra toolchain step stands between a contributor and a
// green build. The cost — adding a port method means adding a func field and
// a default by hand — is bounded by the guardrails: the compile-time
// assertion and the architecture tests fail loudly until parity is restored.
// Revisit only if the port surface grows far beyond its current size.
package mocks
