// Package event names Neru's lifecycle events and delivers them to
// subscribers on a bus.
//
// The names here are the only spelling of an event. A consumer that shows
// one to a user, such as a hook key or a field on the wire, derives it from
// Name rather than writing its own (ADR 0008).
//
// An event carries mode, action, slot and display names, bundle IDs,
// coordinates, enums and booleans only: never UI text, an application's
// display name, or anything a key produced. Sticky modifiers are a state the
// user set, not keys they typed, and are carried as such. The logging
// rule draws the same line, and it applies here because events are logged and
// leave the process.
package event
