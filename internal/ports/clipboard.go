package ports

import "context"

// ClipboardPort writes to the system clipboard. It backs the copy_text action,
// which copies the text of the hint the user selected.
//
// Implementations must report derrors.CodeNotSupported when the platform has
// no way to set the clipboard, such as a Linux session with none of wl-copy,
// xclip or xsel installed. A write that cannot finish within the context's
// deadline fails rather than blocks.
type ClipboardPort interface {
	// WriteText replaces the clipboard's contents with text.
	WriteText(ctx context.Context, text string) error
}
