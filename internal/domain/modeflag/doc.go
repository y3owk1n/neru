// Package modeflag is the flag vocabulary the mode commands are written in.
//
// A mode command crosses a process boundary: the CLI parses the user's flags,
// writes them to a socket, and the daemon parses them again. A hotkey binding
// skips the CLI and arrives as the same text. Every reader has to agree on each
// flag's name, short form, and whether it takes a value.
//
// Spelling that out in each place is what this replaces. A flag renamed on one
// side only produced no error anywhere, because the daemon skips what it does
// not recognize. The flag just stopped working.
package modeflag
