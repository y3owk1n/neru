// Package modeflag is the flag vocabulary the mode commands are written in.
//
// A mode command crosses a process boundary. The CLI parses the user's flags,
// re-serializes them onto a socket, and the daemon parses them again on the far
// side; a hotkey binding skips the CLI and arrives as the same text. Every one
// of those readers has to agree on each flag's name, its short form, and
// whether it carries a value.
//
// Spelling that out separately in each place is what this package replaces. A
// flag renamed on one side and not the other produced no error anywhere: the
// CLI would send something the daemon did not recognize, and the daemon skips
// what it does not recognize, so the flag simply stopped working.
package modeflag
