// Package derrors provides domain-specific error types and utilities.
//
// An Error carries a Code that callers branch on, a message, an optional
// cause, and optional context fields. errors.Is matches two Errors by code.
package derrors
