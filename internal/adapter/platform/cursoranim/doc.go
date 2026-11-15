// Package cursoranim glides the cursor to a target in steps, for the Linux and
// Windows system adapters.
//
// Both adapters animate a smooth cursor move the same way, and only how a
// single step reaches the screen differs, so each passes its own move
// function. macOS keeps an animator of its own, because it posts drag event
// types and counts generations, which neither of the others needs.
package cursoranim
