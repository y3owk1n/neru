// Package modifierstate plans the key presses and releases that make a
// real-key backend present exactly the modifiers an action asked for.
//
// macOS stamps a modifier set onto the event itself, so what the user happens
// to be holding cannot reach it. A backend that has to press real keys has no
// such field: whatever the keyboard reports held is part of every event it
// injects, so a plain scroll fired from a Ctrl+J binding arrives as
// ctrl+scroll. Presenting the requested set means releasing what is held and
// was not asked for, pressing what was asked for and is not held, and undoing
// both afterwards.
//
// The decision is separated from the backend because the backend is the part no
// test can drive: reading the live keyboard and injecting a key are two native
// calls, while which keys to touch is arithmetic on what they answer.
package modifierstate
